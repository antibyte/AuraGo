package agent

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const go2RTCReachabilityNote = "Reachability reflects current codec metadata, not an active connection test. An idle stream may report reachable=false and still deliver a snapshot. Use snapshot to check image availability."

// prepareRegisteredVisionInput grants access only to an exact, live image entry.
// Unregistered agent input continues through the ordinary workspace guard.
func prepareRegisteredVisionInput(db *sql.DB, cfg *config.Config, input string) (string, *config.Config, func(), error) {
	var item *tools.MediaItem
	var err error
	if strings.HasPrefix(strings.TrimSpace(input), "/files/") {
		item, err = tools.GetMediaByWebPath(db, input)
	} else {
		item, err = tools.GetMediaByFilePath(db, input)
	}
	if err != nil || item == nil {
		return input, cfg, func() {}, nil
	}
	if !strings.EqualFold(item.MediaType, "image") {
		return "", cfg, func() {}, fmt.Errorf("registered media is not an image")
	}
	return prepareManagedVisionInput(item.FilePath, cfg)
}

// prepareManagedVisionInput accepts paths produced by AuraGo, never raw agent
// paths. It opens data-dir images through os.Root and gives the existing native
// and MCP vision paths a private, bounded input with its own workspace root.
func prepareManagedVisionInput(input string, cfg *config.Config) (string, *config.Config, func(), error) {
	noop := func() {}
	if cfg == nil {
		return "", cfg, noop, fmt.Errorf("vision configuration is not available")
	}
	if resolved, err := tools.ResolveToolInputPath(input, cfg); err == nil {
		return resolved, cfg, noop, nil
	}
	if strings.TrimSpace(cfg.Directories.DataDir) == "" {
		return "", cfg, noop, fmt.Errorf("managed image data directory is not configured")
	}
	dataDir, err := filepath.Abs(cfg.Directories.DataDir)
	if err != nil {
		return "", cfg, noop, fmt.Errorf("resolve managed image root: %w", err)
	}
	absInput, err := filepath.Abs(input)
	if err != nil {
		return "", cfg, noop, fmt.Errorf("resolve managed image path: %w", err)
	}
	rel, err := filepath.Rel(dataDir, absInput)
	if err != nil || !filepath.IsLocal(rel) {
		return "", cfg, noop, fmt.Errorf("managed image is outside the configured data directory")
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return "", cfg, noop, fmt.Errorf("open managed image root: %w", err)
	}
	defer root.Close()
	file, err := root.Open(rel)
	if err != nil {
		return "", cfg, noop, fmt.Errorf("open managed image: %w", err)
	}
	defer file.Close()
	const maxImageBytes = 50 << 20
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxImageBytes {
		return "", cfg, noop, fmt.Errorf("managed image must be a regular file of at most 50 MB")
	}
	probe := make([]byte, 512)
	n, err := file.Read(probe)
	if err != nil && err != io.EOF {
		return "", cfg, noop, fmt.Errorf("read managed image: %w", err)
	}
	extensions := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/gif": ".gif", "image/webp": ".webp", "image/bmp": ".bmp"}
	ext, ok := extensions[http.DetectContentType(probe[:n])]
	if !ok {
		return "", cfg, noop, fmt.Errorf("managed file is not a supported image")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", cfg, noop, fmt.Errorf("rewind managed image: %w", err)
	}
	dir, err := os.MkdirTemp("", "aurago-vision-*")
	if err != nil {
		return "", cfg, noop, fmt.Errorf("create private vision input: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	path := filepath.Join(dir, "image"+ext)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		cleanup()
		return "", cfg, noop, fmt.Errorf("create private vision image: %w", err)
	}
	written, copyErr := io.Copy(out, io.LimitReader(file, maxImageBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written > maxImageBytes {
		cleanup()
		return "", cfg, noop, fmt.Errorf("could not stage managed image within the 50 MB limit")
	}
	visionCfg := *cfg
	visionCfg.Directories.WorkspaceDir = dir
	return path, &visionCfg, cleanup, nil
}

func analyzeManagedImageWithPrompt(input, prompt string, cfg *config.Config) (string, int, int, error) {
	path, visionCfg, cleanup, err := prepareManagedVisionInput(input, cfg)
	if err != nil {
		return "", 0, 0, err
	}
	defer cleanup()
	return dispatchAnalyzeImageWithPrompt(path, prompt, visionCfg)
}
