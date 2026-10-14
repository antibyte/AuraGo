package gog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

const Version = "0.44.0-aurago1"
const SourceCommit = "cf42bb7c9f67d3369090de95a7b84ee258be9b02"
const MaxArchive = 128 << 20

//go:embed build/artifacts.json
var artifactManifest []byte

type Artifact struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archive_sha256"`
	BinarySHA256  string `json:"binary_sha256"`
}

var installMu sync.Mutex

func CurrentArtifact() (Artifact, error) {
	var all []Artifact
	if json.Unmarshal(artifactManifest, &all) != nil {
		return Artifact{}, fmt.Errorf("invalid_release_manifest")
	}
	for _, a := range all {
		if a.OS == runtime.GOOS && a.Arch == runtime.GOARCH && len(a.ArchiveSHA256) == 64 && len(a.BinarySHA256) == 64 {
			return a, nil
		}
	}
	return Artifact{}, fmt.Errorf("unsupported_platform")
}
func executableName() string {
	if runtime.GOOS == "windows" {
		return "gog.exe"
	}
	return "gog"
}
func (s *Service) binaryPath(a Artifact) string {
	return filepath.Join(s.Root, "bin", a.BinarySHA256, executableName())
}
func (s *Service) VerifiedBinary() (string, error) {
	a, err := CurrentArtifact()
	if err != nil {
		return "", err
	}
	p := s.binaryPath(a)
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxArchive {
		return "", fmt.Errorf("invalid_binary")
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	if hex.EncodeToString(h.Sum(nil)) != a.BinarySHA256 {
		return "", fmt.Errorf("binary_checksum_mismatch")
	}
	return p, nil
}
func (s *Service) Install(ctx context.Context) error {
	installMu.Lock()
	defer installMu.Unlock()
	a, err := CurrentArtifact()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	allowed := func() bool {
		c := s.Config()
		return c != nil && c.Agent.AllowNetworkRequests && c.Agent.AllowFilesystemWrite
	}
	if !allowed() {
		return fmt.Errorf("installation_permissions_required")
	}
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if !allowed() {
					cancel()
					return
				}
			}
		}
	}()
	client := security.NewSSRFProtectedHTTPClient(5 * time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://github.com/antibyte/AuraGo/releases/latest/download/"+a.Archive, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download gog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("gog package unavailable: HTTP %d", resp.StatusCode)
	}
	if !allowed() {
		return fmt.Errorf("installation_permissions_revoked")
	}
	return s.installArchive(ctx, a, resp.Body)
}
func (s *Service) installArchive(ctx context.Context, a Artifact, r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, MaxArchive+1))
	if err != nil {
		return err
	}
	if len(data) > MaxArchive {
		return fmt.Errorf("archive_too_large")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != a.ArchiveSHA256 {
		return fmt.Errorf("archive_checksum_mismatch")
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid_archive: %w", err)
	}
	if len(z.File) != 1 && len(z.File) != 2 {
		return fmt.Errorf("unexpected_archive_entries")
	}
	if len(z.File) == 2 && (z.File[1].Name != "LICENSE.gog" || !z.File[1].Mode().IsRegular() || z.File[1].UncompressedSize64 > 16<<10) {
		return fmt.Errorf("unsafe_license_entry")
	}
	entry := z.File[0]
	if entry.Name != executableName() || !entry.Mode().IsRegular() || entry.UncompressedSize64 > MaxArchive {
		return fmt.Errorf("unsafe_archive_entry")
	}
	dir := filepath.Dir(s.binaryPath(a))
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".install-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	input, err := entry.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(input, MaxArchive+1))
	if err != nil || n > MaxArchive {
		return fmt.Errorf("invalid_binary_content")
	}
	if hex.EncodeToString(h.Sum(nil)) != a.BinarySHA256 {
		return fmt.Errorf("binary_checksum_mismatch")
	}
	if err = f.Chmod(0700); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return fileutil.RenameContext(ctx, f.Name(), s.binaryPath(a))
}
