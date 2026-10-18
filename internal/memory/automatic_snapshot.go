package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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

type automaticSnapshotManifest struct {
	Kind        string            `json:"kind"`
	Consistency string            `json:"consistency,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Files       map[string]string `json:"files"`
}

// CreateAutomaticSnapshot saves the configured memory databases before maintenance.
// Only this verified complete series participates in 30-day/minimum-three retention.
func CreateAutomaticSnapshot(ctx context.Context, cfg *config.Config, now time.Time) error {
	if cfg == nil || cfg.Directories.DataDir == "" {
		return nil
	}
	sources := map[string]string{}
	for name, path := range map[string]string{"short_term.sqlite": cfg.SQLite.ShortTermPath, "knowledge_graph.sqlite": cfg.SQLite.KnowledgeGraphPath, "documents.sqlite": config.DocumentCorpusPath(cfg)} {
		if path != "" {
			sources[name] = path
		}
	}
	if len(sources) < 3 {
		return nil
	}
	root := filepath.Join(cfg.Directories.DataDir, "memory_backups")
	if err := rejectSnapshotSymlinks(root); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return fmt.Errorf("create automatic backup directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(root); err != nil {
		return fmt.Errorf("protect automatic backup directory: %w", err)
	}
	snapshots, err := verifiedAutomaticSnapshots(root)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		complete := len(snapshot.manifest.Files) == len(sources)
		for name := range sources {
			if _, ok := snapshot.manifest.Files[name]; !ok {
				complete = false
			}
		}
		if complete && snapshot.manifest.CreatedAt.In(now.Location()).Format("2006-01-02") == now.Format("2006-01-02") {
			return retainAutomaticSnapshots(root, snapshots, now)
		}
	}
	dir := filepath.Join(root, "auto-"+now.UTC().Format("20060102T150405Z")+"-"+rand.Text())
	if err := os.Mkdir(dir, 0700); err != nil {
		return fmt.Errorf("create automatic snapshot: %w", err)
	}
	if err := fileutil.RestrictToOwner(dir); err != nil {
		return err
	}
	manifest := automaticSnapshotManifest{Kind: "auto-memory-v2", Consistency: memorySnapshotConsistency, CreatedAt: now.UTC(), Files: map[string]string{}}
	destinations := make(map[string]string, len(sources))
	for name := range sources {
		destinations[name] = filepath.Join(dir, name)
	}
	if err := CaptureMemorySnapshots(ctx, cfg, destinations); err != nil {
		return err
	}
	for name := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		dst := filepath.Join(dir, name)
		check, err := sql.Open("sqlite", snapshotSQLiteURI(dst, "mode=ro&immutable=1"))
		if err != nil {
			return err
		}
		var result string
		err = check.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result)
		check.Close()
		if err != nil || result != "ok" {
			return fmt.Errorf("automatic snapshot integrity check: %s (%v)", result, err)
		}
		digest, err := automaticSnapshotDigest(dst)
		if err != nil {
			return err
		}
		manifest.Files[name] = digest
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fileutil.WriteFileContext(ctx, filepath.Join(dir, "complete.json"), data, 0600); err != nil {
		return fmt.Errorf("publish complete automatic snapshot: %w", err)
	}
	snapshots, err = verifiedAutomaticSnapshots(root)
	if err != nil {
		return err
	}
	return retainAutomaticSnapshots(root, snapshots, now)
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

func verifiedAutomaticSnapshots(root string) ([]verifiedAutomaticSnapshot, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var snapshots []verifiedAutomaticSnapshot
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "auto-") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if err := rejectSnapshotSymlinks(dir); err != nil {
			continue
		}
		marker := filepath.Join(dir, "complete.json")
		info, err := os.Lstat(marker)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(marker)
		if err != nil {
			continue
		}
		var manifest automaticSnapshotManifest
		if json.Unmarshal(data, &manifest) != nil || manifest.Kind != "auto-memory-v2" || manifest.Consistency != memorySnapshotConsistency || manifest.CreatedAt.IsZero() || len(manifest.Files) != 3 {
			continue
		}
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		valid := len(files) == len(manifest.Files)+1
		for name, want := range manifest.Files {
			if name != "short_term.sqlite" && name != "knowledge_graph.sqlite" && name != "documents.sqlite" {
				valid = false
				break
			}
			path := filepath.Join(dir, name)
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				valid = false
				break
			}
			digest, err := automaticSnapshotDigest(path)
			if err != nil || digest != want {
				valid = false
				break
			}
		}
		if valid {
			snapshots = append(snapshots, verifiedAutomaticSnapshot{entry.Name(), manifest})
		}
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].manifest.CreatedAt.After(snapshots[j].manifest.CreatedAt) })
	return snapshots, nil
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
	cutoff := now.AddDate(0, 0, -30)
	for i, snapshot := range snapshots {
		if i < 3 || !snapshot.manifest.CreatedAt.Before(cutoff) {
			continue
		}
		dir := filepath.Join(root, snapshot.name)
		if err := rejectSnapshotSymlinks(dir); err != nil {
			return err
		}
		current, err := verifiedAutomaticSnapshots(root)
		if err != nil {
			return err
		}
		unchanged := false
		for _, candidate := range current {
			if candidate.name == snapshot.name && candidate.manifest.CreatedAt.Equal(snapshot.manifest.CreatedAt) && maps.Equal(candidate.manifest.Files, snapshot.manifest.Files) {
				unchanged = true
				break
			}
		}
		if !unchanged {
			continue
		}
		// Remove only the verified known files; unknown files and incomplete runs survive.
		for name := range snapshot.manifest.Files {
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
