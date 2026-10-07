package fbhttp

import (
	"context"
	"errors"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
)

// thumbnailCache is what the background scan needs on top of a FileCache. The
// disk cache has it, the no-op cache does not, which switches the scan off.
type thumbnailCache interface {
	FileCache
	FileName(key string) string
	Exists(key string) bool
	Sweep(keep map[string]struct{}) (int, error)
}

type allowAll struct{}

func (allowAll) Check(string) bool { return true }

// StartThumbnailWorker creates missing thumbnails for images and videos in the
// background and removes cached ones whose file no longer exists. It scans
// shortly after start and then every interval. It does nothing when thumbnails
// are disabled or no cache directory is configured.
func StartThumbnailWorker(
	ctx context.Context,
	store *storage.Storage,
	server *settings.Server,
	imgSvc ImgService,
	fileCache FileCache,
	interval time.Duration,
) {
	cache, ok := fileCache.(thumbnailCache)
	if !ok || !server.EnableThumbnails {
		return
	}

	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}

			scanThumbnails(ctx, store, server, imgSvc, cache)
			timer.Reset(interval)
		}
	}()
}

func scanThumbnails(
	ctx context.Context,
	store *storage.Storage,
	server *settings.Server,
	imgSvc ImgService,
	cache thumbnailCache,
) {
	usrs, err := store.Users.Gets(server.Root, server.FollowExternalSymlinks)
	if err != nil {
		log.Printf("thumbnails: can't list users: %v", err)
		return
	}

	var (
		keep      = map[string]struct{}{}
		generated = 0
		failed    = 0
		seen      = 0
		complete  = true
	)

	for _, user := range usrs {
		err := afero.Walk(user.Fs, "/", func(name string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				// One unreadable entry should not stop the scan, but then the
				// result is incomplete and nothing may be deleted from it.
				complete = false
				if info != nil && info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() {
				return nil
			}

			kind := thumbnailKind(info.Name())
			if kind == "" {
				return nil
			}

			file, err := files.NewFileInfo(&files.FileOptions{
				Fs:      user.Fs,
				Path:    name,
				Checker: allowAll{},
			})
			if err != nil {
				complete = false
				return nil
			}
			seen++

			// Keep the big preview of images too, it lives in the same cache.
			keep[cache.FileName(previewCacheKey(file, PreviewSizeThumb))] = struct{}{}
			if kind == "image" {
				keep[cache.FileName(previewCacheKey(file, PreviewSizeBig))] = struct{}{}
			}

			if cache.Exists(previewCacheKey(file, PreviewSizeThumb)) {
				return nil
			}

			switch kind {
			case "video":
				if !ffmpegAvailable() {
					return nil
				}
				_, err = createVideoPreview(ctx, cache, file)
			case "image":
				format, ferr := imgSvc.FormatFromExtension(file.Extension)
				if ferr != nil || format == img.FormatGif {
					return nil
				}
				_, err = createPreview(imgSvc, cache, file, PreviewSizeThumb)
			}

			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				failed++
				log.Printf("thumbnails: %s: %v", name, err)
				return nil
			}
			generated++
			return nil
		})
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("thumbnails: scan stopped: %v", err)
			}
			return
		}
	}

	removed := 0
	// An empty result usually means the media folder is not mounted, so keep the
	// cache then instead of wiping it.
	if complete && seen > 0 {
		removed, err = cache.Sweep(keep)
		if err != nil {
			log.Printf("thumbnails: cleanup failed: %v", err)
		}
	}

	log.Printf("thumbnails: scanned %d files, created %d, failed %d, removed %d", seen, generated, failed, removed)
}

// videoExtensions covers common video formats the system MIME table may lack.
var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".mov": true, ".avi": true,
	".webm": true, ".wmv": true, ".flv": true, ".mpg": true, ".mpeg": true,
	".ts": true, ".m2ts": true, ".mts": true,
}

// thumbnailKind tells from the extension alone whether a file gets a thumbnail.
func thumbnailKind(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if videoExtensions[ext] {
		return "video"
	}

	mimeType := mime.TypeByExtension(ext)
	switch {
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	default:
		return ""
	}
}
