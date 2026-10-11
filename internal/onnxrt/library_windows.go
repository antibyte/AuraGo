//go:build windows

package onnxrt

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// Library is a dynamically loaded native library.
type Library struct {
	handle windows.Handle
}

// OpenLibrary loads the native library at path.
func OpenLibrary(path string) (*Library, error) {
	handle, err := windows.LoadLibrary(path)
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
	address, err := windows.GetProcAddress(l.handle, name)
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
	err := windows.FreeLibrary(l.handle)
	l.handle = 0
	return err
}
