package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// browserOutputRoot pins browser publication to the configured filesystem jail.
func browserOutputRoot(ctx context.Context, workspaceDir, outputDir string) (*os.Root, string, error) {
	perms, configured := EffectiveRuntimePermissions(ctx)
	if !configured || !perms.AllowFilesystemWrite {
		return nil, "", fmt.Errorf("filesystem write is disabled by runtime permissions")
	}
	if workspaceDir == "" {
		workspaceDir = "agent_workspace/workdir"
	}
	if outputDir == "" {
		outputDir = "."
	}
	resolved, err := secureResolve(workspaceDir, outputDir)
	if err != nil {
		return nil, "", err
	}
	if err := requireUnprotectedNotesPath(resolved, true); err != nil {
		return nil, "", err
	}
	_, jail := filesystemRoots(workspaceDir)
	root, err := os.OpenRoot(jail)
	if err != nil {
		return nil, "", err
	}
	rel, err := filepath.Rel(jail, resolved)
	if err != nil {
		root.Close()
		return nil, "", err
	}
	if err := root.MkdirAll(rel, 0o750); err != nil {
		root.Close()
		return nil, "", err
	}
	return root, rel, nil
}
