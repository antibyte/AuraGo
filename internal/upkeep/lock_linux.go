//go:build linux

package upkeep

import (
	"fmt"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

// The updater passes its flock descriptor to maintenance commands only. This
// keeps one lock across backup, replacement, readiness and garbage collection.
func inheritedLock(path string) (func(), bool, error) {
	v := os.Getenv("AURAGO_UPDATE_LOCK_FD")
	if v == "" {
		return nil, false, nil
	}
	fd, err := strconv.Atoi(v)
	if err != nil || fd < 3 {
		return nil, true, fmt.Errorf("invalid inherited update lock")
	}
	dup, err := unix.Dup(fd)
	if err != nil {
		return nil, true, err
	}
	f := os.NewFile(uintptr(dup), "update-lock")
	a, err := f.Stat()
	b, err2 := os.Stat(path)
	if err != nil || err2 != nil || !os.SameFile(a, b) {
		f.Close()
		return nil, true, fmt.Errorf("inherited lock does not belong to installation")
	}
	if err := unix.Flock(dup, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, true, err
	}
	// Do not unlock: the parent owns this open file description.
	return func() { f.Close() }, true, nil
}

func sameOwner(a, b os.FileInfo) bool {
	uid := fileUID(a)
	return uid != ^uint32(0) && uid == fileUID(b)
}
