package upkeep

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/webassets"
)

func legacyBinding(root, path string) error {
	if !strings.HasPrefix(filepath.Base(path), "aurago-backup-") {
		return fmt.Errorf("not a legacy update backup")
	}
	if _, err := treeBytes(path); err != nil {
		return err
	}
	a, err := os.Stat(path)
	if err != nil {
		return err
	}
	b, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !sameOwner(a, b) {
		return fmt.Errorf("legacy backup has different owner")
	}
	marker := filepath.Join(path, "tsnet-state.path")
	if err := regularPath(marker); err != nil {
		return err
	}
	f, err := os.Open(marker)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	f.Close()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(string(data))
	if len(data) > 4096 || !filepath.IsAbs(name) || !within(filepath.Join(root, "data"), filepath.Clean(name)) {
		return fmt.Errorf("legacy backup has no unambiguous installation binding")
	}
	return nil
}

func legacyBackups(o Options, r *Report) ([]backup, error) {
	if err := noLinks(o.LegacyDir); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(o.LegacyDir, "aurago-backup-*"))
	if err != nil {
		return nil, err
	}
	var result []backup
	for _, p := range paths {
		if err := legacyBinding(o.Root, p); err != nil {
			r.keep(p, "unassigned legacy backup")
			continue
		}
		bin := filepath.Join(p, "bin", "aurago_linux")
		if _, err := os.Stat(bin); os.IsNotExist(err) {
			bin = filepath.Join(p, "bin", "aurago")
		}
		id, err := o.ReadPin(bin)
		if err != nil {
			r.keep(p, "legacy binary cannot be identified")
			continue
		}
		h, err := digestFile(bin)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		result = append(result, backup{Transaction: Transaction{Version: 1, Root: o.Root, ID: filepath.Base(p), Created: info.ModTime().UnixMilli(), Status: "confirmed", PreviousVersion: h, PreviousAsset: id, BackupComplete: true}, path: p, legacy: true})
	}
	return result, nil
}

func archiveIdentity(name, id string) error {
	if !hashPattern.MatchString(id) {
		return fmt.Errorf("invalid archive ID")
	}
	if err := regularPath(name); err != nil {
		return err
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer g.Close()
	t := tar.NewReader(g)
	h, err := t.Next()
	if err != nil {
		return err
	}
	if h.Name != webassets.ManifestName || h.Typeflag != tar.TypeReg || h.Size > 8<<20 || h.Size <= 0 {
		return fmt.Errorf("not a resource archive")
	}
	b, err := io.ReadAll(io.LimitReader(t, 8<<20))
	if err != nil {
		return err
	}
	m, err := webassets.ParseManifest(b, id)
	if err != nil {
		return err
	}
	expected := make(map[string]webassets.Entry, len(m.Files))
	for _, entry := range m.Files {
		expected[entry.Path] = entry
	}
	seen := map[string]bool{}
	for {
		h, err = t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entry, ok := expected[h.Name]
		if !ok || seen[h.Name] || h.Typeflag != tar.TypeReg || h.Size != entry.Size {
			return fmt.Errorf("unexpected archive payload")
		}
		digest := sha256.New()
		n, err := io.Copy(digest, t)
		if err != nil || n != entry.Size || hex.EncodeToString(digest.Sum(nil)) != entry.SHA256 {
			return fmt.Errorf("archive payload integrity failure")
		}
		seen[h.Name] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("incomplete resource archive")
	}
	// Consume gzip EOF/CRC and reject concatenated or trailing payloads.
	tail, err := io.ReadAll(io.LimitReader(g, 1))
	if err != nil {
		return err
	}
	if len(tail) != 0 {
		return fmt.Errorf("trailing archive payload")
	}
	return nil
}

// Copy into private staging before publishing, including when /tmp is a
// different filesystem. Failed adoption leaves the original backup untouched.
func adoptBackup(o Options, b backup) (string, error) {
	if err := legacyBinding(o.Root, b.path); err != nil {
		return "", err
	}
	if !hashPattern.MatchString(b.PreviousAsset) {
		return "", fmt.Errorf("legacy rollback has no verifiable external resource pin")
	}
	id := "txn-legacy" + webassets.Digest([]byte(b.path))[:24]
	destination := filepath.Join(o.Root, StateDir, "transactions", id)
	if err := noLinks(destination); err != nil {
		return "", err
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return "", fmt.Errorf("legacy adoption destination already exists")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Join(o.Root, StateDir), "work.adopt-")
	if err != nil {
		return "", err
	}
	r, err := os.OpenRoot(filepath.Join(o.Root, StateDir))
	if err != nil {
		return "", err
	}
	defer r.Close()
	defer r.RemoveAll(filepath.Base(stage))
	sourceInfo, err := snapshotInfo(b.path)
	if err != nil {
		return "", err
	}
	if err := os.CopyFS(stage, os.DirFS(b.path)); err != nil {
		return "", err
	}
	m := b.Transaction
	m.ID = id
	if err := writeJSON(filepath.Join(stage, "manifest.json"), m); err != nil {
		return "", err
	}
	if err := verifyBackup(o, backup{Transaction: m, path: stage}); err != nil {
		return "", err
	}
	if err := os.Rename(stage, destination); err != nil {
		return "", err
	}
	if err := legacyBinding(o.Root, b.path); err != nil {
		return "", err
	}
	legacyRoot, err := os.OpenRoot(o.LegacyDir)
	if err != nil {
		return "", err
	}
	defer legacyRoot.Close()
	if err := removeTree(legacyRoot, filepath.Base(b.path), sourceInfo); err != nil {
		return "", err
	}
	return destination, nil
}
