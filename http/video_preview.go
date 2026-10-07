package fbhttp

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/files"
)

const (
	// ffmpegTimeout bounds one frame extraction, including probing the length.
	ffmpegTimeout = 60 * time.Second
	// Thumbnails are 16:9, the usual shape of a video, for videos and images alike.
	thumbWidth  = 480
	thumbHeight = 270
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

// defaultThumbPosition is where in the video the first thumbnail is taken: the
// middle, which avoids intros and credits.
const defaultThumbPosition = 0.5

// randomThumbPosition picks another spot for a renewed thumbnail, away from the
// very start and end.
func randomThumbPosition() float64 {
	return 0.08 + rand.Float64()*0.84 //nolint:gosec // not security relevant
}

// videoThumbnail returns a JPEG taken at the given position (0 to 1) of the
// video. realPath must be an absolute path. duration is the length in seconds,
// or 0 when it is not known.
func videoThumbnail(ctx context.Context, realPath string, duration, position float64) ([]byte, error) {
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
		seek = duration * position
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
		"-vf", fmt.Sprintf("scale=%[1]d:%[2]d:force_original_aspect_ratio=increase,crop=%[1]d:%[2]d", thumbWidth, thumbHeight),
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

// createVideoPreview extracts the thumbnail of file at the given position and
// stores it in the cache, replacing an earlier one.
func createVideoPreview(ctx context.Context, fileCache FileCache, file *files.FileInfo, position float64) ([]byte, error) {
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

	data, err := videoThumbnail(ctx, file.RealPath(), duration, position)
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
		data, err = createVideoPreview(r.Context(), fileCache, file, defaultThumbPosition)
		if err != nil {
			return errToStatus(err), err
		}
	}

	// The thumbnail can be renewed while the video stays the same, so the
	// browser has to ask again every time. The ETag keeps that cheap.
	sum := sha1.Sum(data) //nolint:gosec // only a cache validator
	w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:8])+`"`)
	w.Header().Set("Cache-Control", "private, no-cache")
	http.ServeContent(w, r, file.Name+".jpg", time.Time{}, bytes.NewReader(data))

	return 0, nil
}

// thumbnailRenewHandler replaces the thumbnail of one video with a frame from
// another position, so a thumbnail that does not fit can be rerolled.
func thumbnailRenewHandler(fileCache FileCache, enableThumbnails bool) handleFunc {
	return withUser(func(_ http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if !d.user.Perm.Download {
			return http.StatusForbidden, nil
		}
		if !enableThumbnails || !ffmpegAvailable() {
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

		if _, err := createVideoPreview(r.Context(), fileCache, file, randomThumbPosition()); err != nil {
			return errToStatus(err), err
		}

		return http.StatusNoContent, nil
	})
}
