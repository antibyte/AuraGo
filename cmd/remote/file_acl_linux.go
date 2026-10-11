package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func preserveFileACL(source, target *os.File) error {
	const attribute = "system.posix_acl_access"
	// Bound ACL reads; an unsupported size or a read failure aborts replacement.
	data := make([]byte, 64<<10)
	n, err := unix.Fgetxattr(int(source.Fd()), attribute, data)
	if errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) {
		err = unix.Fremovexattr(int(target.Fd()), attribute)
		if errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) {
			return nil
		}
	} else if err == nil {
		err = unix.Fsetxattr(int(target.Fd()), attribute, data[:n], 0)
	}
	if err != nil {
		return fmt.Errorf("preserve POSIX ACL: %w", err)
	}
	return nil
}
