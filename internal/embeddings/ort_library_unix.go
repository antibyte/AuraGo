//go:build !windows

package embeddings

import (
	"fmt"

	"github.com/ebitengine/purego"
)

type nativeLibrary struct {
	handle uintptr
}

func openNativeLibrary(path string) (nativeLibrary, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nativeLibrary{}, fmt.Errorf("load native library %s: %w", path, err)
	}
	return nativeLibrary{handle: handle}, nil
}

func (library nativeLibrary) lookup(name string) (uintptr, error) {
	address, err := purego.Dlsym(library.handle, name)
	if err != nil {
		return 0, fmt.Errorf("resolve native symbol %s: %w", name, err)
	}
	return address, nil
}

func (library nativeLibrary) close() error {
	if library.handle == 0 {
		return nil
	}
	return purego.Dlclose(library.handle)
}
