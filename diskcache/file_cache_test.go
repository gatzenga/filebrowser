package diskcache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestFileCache(t *testing.T) {
	ctx := context.Background()
	const (
		key            = "key"
		value          = "some text"
		newValue       = "new text"
		cacheRoot      = "/cache"
		cachedFilePath = "a/62/a62f2225bf70bfaccbc7f1ef2a397836717377de"
	)

	fs := afero.NewMemMapFs()
	cache := New(fs, "/cache")

	// store new key
	err := cache.Store(ctx, key, []byte(value))
	require.NoError(t, err)
	checkValue(ctx, t, fs, filepath.Join(cacheRoot, cachedFilePath), cache, key, value)

	// update existing key
	err = cache.Store(ctx, key, []byte(newValue))
	require.NoError(t, err)
	checkValue(ctx, t, fs, filepath.Join(cacheRoot, cachedFilePath), cache, key, newValue)

	// delete key
	err = cache.Delete(ctx, key)
	require.NoError(t, err)
	exists, err := afero.Exists(fs, filepath.Join(cacheRoot, cachedFilePath))
	require.NoError(t, err)
	require.False(t, exists)
}

func checkValue(ctx context.Context, t *testing.T, fs afero.Fs, fileFullPath string, cache *FileCache, key, wantValue string) {
	t.Helper()
	// check actual file content
	b, err := afero.ReadFile(fs, fileFullPath)
	require.NoError(t, err)
	require.Equal(t, wantValue, string(b))

	// check cache content
	b, ok, err := cache.Load(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, wantValue, string(b))
}

func TestSweepKeepsWantedAndRecentlySeenEntries(t *testing.T) {
	ctx := context.Background()
	fs := afero.NewMemMapFs()
	cache := New(fs, "/cache")

	for _, key := range []string{"keep", "orphan-old", "orphan-new"} {
		if err := cache.Store(ctx, key, []byte(key)); err != nil {
			t.Fatalf("store %s: %v", key, err)
		}
	}

	// "keep" and "orphan-old" were last touched long ago, "orphan-new" just now.
	long := time.Now().Add(-60 * 24 * time.Hour)
	for _, key := range []string{"keep", "orphan-old"} {
		if err := fs.Chtimes("/cache/"+cache.FileName(key), long, long); err != nil {
			t.Fatalf("chtimes %s: %v", key, err)
		}
	}

	keep := map[string]struct{}{cache.FileName("keep"): {}}
	removed, err := cache.Sweep(keep, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	if !cache.Exists("keep") {
		t.Error("a wanted entry was removed")
	}
	if cache.Exists("orphan-old") {
		t.Error("an entry unseen for longer than the grace period is still there")
	}
	if !cache.Exists("orphan-new") {
		t.Error("an unwanted entry inside the grace period was removed")
	}

	// Being wanted renews the entry, so it survives later sweeps without it.
	info, err := fs.Stat("/cache/" + cache.FileName("keep"))
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Errorf("a wanted entry was not marked as seen (mtime %v)", info.ModTime())
	}
}
