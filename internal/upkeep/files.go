// Package upkeep owns bounded, installation-scoped update artifact retention.
package upkeep

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/webassets"
)

const StateDir = ".aurago-update"
const CacheLimit int64 = 4 << 30

var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var transactionPattern = regexp.MustCompile(`^txn-[a-zA-Z0-9]+$`)
var workPattern = regexp.MustCompile(`^work\.([a-zA-Z0-9]{6}|adopt-[0-9]+)$`)
var embeddedID = regexp.MustCompile(`(?:^|[ =])aurago/internal/webassets.SetID=([a-f0-9]{64})(?:\s|$)`)

// PinFromBinary reads Go build metadata without executing archived binaries.
func PinFromBinary(name string) (string, error) {
	if err := regularPath(name); err != nil {
		return "", err
	}
	b, err := buildinfo.ReadFile(name)
	if err != nil {
		return "", err
	}
	if b.Path != "aurago/cmd/aurago" {
		return "", fmt.Errorf("not an AuraGo executable")
	}
	for _, s := range b.Settings {
		if s.Key == "-ldflags" {
			if m := embeddedID.FindStringSubmatch(s.Value); len(m) == 2 {
				return m[1], nil
			}
		}
	}
	// Pre-resource-set binaries embedded their UI and need no external pin.
	return "", nil
}

func regularPath(name string) error {
	if err := noLinks(name); err != nil {
		return err
	}
	s, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !s.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", name)
	}
	return nil
}

// Refuse symlinks in every existing path component, including ancestors.
func noLinks(name string) error {
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	for p := name; ; p = filepath.Dir(p) {
		s, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil && s.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink refused: %s", p)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func within(root, name string) bool {
	r, err := filepath.Rel(root, name)
	return err == nil && r != "." && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) && !filepath.IsAbs(r)
}

func retiredName(root, name string) string {
	return "." + name + ".aurago-retired-" + webassets.Digest([]byte(root))[:16]
}

func retiredOriginal(root, name string) string {
	suffix := ".aurago-retired-" + webassets.Digest([]byte(root))[:16]
	if !strings.HasPrefix(name, ".") || !strings.HasSuffix(name, suffix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, "."), suffix)
}

func treeBytes(name string) (int64, error) {
	if err := noLinks(name); err != nil {
		return 0, err
	}
	var size int64
	err := filepath.WalkDir(name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked content refused: %s", p)
		}
		i, err := d.Info()
		if err != nil {
			return err
		}
		if !i.IsDir() && !i.Mode().IsRegular() {
			return fmt.Errorf("special file refused: %s", p)
		}
		if i.Mode().IsRegular() {
			size += i.Size()
		}
		return nil
	})
	return size, err
}

func digestFile(name string) (string, error) {
	if err := regularPath(name); err != nil {
		return "", err
	}
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readJSON(name string, value any) error {
	if err := regularPath(name); err != nil {
		return err
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 64<<10 {
		return fmt.Errorf("oversized JSON state")
	}
	d := json.NewDecoder(io.LimitReader(f, 64<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func writeJSON(name string, value any) error {
	if err := noLinks(name); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), name)
}

func verifyAssets(root, id string) error {
	if !hashPattern.MatchString(id) {
		return fmt.Errorf("invalid asset ID")
	}
	if err := noLinks(filepath.Join(root, id)); err != nil {
		return err
	}
	s := webassets.Open(root, webassets.Pin{ID: id})
	defer s.Close()
	return s.Error
}

// removeTree uses an os.Root in addition to no-link checks. Even a rename race
// cannot turn a candidate into a recursive deletion outside the installation.
func snapshotInfo(path string) (os.FileInfo, error) {
	i, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	// Windows lazily loads file IDs for Lstat results. Force that load now,
	// otherwise SameFile could inspect a replacement at the old pathname later.
	if !os.SameFile(i, i) {
		return nil, fmt.Errorf("cannot snapshot file identity")
	}
	return i, nil
}

func removeTree(root *os.Root, name string, expected os.FileInfo) error {
	if !filepath.IsLocal(name) || name == "." {
		return fmt.Errorf("invalid deletion name")
	}
	current, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !os.SameFile(expected, current) {
		return fmt.Errorf("candidate changed during cleanup")
	}
	return root.RemoveAll(name)
}

func allocatedBytes(name string) (int64, error) {
	var total int64
	err := filepath.WalkDir(name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			total += allocatedSize(info)
		}
		return nil
	})
	return total, err
}
