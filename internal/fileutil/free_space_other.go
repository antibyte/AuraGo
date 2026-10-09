//go:build !linux && !darwin && !freebsd && !windows

package fileutil

import "errors"

// freeDiskBytes has no portable probe on this platform; callers treat the error as "unknown".
func freeDiskBytes(string) (int64, error) {
	return 0, errors.ErrUnsupported
}
