//go:build windows

package embeddings

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type nativeLibrary struct {
	handle windows.Handle
}

func openNativeLibrary(path string) (nativeLibrary, error) {
	handle, err := windows.LoadLibrary(path)
	if err != nil {
		return nativeLibrary{}, fmt.Errorf("load native library %s: %w", path, err)
	}
	return nativeLibrary{handle: windows.Handle(handle)}, nil
}

func (library nativeLibrary) lookup(name string) (uintptr, error) {
	address, err := windows.GetProcAddress(library.handle, name)
	if err != nil {
		return 0, fmt.Errorf("resolve native symbol %s: %w", name, err)
	}
	return address, nil
}

func (library nativeLibrary) close() error {
	if library.handle == 0 {
		return nil
	}
	return windows.FreeLibrary(library.handle)
}
