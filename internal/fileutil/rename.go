package fileutil

import (
	"context"
	"errors"
	"os"
	"runtime"
	"syscall"
	"time"
)

// Rename replaces a file, retrying transient Windows reader locks.
func Rename(source, destination string) error {
	return RenameContext(context.Background(), source, destination)
}

// RenameContext retains the original on failure and stops retries on cancellation.
func RenameContext(ctx context.Context, source, destination string) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := os.Rename(source, destination)
		if err == nil {
			return nil
		}
		// ERROR_SHARING_VIOLATION is 32 on Windows; never unlink the original.
		retryable := os.IsPermission(err) || errors.Is(err, syscall.Errno(32))
		if runtime.GOOS != "windows" || !retryable || attempt == 7 {
			return err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 15 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
