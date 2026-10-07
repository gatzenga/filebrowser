package fbhttp

import (
	"context"
	"fmt"
	"strconv"

	"github.com/filebrowser/filebrowser/v2/files"
)

// durationCacheKey identifies the stored length of a video. Like the
// thumbnail key it changes when the file is replaced.
func durationCacheKey(f *files.FileInfo) string {
	return fmt.Sprintf("duration-%x%x%x", f.RealPath(), f.ModTime.Unix(), f.Size)
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
