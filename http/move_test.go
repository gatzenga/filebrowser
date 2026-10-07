package fbhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/diskcache"
	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/img"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
	"github.com/filebrowser/filebrowser/v2/storage/bolt"
	"github.com/filebrowser/filebrowser/v2/users"
)

type moveEnv struct {
	root  string
	st    *storage.Storage
	cache FileCache
	key   []byte
}

func newMoveEnv(t *testing.T, perm users.Permissions) *moveEnv {
	t.Helper()

	root := t.TempDir()
	for _, dir := range []string{"_inbox/sub", "_other", "Filme/Alt", "normal"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{
		"_inbox/a.txt":     "a",
		"_inbox/b.txt":     "b",
		"_inbox/sub/c.txt": "c",
		"Filme/b.txt":      "already there",
		"Filme/Alt/d.txt":  "d",
		"normal/x.txt":     "x",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
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
	if err := st.Users.Save(&users.User{Username: "u", Password: "pw", Scope: ".", Perm: perm}); err != nil {
		t.Fatal(err)
	}
	key := []byte("test-signing-key")
	if err := st.Settings.Save(&settings.Settings{Key: key}); err != nil {
		t.Fatal(err)
	}

	return &moveEnv{root: root, st: st, cache: diskcache.New(afero.NewOsFs(), t.TempDir()), key: key}
}

func (e *moveEnv) move(t *testing.T, perm users.Permissions, destination string, items ...string) int {
	t.Helper()

	body, _ := json.Marshal(moveRequest{Items: items, Destination: destination})
	req, _ := http.NewRequest(http.MethodPost, "/move", bytes.NewReader(body))
	req.Header.Set("X-Auth", signToken(t, perm, e.key))
	rec := httptest.NewRecorder()
	handle(moveHandler(e.cache), "", e.st, &settings.Server{Root: e.root}).ServeHTTP(rec, req)
	return rec.Code
}

func (e *moveEnv) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(e.root, rel))
	return err == nil
}

func TestMoveOutOfAStagingFolder(t *testing.T) {
	perm := users.Permissions{Rename: true, Download: true}

	t.Run("moves files and folders into a normal folder", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		if code := e.move(t, perm, "/Filme", "/_inbox/a.txt", "/_inbox/sub"); code != http.StatusNoContent {
			t.Fatalf("move = %d, want 204", code)
		}
		if e.exists("_inbox/a.txt") || e.exists("_inbox/sub") {
			t.Error("the items are still in the staging folder")
		}
		if !e.exists("Filme/a.txt") || !e.exists("Filme/sub/c.txt") {
			t.Error("the items did not arrive")
		}
	})

	t.Run("refuses items that are not in a staging folder", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		for _, item := range []string{"/normal/x.txt", "/normal", "/", "/Filme/Alt/d.txt", "/_inbox/../normal/x.txt"} {
			if code := e.move(t, perm, "/Filme", item); code != http.StatusForbidden {
				t.Errorf("moving %q = %d, want 403", item, code)
			}
		}
		if !e.exists("normal/x.txt") || !e.exists("Filme/Alt/d.txt") {
			t.Error("something outside a staging folder was moved")
		}
	})

	t.Run("refuses a destination inside a staging folder", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		for _, destination := range []string{"/_other", "/_inbox/sub", "/Filme/_hidden", "/_inbox"} {
			if code := e.move(t, perm, destination, "/_inbox/a.txt"); code != http.StatusForbidden {
				t.Errorf("moving into %q = %d, want 403", destination, code)
			}
		}
		if !e.exists("_inbox/a.txt") {
			t.Error("the item moved although the destination was not allowed")
		}
	})

	t.Run("never overwrites and moves nothing when one item clashes", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		if code := e.move(t, perm, "/Filme", "/_inbox/a.txt", "/_inbox/b.txt"); code != http.StatusConflict {
			t.Fatalf("move = %d, want 409", code)
		}
		if !e.exists("_inbox/a.txt") || !e.exists("_inbox/b.txt") {
			t.Error("a clash still moved the other item")
		}
		if got, _ := os.ReadFile(filepath.Join(e.root, "Filme/b.txt")); string(got) != "already there" {
			t.Error("an existing file was overwritten")
		}
	})

	t.Run("destination must be a folder that exists", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		if code := e.move(t, perm, "/Filme/b.txt", "/_inbox/a.txt"); code != http.StatusBadRequest {
			t.Errorf("into a file = %d, want 400", code)
		}
		if code := e.move(t, perm, "/missing", "/_inbox/a.txt"); code != http.StatusNotFound {
			t.Errorf("into a missing folder = %d, want 404", code)
		}
	})

	t.Run("needs the permission to rename", func(t *testing.T) {
		noRename := users.Permissions{Download: true}
		e := newMoveEnv(t, noRename)
		if code := e.move(t, noRename, "/Filme", "/_inbox/a.txt"); code != http.StatusForbidden {
			t.Errorf("move = %d, want 403", code)
		}
	})
}

// A thumbnail that was chosen has to stay with its video when it is moved.
func TestMoveKeepsThumbnails(t *testing.T) {
	requireFFmpeg(t)

	perm := users.Permissions{Rename: true, Download: true}
	e := newMoveEnv(t, perm)
	makeTestVideo(t, filepath.Join(e.root, "_inbox", "film.mkv"))
	makeTestVideo(t, filepath.Join(e.root, "_inbox", "sub", "episode.mp4"))

	server := &settings.Server{Root: e.root, EnableThumbnails: true}
	scanThumbnails(context.Background(), e.st, server, img.New(1), e.cache.(thumbnailCache))

	thumbnail := func(rel string) []byte {
		t.Helper()
		usrs, err := e.st.Users.Gets(e.root, false)
		if err != nil || len(usrs) != 1 {
			t.Fatalf("users: %v (%d)", err, len(usrs))
		}
		file, err := files.NewFileInfo(&files.FileOptions{Fs: usrs[0].Fs, Path: rel, Checker: allowAll{}})
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		data, _, _ := e.cache.Load(context.Background(), previewCacheKey(file, PreviewSizeThumb))
		return data
	}

	film, episode := thumbnail("/_inbox/film.mkv"), thumbnail("/_inbox/sub/episode.mp4")
	if film == nil || episode == nil {
		t.Fatal("the scan made no thumbnails")
	}

	if code := e.move(t, perm, "/Filme", "/_inbox/film.mkv", "/_inbox/sub"); code != http.StatusNoContent {
		t.Fatalf("move = %d, want 204", code)
	}

	if got := thumbnail("/Filme/film.mkv"); !bytes.Equal(got, film) {
		t.Error("the thumbnail of a moved video did not follow it")
	}
	if got := thumbnail("/Filme/sub/episode.mp4"); !bytes.Equal(got, episode) {
		t.Error("the thumbnail of a video in a moved folder did not follow it")
	}
}

func (e *moveEnv) discard(t *testing.T, perm users.Permissions, items ...string) int {
	t.Helper()

	body, _ := json.Marshal(deleteRequest{Items: items})
	req, _ := http.NewRequest(http.MethodPost, "/delete", bytes.NewReader(body))
	req.Header.Set("X-Auth", signToken(t, perm, e.key))
	rec := httptest.NewRecorder()
	handle(deleteStagedHandler(), "", e.st, &settings.Server{Root: e.root}).ServeHTTP(rec, req)
	return rec.Code
}

func TestDeleteInAStagingFolder(t *testing.T) {
	perm := users.Permissions{Delete: true, Download: true}

	t.Run("deletes files and folders of a staging folder", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		if code := e.discard(t, perm, "/_inbox/a.txt", "/_inbox/sub"); code != http.StatusNoContent {
			t.Fatalf("delete = %d, want 204", code)
		}
		if e.exists("_inbox/a.txt") || e.exists("_inbox/sub") {
			t.Error("the items are still there")
		}
		if !e.exists("_inbox/b.txt") {
			t.Error("an item that was not asked for was deleted")
		}
	})

	t.Run("refuses everything outside a staging folder", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		for _, item := range []string{"/normal/x.txt", "/normal", "/", "/Filme/Alt/d.txt", "/_inbox/../normal/x.txt", "/_inbox", "/_inbox/"} {
			if code := e.discard(t, perm, item); code != http.StatusForbidden {
				t.Errorf("deleting %q = %d, want 403", item, code)
			}
		}
		for _, rel := range []string{"normal/x.txt", "Filme/Alt/d.txt", "_inbox/a.txt"} {
			if !e.exists(rel) {
				t.Errorf("%s was deleted", rel)
			}
		}
	})

	t.Run("deletes nothing when one item is not allowed", func(t *testing.T) {
		e := newMoveEnv(t, perm)
		if code := e.discard(t, perm, "/_inbox/a.txt", "/normal/x.txt"); code != http.StatusForbidden {
			t.Fatalf("delete = %d, want 403", code)
		}
		if !e.exists("_inbox/a.txt") {
			t.Error("an allowed item was deleted although another one was refused")
		}
	})

	t.Run("needs the permission to delete", func(t *testing.T) {
		noDelete := users.Permissions{Download: true}
		e := newMoveEnv(t, noDelete)
		if code := e.discard(t, noDelete, "/_inbox/a.txt"); code != http.StatusForbidden {
			t.Errorf("delete = %d, want 403", code)
		}
		if !e.exists("_inbox/a.txt") {
			t.Error("the item was deleted without permission")
		}
	})
}
