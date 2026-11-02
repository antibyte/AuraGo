package storymaker

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
)

// MemStore is an in-memory Store for tests of this package and of the packages that build on
// it. It follows the Store contract exactly, including etags, MustNotExist, recursive Remove
// and List of a missing directory.
type MemStore struct {
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{files: map[string][]byte{}, dirs: map[string]bool{}}
}

func memClean(p string) string {
	return strings.Trim(path.Clean(strings.ReplaceAll(p, "\\", "/")), "/")
}

// ReadFile implements Store.
func (m *MemStore) ReadFile(_ context.Context, p string) ([]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[memClean(p)]
	if !ok {
		return nil, "", fmt.Errorf("storymaker: %s: %w", p, fs.ErrNotExist)
	}
	return append([]byte(nil), data...), ETag(data), nil
}

// WriteFile implements Store.
func (m *MemStore) WriteFile(_ context.Context, p string, data []byte, ifMatch string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(p)
	cur, exists := m.files[key]
	switch {
	case ifMatch == "":
	case ifMatch == MustNotExist:
		if exists {
			return "", ErrConflict
		}
	default:
		if !exists || ETag(cur) != ifMatch {
			return "", ErrConflict
		}
	}
	if m.dirs[key] {
		return "", fmt.Errorf("storymaker: %s is a directory", p)
	}
	m.mkdirs(path.Dir(key))
	m.files[key] = append([]byte(nil), data...)
	return ETag(data), nil
}

// Remove implements Store: it removes a file or a directory tree; a missing path is fine.
func (m *MemStore) Remove(_ context.Context, p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(p)
	delete(m.files, key)
	delete(m.dirs, key)
	prefix := key + "/"
	for f := range m.files {
		if strings.HasPrefix(f, prefix) {
			delete(m.files, f)
		}
	}
	for d := range m.dirs {
		if strings.HasPrefix(d, prefix) {
			delete(m.dirs, d)
		}
	}
	return nil
}

// List implements Store: the base names of the direct children, sorted; (nil, nil) when the
// directory does not exist.
func (m *MemStore) List(_ context.Context, dir string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memClean(dir)
	prefix := key + "/"
	if key == "." || key == "" {
		prefix = ""
	}
	seen := map[string]bool{}
	collect := func(full string) {
		if !strings.HasPrefix(full, prefix) || full == strings.TrimSuffix(prefix, "/") {
			return
		}
		rest := strings.TrimPrefix(full, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			seen[rest] = true
		}
	}
	for f := range m.files {
		collect(f)
	}
	for d := range m.dirs {
		collect(d)
	}
	if len(seen) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// MkdirAll implements Store.
func (m *MemStore) MkdirAll(_ context.Context, dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirs(memClean(dir))
	return nil
}

func (m *MemStore) mkdirs(dir string) {
	for dir != "." && dir != "" && dir != "/" {
		m.dirs[dir] = true
		dir = path.Dir(dir)
	}
}

// Paths lists every file path in the store, sorted. It is a test helper.
func (m *MemStore) Paths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.files))
	for f := range m.files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Dirs lists every directory path in the store, sorted. It is a test helper.
func (m *MemStore) Dirs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.dirs))
	for d := range m.dirs {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}
