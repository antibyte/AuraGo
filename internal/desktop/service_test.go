package desktop

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	svc.SetIntegritySecretStore(newTestIntegritySecretStore())
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
	svc.SetIntegritySecretStore(newTestIntegritySecretStore())
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func testServiceWithConfig(t *testing.T, cfg Config) *Service {
	t.Helper()
	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetIntegritySecretStore(newTestIntegritySecretStore())
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

func boolPtr(value bool) *bool {
	return &value
}

type testIntegritySecretStore struct {
	values map[string]string
}

func newTestIntegritySecretStore() *testIntegritySecretStore {
	return &testIntegritySecretStore{values: map[string]string{}}
}

func (s *testIntegritySecretStore) ReadSecret(key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", os.ErrNotExist
	}
	return value, nil
}

func (s *testIntegritySecretStore) WriteSecret(key, value string) error {
	s.values[key] = value
	return nil
}

func testFindApp(t *testing.T, apps []AppManifest, id string) AppManifest {
	t.Helper()
	for _, app := range apps {
		if app.ID == id {
			return app
		}
	}
	t.Fatalf("app %q not found in %+v", id, apps)
	return AppManifest{}
}

func TestBuiltinAppsExposeCheaterAndMissionControlMetadata(t *testing.T) {
	apps := BuiltinApps()

	cheater := testFindApp(t, apps, "cheater")
	if !cheater.Builtin || cheater.Deletable || !cheater.DockVisible || !cheater.StartVisible {
		t.Fatalf("cheater visibility = builtin:%v deletable:%v dock:%v start:%v, want first-party visible app", cheater.Builtin, cheater.Deletable, cheater.DockVisible, cheater.StartVisible)
	}
	if cheater.Entry != "builtin://cheater" || cheater.Icon != "cheater" {
		t.Fatalf("cheater manifest entry/icon = %q/%q, want builtin://cheater/cheater", cheater.Entry, cheater.Icon)
	}

	missionControl := testFindApp(t, apps, "mission-control")
	if missionControl.Icon != "workflow" {
		t.Fatalf("mission-control icon = %q, want workflow", missionControl.Icon)
	}

	nasscad := testFindApp(t, apps, "nasscad")
	if nasscad.Entry != "builtin://nasscad" || nasscad.Icon != "nasscad" {
		t.Fatalf("nasscad manifest entry/icon = %q/%q, want builtin://nasscad/nasscad", nasscad.Entry, nasscad.Icon)
	}
	if nasscad.Version != "4.3.0" {
		t.Fatalf("nasscad version = %q, want 4.3.0", nasscad.Version)
	}
	if nasscad.Metadata["open_maximized"] != "true" {
		t.Fatalf("nasscad must open maximized for CAD workspace: %#v", nasscad.Metadata)
	}
	if nasscad.Metadata["workspace_entry"] != "Apps/nasscad/index.html" {
		t.Fatalf("nasscad workspace entry = %q, want Apps/nasscad/index.html", nasscad.Metadata["workspace_entry"])
	}
}

func TestBuiltinVirtualComputersOpensMaximized(t *testing.T) {
	virtualComputers := testFindApp(t, BuiltinApps(), "virtual-computers")
	if virtualComputers.Metadata["open_maximized"] != "true" {
		t.Fatalf("virtual computers must open maximized: %#v", virtualComputers.Metadata)
	}
}

func TestBuiltinAppsExposeLiveSpeech(t *testing.T) {
	liveSpeech := testFindApp(t, BuiltinApps(), "live-speech")
	if !liveSpeech.Builtin || liveSpeech.Deletable || !liveSpeech.DockVisible || !liveSpeech.StartVisible {
		t.Fatalf("live speech visibility = builtin:%v deletable:%v dock:%v start:%v, want first-party visible app", liveSpeech.Builtin, liveSpeech.Deletable, liveSpeech.DockVisible, liveSpeech.StartVisible)
	}
	if liveSpeech.Entry != "builtin://live-speech" || liveSpeech.Icon != "audio" || liveSpeech.Runtime != BuiltinRuntime {
		t.Fatalf("live speech manifest = entry:%q icon:%q runtime:%q", liveSpeech.Entry, liveSpeech.Icon, liveSpeech.Runtime)
	}
}

func TestBuiltinAppsExposeNetworkCamerasMetadata(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "network-cameras")
	if !app.Builtin || app.Deletable || !app.DockVisible || !app.StartVisible {
		t.Fatalf("network cameras visibility = builtin:%v deletable:%v dock:%v start:%v, want first-party visible app", app.Builtin, app.Deletable, app.DockVisible, app.StartVisible)
	}
	if app.Entry != "builtin://network-cameras" || app.Icon != "camera" || app.Runtime != BuiltinRuntime {
		t.Fatalf("network cameras manifest = entry:%q icon:%q runtime:%q", app.Entry, app.Icon, app.Runtime)
	}
	if !stringSliceContains(app.Permissions, "notifications") {
		t.Fatalf("network cameras permissions = %#v, want notifications", app.Permissions)
	}
}

func TestBuiltinAppsExposeTeeVeeMetadata(t *testing.T) {
	apps := BuiltinApps()

	teevee := testFindApp(t, apps, "teevee")
	if !teevee.Builtin || teevee.Deletable || !teevee.DockVisible || !teevee.StartVisible {
		t.Fatalf("teevee visibility = builtin:%v deletable:%v dock:%v start:%v, want first-party visible app", teevee.Builtin, teevee.Deletable, teevee.DockVisible, teevee.StartVisible)
	}
	if teevee.Name != "TeeVee" {
		t.Fatalf("teevee name = %q, want TeeVee", teevee.Name)
	}
	if teevee.Icon != "teevee" || teevee.Entry != "builtin://teevee" || teevee.Runtime != BuiltinRuntime {
		t.Fatalf("teevee manifest icon/entry/runtime = %q/%q/%q, want teevee/builtin://teevee/%q", teevee.Icon, teevee.Entry, teevee.Runtime, BuiltinRuntime)
	}
	if !strings.Contains(strings.ToLower(teevee.Description), "iptv") {
		t.Fatalf("teevee description should mention IPTV source, got %q", teevee.Description)
	}
}

func TestBuiltinAppsExposeChessMetadata(t *testing.T) {
	apps := BuiltinApps()

	chess := testFindApp(t, apps, "chess")
	if !chess.Builtin || chess.Deletable || !chess.DockVisible || !chess.StartVisible {
		t.Fatalf("chess visibility = builtin:%v deletable:%v dock:%v start:%v, want first-party visible app", chess.Builtin, chess.Deletable, chess.DockVisible, chess.StartVisible)
	}
	if chess.Name != "Chess" {
		t.Fatalf("chess name = %q, want Chess", chess.Name)
	}
	if chess.Icon != "chess" || chess.Entry != "builtin://chess" || chess.Runtime != BuiltinRuntime {
		t.Fatalf("chess manifest icon/entry/runtime = %q/%q/%q, want chess/builtin://chess/%q", chess.Icon, chess.Entry, chess.Runtime, BuiltinRuntime)
	}
	if !strings.Contains(strings.ToLower(chess.Description), "chess") {
		t.Fatalf("chess description should mention chess, got %q", chess.Description)
	}
}

func TestServiceMutationLockIsSharedAcrossServices(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	cfg := Config{
		Enabled:       true,
		WorkspaceDir:  root,
		DBPath:        dbPath,
		MaxFileSizeMB: 1,
		ControlLevel:  ControlConfirmDestructive,
	}
	svc1 := testServiceWithConfig(t, cfg)
	svc2 := testServiceWithConfig(t, cfg)

	entered := make(chan struct{})
	release := make(chan struct{})
	conditionalDone := make(chan error, 1)
	go func() {
		_, err := svc1.WriteFileBytesConditional(context.Background(), "Documents/shared.txt", []byte("conditional"), SourceUser, func(FileWriteState) error {
			close(entered)
			<-release
			return nil
		})
		conditionalDone <- err
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("conditional write precondition did not start")
	}

	directDone := make(chan error, 1)
	go func() {
		directDone <- svc2.WriteFileBytes(context.Background(), "Documents/shared.txt", []byte("direct"), SourceAgent)
	}()

	select {
	case err := <-directDone:
		t.Fatalf("second service write completed before shared mutation lock was released: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-conditionalDone; err != nil {
		t.Fatalf("conditional write: %v", err)
	}
	if err := <-directDone; err != nil {
		t.Fatalf("direct write: %v", err)
	}
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
	if bootstrap.Workspace.Root != "/" {
		t.Fatalf("workspace root = %q, want opaque browser root", bootstrap.Workspace.Root)
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

func TestCleanupStaleDeletesOnlyRemovesStagedDeleteMarkers(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	workspace := svc.Config().WorkspaceDir
	staged := filepath.Join(workspace, "old-app.delete-20260530123456.123456789")
	userFile := filepath.Join(workspace, "notes.delete-draft")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		t.Fatalf("create staged delete dir: %v", err)
	}
	if err := os.WriteFile(userFile, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("create user file: %v", err)
	}

	svc.cleanupStaleDeletes()

	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("staged delete marker still exists or stat failed differently: %v", err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Fatalf("user file with .delete- in name should remain: %v", err)
	}
}

func TestMediaTypeSQLPredicateUsesPlaceholders(t *testing.T) {
	t.Parallel()

	clause, args := mediaTypeSQLPredicate("audio")
	if strings.Contains(clause, "'") {
		t.Fatalf("media predicate must use placeholders, got %q", clause)
	}
	if clause != "media_type IN (?, ?)" {
		t.Fatalf("audio predicate = %q, want placeholder IN clause", clause)
	}
	if len(args) != 2 || args[0] != "audio" || args[1] != "music" {
		t.Fatalf("audio predicate args = %#v", args)
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
			foundGallery = app.Icon == "gallery" && app.Entry == "builtin://gallery"
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
			foundCodeStudio = app.Icon == "code-studio" && app.Entry == "builtin://code-studio"
		}
	}
	if !foundCodeStudio {
		t.Fatalf("builtin apps missing code studio app: %+v", bootstrap.BuiltinApps)
	}
}

func TestServiceBootstrapUsesDistinctBuiltinAppIcons(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got := map[string]AppManifest{}
	for _, app := range bootstrap.BuiltinApps {
		got[app.ID] = app
	}
	for id, wantIcon := range map[string]string{
		"writer":         "writer",
		"launchpad":      "launchpad",
		"gallery":        "gallery",
		"music-player":   "audio-player",
		"radio":          "radio",
		"teevee":         "teevee",
		"agent-chat":     "agent-chat",
		"code-studio":    "code-studio",
		"looper":         "looper",
		"software-store": "software-store",
		"zipper":         "zipper",
		"pixel":          "pixel",
	} {
		if got[id].Icon != wantIcon {
			t.Fatalf("builtin app %q icon = %q, want %q", id, got[id].Icon, wantIcon)
		}
	}
}

func TestServiceBootstrapIncludesOfficeApps(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	got := map[string]AppManifest{}
	for _, app := range bootstrap.BuiltinApps {
		got[app.ID] = app
	}
	if got["writer"].Icon != "writer" || got["writer"].Entry != "builtin://writer" {
		t.Fatalf("writer app = %+v", got["writer"])
	}
	if got["sheets"].Icon != "spreadsheet" || got["sheets"].Entry != "builtin://sheets" {
		t.Fatalf("sheets app = %+v", got["sheets"])
	}
}

func TestServiceBinaryFileRoundTrip(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	want := []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0xff, 0x10, 0x80, 0x00, 0x01}
	if err := svc.WriteFileBytes(ctx, "Documents/book.xlsx", want, SourceUser); err != nil {
		t.Fatalf("WriteFileBytes: %v", err)
	}
	got, entry, err := svc.ReadFileBytes(ctx, "Documents/book.xlsx")
	if err != nil {
		t.Fatalf("ReadFileBytes: %v", err)
	}
	if entry.Name != "book.xlsx" || entry.Path != "Documents/book.xlsx" {
		t.Fatalf("entry = %+v", entry)
	}
	if string(got) != string(want) {
		t.Fatalf("bytes = %v, want %v", got, want)
	}
	if _, _, err := svc.ReadFile(ctx, "Documents/book.xlsx"); err == nil {
		t.Fatal("ReadFile should reject binary office files; use ReadFileBytes")
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
	for _, want := range []string{"analytics", "chat", "chess", "cloud", "mail", "notes", "server", "settings", "tools", "weather", "software-store", "pixel", "zipper", "trash-empty", "trash-full", "folder", "file-plus", "folder-plus", "refresh", "search", "run", "save", "stop", "eye", "heart", "maximize", "minus", "redo", "undo", "zoom-in", "zoom-out", "zoom-reset"} {
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
	for category, icons := range map[string][]string{
		"games":        {"chess", "run", "video", "apps"},
		"office":       {"writer", "spreadsheet", "calendar"},
		"tools":        {"tools", "settings", "terminal"},
		"productivity": {"notes", "check-square", "workflow"},
	} {
		got := bootstrap.IconCatalog.Categories[category]
		for _, want := range icons {
			if !stringSliceContains(got, want) {
				t.Fatalf("icon catalog category %q missing %q: %+v", category, want, got)
			}
		}
	}
	if bootstrap.IconCatalog.Aliases["game"] != "run" {
		t.Fatalf("game alias = %q, want run", bootstrap.IconCatalog.Aliases["game"])
	}
	if bootstrap.IconCatalog.Aliases["space-invaders"] != "run" {
		t.Fatalf("space-invaders alias = %q, want run", bootstrap.IconCatalog.Aliases["space-invaders"])
	}
	if bootstrap.IconCatalog.Aliases["board-game"] != "chess" {
		t.Fatalf("board-game alias = %q, want chess", bootstrap.IconCatalog.Aliases["board-game"])
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

func TestServiceRejectsSymlinkEscapeForMissingChild(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	root := svc.Config().WorkspaceDir
	outside := t.TempDir()
	link := filepath.Join(root, "Documents", "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := svc.ResolvePath("Documents/outside-link/new.txt"); err == nil {
		t.Fatal("expected symlink escape with missing child to be rejected")
	}
	if err := svc.WriteFile(context.Background(), "Documents/outside-link/new.txt", "nope", SourceAgent); err == nil {
		t.Fatal("expected write through symlink escape to be rejected")
	}
}

func TestServiceRejectsBrokenSymlinkTraversal(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	root := svc.Config().WorkspaceDir
	link := filepath.Join(root, "Documents", "broken-link")
	if err := os.Symlink(filepath.Join(root, "missing-target"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := svc.ResolvePath("Documents/broken-link/new.txt"); err == nil {
		t.Fatal("expected broken symlink traversal to be rejected")
	}
}

func TestServiceRejectsSymlinkLoopTraversal(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	root := svc.Config().WorkspaceDir
	linkA := filepath.Join(root, "Documents", "loop-a")
	linkB := filepath.Join(root, "Documents", "loop-b")
	if err := os.Symlink(linkB, linkA); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(linkA, linkB); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := svc.ResolvePath("Documents/loop-a/file.txt"); err == nil {
		t.Fatal("expected symlink loop traversal to be rejected")
	}
}

func TestServiceCopyRejectsNestedSymlinkEscape(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	root := svc.Config().WorkspaceDir
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Documents", "source"), 0o755); err != nil {
		t.Fatalf("create source dir: %v", err)
	}
	link := filepath.Join(root, "Documents", "source", "secret-link.txt")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := svc.CopyPath(context.Background(), "Documents/source", "Documents/copy", SourceAgent); err == nil {
		t.Fatal("expected copy with nested symlink escape to be rejected")
	}
}

func TestServiceReadFileBytesRejectsWorkspaceSymlinkFinalTarget(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Documents/source.txt", "secret", SourceAgent); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	source, err := svc.ResolvePath("Documents/source.txt")
	if err != nil {
		t.Fatalf("ResolvePath source: %v", err)
	}
	link, err := svc.resolveRenamePath("Documents/source-link.txt")
	if err != nil {
		t.Fatalf("resolve link path: %v", err)
	}
	createTestSymlinkOrSkip(t, source, link)

	if _, _, err := svc.ReadFileBytes(ctx, "Documents/source-link.txt"); err == nil {
		t.Fatal("ReadFileBytes should reject final symlink target")
	}
}

func TestServiceReadFileBytesRejectsWorkspaceSymlinkInPath(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Documents/source/file.txt", "secret", SourceAgent); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	sourceDir, err := svc.ResolvePath("Documents/source")
	if err != nil {
		t.Fatalf("ResolvePath source dir: %v", err)
	}
	linkDir, err := svc.resolveRenamePath("Documents/source-link")
	if err != nil {
		t.Fatalf("resolve link dir path: %v", err)
	}
	createTestSymlinkOrSkip(t, sourceDir, linkDir)

	if _, _, err := svc.ReadFileBytes(ctx, "Documents/source-link/file.txt"); err == nil {
		t.Fatal("ReadFileBytes should reject symlink in path")
	}
}

func TestServiceWriteFileRejectsWorkspaceSymlinkFinalTarget(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Documents/source.txt", "secret", SourceAgent); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	source, err := svc.ResolvePath("Documents/source.txt")
	if err != nil {
		t.Fatalf("ResolvePath source: %v", err)
	}
	link, err := svc.resolveRenamePath("Documents/source-link.txt")
	if err != nil {
		t.Fatalf("resolve link path: %v", err)
	}
	createTestSymlinkOrSkip(t, source, link)

	if err := svc.WriteFile(ctx, "Documents/source-link.txt", "changed", SourceAgent); err == nil {
		t.Fatal("WriteFile should reject final symlink target")
	}
	got, _, err := svc.ReadFile(ctx, "Documents/source.txt")
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	if got != "secret" {
		t.Fatalf("source content = %q, want unchanged", got)
	}
}

func TestServiceWriteFileRejectsWorkspaceSymlinkInPath(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Documents/source/file.txt", "secret", SourceAgent); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	sourceDir, err := svc.ResolvePath("Documents/source")
	if err != nil {
		t.Fatalf("ResolvePath source dir: %v", err)
	}
	linkDir, err := svc.resolveRenamePath("Documents/source-link")
	if err != nil {
		t.Fatalf("resolve link dir path: %v", err)
	}
	createTestSymlinkOrSkip(t, sourceDir, linkDir)

	if err := svc.WriteFile(ctx, "Documents/source-link/file.txt", "changed", SourceAgent); err == nil {
		t.Fatal("WriteFile should reject symlink in path")
	}
	got, _, err := svc.ReadFile(ctx, "Documents/source/file.txt")
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	if got != "secret" {
		t.Fatalf("source content = %q, want unchanged", got)
	}
}

func TestServiceAuditMigrationAddsRequestColumns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "desktop.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE desktop_audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT NOT NULL,
		target TEXT,
		source TEXT,
		details_json TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create old audit table: %v", err)
	}
	_ = db.Close()

	svc := testServiceWithConfig(t, Config{
		Enabled:            true,
		WorkspaceDir:       filepath.Join(dir, "workspace"),
		DBPath:             dbPath,
		MaxFileSizeMB:      1,
		AllowGeneratedApps: true,
		AllowAgentControl:  true,
		ControlLevel:       ControlConfirmDestructive,
	})

	rows, err := svc.getDB().Query(`SELECT name FROM pragma_table_info('desktop_audit')`)
	if err != nil {
		t.Fatalf("pragma table info: %v", err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		columns[name] = true
	}
	for _, want := range []string{"client_ip", "session_hash", "user_agent"} {
		if !columns[want] {
			t.Fatalf("desktop_audit missing column %q: %v", want, columns)
		}
	}
}

func TestServiceAuditWithRequestStoresAttributionAndPlainAuditStaysCompatible(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.AuditWithRequest(ctx, "desktop_vnc_connect", "device-1", map[string]string{"status": "attempt"}, SourceUser, AuditRequestInfo{
		ClientIP:    "203.0.113.7",
		SessionHash: "abc123",
		UserAgent:   "desktop-test",
	}); err != nil {
		t.Fatalf("AuditWithRequest: %v", err)
	}
	if err := svc.Audit(ctx, "install_app", "app-1", map[string]string{"status": "ok"}, SourceAgent); err != nil {
		t.Fatalf("Audit: %v", err)
	}

	var clientIP, sessionHash, userAgent string
	if err := svc.getDB().QueryRow(`SELECT client_ip, session_hash, user_agent FROM desktop_audit WHERE action = ?`, "desktop_vnc_connect").Scan(&clientIP, &sessionHash, &userAgent); err != nil {
		t.Fatalf("query request audit row: %v", err)
	}
	if clientIP != "203.0.113.7" || sessionHash != "abc123" || userAgent != "desktop-test" {
		t.Fatalf("request audit attribution = %q/%q/%q", clientIP, sessionHash, userAgent)
	}
	if err := svc.getDB().QueryRow(`SELECT client_ip, session_hash, user_agent FROM desktop_audit WHERE action = ?`, "install_app").Scan(&clientIP, &sessionHash, &userAgent); err != nil {
		t.Fatalf("query plain audit row: %v", err)
	}
	if clientIP != "" || sessionHash != "" || userAgent != "" {
		t.Fatalf("plain audit attribution = %q/%q/%q, want empty", clientIP, sessionHash, userAgent)
	}
}

func createTestSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func TestServiceCopyRejectsDirectoryTreesPastDepthLimit(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	root := svc.Config().WorkspaceDir
	current := filepath.Join(root, "Documents", "deep")
	for i := 0; i < 70; i++ {
		current = filepath.Join(current, "d")
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatalf("create deep fixture: %v", err)
	}
	if err := svc.CopyPath(context.Background(), "Documents/deep", "Documents/deep-copy", SourceAgent); err == nil {
		t.Fatal("expected deep copy to be rejected")
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
		{"AuraGo Documents", "Media", "song.mp3", "audio", "/files/documents/Media/song.mp3", "audio/mpeg", cfg.DocumentDir},
		{"AuraGo Documents", "Media", "clip.mp4", "video", "/files/documents/Media/clip.mp4", "video/mp4", cfg.DocumentDir},
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

func TestServiceMovePathCanReplaceDanglingSymlinkDestination(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Documents/The Last Lantern Keeper.md", "story", SourceUser); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(svc.Config().WorkspaceDir, "Desktop", "The Last Lantern Keeper.md")
	if err := os.Symlink(filepath.Join(svc.Config().WorkspaceDir, "Desktop", "missing.md"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := svc.MovePath(ctx, "Documents/The Last Lantern Keeper.md", "Desktop/The Last Lantern Keeper.md", SourceUser); err != nil {
		t.Fatalf("MovePath should replace dangling destination symlink entry: %v", err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("destination after move: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("destination should be the moved file, not the stale symlink")
	}
	data, _, err := svc.ReadFile(ctx, "Desktop/The Last Lantern Keeper.md")
	if err != nil {
		t.Fatalf("ReadFile moved file: %v", err)
	}
	if string(data) != "story" {
		t.Fatalf("moved file content = %q, want story", string(data))
	}
}

func TestServiceMovePathCanMoveDanglingSymlinkEntryItself(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	src := filepath.Join(svc.Config().WorkspaceDir, "Desktop", "Broken Link.md")
	if err := os.Symlink(filepath.Join(svc.Config().WorkspaceDir, "Desktop", "missing.md"), src); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := svc.MovePath(ctx, "Desktop/Broken Link.md", "Documents/Broken Link.md", SourceUser); err != nil {
		t.Fatalf("MovePath should move dangling symlink entries without following them: %v", err)
	}
	dst := filepath.Join(svc.Config().WorkspaceDir, "Documents", "Broken Link.md")
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("destination link after move: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("moved entry should remain a symlink")
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Fatalf("source link still exists or unexpected error: %v", err)
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
	if bootstrap.Settings["appearance.wallpaper"] != "groupshoot" {
		t.Fatalf("default wallpaper = %q", bootstrap.Settings["appearance.wallpaper"])
	}
	if bootstrap.Settings["appearance.theme"] != "standard" {
		t.Fatalf("default theme = %q", bootstrap.Settings["appearance.theme"])
	}
	if bootstrap.Settings["appearance.icon_theme"] != "papirus" {
		t.Fatalf("default icon theme = %q", bootstrap.Settings["appearance.icon_theme"])
	}
	if err := svc.SetSetting(ctx, "appearance.wallpaper", "forest", SourceUser); err != nil {
		t.Fatalf("SetSetting valid: %v", err)
	}
	if err := svc.SetSetting(ctx, "appearance.theme", "fruity", SourceUser); err != nil {
		t.Fatalf("SetSetting theme valid: %v", err)
	}
	if err := svc.SetSetting(ctx, "appearance.icon_theme", "whitesur", SourceUser); err != nil {
		t.Fatalf("SetSetting icon theme valid: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after setting: %v", err)
	}
	if bootstrap.Settings["appearance.wallpaper"] != "forest" {
		t.Fatalf("stored wallpaper = %q", bootstrap.Settings["appearance.wallpaper"])
	}
	if bootstrap.Settings["appearance.theme"] != "fruity" {
		t.Fatalf("stored theme = %q", bootstrap.Settings["appearance.theme"])
	}
	if bootstrap.Settings["appearance.icon_theme"] != "whitesur" {
		t.Fatalf("stored icon theme = %q", bootstrap.Settings["appearance.icon_theme"])
	}
	if err := svc.SetSetting(ctx, "appearance.wallpaper", "../../bad", SourceUser); err == nil {
		t.Fatal("expected invalid setting value to be rejected")
	}
	if err := svc.SetSetting(ctx, "appearance.theme", "citrus", SourceUser); err == nil {
		t.Fatal("expected invalid theme to be rejected")
	}
	if err := svc.SetSetting(ctx, "appearance.icon_theme", "aurago", SourceUser); err == nil {
		t.Fatal("expected removed AuraGo Classic icon theme to be rejected")
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

func TestServiceInstallAppAddsIntegrityAndDetectsTampering(t *testing.T) {
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
	files := map[string]string{
		"index.html": "<main>Quick Notes</main>",
		"app.js":     "window.quickNotes = true;",
	}
	if err := svc.InstallApp(ctx, manifest, files, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	app := testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if app.Integrity == nil {
		t.Fatal("installed app integrity is missing")
	}
	if got := app.Integrity.Hashes["index.html"]; !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("index.html hash = %q, want sha256 hash", got)
	}
	if got := app.Integrity.Hashes["app.js"]; !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("app.js hash = %q, want sha256 hash", got)
	}
	if app.Integrity.Signature == nil || app.Integrity.Signature.Algorithm != "ed25519" || app.Integrity.Signature.Value == "" {
		t.Fatalf("signature missing or invalid: %+v", app.Integrity.Signature)
	}
	ok, reason, err := svc.VerifyGeneratedAssetIntegrity(ctx, "Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("VerifyGeneratedAssetIntegrity: %v", err)
	}
	if !ok || reason != "" {
		t.Fatalf("integrity verification before tamper = %v/%q, want ok", ok, reason)
	}

	appPath, err := svc.ResolvePath("Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if err := os.WriteFile(appPath, []byte("<main>Tampered but non-empty</main>"), 0o644); err != nil {
		t.Fatalf("tamper app entry: %v", err)
	}
	svc.invalidateBootstrapCache()
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after tamper: %v", err)
	}
	app = testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if app.Health != "broken" || app.HealthReason != "integrity_hash_mismatch" {
		t.Fatalf("health after tamper = %s/%s, want broken/integrity_hash_mismatch", app.Health, app.HealthReason)
	}
	ok, reason, err = svc.VerifyGeneratedAssetIntegrity(ctx, "Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("VerifyGeneratedAssetIntegrity after tamper: %v", err)
	}
	if ok || reason != "integrity_hash_mismatch" {
		t.Fatalf("integrity verification after tamper = %v/%q, want integrity_hash_mismatch", ok, reason)
	}
}

func TestServiceBootstrapMarksGeneratedAppWithMissingEntryBroken(t *testing.T) {
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
	if err := svc.InstallApp(ctx, manifest, map[string]string{"index.html": "<main>Quick Notes</main>"}, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	entryPath, err := svc.ResolvePath("Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if err := os.Remove(entryPath); err != nil {
		t.Fatalf("remove entry file: %v", err)
	}

	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, app := range bootstrap.InstalledApps {
		if app.ID != "quick-notes" {
			continue
		}
		if app.Health != "broken" {
			t.Fatalf("health = %q, want broken; app=%+v", app.Health, app)
		}
		if app.HealthReason != "missing_entry_file" {
			t.Fatalf("health_reason = %q, want missing_entry_file; app=%+v", app.HealthReason, app)
		}
		if app.EntryPath != "Apps/quick-notes/index.html" {
			t.Fatalf("entry_path = %q, want Apps/quick-notes/index.html", app.EntryPath)
		}
		return
	}
	t.Fatalf("quick-notes app missing from bootstrap: %+v", bootstrap.InstalledApps)
}

func TestServiceBootstrapMarksGeneratedAppWithEmptyEntryBroken(t *testing.T) {
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
	if err := svc.InstallApp(ctx, manifest, map[string]string{"index.html": "<main>Quick Notes</main>"}, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	entryPath, err := svc.ResolvePath("Apps/quick-notes/index.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte(" \n\t"), 0o644); err != nil {
		t.Fatalf("empty entry file: %v", err)
	}

	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, app := range bootstrap.InstalledApps {
		if app.ID != "quick-notes" {
			continue
		}
		if app.Health != "broken" {
			t.Fatalf("health = %q, want broken; app=%+v", app.Health, app)
		}
		if app.HealthReason != "empty_entry_file" {
			t.Fatalf("health_reason = %q, want empty_entry_file; app=%+v", app.HealthReason, app)
		}
		if app.EntryPath != "Apps/quick-notes/index.html" {
			t.Fatalf("entry_path = %q, want Apps/quick-notes/index.html", app.EntryPath)
		}
		return
	}
	t.Fatalf("quick-notes app missing from bootstrap: %+v", bootstrap.InstalledApps)
}

func TestServiceFileMutationsInvalidateBootstrapAppHealthCache(t *testing.T) {
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
	if err := svc.InstallApp(ctx, manifest, map[string]string{"index.html": "<main>Quick Notes</main>"}, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap warm cache: %v", err)
	}
	if got := testFindApp(t, bootstrap.InstalledApps, "quick-notes").Health; got == "broken" {
		t.Fatalf("initial health = %q, want healthy", got)
	}

	if err := svc.WriteFile(ctx, "Apps/quick-notes/index.html", " \n\t", SourceAgent); err != nil {
		t.Fatalf("WriteFile empty app entry: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after write: %v", err)
	}
	app := testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if app.Health != "broken" || app.HealthReason != "empty_entry_file" {
		t.Fatalf("health after empty write = %s/%s, want broken/empty_entry_file", app.Health, app.HealthReason)
	}

	if err := svc.WriteFile(ctx, "Apps/quick-notes/index.html", "<main>Quick Notes</main>", SourceAgent); err != nil {
		t.Fatalf("WriteFile restored app entry: %v", err)
	}
	if _, err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap rewarm after restore: %v", err)
	}
	if err := svc.DeletePath(ctx, "Apps/quick-notes/index.html", SourceUser); err != nil {
		t.Fatalf("DeletePath app entry: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after delete: %v", err)
	}
	app = testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if app.Health != "broken" || app.HealthReason != "missing_entry_file" {
		t.Fatalf("health after delete = %s/%s, want broken/missing_entry_file", app.Health, app.HealthReason)
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

func TestServiceAppVisibilityDefaultsAndTogglesBuiltinAndInstalledApps(t *testing.T) {
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
	if err := svc.InstallApp(ctx, manifest, map[string]string{"index.html": "<main>Quick Notes</main>"}, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}

	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	files := testFindApp(t, bootstrap.BuiltinApps, "files")
	if !files.Builtin || files.Deletable || !files.DockVisible || !files.StartVisible {
		t.Fatalf("builtin app default visibility/deletable flags wrong: %+v", files)
	}
	quickNotes := testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if quickNotes.Builtin || !quickNotes.Deletable || !quickNotes.DockVisible || !quickNotes.StartVisible {
		t.Fatalf("installed app default visibility/deletable flags wrong: %+v", quickNotes)
	}

	if err := svc.SetAppVisibility(ctx, "files", boolPtr(false), boolPtr(false), SourceUser); err != nil {
		t.Fatalf("SetAppVisibility builtin false: %v", err)
	}
	if err := svc.SetAppVisibility(ctx, "quick-notes", boolPtr(false), nil, SourceUser); err != nil {
		t.Fatalf("SetAppVisibility installed dock false: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after visibility: %v", err)
	}
	files = testFindApp(t, bootstrap.BuiltinApps, "files")
	if files.DockVisible || files.StartVisible {
		t.Fatalf("builtin app should be hidden from dock and start menu: %+v", files)
	}
	quickNotes = testFindApp(t, bootstrap.InstalledApps, "quick-notes")
	if quickNotes.DockVisible || !quickNotes.StartVisible {
		t.Fatalf("installed app should only be hidden from dock: %+v", quickNotes)
	}

	if err := svc.SetAppVisibility(ctx, "files", boolPtr(true), boolPtr(true), SourceUser); err != nil {
		t.Fatalf("SetAppVisibility builtin true: %v", err)
	}
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after restore: %v", err)
	}
	files = testFindApp(t, bootstrap.BuiltinApps, "files")
	if !files.DockVisible || !files.StartVisible {
		t.Fatalf("builtin app should be restored to dock and start menu: %+v", files)
	}
}

func TestServiceDeleteAppRemovesGeneratedAppVisibility(t *testing.T) {
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
	if err := svc.InstallApp(ctx, manifest, map[string]string{"index.html": "<main>Quick Notes</main>"}, SourceAgent); err != nil {
		t.Fatalf("InstallApp: %v", err)
	}
	if err := svc.SetAppVisibility(ctx, "quick-notes", boolPtr(false), boolPtr(false), SourceUser); err != nil {
		t.Fatalf("SetAppVisibility: %v", err)
	}
	if err := svc.DeleteApp(ctx, "quick-notes", SourceUser); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}

	svc.mu.Lock()
	db := svc.db
	svc.mu.Unlock()
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM desktop_app_visibility WHERE app_id = ?`, "quick-notes").Scan(&rows); err != nil {
		t.Fatalf("query app visibility rows: %v", err)
	}
	if rows != 0 {
		t.Fatalf("deleted app visibility rows = %d, want 0", rows)
	}
}

func TestServiceInitMigratesStoreAppsBackIntoDockOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "workspace")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE desktop_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE desktop_apps (id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL, icon TEXT NOT NULL, entry TEXT NOT NULL, manifest_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE desktop_app_visibility (app_id TEXT PRIMARY KEY, dock_visible INTEGER NOT NULL DEFAULT 1, start_visible INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("seed schema: %v", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	manifestJSON := `{"id":"store-n8n","name":"n8n","version":"1.0.0","icon":"workflow","entry":"index.html","runtime":"container_web_app","metadata":{"store_app_id":"n8n"}}`
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_apps(id, name, version, icon, entry, manifest_json, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`, "store-n8n", "n8n", "1.0.0", "workflow", "index.html", manifestJSON, now, now); err != nil {
		t.Fatalf("seed store app: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO desktop_app_visibility(app_id, dock_visible, start_visible, updated_at) VALUES(?, 0, 1, ?)`, "store-n8n", now); err != nil {
		t.Fatalf("seed hidden visibility: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	svc, err := NewService(Config{
		Enabled:            true,
		WorkspaceDir:       root,
		DBPath:             dbPath,
		MaxFileSizeMB:      1,
		AllowGeneratedApps: true,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	if err := svc.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}

	svc.mu.Lock()
	liveDB := svc.db
	svc.mu.Unlock()
	var dockVisible int
	if err := liveDB.QueryRowContext(ctx, `SELECT dock_visible FROM desktop_app_visibility WHERE app_id = ?`, "store-n8n").Scan(&dockVisible); err != nil {
		t.Fatalf("read migrated visibility: %v", err)
	}
	if dockVisible != 1 {
		t.Fatalf("store app dock visibility = %d, want migrated visible", dockVisible)
	}
	var migrated string
	if err := liveDB.QueryRowContext(ctx, `SELECT value FROM desktop_meta WHERE key = 'store_apps_dock_visibility_migrated'`).Scan(&migrated); err != nil {
		t.Fatalf("read migration marker: %v", err)
	}
	if migrated != "true" {
		t.Fatalf("migration marker = %q, want true", migrated)
	}

	if _, err := liveDB.ExecContext(ctx, `UPDATE desktop_app_visibility SET dock_visible = 0 WHERE app_id = ?`, "store-n8n"); err != nil {
		t.Fatalf("simulate user hiding app after migration: %v", err)
	}
	if err := svc.migrateLocked(ctx); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
	if err := liveDB.QueryRowContext(ctx, `SELECT dock_visible FROM desktop_app_visibility WHERE app_id = ?`, "store-n8n").Scan(&dockVisible); err != nil {
		t.Fatalf("read rerun visibility: %v", err)
	}
	if dockVisible != 0 {
		t.Fatalf("migration must not override later user hide, dock visibility = %d", dockVisible)
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

func TestServiceInstallAppRejectsUnsafeSDKPermissions(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	files := map[string]string{"index.html": "<main id=\"app\"></main>"}
	manifest := AppManifest{
		ID:          "unsafe-sdk",
		Name:        "Unsafe SDK",
		Version:     "1.0.0",
		Icon:        "apps",
		Entry:       "index.html",
		Permissions: []string{"files:read", "*"},
	}
	err := svc.InstallApp(context.Background(), manifest, files, SourceAgent)
	if err == nil {
		t.Fatal("expected wildcard SDK permission to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported desktop permission") {
		t.Fatalf("error = %q, want unsupported permission rejection", err)
	}

	manifest.ID = "unsafe-admin"
	manifest.Permissions = []string{"filesystem:delete"}
	err = svc.InstallApp(context.Background(), manifest, files, SourceAgent)
	if err == nil {
		t.Fatal("expected unknown SDK permission to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported desktop permission") {
		t.Fatalf("error = %q, want unsupported permission rejection", err)
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
	if got.Icon != "audio-player" {
		t.Fatalf("normalized widget icon = %q, want audio-player", got.Icon)
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

func TestBuiltinViewerIsInternalOnly(t *testing.T) {
	t.Parallel()

	var viewer AppManifest
	for _, app := range BuiltinApps() {
		if app.ID == "viewer" {
			viewer = app
			break
		}
	}
	if viewer.ID == "" {
		t.Fatal("viewer builtin app missing")
	}
	if !viewer.Internal {
		t.Fatalf("viewer must be marked internal-only: %+v", viewer)
	}
	if viewer.DockVisible || viewer.StartVisible {
		t.Fatalf("viewer must stay internal-only, got dock_visible=%v start_visible=%v", viewer.DockVisible, viewer.StartVisible)
	}

	bootstrap, err := testService(t).Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	bootViewer := testFindApp(t, bootstrap.BuiltinApps, "viewer")
	if bootViewer.ID == "" {
		t.Fatal("viewer missing from bootstrap builtin apps")
	}
	if !bootViewer.Internal {
		t.Fatalf("bootstrap viewer must stay marked internal-only: %+v", bootViewer)
	}
	if bootViewer.DockVisible || bootViewer.StartVisible {
		t.Fatalf("bootstrap viewer must stay hidden, got dock_visible=%v start_visible=%v", bootViewer.DockVisible, bootViewer.StartVisible)
	}
}

func TestBuiltinOpenSCADIsHiddenUntilStoreInstall(t *testing.T) {
	t.Parallel()

	var app AppManifest
	for _, candidate := range BuiltinApps() {
		if candidate.ID == "openscad" {
			app = candidate
			break
		}
	}
	if app.ID == "" {
		t.Fatal("openscad builtin app missing")
	}
	if app.Entry != "builtin://openscad" || app.Runtime != BuiltinRuntime {
		t.Fatalf("openscad app route = %q/%q, want builtin://openscad/builtin", app.Entry, app.Runtime)
	}
	if app.Icon != "openscad" {
		t.Fatalf("openscad icon = %q, want openscad", app.Icon)
	}
	if app.Internal {
		t.Fatalf("openscad must not be internal; the Store needs to make it visible: %+v", app)
	}
	if app.DockVisible || app.StartVisible {
		t.Fatalf("openscad should be hidden before Store install, got dock_visible=%v start_visible=%v", app.DockVisible, app.StartVisible)
	}

	bootstrap, err := testService(t).Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	bootApp := testFindApp(t, bootstrap.BuiltinApps, "openscad")
	if bootApp.ID == "" {
		t.Fatal("openscad missing from bootstrap builtin apps")
	}
	if bootApp.DockVisible || bootApp.StartVisible {
		t.Fatalf("bootstrap openscad should be hidden before Store install, got dock_visible=%v start_visible=%v", bootApp.DockVisible, bootApp.StartVisible)
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

func TestServiceBootstrapSeedsBuiltinWidgets(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	bootstrap, err := svc.Bootstrap(context.Background())
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	widgetMap := map[string]Widget{}
	for _, w := range bootstrap.AllWidgets {
		widgetMap[w.ID] = w
	}
	clock, ok := widgetMap["builtin-analog-clock"]
	if !ok {
		t.Fatal("builtin-analog-clock widget not seeded")
	}
	if !clock.Builtin {
		t.Fatal("analog clock widget should be builtin")
	}
	if !clock.Visible {
		t.Fatal("analog clock widget should be visible by default")
	}
	chat, ok := widgetMap["builtin-quickchat"]
	if !ok {
		t.Fatal("builtin-quickchat widget not seeded")
	}
	if !chat.Builtin {
		t.Fatal("quickchat widget should be builtin")
	}
}

func TestServiceUpsertWidgetAddsIntegrityAndDetectsTampering(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	if err := svc.WriteFile(ctx, "Widgets/status.html", "<section>Status</section>", SourceAgent); err != nil {
		t.Fatalf("WriteFile widget: %v", err)
	}
	widget := Widget{
		ID:      "status",
		Title:   "Status",
		Type:    WidgetTypeCustom,
		Entry:   "status.html",
		Visible: true,
	}
	if err := svc.UpsertWidget(ctx, widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	found := false
	for _, w := range bootstrap.AllWidgets {
		if w.ID != "status" {
			continue
		}
		found = true
		if w.Integrity == nil {
			t.Fatal("widget integrity is missing")
		}
		if got := w.Integrity.Hashes["status.html"]; !strings.HasPrefix(got, "sha256:") {
			t.Fatalf("status.html hash = %q, want sha256 hash", got)
		}
	}
	if !found {
		t.Fatalf("status widget missing: %+v", bootstrap.AllWidgets)
	}

	widgetPath, err := svc.ResolvePath("Widgets/status.html")
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if err := os.WriteFile(widgetPath, []byte("<section>Tampered</section>"), 0o644); err != nil {
		t.Fatalf("tamper widget: %v", err)
	}
	svc.invalidateBootstrapCache()
	bootstrap, err = svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap after tamper: %v", err)
	}
	for _, w := range bootstrap.AllWidgets {
		if w.ID != "status" {
			continue
		}
		if w.Health != "broken" || w.HealthReason != "integrity_hash_mismatch" {
			t.Fatalf("widget health after tamper = %s/%s, want broken/integrity_hash_mismatch", w.Health, w.HealthReason)
		}
		return
	}
	t.Fatalf("status widget missing after tamper: %+v", bootstrap.AllWidgets)
}

func TestServiceBootstrapSeparatesVisibleAndAllWidgets(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	widget := Widget{ID: "hidden-test", Title: "Hidden", Icon: "apps"}
	if err := svc.UpsertWidget(ctx, widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget: %v", err)
	}
	if err := svc.SetWidgetVisible(ctx, "hidden-test", false, SourceUser); err != nil {
		t.Fatalf("SetWidgetVisible false: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, w := range bootstrap.Widgets {
		if w.ID == "hidden-test" {
			t.Fatal("hidden widget should not be in visible Widgets list")
		}
	}
	var found bool
	for _, w := range bootstrap.AllWidgets {
		if w.ID == "hidden-test" {
			found = true
		}
	}
	if !found {
		t.Fatal("hidden widget should be in AllWidgets list")
	}
}

func TestServiceDeleteWidgetRejectsBuiltinWidgets(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.DeleteWidget(context.Background(), "builtin-analog-clock", SourceUser)
	if err == nil {
		t.Fatal("expected deleting builtin widget to be rejected")
	}
	if !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("error = %q, want builtin rejection", err)
	}
}

func TestServiceSetWidgetVisibleToggles(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	ctx := context.Background()
	widget := Widget{ID: "toggle-test", Title: "Toggle", Icon: "apps"}
	if err := svc.UpsertWidget(ctx, widget, SourceAgent); err != nil {
		t.Fatalf("UpsertWidget: %v", err)
	}
	if err := svc.SetWidgetVisible(ctx, "toggle-test", false, SourceUser); err != nil {
		t.Fatalf("SetWidgetVisible false: %v", err)
	}
	allWidgets, err := svc.ListAllWidgets(ctx)
	if err != nil {
		t.Fatalf("ListAllWidgets: %v", err)
	}
	for _, w := range allWidgets {
		if w.ID == "toggle-test" && w.Visible {
			t.Fatal("widget should be hidden after SetWidgetVisible(false)")
		}
	}
	if err := svc.SetWidgetVisible(ctx, "toggle-test", true, SourceUser); err != nil {
		t.Fatalf("SetWidgetVisible true: %v", err)
	}
	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	var found bool
	for _, w := range bootstrap.Widgets {
		if w.ID == "toggle-test" {
			found = true
		}
	}
	if !found {
		t.Fatal("widget should be back in visible Widgets after SetWidgetVisible(true)")
	}
}

func TestServiceSetWidgetVisibleRejectsUnknownWidget(t *testing.T) {
	t.Parallel()

	svc := testService(t)
	err := svc.SetWidgetVisible(context.Background(), "nonexistent", true, SourceUser)
	if err == nil {
		t.Fatal("expected unknown widget visibility toggle to fail")
	}
}
