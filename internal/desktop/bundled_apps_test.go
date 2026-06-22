package desktop

import (
	"bytes"
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
		MaxFileSizeMB: 20,
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
	if !bytesContainsNasscadMonolithMarkers(data) {
		t.Fatal("seeded nasscad index is not monolithic")
	}
	if !bytes.Contains(data, []byte("NASSCAD V4.3.0")) {
		t.Fatal("seeded nasscad index should contain NASSCAD V4.3.0")
	}
	if nasscadExternalScriptPattern.Match(data) {
		t.Fatal("seeded nasscad index still contains external script tags")
	}

	if err := svc.seedBundledBuiltinAppsLocked(ctx); err != nil {
		t.Fatalf("second seedBundledBuiltinAppsLocked: %v", err)
	}
}
