//go:build linux

package upkeep

import (
	"os"
	"syscall"
)

func fileUID(i os.FileInfo) uint32 {
	if s, ok := i.Sys().(*syscall.Stat_t); ok {
		return s.Uid
	}
	return ^uint32(0)
}

func allocatedSize(i os.FileInfo) int64 {
	if s, ok := i.Sys().(*syscall.Stat_t); ok {
		if s.Nlink > 1 {
			return 0
		}
		return s.Blocks * 512
	}
	return 0
}
