package localwiki

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

// restartRequiredBytes is the free space a restart needs (see downloadJob):
// the part file keeps its kept bytes until the restart file holds kept+1
// bytes and replaces it; written bytes of the restart file are already on
// disk. Before the replacement the restart file needs kept+1-written more
// bytes next to the part file. The replacement frees the kept bytes, and the
// rest of the edition then needs size-kept-1 bytes, so from today's free space
// the whole restart takes size-kept-written. The margin must stay free at
// both points, so the larger need plus the margin is required. Inputs are
// clamped to [0, size] and the sum saturates like requiredBytes.
func restartRequiredBytes(size, kept, written int64) int64 {
	size = max(size, 0)
	kept = min(max(kept, 0), size)
	written = min(max(written, 0), size)
	beforeSwap := kept - written
	if beforeSwap < math.MaxInt64 {
		beforeSwap++
	}
	afterSwap := size - kept - written
	need := max(beforeSwap, afterSwap, 0)
	margin := diskMargin(size)
	if need > math.MaxInt64-margin {
		return math.MaxInt64
	}
	return need + margin
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
// AuraGo can write there. Besides the lexical check, the sensitive-path check
// runs on the resolved form of the directory (see resolveExistingDir): before
// anything is created on its nearest existing ancestor plus the missing
// components, so no directory is ever created inside a protected tree, and
// again on the directory itself once it exists. A link, junction, Windows 8.3
// short name or ignored trailing dot below an innocent name therefore cannot
// point the edition into a system location.
func prepareDataDir(dir string, sensitive func(string) bool) error {
	if err := checkDataDirShape(dir, sensitive); err != nil {
		return err
	}
	if err := checkResolvedAncestor(dir, sensitive); err != nil {
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

// ResolveDirectory returns the real location of an existing directory as the
// storage directory check resolves it (symbolic links, Windows junctions, 8.3
// short names; see resolveExistingDir). The server resolves AuraGo's data
// directory with it, so its Deps.IsSensitivePath recognises the data
// directory in the form the resolved checks below pass in.
func ResolveDirectory(dir string) (string, error) {
	return resolveExistingDir(dir)
}

// checkResolvedDataDir applies the sensitive-path check to the existing dir
// after resolveExistingDir. Without a sensitive-path check there is nothing
// to repeat.
func checkResolvedDataDir(dir string, sensitive func(string) bool) error {
	if sensitive == nil {
		return nil
	}
	resolved, err := resolveExistingDir(dir)
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %v", ErrDataDirInvalid, dir, err)
	}
	return checkResolvedPath(dir, resolved, sensitive)
}

// checkResolvedAncestor applies the sensitive-path check to dir as it will
// resolve once created: its nearest existing ancestor, resolved, joined with
// the components that do not exist yet.
func checkResolvedAncestor(dir string, sensitive func(string) bool) error {
	if sensitive == nil {
		return nil
	}
	existing, missing, err := nearestExistingAncestor(filepath.Clean(dir))
	if err != nil {
		return fmt.Errorf("%w: inspect %s: %v", ErrDataDirInvalid, dir, err)
	}
	resolved, err := resolveExistingDir(existing)
	if err != nil {
		return fmt.Errorf("%w: resolve %s: %v", ErrDataDirInvalid, existing, err)
	}
	return checkResolvedPath(dir, filepath.Join(append([]string{resolved}, missing...)...), sensitive)
}

// nearestExistingAncestor returns the longest existing prefix of the cleaned
// absolute path dir (dir itself when it exists) and the components below it.
// It walks up only past paths that do not exist (or name a file as a
// directory); any other error, such as a denied permission, is returned
// instead of guessing.
func nearestExistingAncestor(dir string) (string, []string, error) {
	var missing []string
	current := dir
	for {
		_, err := os.Stat(current)
		if err == nil {
			return current, missing, nil
		}
		if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
			return "", nil, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current, missing, nil
		}
		missing = append([]string{filepath.Base(current)}, missing...)
		current = parent
	}
}

// checkResolvedPath runs the sensitive-path check on the resolved form of
// lexical. A path that resolves to itself was already checked lexically, and
// so was one whose only difference is an operating system alias (see
// isSystemPathAlias), which keeps legitimate macOS directories under /var and
// /tmp (they resolve below /private, a protected tree) usable.
func checkResolvedPath(lexical, resolved string, sensitive func(string) bool) error {
	if strings.EqualFold(filepath.Clean(resolved), filepath.Clean(lexical)) || isSystemPathAlias(lexical, resolved) {
		return nil
	}
	if sensitive(resolved) {
		return fmt.Errorf("%w: %s resolves to %s, a protected system location", ErrDataDirInvalid, lexical, resolved)
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
