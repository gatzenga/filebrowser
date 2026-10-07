package fbhttp

import (
	"bytes"
	"context"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if !ffmpegAvailable() {
		t.Skip("ffmpeg and ffprobe are not installed")
	}
}

// makeTestVideo writes a 20 second video that is red for the first 8 seconds
// and blue afterwards, so the frame that was picked tells where it came from.
func makeTestVideo(t *testing.T, path string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=320x240:r=10:d=20",
		"-vf", "geq=r='if(lt(T,8),255,0)':g=0:b='if(lt(T,8),0,255)'",
		"-pix_fmt", "yuv420p", path,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("can't create test video: %v: %s", err, out)
	}
}

func TestVideoThumbnailIsTakenFromTheMiddle(t *testing.T) {
	requireFFmpeg(t)

	for _, name := range []string{"film.mkv", "film.mp4"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			makeTestVideo(t, path)

			data, err := videoThumbnail(context.Background(), path)
			if err != nil {
				t.Fatalf("videoThumbnail: %v", err)
			}

			thumb, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("result is not a JPEG: %v", err)
			}
			if b := thumb.Bounds(); b.Dx() != thumbSize || b.Dy() != thumbSize {
				t.Errorf("thumbnail is %dx%d, want %[3]dx%[3]d", b.Dx(), b.Dy(), thumbSize)
			}

			r, _, bl, _ := thumb.At(thumbSize/2, thumbSize/2).RGBA()
			if bl < r {
				t.Errorf("frame is not from the second half (r=%d b=%d)", r>>8, bl>>8)
			}
		})
	}
}

func TestThumbnailScanCreatesAndRemovesThumbnails(t *testing.T) {
	requireFFmpeg(t)

	media := t.TempDir()
	makeTestVideo(t, filepath.Join(media, "film.mkv"))
	if err := os.WriteFile(filepath.Join(media, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Users.Save(&users.User{Username: "u", Password: "pw", Scope: "."}); err != nil {
		t.Fatal(err)
	}

	server := &settings.Server{Root: media, EnableThumbnails: true}
	cacheDir := t.TempDir()
	cache := diskcache.New(afero.NewOsFs(), cacheDir)
	imgSvc := img.New(1)

	cacheFiles := func() int {
		n := 0
		_ = filepath.Walk(cacheDir, func(_ string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				n++
			}
			return nil
		})
		return n
	}

	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 1 {
		t.Fatalf("after the first scan the cache has %d entries, want 1", got)
	}

	// A second scan must not create anything new.
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 1 {
		t.Fatalf("after the second scan the cache has %d entries, want 1", got)
	}

	// A new video gets its thumbnail on the next scan.
	makeTestVideo(t, filepath.Join(media, "second.mp4"))
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 2 {
		t.Fatalf("after adding a video the cache has %d entries, want 2", got)
	}

	// Removing a video removes its thumbnail.
	if err := os.Remove(filepath.Join(media, "film.mkv")); err != nil {
		t.Fatal(err)
	}
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 1 {
		t.Fatalf("after removing a video the cache has %d entries, want 1", got)
	}

	// An empty media folder (for example one that is not mounted) must not wipe the cache.
	if err := os.Remove(filepath.Join(media, "second.mp4")); err != nil {
		t.Fatal(err)
	}
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 1 {
		t.Fatalf("an empty media folder wiped the cache (%d entries left)", got)
	}
}
