package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// ResolveOutgoingAttachmentPath resolves a file that a tool sends to an external recipient
// (Telegram document, email attachment). Only existing regular files inside the agent
// workspace or the document output folder qualify, so configuration, vault and databases
// can never leave AuraGo. Relative paths are tried against the working directory (where
// document_creator writes "data/documents/…") and against both roots.
func ResolveOutgoingAttachmentPath(filePath string, cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return "", fmt.Errorf("attachment path is empty")
	}
	docsDir := strings.TrimSpace(cfg.Tools.DocumentCreator.OutputDir)
	if docsDir == "" {
		docsDir = "data/documents"
	}
	roots := uniqueCanonicalPaths(canonicalExistingRoot(cfg.Directories.WorkspaceDir), canonicalExistingRoot(docsDir))
	if len(roots) == 0 {
		return "", fmt.Errorf("no attachment folders are configured")
	}
	candidates := []string{filePath}
	if !filepath.IsAbs(filePath) {
		for _, root := range roots {
			candidates = append(candidates, filepath.Join(root, filePath))
		}
	}
	for _, candidate := range candidates {
		abs, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		for _, root := range roots {
			if pathWithinCanonicalRoot(root, resolved) {
				return resolved, nil
			}
		}
	}
	return "", fmt.Errorf("attachment %q is missing or outside the workspace and the documents folder", filePath)
}
