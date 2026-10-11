//go:build !windows

package persondetect

import (
	"syscall"
	"time"
)

// processCPUTime is this process's user + system CPU time (all threads).
func processCPUTime() time.Duration {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}
