package diskcache

import (
	"context"
	"path/filepath"
	"testing"

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

func TestSweepRemovesOnlyUnwantedEntries(t *testing.T) {
	ctx := context.Background()
	cache := New(afero.NewMemMapFs(), "/cache")

	for _, key := range []string{"keep-1", "keep-2", "gone-1", "gone-2"} {
		if err := cache.Store(ctx, key, []byte(key)); err != nil {
			t.Fatalf("store %s: %v", key, err)
		}
	}

	keep := map[string]struct{}{
		cache.FileName("keep-1"): {},
		cache.FileName("keep-2"): {},
	}
	removed, err := cache.Sweep(keep)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	for _, key := range []string{"keep-1", "keep-2"} {
		if !cache.Exists(key) {
			t.Errorf("%s was removed but is wanted", key)
		}
	}
	for _, key := range []string{"gone-1", "gone-2"} {
		if cache.Exists(key) {
			t.Errorf("%s is still there but is not wanted", key)
		}
	}
}
