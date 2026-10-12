package ruview

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/webassets"
)

//go:embed artifacts.json
var artifactsJSON []byte

type ArtifactFile struct {
	Name     string `json:"name"`
	URL      string `json:"url,omitempty"`
	Resource string `json:"resource,omitempty"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Offset   int64  `json:"offset,omitempty"`
}
type NativeArtifact struct {
	Platform string         `json:"platform"`
	Files    []ArtifactFile `json:"files"`
}
type Firmware struct {
	ID              string         `json:"id"`
	Chip            string         `json:"chip"`
	FlashBytes      int64          `json:"flash_bytes"`
	Console         string         `json:"console"`
	PartitionSHA256 string         `json:"partition_sha256"`
	Parts           []ArtifactFile `json:"parts"`
}
type Catalog struct {
	Revision        string           `json:"revision"`
	Version         string           `json:"version"`
	FirmwareVersion string           `json:"firmware_version"`
	DockerImage     string           `json:"docker_image"`
	DockerPlatforms []string         `json:"docker_platforms"`
	Native          []NativeArtifact `json:"native"`
	Firmware        []Firmware       `json:"firmware"`
}

func Artifacts() Catalog { var c Catalog; _ = json.Unmarshal(artifactsJSON, &c); return c }
func nativeArtifact() (NativeArtifact, error) {
	for _, a := range Artifacts().Native {
		if a.Platform == runtime.GOOS+"-"+runtime.GOARCH {
			return a, nil
		}
	}
	return NativeArtifact{}, fmt.Errorf("no verified native RuView package for this platform in this release")
}

func (m *Manager) Install(ctx context.Context) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	ctx, cancel := m.bound(ctx)
	defer cancel()
	c := m.config()
	if !c.RuView.Enabled || c.RuView.ReadOnly {
		return fmt.Errorf("RuView installation requires enabled, writable integration")
	}
	if c.RuView.Mode != "native" {
		return fmt.Errorf("native installation is not selected")
	}
	if err := configCheck(c); err != nil {
		return err
	}
	a, err := nativeArtifact()
	if err != nil {
		return err
	}
	dir := filepath.Join(c.Directories.DataDir, "ruview", "runtime", Revision, a.Platform)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("prepare RuView runtime: %w", err)
	}
	for _, file := range a.Files {
		if err := download(ctx, dir, file); err != nil {
			return err
		}
	}
	return nil
}

func verifyFile(dir string, file ArtifactFile) error {
	if filepath.Base(file.Name) != file.Name || file.Size <= 0 || file.Size > 512<<20 {
		return fmt.Errorf("invalid RuView artifact metadata")
	}
	info, err := os.Lstat(filepath.Join(dir, file.Name))
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != file.Size {
		return fmt.Errorf("RuView artifact size/type mismatch")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.Open(file.Name)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err = io.Copy(sum, io.LimitReader(f, file.Size+1)); err != nil {
		return err
	}
	if hex.EncodeToString(sum.Sum(nil)) != file.SHA256 {
		return fmt.Errorf("RuView artifact checksum mismatch")
	}
	return nil
}

func download(ctx context.Context, dir string, a ArtifactFile) error {
	if verifyFile(dir, a) == nil {
		return nil
	}
	if filepath.Base(a.Name) != a.Name || a.Size <= 0 || a.Size > 512<<20 || len(a.SHA256) != 64 {
		return fmt.Errorf("invalid RuView download metadata")
	}
	var source io.ReadCloser
	var err error
	if a.Resource != "" {
		if !fs.ValidPath(a.Resource) || !strings.HasPrefix(a.Resource, "js/vendor/ruview-runtime/"+Revision+"/") {
			return fmt.Errorf("invalid RuView resource path")
		}
		source, err = webassets.Namespace("ui").Open(a.Resource)
	} else {
		source, err = downloadSource(ctx, a.URL)
	}
	if err != nil {
		return err
	}
	defer source.Close()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	temp := ".download-" + hex.EncodeToString(random[:])
	f, err := root.OpenFile(temp, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return err
	}
	defer root.Remove(temp)
	defer f.Close()
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, sum), io.LimitReader(source, a.Size+1))
	if err != nil || n != a.Size || hex.EncodeToString(sum.Sum(nil)) != a.SHA256 {
		return fmt.Errorf("RuView download integrity check failed")
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
	return fileutil.RenameRootContext(ctx, root, temp, a.Name)
}

// Release assets redirect to public object storage. Each hop gets a new
// DNS-pinned client, without credentials or an ambient proxy.
func downloadSource(ctx context.Context, raw string) (io.ReadCloser, error) {
	for hop := 0; hop < 5; hop++ {
		if !strings.HasPrefix(raw, "https://") {
			return nil, fmt.Errorf("RuView artifacts require HTTPS")
		}
		client, err := security.NewStrictPublicHTTPClientForURL(raw, 10*time.Minute)
		if err != nil {
			return nil, fmt.Errorf("RuView artifact origin is unavailable or blocked")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			client.CloseIdleConnections()
			return nil, err
		}
		resp, err := client.Do(req)
		client.CloseIdleConnections()
		if err != nil {
			return nil, fmt.Errorf("RuView artifact download failed")
		}
		if resp.StatusCode == 200 {
			return resp.Body, nil
		}
		resp.Body.Close()
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return nil, fmt.Errorf("RuView artifact download returned HTTP %d", resp.StatusCode)
		}
		next, err := resp.Location()
		if err != nil {
			return nil, fmt.Errorf("invalid RuView artifact redirect")
		}
		raw = next.String()
	}
	return nil, fmt.Errorf("too many RuView artifact redirects")
}
