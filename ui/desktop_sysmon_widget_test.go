package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sysmon widget runtime must live in its own core file, be registered as a
// main-bundle part and be wired into the builtin widget dispatch.
func TestDesktopSysmonWidgetRuntimeIsRegistered(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-sysmon-runtime.js")
	for _, want := range []string{
		"function renderSysmonWidget(container)",
		"api('/api/dashboard/system')",
		"window.AuraSSE.on('system_metrics', sseHandler)",
		"window.AuraSSE.off('system_metrics', sseHandler)",
		"registerWidgetCleanup(() => {",
		"SYSMON_HISTORY_LEN",
		"host_uptime_seconds",
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("sysmon widget runtime missing marker %q", want)
		}
	}

	// Live updates must happen in place: exactly one innerHTML assignment
	// (the initial DOM build), never a rebuild inside the update path.
	if got := strings.Count(runtime, "innerHTML"); got != 1 {
		t.Fatalf("sysmon widget runtime must build DOM once via innerHTML, found %d occurrences", got)
	}

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "widget.id === 'builtin-sysmon'") {
		t.Fatal("builtin widget dispatch is missing the builtin-sysmon branch")
	}
	if !strings.Contains(shell, "renderSysmonWidget(container)") {
		t.Fatal("builtin widget dispatch does not call renderSysmonWidget")
	}
}

// The bundle part order is contractual: the sysmon runtime ships in the main
// desktop bundle between the widget drawer runtime and menus-and-routing.
func TestDesktopSysmonWidgetRuntimeIsBundled(t *testing.T) {
	t.Parallel()

	buildScript, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read build-ui-bundles.js: %v", err)
	}
	text := string(buildScript)
	const part = "'ui/js/desktop/core/widget-sysmon-runtime.js'"
	if !strings.Contains(text, part) {
		t.Fatal("build-ui-bundles.js is missing the widget-sysmon-runtime.js bundle part")
	}
	drawerIdx := strings.Index(text, "'ui/js/desktop/core/widget-drawer-runtime.js'")
	sysmonIdx := strings.Index(text, part)
	menusIdx := strings.Index(text, "'ui/js/desktop/core/menus-and-routing.js'")
	if drawerIdx < 0 || menusIdx < 0 || sysmonIdx < drawerIdx || sysmonIdx > menusIdx {
		t.Fatal("widget-sysmon-runtime.js must be bundled after widget-drawer-runtime.js and before menus-and-routing.js")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "function renderSysmonWidget(container)") {
		t.Fatal("main desktop bundle does not contain the sysmon widget runtime (run npm run build:ui)")
	}
}

// Widget styles follow the shared widget visual language and stay token-based
// so both desktop themes render them correctly.
func TestDesktopSysmonWidgetStyles(t *testing.T) {
	t.Parallel()

	css := readAllDesktopCSS(t)
	for _, want := range []string{
		".vd-sysmon {",
		".vd-sysmon.is-ready",
		".vd-sysmon-title-row",
		".vd-sysmon-bar {",
		".vd-sysmon-bar-fill {",
		".vd-sysmon-fill-cpu",
		".vd-sysmon-fill-mem",
		".vd-sysmon-fill-disk",
		".vd-sysmon-spark-svg",
		".vd-sysmon-footer",
		"var(--vd-accent)",
		"var(--vd-coral)",
		"var(--vd-amber)",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop stylesheets missing sysmon widget marker %q", want)
		}
	}
}

// The widget title must be translated in every supported desktop locale.
func TestDesktopSysmonWidgetI18n(t *testing.T) {
	t.Parallel()

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
		if strings.TrimSpace(values["desktop.widget_sysmon_title"]) == "" {
			t.Fatalf("%s missing non-empty translation for desktop.widget_sysmon_title", path)
		}
	}
}
