package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The loader, the shell routing, the window sizing and the generated main bundle know HomeDash. The engine scripts load in the order of
// APP_ORDER in scripts/homedash-tests/harness.mjs, then the two desktop host files. Behaviour: npm run test:homedash and TestDesktopHomeDashBrowser.
func TestHomeDashWiring(t *testing.T) {
	t.Parallel()
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	entry := regexp.MustCompile(`homedash: \{\s*styles: appStyles\('/css/homedash\.bundle\.css', '/css/desktop-app-homedash\.css'\),\s*scripts: \[([^\]]*)\]\s*\}`).FindStringSubmatch(loader)
	if entry == nil {
		t.Fatal("module-loader.js must register the homedash assets")
	}
	var scripts []string
	for _, match := range regexp.MustCompile(`'/js/([a-z/-]+\.js)'`).FindAllStringSubmatch(entry[1], -1) {
		scripts = append(scripts, match[1])
	}
	engine := []string{"hd-core", "hd-format", "hd-colour", "hd-grid", "hd-layout", "hd-doc", "hd-history", "hd-icons", "hd-geometry", "hd-input", "hd-prim-displays", "hd-prim-controls", "hd-widgets",
		"widgets/hd-w-misc", "widgets/hd-w-climate", "widgets/hd-w-light", "widgets/hd-w-sensor", "widgets/hd-w-switch",
		"hd-tone", "hd-api", "hd-stream", "hd-store", "hd-command", "hd-catalog", "hd-render", "hd-sheet", "hd-view", "hd-app"}
	var want []string
	for _, name := range engine {
		want = append(want, "homedash/"+name+".js")
	}
	want = append(want, "desktop/apps/homedash-home.js", "desktop/apps/homedash.js")
	if strings.Join(scripts, ",") != strings.Join(want, ",") {
		t.Fatalf("homedash scripts = %v, want %v", scripts, want)
	}
	if !strings.Contains(loader, "homedash: ['homedash']") {
		t.Fatal("module-loader.js must load the homedash translation section for the app")
	}
	for file, markers := range map[string][]string{
		"js/desktop/core/menus-and-routing.js":    {"appId === 'homedash' && window.HomeDashApp", "window.HomeDashApp.render(contentEl(id), id,"},
		"js/desktop/core/desktop-foundation.js":   {"homedash: 'HomeDashApp'"},
		"js/desktop/core/window-shell-runtime.js": {"homedash: { width: 1180, height: 780 }", "appId === 'homedash') return { width: 420, height: 360 }", "if (appId === 'homedash') return !(context && context.dashboard_id)"},
		"js/desktop/bundles/main.bundle.js":       {"appId === 'homedash' && window.HomeDashApp", "homedash: 'HomeDashApp'", "homedash: { width: 1180, height: 780 }", "if (appId === 'homedash') return !(context && context.dashboard_id)"},
	} {
		source := readDesktopAssetText(t, file)
		for _, marker := range markers {
			if !strings.Contains(source, marker) {
				t.Errorf("%s is missing %q (run npm run build:ui after editing the core files)", file, marker)
			}
		}
	}
}
