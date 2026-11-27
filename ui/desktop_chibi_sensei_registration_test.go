package ui

import (
	"os"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// chibiSenseiModules is the load order of the app: the shared core first, the composition root last. A module that does not exist
// yet (later batches add them) is skipped, so the test holds in every batch.
var chibiSenseiModules = []string{"core", "api", "mascot", "projects", "steps", "table", "data", "generate", "training", "chart", "run", "chat", "test"}

func TestDesktopChibiSenseiRegistration(t *testing.T) {
	t.Parallel()
	var app *desktop.AppManifest
	for _, candidate := range desktop.BuiltinApps() {
		if candidate.ID == "chibi-sensei" {
			found := candidate
			app = &found
		}
	}
	if app == nil {
		t.Fatal("chibi-sensei is not a builtin app")
	}
	if app.Name != "Chibi Sensei" || app.Category != "ai" || app.Entry != "builtin://chibi-sensei" || strings.Join(app.Requires, ",") != "chibi_sensei" || !app.StartVisible {
		t.Fatalf("manifest = %+v", *app)
	}
	if app.Metadata["open_maximized"] != "true" {
		t.Fatalf("metadata = %v, want open_maximized", app.Metadata)
	}
	logo := app.Metadata["logo_path"]
	if logo != "/img/desktop-icons/chibi-sensei.svg" {
		t.Fatalf("logo_path = %q", logo)
	}
	svg := readDesktopAssetText(t, strings.TrimPrefix(logo, "/"))
	if !strings.HasPrefix(svg, "<svg ") || strings.Contains(svg, "href") || strings.Contains(svg, "<script") || strings.Contains(svg, "<image") {
		t.Fatalf("the Chibi Sensei logo must be a self-contained SVG: %.120s", svg)
	}

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	start := strings.Index(loader, "        'chibi-sensei': {")
	if start < 0 {
		t.Fatal("module-loader.js has no chibi-sensei entry in DESKTOP_APP_ASSETS")
	}
	entry := loader[start : start+strings.Index(loader[start:], "\n        },")]
	if !strings.Contains(entry, "appStyles('/css/desktop-app-chibi-sensei.css')") {
		t.Error("the chibi-sensei loader entry must load css/desktop-app-chibi-sensei.css")
	}
	offset := 0
	for _, module := range append(append([]string{}, chibiSenseiModules...), "") {
		file := "chibi-sensei-" + module + ".js"
		if module == "" {
			file = "chibi-sensei.js"
		} else if _, err := os.Stat("js/desktop/apps/chibi-sensei/" + file); err != nil {
			continue
		}
		script := "/js/desktop/apps/chibi-sensei/" + file + "'"
		index := strings.Index(entry[offset:], script)
		if index < 0 {
			t.Fatalf("the chibi-sensei loader entry lists %s out of order or not at all (the core first, the composition root last)", script)
		}
		offset += index + len(script)
	}
	if !strings.Contains(loader, "'chibi-sensei': ['chibi_sensei'],") {
		t.Error("APP_I18N_SECTIONS has no chibi-sensei section")
	}

	foundation := rawDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	routing := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	shell := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	bundle := rawDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, m := range []struct{ name, source, marker string }{
		{"foundation", foundation, "'chibi-sensei': 'ChibiSenseiApp',"},
		{"routing", routing, "if (appId === 'chibi-sensei') {"},
		{"routing", routing, "window.AuraDesktopModules.loadAppScript('chibi-sensei')"},
		{"routing", routing, "window.ChibiSenseiApp.render(contentEl(id), id, withDesktopFileDialogs(context, {"},
		{"size", shell, "'chibi-sensei': { width: 1280, height: 820 },"},
		{"min size", shell, "if (appId === 'chibi-sensei') return { width: 900, height: 600 };"},
		{"multi instance", shell, "'layerling', 'chibi-sensei'].includes(appId);"},
	} {
		if !strings.Contains(m.source, m.marker) {
			t.Errorf("%s lacks %q", m.name, m.marker)
		}
		if !strings.Contains(bundle, m.marker) {
			t.Errorf("main.bundle.js lacks the %s marker %q (run npm run build:ui)", m.name, m.marker)
		}
	}

	// The window hands the app what it needs: translation, dialogs, read-only state, the way to open Files.
	branchStart := strings.Index(routing, "if (appId === 'chibi-sensei') {")
	if branchStart < 0 {
		t.Fatal("menus-and-routing.js has no chibi-sensei branch")
	}
	branch := routing[branchStart:]
	if next := strings.Index(branch[1:], "\n        if (appId ==="); next >= 0 {
		branch = branch[:next+1]
	}
	for _, ctx := range []string{"readonly: desktopReadonly()", "openApp, confirmDialog, modalDialog", "setWindowBeforeClose:"} {
		if !strings.Contains(branch, ctx) {
			t.Errorf("the chibi-sensei render dispatch does not pass %q", ctx)
		}
	}

	root := rawDesktopAssetText(t, "js/desktop/apps/chibi-sensei/chibi-sensei.js")
	if !strings.Contains(root, "window.ChibiSenseiApp = { render, dispose, open };") {
		t.Error("chibi-sensei.js does not publish window.ChibiSenseiApp = { render, dispose, open };")
	}
	for _, lang := range storymakerLanguages {
		if strings.TrimSpace(readMergedLangMap(t, lang)["desktop.app_chibi_sensei"]) == "" {
			t.Errorf("%s lacks desktop.app_chibi_sensei", lang)
		}
	}
}
