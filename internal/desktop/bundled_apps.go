package desktop

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed bundled_apps/nasscad/*
var bundledAppAssets embed.FS

const nasscadBundledVersion = "4.2.7.1"

func (s *Service) seedBundledBuiltinAppsLocked(ctx context.Context) error {
	if err := s.seedNasscadAppLocked(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Service) seedNasscadAppLocked(ctx context.Context) error {
	const metaKey = "desktop_bundled_app_nasscad_version"
	var seededVersion string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM desktop_meta WHERE key = ?`, metaKey).Scan(&seededVersion)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read nasscad bundled app seed state: %w", err)
	}
	if seededVersion == nasscadBundledVersion {
		if _, err := os.Stat(filepath.Join(s.cfg.WorkspaceDir, "Apps", "nasscad", "index.html")); err == nil {
			if _, err := os.Stat(filepath.Join(s.cfg.WorkspaceDir, "Apps", "nasscad", "three.js")); err == nil {
				if _, err := os.Stat(filepath.Join(s.cfg.WorkspaceDir, "Apps", "nasscad", "nasscad_logs.js")); err == nil {
					return nil
				}
			}
		}
	}

	if err := fs.WalkDir(bundledAppAssets, "bundled_apps/nasscad", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		content, err := bundledAppAssets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read bundled nasscad asset %s: %w", path, err)
		}
		rel := strings.TrimPrefix(path, "bundled_apps/nasscad/")
		workspacePath := filepath.ToSlash(filepath.Join("Apps", "nasscad", rel))
		if err := s.seedWorkspaceFileLocked(workspacePath, content); err != nil {
			return fmt.Errorf("seed nasscad asset %s: %w", workspacePath, err)
		}
		return nil
	}); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `INSERT INTO desktop_meta(key, value) VALUES(?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, metaKey, nasscadBundledVersion); err != nil {
		return fmt.Errorf("mark nasscad bundled app seeded: %w", err)
	}
	return nil
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