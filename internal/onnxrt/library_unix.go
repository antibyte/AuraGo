//go:build !windows

package onnxrt

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// Library is a dynamically loaded native library. It is not safe for
// concurrent use: its owner resolves the symbols it needs and closes the
// Library only after all lookups have finished.
type Library struct {
	handle uintptr
}

// OpenLibrary loads the native library at path (RTLD_NOW | RTLD_GLOBAL).
func OpenLibrary(path string) (*Library, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("load native library %s: %w", path, err)
	}
	return &Library{handle: handle}, nil
}

// Lookup resolves an exported symbol.
func (l *Library) Lookup(name string) (uintptr, error) {
	if l == nil || l.handle == 0 {
		return 0, fmt.Errorf("resolve native symbol %s: library is closed", name)
	}
	address, err := purego.Dlsym(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("resolve native symbol %s: %w", name, err)
	}
	return address, nil
}

// Close unloads the library. Closing twice is safe.
func (l *Library) Close() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := purego.Dlclose(l.handle)
	l.handle = 0
	return err
}
