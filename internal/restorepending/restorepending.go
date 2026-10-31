// Package restorepending implements the deferred ("apply at next start") system
// restore. The import handler validates a backup and moves the staged files into
// a single pending directory; the next process start publishes them before any
// database is opened, with a rollback copy of everything that is replaced.
package restorepending

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

const (
	pendingDirName  = ".restore-pending"
	rollbackDirName = ".restore-rollback"
	lastFileName    = ".restore-last.json"
	manifestName    = "manifest.json"
	filesDirName    = "files"
	recordsName     = "records.json"

	StatusApplied    = "applied"
	StatusRolledBack = "rolled_back"
	StatusFailed     = "failed"
)

// Entry describes one destination of the pending restore.
type Entry struct {
	// Rel is the slash-separated path of the staged file below the files tree.
	Rel string `json:"rel"`
	// Dest is the absolute destination path.
	Dest   string `json:"dest"`
	Dir    bool   `json:"dir,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
	SQLite bool   `json:"sqlite,omitempty"`
}

// Manifest is stored next to the staged files.
type Manifest struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"created_at"`
	Entries   []Entry `json:"entries"`
}

// Result is the outcome of the last apply attempt.
type Result struct {
	ID        string `json:"id"`
	AppliedAt string `json:"applied_at"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// State is what the status endpoint reports.
type State struct {
	Pending          bool
	PendingCreatedAt string
	Last             *Result
}

// Options tune Apply.
type Options struct {
	Logger *slog.Logger
	// Validate runs after publishing when config.yaml was replaced; an error
	// triggers a rollback.
	Validate func() error
	// afterPublish is a test hook; an error simulates a failure mid-way.
	afterPublish func(index int, e Entry) error
}

// ConfigRoot normalizes a config file path to the instance root directory.
func ConfigRoot(configPath string) (string, error) {
	abs, err := filepath.Abs(filepath.Dir(configPath))
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

func dataDir(root string) string    { return filepath.Join(root, "data") }
func pendingDir(root string) string { return filepath.Join(dataDir(root), pendingDirName) }
func lastPath(root string) string   { return filepath.Join(dataDir(root), lastFileName) }

// IsReservedPath reports whether a restore-relative path points into the
// bookkeeping directories of this package (they must never be restored).
func IsReservedPath(clean string) bool {
	slash := filepath.ToSlash(filepath.Clean(clean))
	return strings.HasPrefix(slash, "data/.restore-")
}

// StageFilePath returns where a staged file with the given relative path lives
// inside a staging tree that is later passed to Stage.
func StageFilePath(stagingRoot, rel string) string {
	return filepath.Join(stagingRoot, filepath.FromSlash(rel))
}

func newID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(b[:])
}

// Stage moves a validated staging tree into the pending location, replacing any
// older pending restore. stagingRoot must be on the same volume as the config
// root and is consumed (renamed) on success.
func Stage(configRoot, stagingRoot string, entries []Entry) (string, error) {
	if err := os.MkdirAll(dataDir(configRoot), 0o750); err != nil {
		return "", err
	}
	id := newID()
	next, err := os.MkdirTemp(dataDir(configRoot), pendingDirName+"-new-")
	if err != nil {
		return "", err
	}
	cleanupNext := true
	defer func() {
		if cleanupNext {
			_ = os.RemoveAll(next)
		}
	}()
	if err := fileutil.RestrictToOwner(next); err != nil {
		return "", err
	}
	if err := fileutil.Rename(stagingRoot, filepath.Join(next, filesDirName)); err != nil {
		return "", fmt.Errorf("move staging tree: %w", err)
	}
	m := Manifest{ID: id, CreatedAt: time.Now().UTC().Format(time.RFC3339), Entries: entries}
	if err := writeJSON(filepath.Join(next, manifestName), m); err != nil {
		return "", err
	}
	// Replace an older pending restore: move it aside first so the swap leaves
	// no window in which a half-deleted tree is visible under the pending name.
	pending := pendingDir(configRoot)
	var trash string
	if _, err := os.Lstat(pending); err == nil {
		trash = pending + "-old-" + id
		if err := fileutil.Rename(pending, trash); err != nil {
			return "", fmt.Errorf("replace older pending restore: %w", err)
		}
	}
	if err := fileutil.Rename(next, pending); err != nil {
		if trash != "" {
			_ = fileutil.Rename(trash, pending)
		}
		return "", err
	}
	cleanupNext = false
	if trash != "" {
		_ = os.RemoveAll(trash)
	}
	return id, nil
}

// StagedPath returns the location of a staged file inside the pending tree
// (useful to tests).
func StagedPath(configRoot, rel string) string {
	return filepath.Join(pendingDir(configRoot), filesDirName, filepath.FromSlash(rel))
}

// ReadState reports whether a restore is pending and what the last result was.
func ReadState(configRoot string) State {
	var st State
	var m Manifest
	if err := readJSON(filepath.Join(pendingDir(configRoot), manifestName), &m); err == nil {
		st.Pending = true
		st.PendingCreatedAt = m.CreatedAt
	}
	var r Result
	if err := readJSON(lastPath(configRoot), &r); err == nil {
		st.Last = &r
	}
	return st
}

type record struct {
	Dest    string `json:"dest"`
	Backup  string `json:"backup,omitempty"`
	Existed bool   `json:"existed"`
	Mode    uint32 `json:"mode,omitempty"`
}

// Apply publishes a pending restore, if any. It returns nil, nil when nothing
// is pending. On failure everything replaced so far is restored from the
// rollback directory and a Result with a non-applied status is returned
// together with the error.
func Apply(configRoot string, opts Options) (*Result, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	pending := pendingDir(configRoot)
	var m Manifest
	if err := readJSON(filepath.Join(pending, manifestName), &m); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		res := finish(configRoot, pending, Result{ID: "unknown", Status: StatusFailed, Message: "pending restore manifest unreadable: " + err.Error()})
		log.Error("[Restore] Pending restore manifest unreadable, discarding", "error", err)
		return res, err
	}
	rollbackRoot := filepath.Join(dataDir(configRoot), rollbackDirName)
	rbDir := filepath.Join(rollbackRoot, m.ID)

	records, err := backupExisting(rbDir, m)
	if err == nil {
		err = publish(configRoot, pending, m, records, opts)
	}
	if err != nil {
		log.Error("[Restore] Applying pending restore failed, rolling back", "id", m.ID, "error", err)
		if rbErr := rollback(records); rbErr != nil {
			log.Error("[Restore] Rollback incomplete", "id", m.ID, "error", rbErr)
			res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusFailed,
				Message: fmt.Sprintf("restore failed (%v) and rollback was incomplete (%v); rollback copies are in %s", err, rbErr, rbDir)})
			return res, err
		}
		res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusRolledBack,
			Message: "restore failed and was rolled back: " + err.Error()})
		return res, err
	}
	res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusApplied,
		Message: fmt.Sprintf("%d entries restored", len(m.Entries))})
	pruneRollbacks(rollbackRoot, m.ID)
	log.Info("[Restore] Pending restore applied", "id", m.ID, "entries", len(m.Entries))
	return res, nil
}

// finish records the result and removes the pending directory so a broken
// restore is never retried in a loop.
func finish(configRoot, pending string, r Result) *Result {
	r.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	_ = writeJSON(lastPath(configRoot), r)
	_ = os.RemoveAll(pending)
	return &r
}

func sidecars(dest string) []string { return []string{dest + "-wal", dest + "-shm"} }

// backupExisting copies every file that will be replaced (and the SQLite
// sidecars next to replaced databases) into the rollback directory.
func backupExisting(rbDir string, m Manifest) ([]record, error) {
	if err := os.RemoveAll(rbDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(rbDir, 0o700); err != nil {
		return nil, err
	}
	var records []record
	seen := map[string]bool{}
	add := func(dest string) error {
		if seen[dest] {
			return nil
		}
		seen[dest] = true
		rec := record{Dest: dest}
		info, err := os.Lstat(dest)
		switch {
		case err == nil && info.Mode().IsRegular():
			rec.Existed = true
			rec.Mode = uint32(info.Mode().Perm())
			rec.Backup = filepath.Join(rbDir, fmt.Sprintf("%04d.bak", len(records)))
			if err := copyFile(dest, rec.Backup, 0o600); err != nil {
				return fmt.Errorf("back up %s: %w", dest, err)
			}
		case err == nil:
			return fmt.Errorf("destination %s is not a regular file", dest)
		case !errors.Is(err, os.ErrNotExist):
			return err
		}
		records = append(records, rec)
		return nil
	}
	for _, e := range m.Entries {
		if e.Dir {
			continue
		}
		if !filepath.IsAbs(e.Dest) {
			return records, fmt.Errorf("destination %q is not absolute", e.Dest)
		}
		if err := add(e.Dest); err != nil {
			return records, err
		}
		if e.SQLite {
			for _, sc := range sidecars(e.Dest) {
				if err := add(sc); err != nil {
					return records, err
				}
			}
		}
	}
	return records, writeJSON(filepath.Join(rbDir, recordsName), records)
}

func publish(configRoot, pending string, m Manifest, records []record, opts Options) error {
	files := filepath.Join(pending, filesDirName)
	configReplaced := false
	for i, e := range m.Entries {
		if e.Dir {
			if err := os.MkdirAll(e.Dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if e.SQLite {
			for _, sc := range sidecars(e.Dest) {
				if err := os.Remove(sc); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove stale %s: %w", sc, err)
				}
			}
		}
		perm := os.FileMode(e.Mode)
		if e.SQLite {
			perm = 0o600
		}
		if perm == 0 {
			perm = 0o644
		}
		if err := copyFile(filepath.Join(files, filepath.FromSlash(e.Rel)), e.Dest, perm); err != nil {
			return fmt.Errorf("publish %s: %w", e.Rel, err)
		}
		if e.Rel == "config.yaml" {
			configReplaced = true
		}
		if opts.afterPublish != nil {
			if err := opts.afterPublish(i, e); err != nil {
				return err
			}
		}
	}
	if configReplaced && opts.Validate != nil {
		if err := opts.Validate(); err != nil {
			return fmt.Errorf("restored config.yaml is not loadable: %w", err)
		}
	}
	return nil
}

func rollback(records []record) error {
	var errs []error
	for i := len(records) - 1; i >= 0; i-- {
		rec := records[i]
		if rec.Existed {
			if err := copyFile(rec.Backup, rec.Dest, os.FileMode(rec.Mode)); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.Remove(rec.Dest); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func pruneRollbacks(rollbackRoot, keep string) {
	entries, err := os.ReadDir(rollbackRoot)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Name() != keep {
			_ = os.RemoveAll(filepath.Join(rollbackRoot, e.Name()))
		}
	}
}

// copyFile copies src to dest through a temp file in the destination directory.
func copyFile(src, dest string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".ago-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if perm.Perm()&0o077 == 0 {
		if err := fileutil.RestrictToOwner(tmpName); err != nil {
			tmp.Close()
			return err
		}
	}
	in, err := os.Open(src)
	if err != nil {
		tmp.Close()
		return err
	}
	_, copyErr := io.Copy(tmp, in)
	in.Close()
	if copyErr != nil {
		tmp.Close()
		return copyErr
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return fileutil.Rename(tmpName, dest)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return fileutil.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
