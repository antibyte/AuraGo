// Package webassets owns the version-bound, external browser resource set.
package webassets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Release builds inject these values from cmd/assetpack. An ordinary go build
// intentionally starts in recovery mode; it never trusts a working-directory UI.
var SetID, ArchiveSHA256, ArchiveBytes, DownloadURL string

const ManifestName = "manifest.json"
const MaxArchiveBytes int64 = 512 << 20
const MaxExpandedBytes int64 = 1 << 30
const MaxFiles = 20000

var ErrUnavailable = errors.New("web resources unavailable; install the matching asset set and restart AuraGo")

type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version int     `json:"version"`
	Files   []Entry `json:"files"`
}

type Pin struct {
	ID     string `json:"asset_set_id"`
	SHA256 string `json:"archive_sha256"`
	Bytes  int64  `json:"archive_bytes"`
	URL    string `json:"download_url,omitempty"`
}

func ReleasePin() Pin {
	var size int64
	fmt.Sscan(ArchiveBytes, &size)
	return Pin{SetID, ArchiveSHA256, size, DownloadURL}
}

func Digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func validHash(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && s == strings.ToLower(s)
}

func ValidPath(name string) bool {
	if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, ". ") != part || strings.HasPrefix(part, ".") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}

func ParseManifest(data []byte, id string) (Manifest, error) {
	var m Manifest
	if !validHash(id) || Digest(data) != id {
		return m, fmt.Errorf("asset manifest digest mismatch")
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("decode asset manifest: %w", err)
	}
	if m.Version != 1 || len(m.Files) == 0 || len(m.Files) > MaxFiles {
		return m, fmt.Errorf("unsupported asset manifest")
	}
	seen := make(map[string]bool)
	var total int64
	for _, e := range m.Files {
		key := strings.ToLower(e.Path)
		if !ValidPath(e.Path) || e.Path == ManifestName || seen[key] || e.Size < 0 || e.Size > MaxExpandedBytes || !validHash(e.SHA256) {
			return m, fmt.Errorf("invalid asset entry %q", e.Path)
		}
		seen[key] = true
		total += e.Size
		if total > MaxExpandedBytes {
			return m, fmt.Errorf("asset set exceeds expanded size limit")
		}
	}
	return m, nil
}

// Files adds the stdlib convenience methods used by existing resource consumers.
type Files struct{ fs.FS }

func (f Files) ReadFile(name string) ([]byte, error)       { return fs.ReadFile(f.FS, name) }
func (f Files) ReadDir(name string) ([]fs.DirEntry, error) { return fs.ReadDir(f.FS, name) }

type Store struct {
	Dir     string
	Pin     Pin
	Error   error
	root    *os.Root
	allowed map[string]bool
}

// Default is initialized once, before services start. Installed sets are only
// activated at the next process start, keeping templates and files coherent.
var Default = &Store{Error: ErrUnavailable}

func Open(dir string, pin Pin) *Store {
	s := &Store{Dir: dir, Pin: pin}
	s.Error = s.verify()
	if s.Error != nil && s.root != nil {
		s.root.Close()
		s.root = nil
	}
	return s
}

func (s *Store) Close() error {
	if s.root != nil {
		return s.root.Close()
	}
	return nil
}
func (s *Store) Ready() bool { return s != nil && s.Error == nil && s.root != nil }

func (s *Store) verify() error {
	if !validHash(s.Pin.ID) {
		return ErrUnavailable
	}
	root, err := os.OpenRoot(filepath.Join(s.Dir, s.Pin.ID))
	if err != nil {
		return fmt.Errorf("open asset set: %w", err)
	}
	s.root = root
	f, err := root.Open(ManifestName)
	if err != nil {
		return fmt.Errorf("open asset manifest: %w", err)
	}
	data, err := io.ReadAll(io.LimitReader(f, 8<<20))
	f.Close()
	if err != nil {
		return err
	}
	m, err := ParseManifest(data, s.Pin.ID)
	if err != nil {
		return err
	}
	s.allowed = map[string]bool{".": true}
	for _, e := range m.Files {
		// Reject links even when they happen to point inside this set.
		for name := e.Path; name != "."; name = path.Dir(name) {
			info, err := root.Lstat(name)
			if err != nil || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("invalid asset path %q", name)
			}
			s.allowed[name] = true
		}
		file, err := root.Open(e.Path)
		if err != nil {
			return err
		}
		info, statErr := file.Stat()
		h := sha256.New()
		n, copyErr := io.Copy(h, io.LimitReader(file, e.Size+1))
		file.Close()
		if statErr != nil || !info.Mode().IsRegular() || copyErr != nil || n != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
			return fmt.Errorf("asset integrity check failed: %s", e.Path)
		}
	}
	return nil
}

func (s *Store) Open(name string) (fs.File, error) {
	if !s.Ready() {
		return nil, ErrUnavailable
	}
	if !fs.ValidPath(name) || !s.allowed[name] {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return s.root.Open(name)
}

func (s *Store) Namespace(name string) Files { return Files{namespace{s, name}} }

type namespace struct {
	store  *Store
	prefix string
}

func (n namespace) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrInvalid
	}
	s := n.store
	if s == nil {
		s = Default
	}
	if !s.Ready() || !s.allowed[n.prefix] {
		return nil, ErrUnavailable
	}
	return s.Open(path.Join(n.prefix, name))
}
func Namespace(name string) Files { return Files{namespace{nil, name}} }
