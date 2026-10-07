package fbhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/filebrowser/filebrowser/v2/files"
)

const (
	// ffmpegTimeout bounds one frame extraction, including probing the length.
	ffmpegTimeout = 60 * time.Second
	// thumbSize is the edge length of the square thumbnails, the same as for images.
	thumbSize = 256
)

// ffmpegSlots limits how many ffmpeg processes run at once, so opening a big
// folder or the background scan cannot swamp the machine.
var ffmpegSlots = make(chan struct{}, 2)

var (
	ffmpegOnce      sync.Once
	ffmpegInstalled bool
)

func ffmpegAvailable() bool {
	ffmpegOnce.Do(func() {
		_, errFfmpeg := exec.LookPath("ffmpeg")
		_, errFfprobe := exec.LookPath("ffprobe")
		ffmpegInstalled = errFfmpeg == nil && errFfprobe == nil
	})
	return ffmpegInstalled
}

// videoDuration asks ffprobe for the length of the video in seconds.
func videoDuration(ctx context.Context, realPath string) (float64, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-protocol_whitelist", "file",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		realPath,
	).Output()
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// acquireFFmpeg waits for a free ffmpeg slot and returns the function that
// gives it back.
func acquireFFmpeg(ctx context.Context) (func(), error) {
	select {
	case ffmpegSlots <- struct{}{}:
		return func() { <-ffmpegSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// probeDuration returns the length of the video in seconds.
func probeDuration(ctx context.Context, realPath string) (float64, error) {
	release, err := acquireFFmpeg(ctx)
	if err != nil {
		return 0, err
	}
	defer release()

	ctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()

	return videoDuration(ctx, realPath)
}

// videoThumbnail returns a JPEG taken from the middle of the video, which
// avoids intros and credits. realPath must be an absolute path. duration is
// the length in seconds, or 0 when it is not known.
func videoThumbnail(ctx context.Context, realPath string, duration float64) ([]byte, error) {
	release, err := acquireFFmpeg(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	ctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()

	// Without a usable length, fall back to a few seconds in.
	seek := 3.0
	if duration > 0 {
		seek = duration / 2
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-nostdin",
		"-v", "error",
		"-protocol_whitelist", "file",
		"-ss", strconv.FormatFloat(seek, 'f', 3, 64),
		"-i", realPath,
		"-an", "-sn",
		"-frames:v", "1",
		"-vf", fmt.Sprintf("scale=%[1]d:%[1]d:force_original_aspect_ratio=increase,crop=%[1]d:%[1]d", thumbSize),
		"-q:v", "5",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-",
	)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, errors.New("ffmpeg produced no frame")
	}

	return stdout.Bytes(), nil
}

// createVideoPreview extracts the thumbnail of file and stores it in the cache.
func createVideoPreview(ctx context.Context, fileCache FileCache, file *files.FileInfo) ([]byte, error) {
	// Opening through the user's filesystem enforces its scope and symlink
	// rules before ffmpeg is handed the real path.
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return nil, err
	}
	_ = fd.Close()

	// The length is needed for the middle of the video and is worth keeping
	// for the listing, so look it up once and store it.
	duration, known := loadDuration(ctx, fileCache, file)
	if !known {
		if probed, err := probeDuration(ctx, file.RealPath()); err == nil {
			duration = probed
			storeDuration(ctx, fileCache, file, duration)
		}
	}

	data, err := videoThumbnail(ctx, file.RealPath(), duration)
	if err != nil {
		return nil, err
	}

	if err := fileCache.Store(ctx, previewCacheKey(file, PreviewSizeThumb), data); err != nil {
		return nil, err
	}

	return data, nil
}

func handleVideoPreview(
	w http.ResponseWriter,
	r *http.Request,
	fileCache FileCache,
	file *files.FileInfo,
	previewSize PreviewSize,
	enableThumbnails bool,
) (int, error) {
	if previewSize != PreviewSizeThumb || !enableThumbnails || !ffmpegAvailable() {
		return http.StatusNotImplemented, nil
	}

	data, ok, err := fileCache.Load(r.Context(), previewCacheKey(file, previewSize))
	if err != nil {
		return errToStatus(err), err
	}
	if !ok {
		data, err = createVideoPreview(r.Context(), fileCache, file)
		if err != nil {
			return errToStatus(err), err
		}
	}

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name+".jpg", file.ModTime, bytes.NewReader(data))

	return 0, nil
}
