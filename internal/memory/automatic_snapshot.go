package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
)

var ErrAutomaticSnapshotIncompleteConfig = errors.New("automatic snapshot requires all persistent memory stores")

type automaticSnapshotManifest struct {
	Kind        string            `json:"kind"`
	Consistency string            `json:"consistency,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Files       map[string]string `json:"files"`
}

// CreateAutomaticSnapshot saves the configured memory databases before maintenance.
// Only this verified complete series participates in 30-day/minimum-three retention.
func CreateAutomaticSnapshot(ctx context.Context, cfg *config.Config, now time.Time) error {
	_, _, err := CreateAutomaticSnapshotWithStatus(ctx, cfg, now)
	return err
}

// CreateAutomaticSnapshotWithStatus creates or reuses today's snapshot and reports
// whether automatic snapshots are configured and verified for the current day.
func CreateAutomaticSnapshotWithStatus(ctx context.Context, cfg *config.Config, now time.Time) (configured, haveToday bool, resultErr error) {
	sources := automaticSnapshotSources(cfg)
	if sources == nil {
		return false, false, nil
	}
	if len(sources) != 3 || cfg.Directories.DataDir == "" {
		return true, false, ErrAutomaticSnapshotIncompleteConfig
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	if err := rejectSnapshotSymlinks(root); err != nil {
		return true, false, err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return true, false, fmt.Errorf("create automatic backup directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(root); err != nil {
		return true, false, fmt.Errorf("protect automatic backup directory: %w", err)
	}
	var reads automaticSnapshotReadCounts
	snapshots, err := verifiedAutomaticSnapshotsWithReads(root, &reads)
	if err != nil {
		return true, false, err
	}
	for _, snapshot := range snapshots {
		if snapshot.manifest.CreatedAt.In(now.Location()).Format("2006-01-02") == now.Format("2006-01-02") {
			if err := retainAutomaticSnapshotsWithReads(root, snapshots, now, &reads); err != nil {
				return true, true, err
			}
			return true, true, nil
		}
	}
	dir := filepath.Join(root, "auto-"+now.UTC().Format("20060102T150405Z")+"-"+rand.Text())
	if err := os.Mkdir(dir, 0700); err != nil {
		return true, false, fmt.Errorf("create automatic snapshot: %w", err)
	}
	published := false
	defer func() {
		// Only this attempt's newly created, unpublished directory is ours to clean.
		if published || resultErr == nil {
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove partial automatic snapshot: %w", err))
		}
	}()
	if err := fileutil.RestrictToOwner(dir); err != nil {
		return true, false, err
	}
	manifest := automaticSnapshotManifest{Kind: "auto-memory-v2", Consistency: memorySnapshotConsistency, CreatedAt: now.UTC(), Files: map[string]string{}}
	destinations := make(map[string]string, len(sources))
	for name := range sources {
		destinations[name] = filepath.Join(dir, name)
	}
	if err := CaptureMemorySnapshots(ctx, cfg, destinations); err != nil {
		return true, false, err
	}
	for name := range sources {
		if err := ctx.Err(); err != nil {
			return true, false, err
		}
		dst := filepath.Join(dir, name)
		check, err := sql.Open("sqlite", snapshotSQLiteURI(dst, "mode=ro&immutable=1"))
		if err != nil {
			return true, false, err
		}
		var result string
		err = check.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result)
		check.Close()
		if err != nil || result != "ok" {
			return true, false, fmt.Errorf("automatic snapshot integrity check: %s (%v)", result, err)
		}
		digest, err := automaticSnapshotDigest(dst)
		if err != nil {
			return true, false, err
		}
		manifest.Files[name] = digest
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return true, false, err
	}
	if err := ctx.Err(); err != nil {
		return true, false, err
	}
	if err := fileutil.WriteFileContext(ctx, filepath.Join(dir, "complete.json"), data, 0600); err != nil {
		return true, false, fmt.Errorf("publish complete automatic snapshot: %w", err)
	}
	created, valid, err := verifyAutomaticSnapshot(root, filepath.Base(dir), &reads)
	if err != nil || !valid {
		if err == nil {
			err = errors.New("published automatic snapshot failed verification")
		}
		return true, false, err
	}
	published = true
	snapshots = append(snapshots, created)
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].manifest.CreatedAt.After(snapshots[j].manifest.CreatedAt) })
	if err := retainAutomaticSnapshotsWithReads(root, snapshots, now, &reads); err != nil {
		return true, true, err
	}
	return true, true, nil
}

// automaticSnapshotSources returns the databases an automatic snapshot covers,
// or nil when automatic snapshots are deliberately not configured.
func automaticSnapshotSources(cfg *config.Config) map[string]string {
	if cfg == nil {
		return nil
	}
	sources := map[string]string{}
	for name, path := range map[string]string{"short_term.sqlite": cfg.SQLite.ShortTermPath, "knowledge_graph.sqlite": cfg.SQLite.KnowledgeGraphPath, "documents.sqlite": config.DocumentCorpusPath(cfg)} {
		if path != "" {
			sources[name] = path
		}
	}
	if len(sources) == 0 {
		return nil
	}
	return sources
}

// AutomaticSnapshotStatus reports whether automatic snapshots are configured
// and, if so, whether a verified complete snapshot from the day of now exists.
// Unconfigured means CreateAutomaticSnapshot is a deliberate no-op.
func AutomaticSnapshotStatus(cfg *config.Config, now time.Time) (configured, haveToday bool, err error) {
	sources := automaticSnapshotSources(cfg)
	if sources == nil {
		return false, false, nil
	}
	if len(sources) != 3 || cfg.Directories.DataDir == "" {
		return true, false, ErrAutomaticSnapshotIncompleteConfig
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	if err := rejectSnapshotSymlinks(root); err != nil {
		return true, false, err
	}
	if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
		return true, false, nil
	}
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil {
		return true, false, err
	}
	for _, snapshot := range snapshots {
		if snapshot.manifest.CreatedAt.In(now.Location()).Format("2006-01-02") == now.Format("2006-01-02") {
			return true, true, nil
		}
	}
	return true, false, nil
}

func rejectSnapshotSymlinks(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect snapshot path: %w", err)
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot path contains a symlink")
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	return nil
}

type verifiedAutomaticSnapshot struct {
	name     string
	manifest automaticSnapshotManifest
}

type automaticSnapshotReadCounts struct {
	ManifestReads int
	SnapshotFiles int
}

func verifiedAutomaticSnapshots(root string) ([]verifiedAutomaticSnapshot, error) {
	return verifiedAutomaticSnapshotsWithReads(root, nil)
}

func verifiedAutomaticSnapshotsWithReads(root string, reads *automaticSnapshotReadCounts) ([]verifiedAutomaticSnapshot, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var snapshots []verifiedAutomaticSnapshot
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "auto-") {
			continue
		}
		snapshot, valid, err := verifyAutomaticSnapshot(root, entry.Name(), reads)
		if err != nil {
			return nil, err
		}
		if valid {
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].manifest.CreatedAt.After(snapshots[j].manifest.CreatedAt) })
	return snapshots, nil
}

func verifyAutomaticSnapshot(root, name string, reads *automaticSnapshotReadCounts) (verifiedAutomaticSnapshot, bool, error) {
	if !strings.HasPrefix(name, "auto-") {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	dir := filepath.Join(root, name)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	if err := rejectSnapshotSymlinks(dir); err != nil {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	marker := filepath.Join(dir, "complete.json")
	info, err = os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	if reads != nil {
		reads.ManifestReads++
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	var manifest automaticSnapshotManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.Kind != "auto-memory-v2" || manifest.Consistency != memorySnapshotConsistency || manifest.CreatedAt.IsZero() || len(manifest.Files) != 3 {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	expected := []string{"complete.json", "documents.sqlite", "knowledge_graph.sqlite", "short_term.sqlite"}
	files, err := os.ReadDir(dir)
	if err != nil {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	if len(files) != len(expected) {
		return verifiedAutomaticSnapshot{}, false, nil
	}
	for i, file := range files {
		if file.Name() != expected[i] {
			return verifiedAutomaticSnapshot{}, false, nil
		}
	}
	for _, name := range expected[1:] {
		want, ok := manifest.Files[name]
		if !ok {
			return verifiedAutomaticSnapshot{}, false, nil
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return verifiedAutomaticSnapshot{}, false, nil
		}
		if reads != nil {
			reads.SnapshotFiles++
		}
		digest, err := automaticSnapshotDigest(path)
		if err != nil || digest != want {
			return verifiedAutomaticSnapshot{}, false, nil
		}
	}
	return verifiedAutomaticSnapshot{name, manifest}, true, nil
}

func automaticSnapshotDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func retainAutomaticSnapshots(root string, snapshots []verifiedAutomaticSnapshot, now time.Time) error {
	return retainAutomaticSnapshotsWithReads(root, snapshots, now, nil)
}

func retainAutomaticSnapshotsWithReads(root string, snapshots []verifiedAutomaticSnapshot, now time.Time, reads *automaticSnapshotReadCounts) error {
	cutoff := now.AddDate(0, 0, -30)
	for i, snapshot := range snapshots {
		if i < 3 || !snapshot.manifest.CreatedAt.Before(cutoff) {
			continue
		}
		dir := filepath.Join(root, snapshot.name)
		if err := rejectSnapshotSymlinks(dir); err != nil {
			return err
		}
		current, valid, err := verifyAutomaticSnapshot(root, snapshot.name, reads)
		if err != nil {
			return err
		}
		if !valid || !current.manifest.CreatedAt.Equal(snapshot.manifest.CreatedAt) || !maps.Equal(current.manifest.Files, snapshot.manifest.Files) {
			continue
		}
		// Remove only the verified known files; unknown files and incomplete runs survive.
		for _, name := range []string{"short_term.sqlite", "knowledge_graph.sqlite", "documents.sqlite"} {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("remove expired automatic snapshot file: %w", err)
			}
		}
		if err := os.Remove(filepath.Join(dir, "complete.json")); err != nil {
			return err
		}
		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("remove expired automatic snapshot directory: %w", err)
		}
	}
	return nil
}
