//go:build !windows

package localllm

import (
	"os"
	"strconv"
	"syscall"
)

type fileStat struct{ groupID string }

func statFile(path string) (fileStat, error) {
	info, err := os.Stat(path)
	if err != nil {
		return fileStat{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileStat{}, nil
	}
	return fileStat{groupID: strconv.FormatUint(uint64(stat.Gid), 10)}, nil
}
