package fbhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
)

// maxMoveItems bounds how many entries one request may move.
const maxMoveItems = 1000

type moveRequest struct {
	Items       []string `json:"items"`
	Destination string   `json:"destination"`
}

// isUnderscoreName reports whether a folder name marks a staging folder.
func isUnderscoreName(name string) bool {
	return strings.HasPrefix(name, "_")
}

// hasUnderscoreSegment reports whether any folder on the path is a staging folder.
func hasUnderscoreSegment(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if isUnderscoreName(segment) {
			return true
		}
	}

	return false
}

// pathExists reports whether anything is at p, a dangling link included.
func pathExists(fs afero.Fs, p string) bool {
	if lstater, ok := fs.(afero.Lstater); ok {
		_, _, err := lstater.LstatIfPossible(p)
		return err == nil
	}

	_, err := fs.Stat(p)
	return err == nil
}

// movedThumbnail is a stored thumbnail or length, remembered so it can follow
// its file to the new place.
type movedThumbnail struct {
	rel       string
	thumbnail []byte
	duration  []byte
}

// collectThumbnails remembers what the cache holds for src and, if it is a
// folder, everything below it. Keys follow the path, so without this a moved
// file would lose a thumbnail that was chosen for it.
func collectThumbnails(ctx context.Context, d *data, fileCache FileCache, src string) []movedThumbnail {
	if _, ok := fileCache.(thumbnailCache); !ok {
		return nil
	}

	var found []movedThumbnail
	_ = afero.Walk(d.user.Fs, src, func(name string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || thumbnailKind(info.Name()) == "" {
			return nil
		}

		file, err := files.NewFileInfo(&files.FileOptions{Fs: d.user.Fs, Path: name, Checker: allowAll{}})
		if err != nil {
			return nil
		}

		entry := movedThumbnail{rel: strings.TrimPrefix(strings.TrimPrefix(name, src), "/")}
		entry.thumbnail, _, _ = fileCache.Load(ctx, previewCacheKey(file, PreviewSizeThumb))
		entry.duration, _, _ = fileCache.Load(ctx, durationCacheKey(file))
		if entry.thumbnail != nil || entry.duration != nil {
			found = append(found, entry)
		}
		return nil
	})

	return found
}

// storeMovedThumbnails files the remembered entries under the new location.
func storeMovedThumbnails(ctx context.Context, d *data, fileCache FileCache, target string, found []movedThumbnail) {
	for _, entry := range found {
		newPath := target
		if entry.rel != "" {
			newPath = path.Join(target, entry.rel)
		}

		file, err := files.NewFileInfo(&files.FileOptions{Fs: d.user.Fs, Path: newPath, Checker: allowAll{}})
		if err != nil {
			continue
		}

		if entry.thumbnail != nil {
			_ = fileCache.Store(ctx, previewCacheKey(file, PreviewSizeThumb), entry.thumbnail)
		}
		if entry.duration != nil {
			_ = fileCache.Store(ctx, durationCacheKey(file), entry.duration)
		}
	}
}

// stagedSource is one checked entry of a staging folder.
type stagedSource struct {
	src   string
	isDir bool
}

// checkStagedItems makes sure every item is a direct child of a staging folder
// and that the rules allow touching it and, for a folder, everything inside it.
// It returns the status to answer with when something is not allowed.
func checkStagedItems(d *data, items []string) ([]stagedSource, int, error) {
	if len(items) == 0 || len(items) > maxMoveItems {
		return nil, http.StatusBadRequest, errors.New("no items or too many")
	}

	var checked []stagedSource
	for _, item := range items {
		src := slashClean(item)
		if src == "/" || !isUnderscoreName(path.Base(path.Dir(src))) || !d.Check(src) {
			return nil, http.StatusForbidden, nil
		}

		info, err := d.user.Fs.Stat(src)
		if err != nil {
			return nil, errToStatus(err), err
		}

		if info.IsDir() {
			// The rules apply to everything inside a folder too.
			denied := false
			_ = afero.Walk(d.user.Fs, src, func(name string, _ os.FileInfo, err error) error {
				if err == nil && !d.Check(name) {
					denied = true
				}
				return nil
			})
			if denied {
				return nil, http.StatusForbidden, nil
			}
		}

		checked = append(checked, stagedSource{src: src, isDir: info.IsDir()})
	}

	return checked, 0, nil
}

// moveHandler moves entries out of a staging folder, one whose name starts with
// an underscore, into a folder that is not one. That is all it does: nothing is
// renamed, copied or overwritten, and nothing can be moved into or out of any
// other place.
func moveHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Rename {
			return http.StatusForbidden, nil
		}

		var req moveRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			return http.StatusBadRequest, err
		}

		dst := slashClean(req.Destination)
		if hasUnderscoreSegment(dst) || !d.Check(dst) {
			return http.StatusForbidden, nil
		}
		dstInfo, err := d.user.Fs.Stat(dst)
		if err != nil {
			return errToStatus(err), err
		}
		if !dstInfo.IsDir() {
			return http.StatusBadRequest, errors.New("destination is not a folder")
		}

		// Check everything before anything is moved.
		sources, status, err := checkStagedItems(d, req.Items)
		if status != 0 || err != nil {
			return status, err
		}

		type job struct{ src, target string }
		var jobs []job
		names := map[string]bool{}
		for _, source := range sources {
			src := source.src
			if path.Dir(src) == dst {
				return http.StatusForbidden, nil
			}
			if source.isDir && (dst == src || strings.HasPrefix(dst+"/", src+"/")) {
				return http.StatusBadRequest, errors.New("a folder cannot be moved into itself")
			}

			name := path.Base(src)
			target := path.Join(dst, name)
			if names[name] {
				return http.StatusConflict, fmt.Errorf("%s: two items with the same name", name)
			}
			names[name] = true
			if pathExists(d.user.Fs, target) {
				return http.StatusConflict, fmt.Errorf("%s: already exists there", name)
			}
			if !d.Check(target) {
				return http.StatusForbidden, nil
			}

			jobs = append(jobs, job{src: src, target: target})
		}

		for _, j := range jobs {
			remembered := collectThumbnails(r.Context(), d, fileCache, j.src)

			err := d.RunHook(func() error {
				return d.user.Fs.Rename(j.src, j.target)
			}, "rename", j.src, j.target, d.user)
			if err != nil {
				return errToStatus(err), err
			}

			storeMovedThumbnails(r.Context(), d, fileCache, j.target, remembered)
		}

		return http.StatusNoContent, nil
	})
}

type deleteRequest struct {
	Items []string `json:"items"`
}

// deleteStagedHandler deletes entries of a staging folder for good. Like moving
// it only works on what lies directly inside a folder whose name starts with an
// underscore.
func deleteStagedHandler() handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Delete {
			return http.StatusForbidden, nil
		}

		var req deleteRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			return http.StatusBadRequest, err
		}

		sources, status, err := checkStagedItems(d, req.Items)
		if status != 0 || err != nil {
			return status, err
		}

		for _, source := range sources {
			err := d.RunHook(func() error {
				return d.user.Fs.RemoveAll(source.src)
			}, "delete", source.src, "", d.user)
			if err != nil {
				return errToStatus(err), err
			}
		}

		return http.StatusNoContent, nil
	})
}
