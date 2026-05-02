package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	svc, err := NewService(Config{
		Enabled:            true,
		WorkspaceDir:       root,
		DBPath:             dbPath,
		MaxFileSizeMB:      1,
		AllowGeneratedApps: true,
		AllowAgentControl:  true,
		ControlLevel:       ControlConfirmDestructive,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestServiceBootstrapCreatesWorkspaceFolders(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if !bootstrap.Enabled {
		t.Fatal("desktop should be enabled")
	}
	for _, name := range DefaultDirectories() {
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, name)); err != nil {
			t.Fatalf("expected workspace directory %s: %v", name, err)
		}
	}
	if len(bootstrap.BuiltinApps) < 4 {
		t.Fatalf("expected builtin desktop apps, got %d", len(bootstrap.BuiltinApps))
	}
}

func TestServiceRejectsWorkspaceEscape(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	if _, err := svc.ResolvePath("../config.yaml"); err == nil {
		t.Fatal("expected parent-directory escape to be rejected")
	}
	if _, err := svc.ResolvePath(filepath.Join(svc.Config().WorkspaceDir, "..", "outside.txt")); err == nil {
		t.Fatal("expected absolute path escape to be rejected")
	}
	if err := svc.WriteFile(context.Background(), "../outside.txt", "nope", SourceAgent); err == nil {
		t.Fatal("expected write escape to be rejected")
	}
}

func TestServiceWritesAndReadsFilesInsideWorkspace(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	if err := svc.WriteFile(context.Background(), `Documents\hello.txt`, "hello desktop", SourceAgent); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, entry, err := svc.ReadFile(context.Background(), "Documents/hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if content != "hello desktop" {
		t.Fatalf("content = %q", content)
	}
	if entry.Path != "Documents/hello.txt" {
		t.Fatalf("entry path = %q", entry.Path)
	}
}

func TestServiceInstallAppPersistsManifestAndFiles(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	manifest := AppManifest{
		ID:          "quick-notes",
		Name:        "Quick Notes",
		Version:     "1.0.0",
		Icon:        "note",
		Entry:       "index.html",
		Description: "Small generated note app.",
	}
	files := map[string]string{
		"index.html": "<main>Quick Notes</main>",
		"app.js":     "window.quickNotes = true;",
	}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var found bool
	for _, app := range bootstrap.InstalledApps {
		if app.ID == "quick-notes" && app.Entry == "index.html" {
			found = true
		}
	}
	if !found {
		t.Fatalf("installed app was not returned in bootstrap: %+v", bootstrap.InstalledApps)
	}
	appPath, err := svc.ResolvePath("Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if !strings.HasPrefix(appPath, svc.Config().WorkspaceDir) {
		t.Fatalf("app path escaped workspace: %s", appPath)
	}
	if _, err := os.Stat(appPath); err != nil {
		t.Fatalf("installed app file missing: %v", err)
	}
}
