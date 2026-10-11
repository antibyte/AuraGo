package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"github.com/ebitengine/purego"
)

func preserveFileACL(source, target *os.File) error {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return fmt.Errorf("load ACL functions: %w", err)
	}
	defer purego.Dlclose(lib)
	var get func(int32) uintptr
	var set func(int32, uintptr) int32
	var init func(int32) uintptr
	var free func(uintptr) int32
	var errno func() *int32
	for _, binding := range []struct {
		name string
		fn   any
	}{
		{"acl_get_fd", &get}, {"acl_set_fd", &set}, {"acl_init", &init},
		{"acl_free", &free}, {"__error", &errno},
	} {
		symbol, err := purego.Dlsym(lib, binding.name)
		if err != nil {
			return fmt.Errorf("resolve ACL function %s: %w", binding.name, err)
		}
		purego.RegisterFunc(binding.fn, symbol)
	}
	// errno belongs to the OS thread that performed the ACL call.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	acl := get(int32(source.Fd()))
	if acl == 0 {
		if err := syscall.Errno(*errno()); err != syscall.ENOENT {
			return fmt.Errorf("read existing ACL: %w", err)
		}
		// An explicit empty ACL removes any grants inherited by the temporary file.
		acl = init(0)
		if acl == 0 {
			return fmt.Errorf("initialize empty ACL: %w", syscall.Errno(*errno()))
		}
	}
	defer free(acl)
	if set(int32(target.Fd()), acl) != 0 {
		return fmt.Errorf("preserve existing ACL: %w", syscall.Errno(*errno()))
	}
	runtime.KeepAlive(source)
	runtime.KeepAlive(target)
	return nil
}
