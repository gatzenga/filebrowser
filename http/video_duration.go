package fbhttp

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/files"
)

// durationCacheKey identifies the stored length of a video. Like the
// thumbnail key it follows path and size, not the modification time.
func durationCacheKey(f *files.FileInfo) string {
	return fmt.Sprintf("duration-%x%x", f.RealPath(), f.Size)
}

func loadDuration(ctx context.Context, fileCache FileCache, f *files.FileInfo) (float64, bool) {
	data, ok, err := fileCache.Load(ctx, durationCacheKey(f))
	if err != nil || !ok {
		return 0, false
	}

	seconds, err := strconv.ParseFloat(string(data), 64)
	if err != nil || seconds <= 0 {
		return 0, false
	}

	return seconds, true
}

func storeDuration(ctx context.Context, fileCache FileCache, f *files.FileInfo, seconds float64) {
	if seconds <= 0 {
		return
	}

	_ = fileCache.Store(ctx, durationCacheKey(f), []byte(strconv.FormatFloat(seconds, 'f', 3, 64)))
}

// addVideoDurations fills in the length of the videos in a directory listing.
// Only lengths that are already known are used, probing every file here would
// make opening a large folder slow. The thumbnail scan fills the rest in.
func addVideoDurations(ctx context.Context, fileCache FileCache, dir *files.FileInfo) {
	if dir.Listing == nil {
		return
	}

	for _, item := range dir.Items {
		if item.Type != "video" {
			continue
		}
		if seconds, ok := loadDuration(ctx, fileCache, item); ok {
			item.Duration = seconds
		}
	}
}

// videoDurationHandler returns the length of one video. The listing only has
// lengths that are already known, the page asks for the missing ones here, so
// they show up without waiting for the background scan. Lengths are stored in
// the cache when there is one.
func videoDurationHandler(fileCache FileCache) handleFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Download {
			return http.StatusForbidden, nil
		}
		if !ffmpegAvailable() {
			return http.StatusNotImplemented, nil
		}

		file, err := files.NewFileInfo(&files.FileOptions{
			Fs: d.user.Fs,
			// Like previews, this reads its path from mux.Vars.
			Path:       slashClean(mux.Vars(r)["path"]),
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
		})
		if err != nil {
			return errToStatus(err), err
		}
		if file.IsDir || file.Type != "video" {
			return http.StatusBadRequest, nil
		}

		seconds, known := loadDuration(r.Context(), fileCache, file)
		if !known {
			// Opening through the user's filesystem enforces its scope and
			// symlink rules before ffprobe gets the real path.
			fd, err := file.Fs.Open(file.Path)
			if err != nil {
				return errToStatus(err), err
			}
			_ = fd.Close()

			seconds, err = probeDuration(r.Context(), file.RealPath())
			if err != nil {
				return http.StatusNotFound, nil
			}
			storeDuration(r.Context(), fileCache, file, seconds)
		}

		return renderJSON(w, r, map[string]float64{"duration": seconds})
	})
}
