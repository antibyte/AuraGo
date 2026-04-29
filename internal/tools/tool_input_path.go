package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

func resolveToolInputPath(filePath string, cfg *config.Config) (string, error) {
	return resolveToolPathForRead(filePath, cfg, false)
}

func resolveToolPathForRead(filePath string, cfg *config.Config, allowDir bool) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	workspaceDir := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	if workspaceDir == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return "", fmt.Errorf("path traversal denied: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", filePath)
		}
		return "", fmt.Errorf("stat input path: %w", err)
	}
	if info.IsDir() && !allowDir {
		return "", fmt.Errorf("input path %q is a directory", filePath)
	}
	return resolved, nil
}

// ResolveToolInputPath validates and resolves a tool-provided local file path
// against the configured workspace/project boundaries.
func ResolveToolInputPath(filePath string, cfg *config.Config) (string, error) {
	return resolveToolInputPath(filePath, cfg)
}

func resolveToolOutputPath(filePath string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	workspaceDir := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	if workspaceDir == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return "", fmt.Errorf("path traversal denied: %w", err)
	}
	return resolved, nil
}

// securePDFPath is kept for PDF operations that validate read and write paths
// separately at their call sites. It resolves into project/workspace bounds but
// does not require the destination to exist.
func securePDFPath(workspaceDir, userPath string) (string, error) {
	if strings.HasPrefix(filepath.ToSlash(userPath), "/") && !filepath.IsAbs(userPath) {
		return "", fmt.Errorf("path traversal not allowed")
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspaceDir
	resolved, err := resolveToolOutputPath(userPath, cfg)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}
