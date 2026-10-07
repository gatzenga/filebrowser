package fbhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gorilla/mux"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/files"
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

// makeTestVideo writes a video that is red until 45 seconds and blue after
// that, so the frame that was picked tells where it came from.
func makeTestVideo(t *testing.T, path string) {
	t.Helper()
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=160x90:r=2:d=120",
		"-vf", "geq=r='if(lt(T,45),255,0)':g=0:b='if(lt(T,45),0,255)'",
		"-pix_fmt", "yuv420p", path,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("can't create test video: %v: %s", err, out)
	}
}

func TestThumbSeek(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		duration    float64
		wantDefault float64
		windowFrom  float64
		windowTo    float64
	}{
		"feature film":   {duration: 7200, wantDefault: 300, windowFrom: 30, windowTo: 300},
		"episode":        {duration: 1500, wantDefault: 150, windowFrom: 30, windowTo: 300},
		"short clip":     {duration: 120, wantDefault: 30, windowFrom: 30, windowTo: 108},
		"under a minute": {duration: 40, wantDefault: 10, windowFrom: 10, windowTo: 30},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := thumbSeek(tc.duration, false); got != tc.wantDefault {
				t.Errorf("default seek = %v, want %v", got, tc.wantDefault)
			}
			for range 500 {
				if got := thumbSeek(tc.duration, true); got < tc.windowFrom || got > tc.windowTo {
					t.Fatalf("random seek %v is outside %v..%v", got, tc.windowFrom, tc.windowTo)
				}
			}
		})
	}

	if got := thumbSeek(0, false); got != 3 {
		t.Errorf("unknown length seek = %v, want 3", got)
	}
}

func TestVideoThumbnailComesFromTheEarlyWindow(t *testing.T) {
	requireFFmpeg(t)

	for _, name := range []string{"film.mkv", "film.mp4"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name)
			makeTestVideo(t, path)

			// The video is 120 seconds, so the default spot is 30 seconds in,
			// which is still red. The middle (60 seconds) would be blue.
			data, err := videoThumbnail(context.Background(), path, thumbSeek(120, false))
			if err != nil {
				t.Fatalf("videoThumbnail: %v", err)
			}

			thumb, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("result is not a JPEG: %v", err)
			}
			if b := thumb.Bounds(); b.Dx() != thumbWidth || b.Dy() != thumbHeight {
				t.Errorf("thumbnail is %dx%d, want %dx%d", b.Dx(), b.Dy(), thumbWidth, thumbHeight)
			}

			r, _, bl, _ := thumb.At(thumbWidth/2, thumbHeight/2).RGBA()
			if r < bl {
				t.Errorf("frame is not from the first 45 seconds (r=%d b=%d)", r>>8, bl>>8)
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
	if got := cacheFiles(); got != 2 {
		t.Fatalf("after the first scan the cache has %d entries, want 2", got)
	}

	// A second scan must not create anything new.
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 2 {
		t.Fatalf("after the second scan the cache has %d entries, want 2", got)
	}

	// A new video gets its thumbnail on the next scan.
	makeTestVideo(t, filepath.Join(media, "second.mp4"))
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 4 {
		t.Fatalf("after adding a video the cache has %d entries, want 4", got)
	}

	// Removing a video removes its thumbnail.
	if err := os.Remove(filepath.Join(media, "film.mkv")); err != nil {
		t.Fatal(err)
	}
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 2 {
		t.Fatalf("after removing a video the cache has %d entries, want 2", got)
	}

	// An empty media folder (for example one that is not mounted) must not wipe the cache.
	if err := os.Remove(filepath.Join(media, "second.mp4")); err != nil {
		t.Fatal(err)
	}
	scanThumbnails(context.Background(), st, server, imgSvc, cache)
	if got := cacheFiles(); got != 2 {
		t.Fatalf("an empty media folder wiped the cache (%d entries left)", got)
	}
}

func TestListingShowsStoredVideoDurations(t *testing.T) {
	requireFFmpeg(t)

	media := t.TempDir()
	makeTestVideo(t, filepath.Join(media, "film.mkv"))

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
	cache := diskcache.New(afero.NewOsFs(), t.TempDir())

	list := func() *files.FileInfo {
		usrs, err := st.Users.Gets(server.Root, false)
		if err != nil || len(usrs) != 1 {
			t.Fatalf("users: %v (%d)", err, len(usrs))
		}
		dir, err := files.NewFileInfo(&files.FileOptions{Fs: usrs[0].Fs, Path: "/", Expand: true, Checker: allowAll{}})
		if err != nil {
			t.Fatal(err)
		}
		addVideoDurations(context.Background(), cache, dir)
		return dir
	}

	if got := list().Items[0].Duration; got != 0 {
		t.Fatalf("duration before the scan = %v, want 0", got)
	}

	scanThumbnails(context.Background(), st, server, img.New(1), cache)

	got := list().Items[0].Duration
	if got < 119 || got > 121 {
		t.Fatalf("duration after the scan = %v, want about 120", got)
	}
}

func TestRenewThumbnailReplacesTheStoredOne(t *testing.T) {
	requireFFmpeg(t)

	scope := t.TempDir()
	makeTestVideo(t, filepath.Join(scope, "film.mkv"))
	if err := os.WriteFile(filepath.Join(scope, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Users.Save(&users.User{Username: "u", Password: "pw", Scope: ".", Perm: perm}); err != nil {
		t.Fatal(err)
	}
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatal(err)
	}
	cache := diskcache.New(afero.NewOsFs(), t.TempDir())

	renew := func(target string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodPost, "/"+target, http.NoBody)
		req.Header.Set("X-Auth", signToken(t, perm, key))
		req = mux.SetURLVars(req, map[string]string{"path": target})
		rec := httptest.NewRecorder()
		handle(thumbnailRenewHandler(cache, true), "", st, &settings.Server{Root: scope}).ServeHTTP(rec, req)
		return rec
	}

	if rec := renew("film.mkv"); rec.Code != http.StatusNoContent {
		t.Fatalf("renew = %d, body=%q; want 204", rec.Code, rec.Body.String())
	}

	usrs, err := st.Users.Gets(scope, false)
	if err != nil || len(usrs) != 1 {
		t.Fatalf("users: %v (%d)", err, len(usrs))
	}
	file, err := files.NewFileInfo(&files.FileOptions{Fs: usrs[0].Fs, Path: "/film.mkv", Checker: allowAll{}})
	if err != nil {
		t.Fatal(err)
	}
	data, ok, err := cache.Load(context.Background(), previewCacheKey(file, PreviewSizeThumb))
	if err != nil || !ok {
		t.Fatalf("no stored thumbnail after renewing (ok=%v, err=%v)", ok, err)
	}
	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("stored thumbnail is not a JPEG: %v", err)
	}

	if rec := renew("notes.txt"); rec.Code != http.StatusBadRequest {
		t.Errorf("renewing a text file = %d; want 400", rec.Code)
	}
	if rec := renew("missing.mkv"); rec.Code != http.StatusNotFound {
		t.Errorf("renewing a missing file = %d; want 404", rec.Code)
	}
}

func TestVideoDurationHandlerProbesAndStores(t *testing.T) {
	requireFFmpeg(t)

	scope := t.TempDir()
	makeTestVideo(t, filepath.Join(scope, "film.mkv"))
	if err := os.WriteFile(filepath.Join(scope, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	db, err := storm.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := bolt.NewStorage(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Users.Save(&users.User{Username: "u", Password: "pw", Scope: ".", Perm: perm}); err != nil {
		t.Fatal(err)
	}
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatal(err)
	}

	for name, cache := range map[string]FileCache{
		"with a cache":    diskcache.New(afero.NewOsFs(), t.TempDir()),
		"without a cache": diskcache.NewNoOp(),
	} {
		t.Run(name, func(t *testing.T) {
			get := func(target string) *httptest.ResponseRecorder {
				req, _ := http.NewRequest(http.MethodGet, "/"+target, http.NoBody)
				req.Header.Set("X-Auth", signToken(t, perm, key))
				req = mux.SetURLVars(req, map[string]string{"path": target})
				rec := httptest.NewRecorder()
				handle(videoDurationHandler(cache), "", st, &settings.Server{Root: scope}).ServeHTTP(rec, req)
				return rec
			}

			rec := get("film.mkv")
			if rec.Code != http.StatusOK {
				t.Fatalf("duration = %d, body=%q; want 200", rec.Code, rec.Body.String())
			}
			var body struct {
				Duration float64 `json:"duration"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
			}
			if body.Duration < 119 || body.Duration > 121 {
				t.Errorf("duration = %v, want about 120", body.Duration)
			}

			if rec := get("notes.txt"); rec.Code != http.StatusBadRequest {
				t.Errorf("text file = %d; want 400", rec.Code)
			}
			if rec := get("missing.mkv"); rec.Code != http.StatusNotFound {
				t.Errorf("missing file = %d; want 404", rec.Code)
			}
		})
	}
}
