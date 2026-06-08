package desktop

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedNasscadBundledAppWritesWorkspaceEntry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	svc := testServiceWithConfig(t, Config{
		Enabled:       true,
		WorkspaceDir:  root,
		DBPath:        dbPath,
		MaxFileSizeMB: 20,
	})
	ctx := context.Background()

	requiredFiles := []string{
		"Apps/nasscad/index.html",
		"Apps/nasscad/three.js",
		"Apps/nasscad/nasscad_logs.js",
		"Apps/nasscad/manifold.js",
	}

	for _, rel := range requiredFiles {
		appPath, err := svc.ResolvePath(rel)
		if err != nil {
			t.Fatalf("ResolvePath(%s): %v", rel, err)
		}
		data, err := os.ReadFile(appPath)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", rel, err)
		}
		if len(data) == 0 {
			t.Fatalf("bundled nasscad asset %s is empty", rel)
		}
		bundledName := filepath.ToSlash(filepath.Join("bundled_apps/nasscad", filepath.Base(rel)))
		bundled, err := bundledAppAssets.ReadFile(bundledName)
		if err != nil {
			t.Fatalf("ReadFile bundled asset %s: %v", bundledName, err)
		}
		if string(data) != string(bundled) {
			t.Fatalf("workspace file %s does not match bundled asset", rel)
		}
	}

	var bundledCount int
	if err := fs.WalkDir(bundledAppAssets, "bundled_apps/nasscad", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			bundledCount++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk bundled assets: %v", err)
	}
	workspaceDir := filepath.Join(root, "Apps", "nasscad")
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		t.Fatalf("ReadDir workspace nasscad: %v", err)
	}
	if len(entries) != bundledCount {
		t.Fatalf("workspace nasscad file count = %d, want %d", len(entries), bundledCount)
	}

	if err := svc.seedBundledBuiltinAppsLocked(ctx); err != nil {
		t.Fatalf("second seedBundledBuiltinAppsLocked: %v", err)
	}
}