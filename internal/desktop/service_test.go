package desktop

import (
	"context"
	"database/sql"
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

func testMediaService(t *testing.T) *Service {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	dataDir := filepath.Join(t.TempDir(), "data")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	mediaDBPath := filepath.Join(t.TempDir(), "media_registry.db")
	imageDBPath := filepath.Join(t.TempDir(), "image_gallery.db")
	svc, err := NewService(Config{
		Enabled:           true,
		WorkspaceDir:      root,
		DBPath:            dbPath,
		DataDir:           dataDir,
		DocumentDir:       filepath.Join(dataDir, "documents"),
		MediaRegistryPath: mediaDBPath,
		ImageGalleryPath:  imageDBPath,
		MaxFileSizeMB:     1,
		ControlLevel:      ControlConfirmDestructive,
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

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
	for _, name := range workspaceDirectories() {
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, name)); err != nil {
			t.Fatalf("expected workspace directory %s: %v", name, err)
		}
	}
	if len(bootstrap.BuiltinApps) < 4 {
		t.Fatalf("expected builtin desktop apps, got %d", len(bootstrap.BuiltinApps))
	}
}

func TestServiceBootstrapIncludesMediaMountsAndGalleryApp(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, name := range []string{"Music", "Photos", "Videos", "AuraGo Documents"} {
		if !stringSliceContains(bootstrap.Workspace.Directories, name) {
			t.Fatalf("bootstrap directories missing media mount %q: %+v", name, bootstrap.Workspace.Directories)
		}
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, name)); err == nil {
			t.Fatalf("media mount %q must not be created as a workspace folder", name)
		}
	}
	var foundGallery bool
	for _, app := range bootstrap.BuiltinApps {
		if app.ID == "gallery" {
			foundGallery = app.Icon == "image" && app.Entry == "builtin://gallery"
		}
	}
	if !foundGallery {
		t.Fatalf("builtin apps missing gallery app: %+v", bootstrap.BuiltinApps)
	}
}

func TestServiceBootstrapIncludesCodeStudioApp(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var foundCodeStudio bool
	for _, app := range bootstrap.BuiltinApps {
		if app.ID == "code-studio" {
			foundCodeStudio = app.Icon == "code" && app.Entry == "builtin://code-studio"
		}
	}
	if !foundCodeStudio {
		t.Fatalf("builtin apps missing code studio app: %+v", bootstrap.BuiltinApps)
	}
}

func TestServiceBootstrapDefaultDesktopShortcuts(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got := map[string]Shortcut{}
	for _, shortcut := range bootstrap.Shortcuts {
		got[shortcut.ID] = shortcut
	}
	if len(got) != 2 {
		t.Fatalf("default shortcuts = %+v, want files and trash only", bootstrap.Shortcuts)
	}
	if got["app-files"].TargetType != ShortcutTargetApp || got["app-files"].TargetID != "files" {
		t.Fatalf("files shortcut = %+v", got["app-files"])
	}
	if got["dir-Trash"].TargetType != ShortcutTargetDirectory || got["dir-Trash"].Path != "Trash" {
		t.Fatalf("trash shortcut = %+v", got["dir-Trash"])
	}
}

func TestServiceDesktopShortcutsCanHideAndRestoreBuiltinApps(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.RemoveDesktopShortcut(ctx, "app-files", SourceUser); err != nil {
		t.Fatalf("RemoveDesktopShortcut files: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after remove: %v", err)
	}
	for _, shortcut := range bootstrap.Shortcuts {
		if shortcut.ID == "app-files" {
			t.Fatalf("files shortcut returned after removal: %+v", bootstrap.Shortcuts)
		}
	}
	var filesStillInStartMenu bool
	for _, app := range bootstrap.BuiltinApps {
		if app.ID == "files" {
			filesStillInStartMenu = true
		}
	}
	if !filesStillInStartMenu {
		t.Fatalf("builtin app was removed from start menu: %+v", bootstrap.BuiltinApps)
	}
	if err := svc.AddDesktopAppShortcut(ctx, "files", SourceUser); err != nil {
		t.Fatalf("AddDesktopAppShortcut files: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after restore: %v", err)
	}
	var restored bool
	for _, shortcut := range bootstrap.Shortcuts {
		if shortcut.ID == "app-files" && shortcut.TargetID == "files" {
			restored = true
		}
	}
	if !restored {
		t.Fatalf("files shortcut was not restored: %+v", bootstrap.Shortcuts)
	}
}

func TestServiceBootstrapIncludesGeneratedAppIconCatalog(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if bootstrap.IconCatalog.Theme != "papirus" {
		t.Fatalf("icon catalog theme = %q, want papirus", bootstrap.IconCatalog.Theme)
	}
	if bootstrap.IconCatalog.DefaultTheme != "papirus" {
		t.Fatalf("icon catalog default theme = %q, want papirus", bootstrap.IconCatalog.DefaultTheme)
	}
	if bootstrap.IconCatalog.LegacySpritePrefix != "sprite:" {
		t.Fatalf("legacy sprite prefix = %q, want sprite:", bootstrap.IconCatalog.LegacySpritePrefix)
	}
	for _, want := range []string{"analytics", "chat", "cloud", "mail", "notes", "server", "settings", "tools", "weather", "folder", "file-plus", "folder-plus", "refresh", "search", "run", "save", "stop"} {
		if !stringSliceContains(bootstrap.IconCatalog.Preferred, want) {
			t.Fatalf("icon catalog missing preferred icon %q: %+v", want, bootstrap.IconCatalog.Preferred)
		}
	}
	if bootstrap.IconCatalog.Aliases["sparkles"] != "apps" {
		t.Fatalf("sparkles alias = %q, want apps", bootstrap.IconCatalog.Aliases["sparkles"])
	}
	if bootstrap.IconCatalog.Aliases["stats"] != "analytics" {
		t.Fatalf("stats alias = %q, want analytics", bootstrap.IconCatalog.Aliases["stats"])
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

func TestServiceListFilesExposesMediaMountMetadata(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	cfg := svc.Config()
	ctx := context.Background()
	fixtures := []struct {
		mount      string
		subdir     string
		filename   string
		wantKind   string
		wantWeb    string
		wantMIME   string
		baseOnDisk string
	}{
		{"Music", "mixes", "track.mp3", "audio", "/files/audio/mixes/track.mp3", "audio/mpeg", filepath.Join(cfg.DataDir, "audio")},
		{"Photos", "", "sunset.png", "image", "/files/generated_images/sunset.png", "image/png", filepath.Join(cfg.DataDir, "generated_images")},
		{"Videos", "", "clip.mp4", "video", "/files/generated_videos/clip.mp4", "video/mp4", filepath.Join(cfg.DataDir, "generated_videos")},
		{"AuraGo Documents", "", "report.pdf", "document", "/files/documents/report.pdf", "application/pdf", cfg.DocumentDir},
	}
	for _, fixture := range fixtures {
		dir := filepath.Join(fixture.baseOnDisk, filepath.FromSlash(fixture.subdir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir fixture %s: %v", fixture.mount, err)
		}
		if err := os.WriteFile(filepath.Join(dir, fixture.filename), []byte("data"), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", fixture.mount, err)
		}
		rawPath := fixture.mount
		if fixture.subdir != "" {
			rawPath += "/" + fixture.subdir
		}
		files, err := svc.ListFiles(ctx, rawPath)
		if err != nil {
			t.Fatalf("ListFiles(%q): %v", rawPath, err)
		}
		var got FileEntry
		for _, entry := range files {
			if entry.Name == fixture.filename {
				got = entry
				break
			}
		}
		if got.Name == "" {
			t.Fatalf("ListFiles(%q) missing %s: %+v", rawPath, fixture.filename, files)
		}
		if got.Mount != fixture.mount || got.MediaKind != fixture.wantKind || got.WebPath != fixture.wantWeb || got.MIMEType != fixture.wantMIME {
			t.Fatalf("media metadata for %s = %+v", fixture.mount, got)
		}
	}
}

func TestServiceListFilesRecursivePaginatesMediaMounts(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	cfg := svc.Config()
	ctx := context.Background()
	audioDir := filepath.Join(cfg.DataDir, "audio")
	if err := os.MkdirAll(filepath.Join(audioDir, "mixes"), 0o755); err != nil {
		t.Fatalf("mkdir nested audio: %v", err)
	}
	for _, path := range []string{"root.mp3", "mixes/nested.ogg", "mixes/voice.opus"} {
		if err := os.WriteFile(filepath.Join(audioDir, filepath.FromSlash(path)), []byte("audio"), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	first, hasMore, err := svc.ListFilesRecursive(ctx, "Music", 0, 2)
	if err != nil {
		t.Fatalf("ListFilesRecursive first page: %v", err)
	}
	if len(first) != 2 || !hasMore {
		t.Fatalf("first page len=%d hasMore=%v entries=%+v", len(first), hasMore, first)
	}
	second, hasMore, err := svc.ListFilesRecursive(ctx, "Music", 2, 2)
	if err != nil {
		t.Fatalf("ListFilesRecursive second page: %v", err)
	}
	if len(second) != 1 || hasMore {
		t.Fatalf("second page len=%d hasMore=%v entries=%+v", len(second), hasMore, second)
	}
	seen := map[string]bool{}
	for _, entry := range append(first, second...) {
		seen[entry.Path] = true
		if entry.Mount != "Music" || entry.MediaKind != "audio" || entry.WebPath == "" {
			t.Fatalf("recursive media metadata missing: %+v", entry)
		}
	}
	for _, want := range []string{"Music/root.mp3", "Music/mixes/nested.ogg", "Music/mixes/voice.opus"} {
		if !seen[want] {
			t.Fatalf("recursive listing missing %s in %+v %+v", want, first, second)
		}
	}
}

func TestServiceMovePathWithinMediaMountRenamesOriginalAndUpdatesRegistries(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	cfg := svc.Config()
	ctx := context.Background()
	oldAbs := filepath.Join(cfg.DataDir, "generated_images", "old.png")
	newAbs := filepath.Join(cfg.DataDir, "generated_images", "new.png")
	if err := os.WriteFile(oldAbs, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	mediaDB := openTestSQLite(t, cfg.MediaRegistryPath)
	initMediaRegistrySchema(t, mediaDB)
	insertMediaItem(t, mediaDB, "image", "old.png", oldAbs, "/files/generated_images/old.png")
	imageDB := openTestSQLite(t, cfg.ImageGalleryPath)
	initImageGallerySchema(t, imageDB)
	insertGeneratedImage(t, imageDB, "old.png")

	if err := svc.MovePath(ctx, "Photos/old.png", "Photos/new.png", SourceUser); err != nil {
		t.Fatalf("MovePath media mount: %v", err)
	}
	if _, err := os.Stat(newAbs); err != nil {
		t.Fatalf("renamed original file missing: %v", err)
	}
	if _, err := os.Stat(oldAbs); !os.IsNotExist(err) {
		t.Fatalf("old original still exists or unexpected error: %v", err)
	}
	assertSingleString(t, mediaDB, "SELECT filename FROM media_items WHERE file_path = ?", newAbs, "new.png")
	assertSingleString(t, mediaDB, "SELECT web_path FROM media_items WHERE file_path = ?", newAbs, "/files/generated_images/new.png")
	assertSingleString(t, imageDB, "SELECT filename FROM generated_images LIMIT 1", nil, "new.png")
}

func TestServiceDeletePathWithinMediaMountDeletesOriginalAndSoftDeletesRegistry(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	cfg := svc.Config()
	ctx := context.Background()
	absPath := filepath.Join(cfg.DataDir, "generated_videos", "clip.mp4")
	if err := os.WriteFile(absPath, []byte("mp4"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	mediaDB := openTestSQLite(t, cfg.MediaRegistryPath)
	initMediaRegistrySchema(t, mediaDB)
	insertMediaItem(t, mediaDB, "video", "clip.mp4", absPath, "/files/generated_videos/clip.mp4")

	if err := svc.DeletePath(ctx, "Videos/clip.mp4", SourceUser); err != nil {
		t.Fatalf("DeletePath media mount: %v", err)
	}
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("video still exists or unexpected error: %v", err)
	}
	var deleted int
	if err := mediaDB.QueryRow("SELECT deleted FROM media_items WHERE filename = 'clip.mp4'").Scan(&deleted); err != nil {
		t.Fatalf("query media deletion flag: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
}

func TestServiceRejectsMediaMountTraversalAndCrossMountRename(t *testing.T) {
	t.Parallel()

	svc := testMediaService(t)
	cfg := svc.Config()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "audio", "song.mp3"), []byte("mp3"), 0o644); err != nil {
		t.Fatalf("write song: %v", err)
	}
	if _, err := svc.ListFiles(ctx, "Music/../../"); err == nil {
		t.Fatal("expected media mount traversal to be rejected")
	}
	if err := svc.MovePath(ctx, "Music/song.mp3", "Photos/song.mp3", SourceUser); err == nil {
		t.Fatal("expected cross-mount rename to be rejected")
	}
	if err := svc.DeletePath(ctx, "Music", SourceUser); err == nil {
		t.Fatal("expected deleting media mount root to be rejected")
	}
}

func TestServiceCreateMoveAndDeletePath(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.CreateDirectory(ctx, "Documents/Projects", SourceUser); err != nil {
		t.Fatalf("CreateDirectory: %v", err)
	}
	if err := svc.WriteFile(ctx, "Documents/Projects/note.txt", "hello", SourceUser); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := svc.MovePath(ctx, "Documents/Projects/note.txt", "Documents/Projects/renamed.txt", SourceUser); err != nil {
		t.Fatalf("MovePath: %v", err)
	}
	if _, _, err := svc.ReadFile(ctx, "Documents/Projects/renamed.txt"); err != nil {
		t.Fatalf("ReadFile renamed path: %v", err)
	}
	if err := svc.DeletePath(ctx, "Documents/Projects", SourceUser); err != nil {
		t.Fatalf("DeletePath: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "Documents", "Projects")); !os.IsNotExist(err) {
		t.Fatalf("Projects directory still exists or unexpected stat error: %v", err)
	}
}

func TestServiceRejectsDeletingWorkspaceRoot(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	if err := svc.DeletePath(context.Background(), ".", SourceUser); err == nil {
		t.Fatal("expected deleting workspace root to be rejected")
	}
}

func TestServiceSettingsUseDefaultsAndValidateWrites(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if bootstrap.Settings["appearance.wallpaper"] != "aurora" {
		t.Fatalf("default wallpaper = %q", bootstrap.Settings["appearance.wallpaper"])
	}
	if bootstrap.Settings["appearance.icon_theme"] != "papirus" {
		t.Fatalf("default icon theme = %q", bootstrap.Settings["appearance.icon_theme"])
	}
	if err := svc.SetSetting(ctx, "appearance.wallpaper", "forest", SourceUser); err != nil {
		t.Fatalf("SetSetting valid: %v", err)
	}
	if err := svc.SetSetting(ctx, "appearance.icon_theme", "aurago", SourceUser); err != nil {
		t.Fatalf("SetSetting icon theme valid: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after setting: %v", err)
	}
	if bootstrap.Settings["appearance.wallpaper"] != "forest" {
		t.Fatalf("stored wallpaper = %q", bootstrap.Settings["appearance.wallpaper"])
	}
	if bootstrap.Settings["appearance.icon_theme"] != "aurago" {
		t.Fatalf("stored icon theme = %q", bootstrap.Settings["appearance.icon_theme"])
	}
	if err := svc.SetSetting(ctx, "appearance.wallpaper", "../../bad", SourceUser); err == nil {
		t.Fatal("expected invalid setting value to be rejected")
	}
	if err := svc.SetSetting(ctx, "appearance.icon_theme", "unknown", SourceUser); err == nil {
		t.Fatal("expected invalid icon theme to be rejected")
	}
	if err := svc.SetSetting(ctx, "unknown.setting", "true", SourceUser); err == nil {
		t.Fatal("expected unknown setting key to be rejected")
	}
}

func TestServiceRejectsEmptyStandaloneWidgetHTML(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.WriteFile(context.Background(), "Widgets/weather_pforzheim.html", " \n\t", SourceAgent)
	if err == nil {
		t.Fatal("expected empty standalone widget HTML to be rejected")
	}
	if !strings.Contains(err.Error(), "desktop widget HTML file must not be empty") {
		t.Fatalf("error = %q, want empty widget HTML rejection", err)
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

func TestServiceDeleteAppRemovesGeneratedAppShortcutAndFiles(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	manifest := AppManifest{
		ID:      "quick-notes",
		Name:    "Quick Notes",
		Version: "1.0.0",
		Icon:    "note",
		Entry:   "index.html",
	}
	files := map[string]string{"index.html": "<main>Quick Notes</main>"}
	if err := svc.InstallApp(ctx, manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	if err := svc.AddDesktopAppShortcut(ctx, "quick-notes", SourceUser); err != nil {
		t.Fatalf("AddDesktopAppShortcut: %v", err)
	}
	if err := svc.DeleteApp(ctx, "quick-notes", SourceUser); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, app := range bootstrap.InstalledApps {
		if app.ID == "quick-notes" {
			t.Fatalf("deleted app still in start menu: %+v", bootstrap.InstalledApps)
		}
	}
	for _, shortcut := range bootstrap.Shortcuts {
		if shortcut.TargetID == "quick-notes" {
			t.Fatalf("deleted app shortcut still pinned: %+v", bootstrap.Shortcuts)
		}
	}
	if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, "Apps", "quick-notes")); !os.IsNotExist(err) {
		t.Fatalf("app files still exist or unexpected stat error: %v", err)
	}
}

func TestServiceDeleteAppRejectsBuiltinApps(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.DeleteApp(context.Background(), "files", SourceUser)
	if err == nil {
		t.Fatal("expected deleting builtin app to be rejected")
	}
	if !strings.Contains(err.Error(), "built-in desktop apps cannot be deleted") {
		t.Fatalf("error = %q, want builtin rejection", err)
	}
}

func TestServiceInstallAppInfersIconAndRegistersSDKRuntime(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	manifest := AppManifest{
		ID:          "sdk-notes",
		Name:        "SDK Notes",
		Version:     "1.0.0",
		Entry:       "index.html",
		Permissions: []string{" files:read ", "", "widgets:write"},
	}
	files := map[string]string{"index.html": "<main id=\"app\"></main>"}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var got AppManifest
	for _, app := range bootstrap.InstalledApps {
		if app.ID == "sdk-notes" {
			got = app
			break
		}
	}
	if got.ID == "" {
		t.Fatalf("installed app was not registered: %+v", bootstrap.InstalledApps)
	}
	if got.Runtime != AuraDesktopRuntime {
		t.Fatalf("runtime = %q, want %q", got.Runtime, AuraDesktopRuntime)
	}
	if got.Icon != "notes" {
		t.Fatalf("icon = %q, want notes", got.Icon)
	}
	if len(got.Permissions) != 2 || got.Permissions[0] != "files:read" || got.Permissions[1] != "widgets:write" {
		t.Fatalf("permissions were not normalized: %#v", got.Permissions)
	}
}

func TestServiceInstallAppNormalizesIconAliasesAndRejectsEmoji(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	files := map[string]string{"index.html": "<main id=\"app\"></main>"}
	manifest := AppManifest{
		ID:      "spark-app",
		Name:    "Spark App",
		Version: "1.0.0",
		Icon:    "sparkles",
		Entry:   "index.html",
	}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp alias: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var got AppManifest
	for _, app := range bootstrap.InstalledApps {
		if app.ID == "spark-app" {
			got = app
			break
		}
	}
	if got.Icon != "apps" {
		t.Fatalf("normalized icon = %q, want apps", got.Icon)
	}

	manifest.ID = "emoji-app"
	manifest.Icon = "📝"
	err = svc.InstallApp(context.Background(), manifest, files, SourceAgent)
	if err == nil {
		t.Fatal("expected emoji app icon to be rejected")
	}
	if !strings.Contains(err.Error(), "desktop app icon must use") {
		t.Fatalf("error = %q, want icon catalog guidance", err)
	}

	manifest.ID = "sprite-app"
	manifest.Icon = "sprite:terminal"
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp sprite icon: %v", err)
	}
}

func TestServiceInstallAppInfersMissingIconFromManifestIdentity(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	files := map[string]string{"index.html": "<main>Weather</main>"}
	manifest := AppManifest{
		ID:      "weather-dashboard",
		Name:    "Weather Dashboard",
		Version: "1.0.0",
		Entry:   "index.html",
	}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp without icon: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, app := range bootstrap.InstalledApps {
		if app.ID == "weather-dashboard" {
			if app.Icon != "weather" {
				t.Fatalf("inferred icon = %q, want weather", app.Icon)
			}
			return
		}
	}
	t.Fatalf("weather-dashboard was not installed: %+v", bootstrap.InstalledApps)
}

func TestServiceInstallAppRejectsEmptyEntryFile(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	manifest := AppManifest{
		ID:      "empty-entry",
		Name:    "Empty Entry",
		Version: "1.0.0",
		Icon:    "note",
		Entry:   "index.html",
	}
	files := map[string]string{"index.html": "   \n\t"}
	err := svc.InstallApp(context.Background(), manifest, files, SourceAgent)
	if err == nil {
		t.Fatal("expected empty app entry file to be rejected")
	}
	if !strings.Contains(err.Error(), "entry file must not be empty") {
		t.Fatalf("error = %q, want empty entry file rejection", err)
	}
}

func TestServiceUpsertWidgetRegistersRuntimeIconAndEntry(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	manifest := AppManifest{
		ID:      "calendar-widget-host",
		Name:    "Calendar Widget Host",
		Version: "1.0.0",
		Icon:    "calendar",
		Entry:   "index.html",
	}
	files := map[string]string{
		"index.html":  "<main>Calendar</main>",
		"widget.html": "<main>Today</main>",
	}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	widget := Widget{
		ID:          "today-widget",
		AppID:       "calendar-widget-host",
		Title:       "Today",
		Icon:        "calendar",
		Entry:       "widget.html",
		Permissions: []string{"notifications", " widgets:write "},
		Config:      map[string]interface{}{"dense": true},
	}
	if err := svc.UpsertWidget(context.Background(), widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var got Widget
	for _, item := range bootstrap.Widgets {
		if item.ID == "today-widget" {
			got = item
			break
		}
	}
	if got.ID == "" {
		t.Fatalf("widget was not registered: %+v", bootstrap.Widgets)
	}
	if got.Type != "custom" {
		t.Fatalf("type = %q, want custom", got.Type)
	}
	if got.Runtime != AuraDesktopRuntime {
		t.Fatalf("runtime = %q, want %q", got.Runtime, AuraDesktopRuntime)
	}
	if got.Icon != "calendar" {
		t.Fatalf("icon = %q, want calendar", got.Icon)
	}
	if got.Entry != "widget.html" {
		t.Fatalf("entry = %q, want widget.html", got.Entry)
	}
	if len(got.Permissions) != 2 || got.Permissions[0] != "notifications" || got.Permissions[1] != "widgets:write" {
		t.Fatalf("permissions were not normalized: %#v", got.Permissions)
	}
	if _, ok := got.Config["dense"]; !ok {
		t.Fatalf("config was not preserved: %#v", got.Config)
	}
}

func TestServiceUpsertWidgetNormalizesIconAliasesAndRejectsEmoji(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	widget := Widget{
		ID:    "music-widget",
		Title: "Music Widget",
		Icon:  "music-player",
	}
	if err := svc.UpsertWidget(context.Background(), widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget alias: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var got Widget
	for _, item := range bootstrap.Widgets {
		if item.ID == "music-widget" {
			got = item
			break
		}
	}
	if got.Icon != "audio" {
		t.Fatalf("normalized widget icon = %q, want audio", got.Icon)
	}

	widget.ID = "emoji-widget"
	widget.Icon = "🎵"
	err = svc.UpsertWidget(context.Background(), widget, SourceAgent)
	if err == nil {
		t.Fatal("expected emoji widget icon to be rejected")
	}
	if !strings.Contains(err.Error(), "desktop widget icon must use") {
		t.Fatalf("error = %q, want icon catalog guidance", err)
	}
}

func TestServiceUpsertWidgetInfersMissingIconFromWidgetIdentity(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	widget := Widget{
		ID:    "quick_notes",
		Title: "Quick Notes",
	}
	if err := svc.UpsertWidget(context.Background(), widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget without icon: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, item := range bootstrap.Widgets {
		if item.ID == "quick_notes" {
			if item.Icon != "notes" {
				t.Fatalf("inferred widget icon = %q, want notes", item.Icon)
			}
			return
		}
	}
	t.Fatalf("quick_notes widget was not registered: %+v", bootstrap.Widgets)
}

func TestServiceUpsertWidgetRegistersStandaloneEntry(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	if err := svc.WriteFile(context.Background(), "Widgets/weather_pforzheim.html", "<main>Weather</main>", SourceAgent); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	widget := Widget{
		ID:    "weather-pforzheim",
		Title: "Weather Pforzheim",
		Icon:  "weather",
		Entry: "weather_pforzheim.html",
	}
	if err := svc.UpsertWidget(context.Background(), widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget: %v", err)
	}
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var got Widget
	for _, item := range bootstrap.Widgets {
		if item.ID == "weather-pforzheim" {
			got = item
			break
		}
	}
	if got.ID == "" {
		t.Fatalf("standalone widget was not registered: %+v", bootstrap.Widgets)
	}
	if got.AppID != "" {
		t.Fatalf("app_id = %q, want empty standalone widget app_id", got.AppID)
	}
	if got.Entry != "weather_pforzheim.html" {
		t.Fatalf("entry = %q, want weather_pforzheim.html", got.Entry)
	}
}

func TestServiceUpsertWidgetRejectsMissingOrEmptyEntryFile(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	manifest := AppManifest{
		ID:      "weather",
		Name:    "Weather",
		Version: "1.0.0",
		Icon:    "weather",
		Entry:   "index.html",
	}
	files := map[string]string{
		"index.html":  "<main>Weather</main>",
		"widget.html": " \n\t",
	}
	if err := svc.InstallApp(context.Background(), manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}

	err := svc.UpsertWidget(context.Background(), Widget{
		ID:    "missing-weather-widget",
		AppID: "weather",
		Title: "Weather",
		Icon:  "weather",
		Entry: "missing.html",
	}, SourceAgent)
	if err == nil {
		t.Fatal("expected missing widget entry file to be rejected")
	}
	if !strings.Contains(err.Error(), "widget entry file is missing") {
		t.Fatalf("error = %q, want missing widget entry rejection", err)
	}

	err = svc.UpsertWidget(context.Background(), Widget{
		ID:    "empty-weather-widget",
		AppID: "weather",
		Title: "Weather",
		Icon:  "weather",
		Entry: "widget.html",
	}, SourceAgent)
	if err == nil {
		t.Fatal("expected empty widget entry file to be rejected")
	}
	if !strings.Contains(err.Error(), "widget entry file must not be empty") {
		t.Fatalf("error = %q, want empty widget entry rejection", err)
	}

	err = svc.UpsertWidget(context.Background(), Widget{
		ID:    "missing-standalone-widget",
		Title: "Weather",
		Icon:  "weather",
		Entry: "missing.html",
	}, SourceAgent)
	if err == nil {
		t.Fatal("expected missing standalone widget entry file to be rejected")
	}
	if !strings.Contains(err.Error(), "widget entry file is missing") {
		t.Fatalf("error = %q, want standalone missing widget entry rejection", err)
	}

}

func TestServiceUpsertWidgetRejectsUnsafeEntry(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.UpsertWidget(context.Background(), Widget{
		ID:    "bad-widget",
		AppID: "calendar",
		Title: "Bad",
		Icon:  "calendar",
		Entry: "../widget.html",
	}, SourceAgent)
	if err == nil {
		t.Fatal("expected widget entry escape to be rejected")
	}
}

func TestServiceUpsertWidgetRejectsUnsafeAppID(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.UpsertWidget(context.Background(), Widget{
		ID:    "unsafe-widget",
		AppID: "../apps",
		Title: "Unsafe",
		Icon:  "calendar",
	}, SourceAgent)
	if err == nil {
		t.Fatal("expected unsafe widget app_id to be rejected")
	}
}

func openTestSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func initMediaRegistrySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS media_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		media_type TEXT NOT NULL DEFAULT 'image',
		filename TEXT NOT NULL,
		file_path TEXT NOT NULL DEFAULT '',
		web_path TEXT NOT NULL DEFAULT '',
		deleted INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("init media registry schema: %v", err)
	}
}

func initImageGallerySchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS generated_images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatalf("init image gallery schema: %v", err)
	}
}

func insertMediaItem(t *testing.T, db *sql.DB, mediaType, filename, filePath, webPath string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO media_items(media_type, filename, file_path, web_path, deleted) VALUES (?, ?, ?, ?, 0)`, mediaType, filename, filePath, webPath); err != nil {
		t.Fatalf("insert media item: %v", err)
	}
}

func insertGeneratedImage(t *testing.T, db *sql.DB, filename string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO generated_images(filename) VALUES (?)`, filename); err != nil {
		t.Fatalf("insert generated image: %v", err)
	}
}

func assertSingleString(t *testing.T, db *sql.DB, query string, arg interface{}, want string) {
	t.Helper()
	var got string
	var err error
	if arg == nil {
		err = db.QueryRow(query).Scan(&got)
	} else {
		err = db.QueryRow(query, arg).Scan(&got)
	}
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("%q returned %q, want %q", query, got, want)
	}
}
