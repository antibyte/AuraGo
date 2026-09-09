package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/webassets"
)

var bundledAppAssets = webassets.Namespace("desktop")

const nasscadBundledVersion = "4.7.0-github.844d9420"

func (s *Service) seedBundledBuiltinAppsLocked(ctx context.Context) error {
	if err := s.seedNasscadAppLocked(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) seedNasscadAppLocked(ctx context.Context) error {
	// Existing workspace apps belong to the user, even after a bundled upgrade.
	if _, err := os.Lstat(filepath.Join(s.cfg.WorkspaceDir, "Apps", "nasscad", "index.html")); err == nil {
		return nil
	}
	const metaKey = "desktop_bundled_app_nasscad_version"
	runtimeAssets := []string{"nasscad_occt_wasm.js", "opencascade.wasm.wasm"}

	indexHTML, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if errors.Is(err, webassets.ErrUnavailable) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read bundled nasscad index: %w", err)
	}
	monolithic, err := buildMonolithicNasscadHTML(indexHTML, bundledAppAssets, "bundled_apps/nasscad")
	if err != nil {
		return fmt.Errorf("build monolithic nasscad html: %w", err)
	}
	for _, name := range runtimeAssets {
		data, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/" + name)
		if err != nil {
			return fmt.Errorf("read bundled nasscad asset %s: %w", name, err)
		}
		if err := s.seedWorkspaceFileLocked("Apps/nasscad/"+name, data); err != nil {
			return fmt.Errorf("seed nasscad asset %s: %w", name, err)
		}
	}

	// Publish the entry page last, so a failed first install remains retryable.
	if err := s.seedWorkspaceFileLocked("Apps/nasscad/index.html", monolithic); err != nil {
		return fmt.Errorf("seed nasscad app: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO desktop_meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, metaKey, nasscadBundledVersion); err != nil {
		return fmt.Errorf("mark nasscad bundled app seeded: %w", err)
	}
	return nil
}

func bytesContainsNasscadMonolithMarkers(data []byte) bool {
	return len(data) > 0 &&
		bytes.Contains(data, []byte("THREE.REVISION")) &&
		bytes.Contains(data, []byte("function nasLog"))
}

func (s *Service) seedWorkspaceFileLocked(rawPath string, content []byte) error {
	workspaceDir := strings.TrimSpace(s.cfg.WorkspaceDir)
	if workspaceDir == "" {
		return fmt.Errorf("desktop workspace is not configured")
	}
	cleaned := cleanDesktopPath(rawPath)
	path := filepath.Join(workspaceDir, cleaned)
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve bundled app path: %w", err)
	}
	rootAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return fmt.Errorf("resolve desktop root: %w", err)
	}
	if !isWithinPath(rootAbs, pathAbs) {
		return fmt.Errorf("bundled app path escapes workspace")
	}
	if err := validateNoSymlinkComponents(rootAbs, pathAbs, true); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pathAbs), 0o700); err != nil {
		return fmt.Errorf("create bundled app directory: %w", err)
	}
	if _, err := secureWriteWorkspaceFile(pathAbs, content); err != nil {
		return fmt.Errorf("write bundled app file: %w", err)
	}
	return nil
}
