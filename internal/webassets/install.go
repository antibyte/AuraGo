package webassets

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
)

// Install verifies a pinned archive before extraction and publishes a complete
// set with one rename. Existing sets (including rollback versions) stay intact.
func (s *Store) Install(ctx context.Context, source io.Reader) error {
	if !validHash(s.Pin.ID) || !validHash(s.Pin.SHA256) || s.Pin.Bytes <= 0 || s.Pin.Bytes > MaxArchiveBytes {
		return fmt.Errorf("binary has no valid asset release pin")
	}
	if err := os.MkdirAll(s.Dir, 0750); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(s.Dir, ".install.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("acquire asset install lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("acquire asset install lock: %w", ctx.Err())
	}
	defer lock.Unlock()
	if existing := Open(s.Dir, s.Pin); existing.Ready() {
		existing.Close()
		return nil
	}
	archive, err := os.CreateTemp(s.Dir, ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(archive, h), io.LimitReader(source, s.Pin.Bytes+1))
	if err != nil {
		return fmt.Errorf("receive asset archive: %w", err)
	}
	if n != s.Pin.Bytes || hex.EncodeToString(h.Sum(nil)) != s.Pin.SHA256 {
		return fmt.Errorf("asset archive size or SHA-256 mismatch")
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gz.Close()
	staging, err := os.MkdirTemp(s.Dir, ".staging-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	tr := tar.NewReader(gz)
	header, err := tr.Next()
	if err != nil || header.Name != ManifestName || header.Typeflag != tar.TypeReg || header.Size > 8<<20 || header.Size <= 0 {
		return fmt.Errorf("archive must start with its manifest")
	}
	data, err := io.ReadAll(tr)
	if err != nil {
		return err
	}
	m, err := ParseManifest(data, s.Pin.ID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staging, ManifestName), data, 0644); err != nil {
		return err
	}
	entries := make(map[string]Entry, len(m.Files))
	for _, e := range m.Files {
		entries[e.Path] = e
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		e, ok := entries[hdr.Name]
		if !ok || hdr.Typeflag != tar.TypeReg || hdr.Size != e.Size || hdr.Mode & ^int64(0666) != 0 {
			return fmt.Errorf("unexpected archive entry %q", hdr.Name)
		}
		delete(entries, hdr.Name)
		target := filepath.Join(staging, filepath.FromSlash(e.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(f, h), tr)
		closeErr := f.Close()
		if err != nil || closeErr != nil || n != e.Size || hex.EncodeToString(h.Sum(nil)) != e.SHA256 {
			return fmt.Errorf("invalid archive content %q", e.Path)
		}
	}
	if len(entries) != 0 {
		return fmt.Errorf("incomplete asset archive")
	}
	// Consume the gzip trailer so corrupt CRCs and trailing compressed payloads fail.
	if n, err := io.Copy(io.Discard, io.LimitReader(gz, 1)); err != nil || n != 0 {
		return fmt.Errorf("invalid archive trailer")
	}
	return s.publish(ctx, staging)
}

func publishDirectory(ctx context.Context, source, target string) error {
	// Windows scanners can briefly hold newly written WASM/media files open.
	for attempt := 0; ; attempt++ {
		err := os.Rename(source, target)
		if err == nil || runtime.GOOS != "windows" || !os.IsPermission(err) || attempt == 30 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (s *Store) Download(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Pin.URL, nil)
	if err != nil {
		return err
	}
	if req.URL.Scheme != "https" || req.URL.Host != "github.com" || req.URL.User != nil {
		return fmt.Errorf("asset download requires a pinned GitHub release URL")
	}
	client := &http.Client{Timeout: 15 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || r.URL.Scheme != "https" || r.URL.User != nil {
			return fmt.Errorf("invalid asset download redirect")
		}
		switch r.URL.Host {
		case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
		return fmt.Errorf("unapproved asset download host")
	}}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download HTTP %d", res.StatusCode)
	}
	if res.ContentLength >= 0 && res.ContentLength != s.Pin.Bytes {
		return fmt.Errorf("asset download length mismatch")
	}
	return s.Install(ctx, res.Body)
}
