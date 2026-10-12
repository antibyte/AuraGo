//go:build !windows

package main

import (
	"os"
	"syscall"
)

func preserveFilePermissions(source, target *os.File) error {
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if err := target.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}
	}
	if err := preserveFileACL(source, target); err != nil {
		return err
	}
	return target.Chmod(info.Mode())
}
