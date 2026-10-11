//go:build !windows

package persondetect

import (
	"syscall"
	"time"
)

// processCPUTime is this process's user + system CPU time (all threads); ok
// is false when it cannot be read.
func processCPUTime() (time.Duration, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, false
	}
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano()), true
}
