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

// ResolveRegisteredMediaFilePath resolves an application-registered media path
// within the configured workspace, project, or data roots. Unlike normal tool
// input, registry files may live in data_dir outside the agent workdir.
func ResolveRegisteredMediaFilePath(filePath string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("registered media path is empty")
	}

	workspaceRoot := canonicalExistingRoot(cfg.Directories.WorkspaceDir)
	projectRoot := ""
	if workspaceRoot != "" {
		projectRoot = canonicalExistingRoot(detectFilesystemProjectRoot(workspaceRoot))
	}
	dataDir := strings.TrimSpace(cfg.Directories.DataDir)
	if dataDir != "" && !filepath.IsAbs(dataDir) && projectRoot != "" {
		dataDir = filepath.Join(projectRoot, dataDir)
	}
	dataRoot := canonicalExistingRoot(dataDir)
	roots := uniqueCanonicalPaths(workspaceRoot, projectRoot, dataRoot)
	if len(roots) == 0 {
		return "", fmt.Errorf("no media path roots are configured")
	}

	candidates := make([]string, 0, 3)
	if filepath.IsAbs(filePath) {
		candidates = append(candidates, filePath)
	} else {
		for _, root := range roots {
			candidates = append(candidates, filepath.Join(root, filePath))
		}
	}
	for _, candidate := range uniqueCanonicalPaths(candidates...) {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		for _, root := range roots {
			if pathWithinCanonicalRoot(root, resolved) {
				return resolved, nil
			}
		}
	}
	return "", fmt.Errorf("registered media path %q is missing or outside configured roots", filePath)
}

func canonicalExistingRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absRoot); resolveErr == nil {
		absRoot = resolved
	}
	return filepath.Clean(absRoot)
}

func uniqueCanonicalPaths(paths ...string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, value := range paths {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = filepath.Clean(value)
		key := value
		if filepath.Separator == '\\' {
			key = strings.ToLower(key)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
}

func pathWithinCanonicalRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(relative) || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
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
