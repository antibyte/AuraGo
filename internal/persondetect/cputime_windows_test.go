//go:build windows

package persondetect

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// FILETIME intervals are 100 ns ticks split into two 32-bit halves.
func TestFiletimeDuration(t *testing.T) {
	got := filetimeDuration(windows.Filetime{HighDateTime: 1, LowDateTime: 2})
	if want := time.Duration((1<<32)+2) * 100; got != want {
		t.Fatalf("filetimeDuration = %d, want %d", got, want)
	}
}
