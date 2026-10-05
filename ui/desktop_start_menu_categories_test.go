package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

// The categorized start menu keeps the Go category list, the JS rail order, the sixteen locale
// bundles, the markup and the stylesheet in step.
func TestDesktopStartMenuCategoriesStayInSync(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	block := regexp.MustCompile(`(?s)START_MENU_CATEGORIES = \[(.*?)\];`).FindStringSubmatch(js)
	if block == nil {
		t.Fatal("window-shell-runtime.js must declare START_MENU_CATEGORIES")
	}
	var jsIDs []string
	for _, match := range regexp.MustCompile(`id: '([a-z]+)', icon: '([a-z0-9-]+)'`).FindAllStringSubmatch(block[1], -1) {
		jsIDs = append(jsIDs, match[1]+"="+match[2])
	}
	var goIDs []string
	for _, category := range desktop.DesktopAppCategories() {
		goIDs = append(goIDs, category.ID+"="+category.Icon)
	}
	if strings.Join(jsIDs, ",") != strings.Join(goIDs, ",") {
		t.Fatalf("START_MENU_CATEGORIES %v differs from desktop.DesktopAppCategories %v", jsIDs, goIDs)
	}
	for _, want := range []string{
		"function renderStartApps(",
		"function selectStartCategory(",
		"function positionStartRailIndicator(",
		"'aurago.desktop.startCategory.v1'",
		"START_HOVER_INTENT_MS",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("window-shell-runtime.js missing %q", want)
		}
	}

	keys := []string{"desktop.category_all", "desktop.start_categories", "desktop.start_results", "desktop.start_no_results", "desktop.start_category_empty"}
	for _, category := range desktop.DesktopAppCategories() {
		keys = append(keys, "desktop.category_"+category.ID)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}

	html := readDesktopAssetText(t, "desktop.html")
	for _, want := range []string{`id="vd-start-rail"`, `role="tablist"`, `id="vd-start-pane-head"`, `id="vd-start-apps"`, `id="vd-start-search"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("desktop.html missing start menu marker %q", want)
		}
	}

	css := readDesktopAssetText(t, "css/desktop-start-menu.css")
	for _, want := range []string{
		".vd-start-body {",
		".vd-start-rail {",
		".vd-start-category {",
		".vd-start-rail-indicator {",
		".vd-start-pane-head {",
		".vd-start-grid {",
		".vd-start-empty {",
		"--vd-rail-y",
		".vd-start-pane-switching",
		"@media (prefers-reduced-motion: reduce)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-start-menu.css missing %q", want)
		}
	}
	polish := readDesktopAssetText(t, "js/desktop/core/polish-runtime.js")
	if !strings.Contains(polish, ".vd-start-category") {
		t.Fatal("category buttons must receive the pointer light (POINTER_LIGHT_TARGETS)")
	}

	// The outside-click closer must spare every launcher (the Fruity menubar brand opens the menu
	// without stopping propagation) and a target that a re-render detached from the menu.
	bootstrap := readDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	closer := regexp.MustCompile(`if \(!menu\.hidden && [^\n]*closeStartMenu`).FindString(strings.ReplaceAll(bootstrap, "\n", " "))
	if closer == "" {
		closer = regexp.MustCompile(`if \(!menu\.hidden &&[^{]*\{`).FindString(bootstrap)
	}
	for _, want := range []string{"event.target.isConnected", "#vd-start-button", "[data-fruity-dock-orb]", ".vd-global-brand"} {
		if !strings.Contains(closer, want) {
			t.Fatalf("start menu outside-click closer must contain %q, got %q", want, closer)
		}
	}
}
