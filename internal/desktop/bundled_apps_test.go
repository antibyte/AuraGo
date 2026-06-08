package desktop

import (
	"context"
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
		MaxFileSizeMB: 2,
	})
	ctx := context.Background()

	appPath, err := svc.ResolvePath("Apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	data, err := os.ReadFile(appPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("bundled nasscad app is empty")
	}
	bundled, err := bundledAppAssets.ReadFile("bundled_apps/nasscad/index.html")
	if err != nil {
		t.Fatalf("ReadFile bundled asset: %v", err)
	}
	if string(data) != string(bundled) {
		t.Fatalf("workspace nasscad file does not match bundled asset")
	}

	if err := svc.seedBundledBuiltinAppsLocked(ctx); err != nil {
		t.Fatalf("second seedBundledBuiltinAppsLocked: %v", err)
	}
}