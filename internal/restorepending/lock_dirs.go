package restorepending

import (
	"os"
	"path/filepath"
	"sort"
)

// AffectedDataDirectories reads the validated restore destinations without any
// cleanup or writes. The restore state directory is returned even on invalid input.
func AffectedDataDirectories(configRoot string, opts Options) ([]string, error) {
	dirs := []string{dataDir(configRoot)}
	pending := pendingDir(configRoot)
	var m Manifest
	if err := readJSON(filepath.Join(pending, manifestName), &m); err != nil {
		if os.IsNotExist(err) {
			return dirs, nil
		}
		return dirs, err
	}
	if m.AwaitingConfirmation() {
		return dirs, nil
	}
	targets, err := resolve(configRoot, pending, m, opts)
	if err != nil {
		return dirs, err
	}
	seen := map[string]bool{dirs[0]: true}
	add := func(dir string) {
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	for _, t := range targets {
		if t.Dir {
			add(t.dest)
		} else {
			add(filepath.Dir(t.dest))
		}
	}
	// A crash may have published config.yaml already. Rollback still writes the
	// original journal destinations, even if configured paths now differ.
	if records, err := loadJournal(filepath.Join(dataDir(configRoot), rollbackDirName, m.ID)); err == nil {
		for _, r := range records {
			add(filepath.Dir(r.Dest))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}
