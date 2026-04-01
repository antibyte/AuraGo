package tools

import (
	"fmt"
	"os"
	"strings"

	"aurago/internal/config"
)

func resolveToolInputPath(filePath string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	workspaceDir := strings.TrimSpace(cfg.Directories.WorkspaceDir)
	if workspaceDir == "" {
		return "", fmt.Errorf("workspace_dir is not configured")
	}
	resolved, err := secureResolve(workspaceDir, filePath)
	if err != nil {
		return "", fmt.Errorf("resolve input path: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat input path: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("input path %q is a directory", filePath)
	}
	return resolved, nil
}
