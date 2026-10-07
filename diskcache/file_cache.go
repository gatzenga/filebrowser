package diskcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/afero"
)

type FileCache struct {
	fs afero.Fs

	// granular locks
	scopedLocks struct {
		sync.Mutex
		sync.Once
		locks map[string]sync.Locker
	}
}

func New(fs afero.Fs, root string) *FileCache {
	return &FileCache{
		fs: afero.NewBasePathFs(fs, root),
	}
}

func (f *FileCache) Store(_ context.Context, key string, value []byte) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := f.fs.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
		return err
	}

	if err := afero.WriteFile(f.fs, fileName, value, 0700); err != nil {
		return err
	}

	return nil
}

func (f *FileCache) Load(_ context.Context, key string) (value []byte, exist bool, err error) {
	r, ok, err := f.open(key)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer r.Close()

	value, err = io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (f *FileCache) Delete(_ context.Context, key string) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := f.fs.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *FileCache) open(key string) (afero.File, bool, error) {
	fileName := f.getFileName(key)
	file, err := f.fs.Open(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return file, true, nil
}

// getScopedLocks pull lock from the map if found or create a new one
func (f *FileCache) getScopedLocks(key string) (lock sync.Locker) {
	f.scopedLocks.Do(func() { f.scopedLocks.locks = map[string]sync.Locker{} })

	f.scopedLocks.Lock()
	lock, ok := f.scopedLocks.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		f.scopedLocks.locks[key] = lock
	}
	f.scopedLocks.Unlock()

	return lock
}

func (f *FileCache) getFileName(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("%s/%s/%s", hash[:1], hash[1:3], hash)
}

// FileName returns the name under which key is stored, relative to the cache
// directory. It lets callers work out which entries are still wanted.
func (f *FileCache) FileName(key string) string {
	return f.getFileName(key)
}

// Exists reports whether an entry for key is stored.
func (f *FileCache) Exists(key string) bool {
	_, err := f.fs.Stat(f.getFileName(key))
	return err == nil
}

// Sweep removes every stored entry whose file name is not in keep and returns
// how many were removed.
func (f *FileCache) Sweep(keep map[string]struct{}) (int, error) {
	var stale []string
	err := afero.Walk(f.fs, "/", func(name string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if _, ok := keep[strings.TrimPrefix(filepath.ToSlash(name), "/")]; !ok {
			stale = append(stale, name)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, name := range stale {
		if err := f.fs.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
