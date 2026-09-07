package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The MeshCore widget runtime must live in its own core file, render radio
// text via textContent only and clean up every listener and timer it owns.
func TestDesktopMeshCoreWidgetRuntimeIsRegistered(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-meshcore-runtime.js")
	for _, want := range []string{
		"function renderMeshCoreWidget(container)",
		"api('/api/meshcore/messenger/bootstrap')",
		"document.addEventListener('aurago:meshcore-change', onChange)",
		"document.removeEventListener('aurago:meshcore-change', onChange)",
		"registerWidgetCleanup(() => {",
		"clearInterval(pollTimer)",
		"clearTimeout(refreshTimer)",
		"MESH_WIDGET_ID_PATTERN",
		"openApp('meshcore'",
		"preview.textContent = previewText",
		"document.createTextNode(t('desktop.widget_meshcore_protected'))",
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("meshcore widget runtime missing marker %q", want)
		}
	}

	// Radio text must never reach innerHTML: only the static skeleton and the
	// trusted lock icon markup may assign non-empty innerHTML.
	if strings.Contains(runtime, "innerHTML = preview") || strings.Contains(runtime, "innerHTML += ") {
		t.Fatal("meshcore widget runtime must not render message text via innerHTML")
	}

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "widget.id === 'builtin-meshcore'") {
		t.Fatal("builtin widget dispatch is missing the builtin-meshcore branch")
	}
	if !strings.Contains(shell, "renderMeshCoreWidget(container)") {
		t.Fatal("builtin widget dispatch does not call renderMeshCoreWidget")
	}
	if !strings.Contains(shell, "widgetID === 'builtin-meshcore'") {
		t.Fatal("defaultWidgetBounds is missing the builtin-meshcore default slot")
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "if (id === 'builtin-meshcore') return t('desktop.widget_meshcore_title');") {
		t.Fatal("widgetDisplayTitle is missing the builtin-meshcore mapping")
	}
}

// The widget is seeded hidden (opt-in via the widget drawer) and linked to the
// builtin meshcore app for the context-menu "open app" action.
func TestDesktopMeshCoreWidgetIsSeeded(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "internal", "desktop", "service.go"))
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`ID: "builtin-meshcore"`,
		`AppID: "meshcore"`,
		"widget.ID, widget.AppID, widget.Title",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("desktop widget seed missing marker %q", want)
		}
	}
}

// The bundle part order is contractual: the MeshCore widget runtime ships in
// the main desktop bundle directly after the sysmon widget runtime.
func TestDesktopMeshCoreWidgetRuntimeIsBundled(t *testing.T) {
	t.Parallel()

	buildScript, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read build-ui-bundles.js: %v", err)
	}
	text := string(buildScript)
	const part = "'ui/js/desktop/core/widget-meshcore-runtime.js'"
	if !strings.Contains(text, part) {
		t.Fatal("build-ui-bundles.js is missing the widget-meshcore-runtime.js bundle part")
	}
	sysmonIdx := strings.Index(text, "'ui/js/desktop/core/widget-sysmon-runtime.js'")
	meshIdx := strings.Index(text, part)
	mediaIdx := strings.Index(text, "'ui/js/desktop/core/media-keys-runtime.js'")
	if sysmonIdx < 0 || mediaIdx < 0 || meshIdx < sysmonIdx || meshIdx > mediaIdx {
		t.Fatal("widget-meshcore-runtime.js must be bundled after widget-sysmon-runtime.js and before media-keys-runtime.js")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "function renderMeshCoreWidget(container)") {
		t.Fatal("main desktop bundle does not contain the MeshCore widget runtime (run npm run build:ui)")
	}
}

// Widget styles follow the shared widget visual language and stay token-based
// so both desktop themes render them correctly.
func TestDesktopMeshCoreWidgetStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".vd-mesh {",
		".vd-mesh.is-ready",
		".vd-mesh-title-row",
		".vd-mesh-status",
		".vd-mesh-row {",
		".vd-mesh-preview",
		".vd-mesh-row-unread",
		".vd-mesh-notice",
		"var(--vd-accent)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop stylesheets missing meshcore widget marker %q", want)
		}
	}
}

// The widget strings must be translated in every supported desktop locale.
func TestDesktopMeshCoreWidgetI18n(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.widget_meshcore_title",
		"desktop.widget_meshcore_open",
		"desktop.widget_meshcore_protected",
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
}
