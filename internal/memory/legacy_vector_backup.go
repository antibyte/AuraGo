package memory

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"aurago/internal/fileutil"
)

const legacyManifestName = "aurago-legacy-manifest.json"

type legacyFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type legacyManifest struct {
	Version     int          `json:"version"`
	Fingerprint string       `json:"fingerprint"`
	Files       []legacyFile `json:"files"`
}

func inventoryLegacyFiles(dir string) ([]legacyFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []legacyFile
	isGob := func(name string) bool { return strings.HasSuffix(name, ".gob") || strings.HasSuffix(name, ".gob.gz") }
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("legacy vector symlink is unsupported")
		}
		if !entry.IsDir() {
			if isGob(entry.Name()) {
				return nil, fmt.Errorf("orphan legacy document outside a collection")
			}
			continue
		}
		children, err := os.ReadDir(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if !isGob(child.Name()) {
				continue
			}
			rel := entry.Name() + "/" + child.Name()
			if !filepath.IsLocal(filepath.FromSlash(rel)) {
				return nil, fmt.Errorf("invalid legacy vector path")
			}
			full := filepath.Join(dir, filepath.FromSlash(rel))
			info, err := os.Lstat(full)
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("legacy vector must be regular")
			}
			if !strings.HasSuffix(rel, ".gz") && info.Size() > legacyImportReadLimit {
				return nil, fmt.Errorf("legacy GOB %s exceeds 64 MiB read limit", child.Name())
			}
			f, err := os.Open(full)
			if err != nil {
				return nil, err
			}
			opened, err := f.Stat()
			if err != nil || !os.SameFile(info, opened) {
				f.Close()
				return nil, fmt.Errorf("legacy source changed while opening")
			}
			hash := sha256.New()
			size, err := io.Copy(hash, f)
			closeErr := f.Close()
			if err != nil {
				return nil, err
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if size != info.Size() {
				return nil, fmt.Errorf("legacy source changed while hashing")
			}
			files = append(files, legacyFile{Path: rel, Size: size, SHA256: hex.EncodeToString(hash.Sum(nil))})
		}
	}
	return files, nil
}

func makeLegacyManifest(files []legacyFile) (legacyManifest, error) {
	data, err := json.Marshal(files)
	if err != nil {
		return legacyManifest{}, err
	}
	hash := sha256.Sum256(data)
	return legacyManifest{Version: 1, Fingerprint: hex.EncodeToString(hash[:]), Files: files}, nil
}

func verifyLegacyInventory(dir string, want []legacyFile) error {
	got, err := inventoryLegacyFiles(dir)
	if err != nil {
		return err
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("legacy file inventory or checksum differs")
	}
	return nil
}

func findLegacyBackup(dir string, want legacyManifest) (string, error) {
	if len(want.Files) == 0 {
		return "", nil
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		return "", err
	}
	prefix := filepath.Base(dir) + ".legacy-backup-" + want.Fingerprint[:16] + "-"
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		backup := filepath.Join(filepath.Dir(dir), entry.Name())
		name := filepath.Join(backup, legacyManifestName)
		info, err := os.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > legacyImportReadLimit {
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			continue
		}
		var got legacyManifest
		err = json.NewDecoder(io.LimitReader(f, legacyImportReadLimit+1)).Decode(&got)
		f.Close()
		if err != nil || got.Version != want.Version || got.Fingerprint != want.Fingerprint || !slices.Equal(got.Files, want.Files) {
			continue
		}
		if err := verifyLegacyInventory(backup, want.Files); err == nil {
			return backup, nil
		}
	}
	return "", nil
}

func publishLegacyBackup(dir string, manifest legacyManifest) (string, error) {
	stage, err := os.MkdirTemp(filepath.Dir(dir), "."+filepath.Base(dir)+".legacy-stage-")
	if err != nil {
		return "", err
	}
	// Only this invocation's uniquely created staging tree may be removed.
	defer os.RemoveAll(stage)
	if err := fileutil.RestrictToOwner(stage); err != nil {
		return "", err
	}
	for _, entry := range manifest.Files {
		to := filepath.Join(stage, filepath.FromSlash(entry.Path))
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return "", err
		}
		if err := fileutil.RestrictToOwner(filepath.Dir(to)); err != nil {
			return "", err
		}
		if err := copyLegacyFile(filepath.Join(dir, filepath.FromSlash(entry.Path)), to, entry.Size); err != nil {
			return "", fmt.Errorf("backup legacy vectors: %w", err)
		}
	}
	if err := verifyLegacyInventory(stage, manifest.Files); err != nil {
		return "", fmt.Errorf("verify legacy backup: %w", err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	if err := fileutil.WriteFileContext(context.Background(), filepath.Join(stage, legacyManifestName), data, 0600); err != nil {
		return "", err
	}
	if err := fileutil.RestrictToOwner(filepath.Join(stage, legacyManifestName)); err != nil {
		return "", err
	}
	prefix := dir + ".legacy-backup-" + manifest.Fingerprint[:16] + "-"
	for {
		backup := prefix + rand.Text()
		if _, err := os.Lstat(backup); !os.IsNotExist(err) {
			if err != nil {
				return "", err
			}
			continue
		}
		if err := os.Rename(stage, backup); err != nil {
			return "", err
		}
		return backup, nil
	}
}
