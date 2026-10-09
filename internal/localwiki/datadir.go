package localwiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// diskMargin is the reserve kept free: max(1 GiB, 1 % of the edition size).
func diskMargin(size int64) int64 {
	margin := size / 100
	if margin < 1<<30 {
		margin = 1 << 30
	}
	return margin
}

// requiredBytes is the free space a download needs: the missing bytes plus the margin.
func requiredBytes(size, alreadyDownloaded int64) int64 {
	remaining := size - alreadyDownloaded
	if remaining < 0 {
		remaining = 0
	}
	return remaining + diskMargin(size)
}

// checkDataDirShape validates a storage directory without touching the disk.
func checkDataDirShape(dir string, sensitive func(string) bool) error {
	if strings.TrimSpace(dir) == "" || !filepath.IsAbs(dir) {
		return fmt.Errorf("%w: %q is not an absolute path", ErrDataDirInvalid, dir)
	}
	if sensitive != nil && sensitive(dir) {
		return fmt.Errorf("%w: %s is a protected system location", ErrDataDirInvalid, dir)
	}
	return nil
}

// prepareDataDir validates the storage directory, creates it and proves that
// AuraGo can write there.
func prepareDataDir(dir string, sensitive func(string) bool) error {
	if err := checkDataDirShape(dir, sensitive); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w: create %s: %v", ErrDataDirInvalid, dir, err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrDataDirInvalid, dir)
	}
	probe, err := os.CreateTemp(dir, ".aurago-write-check-*")
	if err != nil {
		return fmt.Errorf("%w: %s is not writable: %v", ErrDataDirInvalid, dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}
