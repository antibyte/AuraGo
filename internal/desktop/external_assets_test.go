package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/webassets"
)

func TestExternalAssetsMissingDoesNotBlockDesktop(t *testing.T) {
	apps, pets := bundledAppAssets, petAssets
	bundledAppAssets = webassets.Namespace("missing")
	petAssets = webassets.Namespace("missing")
	t.Cleanup(func() { bundledAppAssets = apps; petAssets = pets })
	root := t.TempDir()
	svc := testServiceWithConfig(t, Config{Enabled: true, WorkspaceDir: filepath.Join(root, "workspace"), DBPath: filepath.Join(root, "desktop.db"), MaxFileSizeMB: 20})
	if _, err := svc.ListPets(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestExternalAssetsPreserveEditedNasscad(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	file := filepath.Join(workspace, "Apps", "nasscad", "index.html")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	content := []byte("user-owned CAD app")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	testServiceWithConfig(t, Config{Enabled: true, WorkspaceDir: workspace, DBPath: filepath.Join(root, "desktop.db"), MaxFileSizeMB: 20})
	got, err := os.ReadFile(file)
	if err != nil || string(got) != string(content) {
		t.Fatal("user app overwritten", err)
	}
}
