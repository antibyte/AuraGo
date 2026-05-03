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
	for _, name := range DefaultDirectories() {
		if _, err := os.Stat(filepath.Join(svc.Config().WorkspaceDir, name)); err != nil {
			t.Fatalf("expected workspace directory %s: %v", name, err)
		}
	}
	if len(bootstrap.BuiltinApps) < 4 {
		t.Fatalf("expected builtin desktop apps, got %d", len(bootstrap.BuiltinApps))
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
	for _, want := range []string{"notes", "settings", "weather", "folder"} {
		if !stringSliceContains(bootstrap.IconCatalog.Preferred, want) {
			t.Fatalf("icon catalog missing preferred icon %q: %+v", want, bootstrap.IconCatalog.Preferred)
		}
	}
	if bootstrap.IconCatalog.Aliases["sparkles"] != "apps" {
		t.Fatalf("sparkles alias = %q, want apps", bootstrap.IconCatalog.Aliases["sparkles"])
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
