package ui

import (
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// storymakerModules is the load order of the app: the shared core first, the composition root last.
var storymakerModules = []string{"core", "model", "api", "assets", "player", "projects", "strip", "inspector", "code", "export", "audio", "voice", "music", "dialog", "handoff"}

func TestDesktopStorymakerRegistration(t *testing.T) {
	t.Parallel()
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "storymaker" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("storymaker is not a builtin app")
	}
	if app.Name != "Storymaker" || app.Category != "creative" || app.Entry != "builtin://storymaker" || strings.Join(app.Requires, ",") != "storymaker" || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	if app.Metadata["open_maximized"] != "true" {
		t.Fatalf("metadata = %v, want open_maximized", app.Metadata)
	}
	logo := app.Metadata["logo_path"]
	if logo != "/img/desktop-icons/storymaker.svg" {
		t.Fatalf("logo_path = %q", logo)
	}
	svg := readDesktopAssetText(t, strings.TrimPrefix(logo, "/"))
	if !strings.HasPrefix(svg, "<svg ") || strings.Contains(svg, "href") || strings.Contains(svg, "<script") || strings.Contains(svg, "<image") {
		t.Fatalf("the Storymaker logo must be a self-contained SVG: %.120s", svg)
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        storymaker: {")
	if start < 0 {
		t.Fatal("module-loader.js has no storymaker entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	if !strings.Contains(entry, "appStyles('/css/desktop-app-storymaker.css')") {
		t.Error("the storymaker loader entry must load css/desktop-app-storymaker.css")
	}
	offset := 0
	for _, module := range append(append([]string{}, storymakerModules...), "") {
		script := "/js/desktop/apps/storymaker-" + module + ".js'"
		if module == "" {
			script = "/js/desktop/apps/storymaker.js'"
		}
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the storymaker loader entry lists %s out of order or not at all (the core first, the composition root last)", script)
		}
		offset += index + len(script)
	}
	if !strings.Contains(loader, "storymaker: ['storymaker'],") {
		t.Error("APP_I18N_SECTIONS has no storymaker section")
	}
	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	bootstrap := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, m := range []struct{ name, source, marker string }{
		{"foundation", foundation, "storymaker: 'StorymakerApp',"},
		{"routing", routing, "if (appId === 'storymaker') {"},
		{"routing", routing, "window.AuraDesktopModules.loadAppScript('storymaker')"},
		{"routing", routing, "window.StorymakerApp.render(contentEl(id), id, Object.assign({}, context || {}, {"},
		{"size", shell, "storymaker: { width: 1320, height: 840 },"},
		{"min size", shell, "if (appId === 'storymaker') return { width: 640, height: 480 };"},
		{"running window launch", shell, "if (appId === 'storymaker' && context && typeof context.projectId === 'string' && context.projectId) window.StorymakerApp?.open?.(existing.id, context.projectId);"},
		{"launch event", bootstrap, "launch.projectId = event.payload.project_id;"},
	} {
		if !strings.Contains(m.source, m.marker) {
			t.Errorf("%s lacks %q", m.name, m.marker)
		}
		if !strings.Contains(bundle, m.marker) {
			t.Errorf("main.bundle.js lacks the %s marker %q (run npm run build:ui)", m.name, m.marker)
		}
	}
	// A launch names the project to open, for a new window and for a running one.
	root := rawDesktopAssetText(t, "js/desktop/apps/storymaker.js")
	for _, marker := range []string{"launchProjectId: typeof c.projectId === 'string' ? c.projectId : ''", "window.StorymakerApp = { render, dispose, open };"} {
		if !strings.Contains(root, marker) {
			t.Errorf("storymaker.js lacks %q", marker)
		}
	}
	// The window hands the app what it needs: translation, dialogs, read-only state, the way to open Files.
	for _, ctx := range []string{"readonly: desktopReadonly()", "openApp, confirmDialog", "setWindowBeforeClose:"} {
		if !strings.Contains(routing, ctx) {
			t.Errorf("the storymaker render dispatch does not pass %q", ctx)
		}
	}
	for _, lang := range storymakerLanguages {
		if strings.TrimSpace(readMergedLangMap(t, lang)["desktop.app_storymaker"]) == "" {
			t.Errorf("%s lacks desktop.app_storymaker", lang)
		}
	}
}
