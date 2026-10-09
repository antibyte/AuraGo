package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

const cloudflaredMaxDownload = 100 << 20

var cloudflaredInstallMu sync.Mutex
var cloudflaredDownloadClient = &http.Client{Timeout: 5 * time.Minute}
var cloudflaredPublish = fileutil.RenameContext

type cloudflaredDownload struct {
	DownloadURL string
	SHA256      string
}

func cloudflaredDownloadMetadata(goos, arch string) (cloudflaredDownload, error) {
	if goos != "linux" || arch != "amd64" && arch != "arm64" {
		return cloudflaredDownload{}, fmt.Errorf("cloudflared automatic download for %s/%s is disabled: no reviewed checksum metadata", goos, arch)
	}
	hash := "d33ff2d14475178d2012c2c56beba87389ac5ded27649519f198a7d3134a99db"
	if arch == "arm64" {
		hash = "e6422b9d4f72d3194bc5a38676f13667c06666523217b842a877d72a80b5ac08"
	}
	return cloudflaredDownload{DownloadURL: "https://github.com/cloudflare/cloudflared/releases/download/2026.10.0/cloudflared-linux-" + arch, SHA256: hash}, nil
}

func verifyCloudflaredChecksum(expected, filePath string) error {
	decoded, err := hex.DecodeString(expected)
	if err != nil || len(decoded) != sha256.Size {
		return fmt.Errorf("invalid reviewed cloudflared SHA-256 checksum")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open cloudflared checksum input: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash cloudflared: %w", err)
	}
	if !strings.EqualFold(expected, hex.EncodeToString(h.Sum(nil))) {
		return fmt.Errorf("cloudflared checksum mismatch")
	}
	return nil
}

func installCloudflaredBinary(destPath string, logger *slog.Logger) string {
	metadata, err := cloudflaredDownloadMetadata(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return errJSON("%v", err)
	}
	return installCloudflaredDownload(destPath, metadata, logger)
}

func installCloudflaredDownload(destPath string, metadata cloudflaredDownload, logger *slog.Logger) string {
	cloudflaredInstallMu.Lock()
	defer cloudflaredInstallMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return errJSON("Create cloudflared directory: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadata.DownloadURL, nil)
	if err != nil {
		return errJSON("Prepare cloudflared download: %v", err)
	}
	resp, err := cloudflaredDownloadClient.Do(req)
	if err != nil {
		return errJSON("Download cloudflared: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.ContentLength > cloudflaredMaxDownload {
		return errJSON("Cloudflared download rejected (HTTP %d; maximum 100 MiB)", resp.StatusCode)
	}
	f, err := os.CreateTemp(filepath.Dir(destPath), ".cloudflared-*")
	if err != nil {
		return errJSON("Create cloudflared temporary file: %v", err)
	}
	defer os.Remove(f.Name())
	n, copyErr := io.Copy(f, io.LimitReader(resp.Body, cloudflaredMaxDownload+1))
	if copyErr == nil && n > cloudflaredMaxDownload {
		copyErr = fmt.Errorf("cloudflared download exceeds 100 MiB")
	}
	if copyErr == nil {
		copyErr = f.Chmod(0755)
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil {
		return errJSON("Prepare cloudflared binary: %v", copyErr)
	}
	if closeErr != nil {
		return errJSON("Close cloudflared binary: %v", closeErr)
	}
	if err := verifyCloudflaredChecksum(metadata.SHA256, f.Name()); err != nil {
		return errJSON("Binary integrity check failed: %v", err)
	}
	if err := cloudflaredPublish(ctx, f.Name(), destPath); err != nil {
		return errJSON("Publish cloudflared binary: %v", err)
	}
	logger.Info("[CloudflareTunnel] Binary installed", "path", destPath, "bytes", n)
	out, _ := json.Marshal(map[string]interface{}{"status": "ok", "message": fmt.Sprintf("cloudflared installed (%d bytes)", n), "path": destPath})
	return string(out)
}
