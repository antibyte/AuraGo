//go:build windows

package persondetect

import (
	"time"

	"golang.org/x/sys/windows"
)

// processCPUTime is this process's kernel + user CPU time (all threads); ok is
// false when it cannot be read. It advances in scheduler ticks (≈15.6 ms by
// default), so callers measure intervals that span several inferences.
func processCPUTime() (time.Duration, bool) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return filetimeDuration(kernel) + filetimeDuration(user), true
}

// filetimeDuration converts a FILETIME interval of 100 ns ticks.
// Filetime.Nanoseconds subtracts the 1601 epoch offset and is wrong for
// intervals.
func filetimeDuration(ft windows.Filetime) time.Duration {
	return time.Duration((int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)) * 100)
}
