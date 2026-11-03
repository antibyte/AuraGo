//go:build !windows

package desktop

import (
	"io/fs"
	"syscall"
)

func archiveSysProcAttr() *syscall.SysProcAttr { return nil }

func regularFileIdentity(_ string, info fs.FileInfo) (fileID, uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fileID{}, 0, fs.ErrInvalid
	}
	return fileID{dev: uint64(stat.Dev), ino: uint64(stat.Ino)}, uint64(stat.Nlink), nil
}
