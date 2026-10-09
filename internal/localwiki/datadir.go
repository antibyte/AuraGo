package localwiki

import (
	"fmt"
	"math"
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

// requiredBytes is the free space a download needs: the missing bytes plus the
// margin. The sum saturates at math.MaxInt64 so that a hostile size can never
// wrap into a small or negative requirement and pass the disk checks.
func requiredBytes(size, alreadyDownloaded int64) int64 {
	if alreadyDownloaded < 0 {
		alreadyDownloaded = 0
	}
	var remaining int64
	if size > alreadyDownloaded {
		remaining = size - alreadyDownloaded
	}
	margin := diskMargin(size)
	if remaining > math.MaxInt64-margin {
		return math.MaxInt64
	}
	return remaining + margin
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
// AuraGo can write there. After the directory exists, the sensitive-path check
// is repeated on the directory with its symbolic links resolved, so a link
// below an innocent name cannot point the edition into a system location.
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
	if err := checkResolvedDataDir(dir, sensitive); err != nil {
		return err
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

// checkResolvedDataDir applies the sensitive-path check to dir after
// filepath.EvalSymlinks. A directory that resolves to itself was already
// checked lexically, and so was one whose only difference is an operating
// system alias (see isSystemPathAlias), which keeps legitimate macOS
// directories under /var and /tmp (they resolve below /private, a protected
// tree) usable. Without a sensitive-path check there is nothing to repeat.
// Limit: Go does not resolve a Windows junction in the last path element, so
// the lexical check is all that protects against that one form.
func checkResolvedDataDir(dir string, sensitive func(string) bool) error {
	if sensitive == nil {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %v", ErrDataDirInvalid, dir, err)
	}
	if strings.EqualFold(filepath.Clean(resolved), filepath.Clean(dir)) || isSystemPathAlias(dir, resolved) {
		return nil
	}
	if sensitive(resolved) {
		return fmt.Errorf("%w: %s resolves to %s, a protected system location", ErrDataDirInvalid, dir, resolved)
	}
	return nil
}

// isSystemPathAlias reports a resolved path that differs from the lexical one
// only by the macOS system links /var, /tmp and /etc, which point into
// /private (so /var/folders/x resolves to /private/var/folders/x). A link a
// user created resolves elsewhere and is not an alias.
func isSystemPathAlias(lexical, resolved string) bool {
	l := filepath.ToSlash(filepath.Clean(lexical))
	r := filepath.ToSlash(filepath.Clean(resolved))
	lower := strings.ToLower(l)
	for _, root := range []string{"/var", "/tmp", "/etc"} {
		if (lower == root || strings.HasPrefix(lower, root+"/")) && strings.EqualFold(r, "/private"+l) {
			return true
		}
	}
	return false
}
