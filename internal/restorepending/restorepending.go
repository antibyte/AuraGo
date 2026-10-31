// Package restorepending implements the deferred ("apply at next start") system
// restore. The import handler validates a backup and moves the staged files into
// a single pending directory; the next process start publishes them before any
// database is opened, with a rollback copy of everything that is replaced.
//
// The pending directory is NOT trusted: Apply recomputes every destination from
// the current instance root, re-runs the restore allowlist and refuses symlinked
// parents or escaping paths.
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
	"path"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

const (
	pendingDirName  = ".restore-pending"
	rollbackDirName = ".restore-rollback"
	lastFileName    = ".restore-last.json"
	manifestName    = "manifest.json"
	filesDirName    = "files"
	recordsName     = "records.json"
	markerName      = "rollback-complete"

	// StagedVaultName is the file name (inside the staging root) of the
	// re-encrypted vault secrets that are merged after the next start.
	StagedVaultName = ".vault.pending.enc"
	vaultAppliedPfx = ".restore-vault-"

	vectorPrefix = "data/vectordb/"

	StatusApplied    = "applied"
	StatusRolledBack = "rolled_back"
	StatusFailed     = "failed"
)

// Entry describes one destination of the pending restore. Destinations are
// derived from Clean at apply time; nothing absolute is stored.
type Entry struct {
	// Clean is the slash-separated archive-relative path (also the location of
	// the staged file below the files tree).
	Clean  string `json:"clean"`
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
	// ResolveVectorRoot returns the configured vector database directory (empty:
	// use the config root). It is only called when data/vectordb/ entries exist.
	ResolveVectorRoot func() (string, error)
	// afterPublish is a test hook; an error simulates a failure mid-way.
	afterPublish func(index int, e Entry) error
	// simulateCrash makes Apply return right after the given number of published
	// entries without rolling back or finishing (test hook).
	simulateCrash int
}

var errSimulatedCrash = errors.New("simulated crash")

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

// IsAllowedRestorePath is the allowlist of archive-relative paths a restore may
// write.
func IsAllowedRestorePath(clean string) bool {
	slash := filepath.ToSlash(filepath.Clean(clean))
	switch slash {
	case "config.yaml", "data", "prompts", "agent_workspace", "agent_workspace/skills", "agent_workspace/agent_skills", "agent_workspace/tools", "agent_workspace/workdir":
		return true
	}
	for _, prefix := range []string{
		"data/",
		"prompts/",
		"agent_workspace/skills/",
		"agent_workspace/agent_skills/",
		"agent_workspace/tools/",
		"agent_workspace/workdir/",
	} {
		if strings.HasPrefix(slash, prefix) {
			return true
		}
	}
	return false
}

// IsReservedPath reports whether a restore-relative path points into the
// bookkeeping files of this package (they must never be restored).
func IsReservedPath(clean string) bool {
	slash := filepath.ToSlash(filepath.Clean(clean))
	return strings.HasPrefix(slash, "data/.restore-")
}

// StageFilePath returns where a staged file with the given relative path lives
// inside a staging tree that is later passed to Stage.
func StageFilePath(stagingRoot, rel string) string {
	return filepath.Join(stagingRoot, filepath.FromSlash(rel))
}

// StagedVaultPath is where the import handler writes the staged vault blob.
func StagedVaultPath(stagingRoot string) string {
	return filepath.Join(stagingRoot, StagedVaultName)
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

// Discard removes a pending restore (including a staged vault). It reports
// whether something was pending.
func Discard(configRoot string) (bool, error) {
	pending := pendingDir(configRoot)
	if _, err := os.Lstat(pending); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := removePending(pending, newID()); err != nil {
		return false, err
	}
	return true, nil
}

// removePending renames first so the manifest never outlives its files.
func removePending(pending, id string) error {
	gone := pending + "-done-" + id
	if err := fileutil.Rename(pending, gone); err != nil {
		return err
	}
	return os.RemoveAll(gone)
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

type target struct {
	Entry
	src  string
	dest string
}

// resolve validates the whole manifest against the current instance and
// computes the destinations. Any violation rejects the entire restore.
func resolve(configRoot, pending string, m Manifest, opts Options) ([]target, error) {
	files := filepath.Join(pending, filesDirName)
	vectorRoot := ""
	vectorResolved := false
	targets := make([]target, 0, len(m.Entries))
	for _, e := range m.Entries {
		clean := path.Clean(strings.ReplaceAll(e.Clean, "\\", "/"))
		if e.Clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || filepath.IsAbs(filepath.FromSlash(clean)) || filepath.VolumeName(filepath.FromSlash(clean)) != "" {
			return nil, fmt.Errorf("unsafe path %q", e.Clean)
		}
		if !IsAllowedRestorePath(clean) {
			return nil, fmt.Errorf("path %q is not restorable", clean)
		}
		if IsReservedPath(clean) {
			return nil, fmt.Errorf("path %q is reserved", clean)
		}
		root, rel := configRoot, clean
		if strings.HasPrefix(clean, vectorPrefix) && opts.ResolveVectorRoot != nil {
			if !vectorResolved {
				v, err := opts.ResolveVectorRoot()
				if err != nil {
					return nil, fmt.Errorf("resolve vector database directory: %w", err)
				}
				if v != "" {
					if v, err = filepath.Abs(v); err != nil {
						return nil, err
					}
				}
				vectorRoot, vectorResolved = v, true
			}
			if vectorRoot != "" {
				root, rel = vectorRoot, strings.TrimPrefix(clean, vectorPrefix)
			}
		}
		dest := filepath.Join(root, filepath.FromSlash(rel))
		if !within(root, dest) {
			return nil, fmt.Errorf("destination of %q escapes the restore root", clean)
		}
		if err := checkNoSymlinks(root, dest, root != configRoot); err != nil {
			return nil, fmt.Errorf("destination of %q: %w", clean, err)
		}
		src := filepath.Join(files, filepath.FromSlash(clean))
		if !within(files, src) {
			return nil, fmt.Errorf("staged file of %q escapes the pending tree", clean)
		}
		if !e.Dir {
			info, err := os.Lstat(src)
			if err != nil || !info.Mode().IsRegular() {
				return nil, fmt.Errorf("staged file for %q is missing or not a regular file", clean)
			}
			if err := checkNoSymlinks(files, src, false); err != nil {
				return nil, fmt.Errorf("staged file of %q: %w", clean, err)
			}
		}
		e.Clean = clean
		targets = append(targets, target{Entry: e, src: src, dest: dest})
	}
	return targets, nil
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

// checkNoSymlinks rejects symlinks on the way from root to p (existing
// components only). checkRoot also applies it to root itself.
func checkNoSymlinks(root, p string, checkRoot bool) error {
	if checkRoot {
		if info, err := os.Lstat(root); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", root)
		}
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return err
	}
	cur := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", cur)
		}
	}
	return nil
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
	cleanupLeftovers(configRoot)
	pending := pendingDir(configRoot)
	var m Manifest
	if err := readJSON(filepath.Join(pending, manifestName), &m); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		res := finish(configRoot, pending, Result{ID: "unknown", Status: StatusFailed, Message: "Die vorgemerkte Wiederherstellung war unlesbar und wurde verworfen. Es wurde nichts übernommen (auch keine Vault-Secrets)."})
		log.Error("[Restore] Pending restore manifest unreadable, discarding", "error", err)
		return res, err
	}
	targets, err := resolve(configRoot, pending, m, opts)
	if err != nil {
		log.Error("[Restore] Pending restore rejected", "id", m.ID, "error", err)
		res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusFailed,
			Message: "Die vorgemerkte Wiederherstellung wurde abgelehnt: " + err.Error() + ". Es wurde nichts übernommen (auch keine Vault-Secrets)."})
		return res, err
	}
	rollbackRoot := filepath.Join(dataDir(configRoot), rollbackDirName)
	rbDir := filepath.Join(rollbackRoot, m.ID)

	records, err := loadJournal(rbDir)
	if err != nil {
		// No sealed journal: (re)build it. Nothing has been replaced yet,
		// because publishing only starts once the journal is sealed.
		records, err = backupExisting(rbDir, targets)
	}
	if err == nil {
		err = publish(targets, opts)
	}
	if errors.Is(err, errSimulatedCrash) {
		return nil, err
	}
	if err != nil {
		log.Error("[Restore] Applying pending restore failed, rolling back", "id", m.ID, "error", err)
		if rbErr := rollback(records); rbErr != nil {
			log.Error("[Restore] Rollback incomplete", "id", m.ID, "error", rbErr)
			res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusFailed,
				Message: fmt.Sprintf("Die Wiederherstellung ist fehlgeschlagen (%v) und das Zurückrollen war unvollständig (%v); Sicherungskopien liegen in %s. Vault-Secrets wurden nicht übernommen.", err, rbErr, rbDir)})
			return res, err
		}
		res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusRolledBack,
			Message: "Die Wiederherstellung ist fehlgeschlagen und wurde zurückgerollt: " + err.Error() + ". Es wurde nichts übernommen (auch keine Vault-Secrets)."})
		return res, err
	}
	// Hand the staged vault over to ApplyStagedVault before the pending tree goes.
	staged := filepath.Join(pending, filesDirName, StagedVaultName)
	if _, statErr := os.Lstat(staged); statErr == nil {
		if mvErr := fileutil.Rename(staged, filepath.Join(dataDir(configRoot), vaultAppliedPfx+m.ID+".enc")); mvErr != nil {
			log.Error("[Restore] Could not keep staged vault secrets", "id", m.ID, "error", mvErr)
		}
	}
	res := finish(configRoot, pending, Result{ID: m.ID, Status: StatusApplied,
		Message: fmt.Sprintf("%d Einträge wiederhergestellt.", len(m.Entries))})
	pruneRollbacks(rollbackRoot, m.ID)
	log.Info("[Restore] Pending restore applied", "id", m.ID, "entries", len(m.Entries))
	return res, nil
}

// ApplyStagedVault merges staged vault secrets into the (already opened) main
// vault, but only when the files of the same restore were applied. Otherwise
// the staged secrets are discarded.
func ApplyStagedVault(configRoot string, vault *security.Vault, log *slog.Logger) {
	if log == nil {
		log = slog.Default()
	}
	matches, _ := filepath.Glob(filepath.Join(dataDir(configRoot), vaultAppliedPfx+"*.enc"))
	for _, file := range matches {
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), vaultAppliedPfx), ".enc")
		var last Result
		lastErr := readJSON(lastPath(configRoot), &last)
		if lastErr != nil || last.ID != id || last.Status != StatusApplied || vault == nil {
			log.Warn("[Restore] Discarding staged vault secrets", "id", id)
			_ = os.Remove(file)
			continue
		}
		note := ""
		count, err := restoreStagedVault(vault, file)
		if err != nil {
			log.Error("[Restore] Staged vault secrets could not be applied", "id", id, "error", err)
			note = " Vault-Secrets konnten nicht übernommen werden: " + err.Error()
		} else {
			log.Info("[Restore] Staged vault secrets applied", "id", id, "count", count)
			note = fmt.Sprintf(" %d Vault-Secrets übernommen.", count)
		}
		last.Message += note
		_ = writeJSON(lastPath(configRoot), last)
		_ = os.Remove(file)
	}
}

func restoreStagedVault(vault *security.Vault, file string) (int, error) {
	enc, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	plain, err := vault.DecryptBytes(enc)
	if err != nil {
		return 0, err
	}
	var snapshot map[string]string
	if err := json.Unmarshal(plain, &snapshot); err != nil {
		return 0, err
	}
	return vault.RestoreBackupSnapshot(snapshot)
}

// cleanupLeftovers removes half-finished swap directories of earlier runs.
func cleanupLeftovers(configRoot string) {
	for _, pat := range []string{"-new-*", "-old-*", "-done-*"} {
		matches, _ := filepath.Glob(filepath.Join(dataDir(configRoot), pendingDirName+pat))
		for _, m := range matches {
			_ = os.RemoveAll(m)
		}
	}
}

// finish records the result and then removes the pending directory (rename
// first, so the manifest never outlives its files) so a broken restore is never
// retried in a loop.
func finish(configRoot, pending string, r Result) *Result {
	r.AppliedAt = time.Now().UTC().Format(time.RFC3339)
	_ = writeJSON(lastPath(configRoot), r)
	if err := removePending(pending, r.ID); err != nil {
		_ = os.RemoveAll(pending)
	}
	return &r
}

func sidecars(dest string) []string { return []string{dest + "-wal", dest + "-shm"} }

// loadJournal returns the records of a sealed rollback journal.
func loadJournal(rbDir string) ([]record, error) {
	if _, err := os.Stat(filepath.Join(rbDir, markerName)); err != nil {
		return nil, err
	}
	var records []record
	if err := readJSON(filepath.Join(rbDir, recordsName), &records); err != nil {
		return nil, err
	}
	return records, nil
}

// backupExisting copies every file that will be replaced (and the SQLite
// sidecars next to replaced databases) into the rollback directory and seals
// the journal with a marker. Only a sealed journal is ever reused.
func backupExisting(rbDir string, targets []target) ([]record, error) {
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
	for _, t := range targets {
		if t.Dir {
			continue
		}
		if err := add(t.dest); err != nil {
			return records, err
		}
		if t.SQLite {
			for _, sc := range sidecars(t.dest) {
				if err := add(sc); err != nil {
					return records, err
				}
			}
		}
	}
	if err := writeJSON(filepath.Join(rbDir, recordsName), records); err != nil {
		return records, err
	}
	if err := writeFileSync(filepath.Join(rbDir, markerName), []byte("ok"), 0o600); err != nil {
		return records, err
	}
	return records, nil
}

func publish(targets []target, opts Options) error {
	configReplaced := false
	for i, t := range targets {
		if t.Dir {
			if err := os.MkdirAll(t.dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if t.SQLite {
			for _, sc := range sidecars(t.dest) {
				if err := os.Remove(sc); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("remove stale %s: %w", sc, err)
				}
			}
		}
		perm := os.FileMode(t.Mode)
		if t.SQLite {
			perm = 0o600
		}
		if perm == 0 {
			perm = 0o644
		}
		if err := copyFile(t.src, t.dest, perm); err != nil {
			return fmt.Errorf("publish %s: %w", t.Clean, err)
		}
		if t.Clean == "config.yaml" {
			configReplaced = true
		}
		if opts.afterPublish != nil {
			if err := opts.afterPublish(i, t.Entry); err != nil {
				return err
			}
		}
		if opts.simulateCrash > 0 && i+1 >= opts.simulateCrash {
			return errSimulatedCrash
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

// writeFileSync writes through a temp file, fsyncs and renames.
func writeFileSync(p string, data []byte, perm os.FileMode) error {
	tmp := p + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return fileutil.Rename(tmp, p)
}

func writeJSON(p string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileSync(p, data, 0o600)
}

func readJSON(p string, v any) error {
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
