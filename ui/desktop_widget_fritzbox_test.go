package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Fritz!Box widget runtime lives in its own core files, talks only to the
// sanitized desktop overview endpoint, renders router text via textContent and
// releases every timer, observer and listener it owns.
func TestDesktopFritzBoxWidgetRuntimeIsRegistered(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-fritzbox-runtime.js")
	for _, want := range []string{
		"function renderFritzBoxWidget(container)",
		"api('/api/desktop/fritzbox/overview?sections=' + encodeURIComponent(sections.join(',')), { signal: request.signal })",
		"new AbortController()",
		"registerWidgetCleanup(() => {",
		"clearTimeout(state.timer)",
		"clearTimeout(state.copyTimer)",
		"document.removeEventListener('visibilitychange', onVisibility)",
		"if (observer) observer.disconnect();",
		"FRITZ_WIDGET_PAGE_KEY",
		"role=\"tablist\"",
		"fritzMergeMonitorSamples(state.history",
		"fritzAreaChartSVG({",
		"err.message === 'fritzbox_disabled'",
		// The widget card captures the pointer on pointerdown so it can be
		// moved. Swipe paging therefore stays touch/pen only and finishes on
		// window-level capture listeners that the cleanup removes again.
		"event.pointerType === 'mouse'",
		"window.addEventListener('pointermove', onPointerMove, true)",
		"window.addEventListener('pointerup', endDrag, true)",
		"window.addEventListener('pointercancel', endDrag, true)",
		"window.removeEventListener('pointermove', onPointerMove, true)",
		"window.removeEventListener('pointerup', endDrag, true)",
		"window.removeEventListener('pointercancel', endDrag, true)",
		"if (event.buttons === 0 ||",
		"refs.viewport.addEventListener('wheel', event => {",
	} {
		if !strings.Contains(runtime, want) {
			t.Fatalf("fritzbox widget runtime missing marker %q", want)
		}
	}

	// Router-provided strings (host names, caller names, numbers, addresses)
	// must only ever reach the DOM through textContent. innerHTML is reserved
	// for the static shell, generated numeric SVG markup and trusted glyphs.
	// Pointer release must never be observed on the viewport alone because
	// the captured card swallows it and the pager would stick in drag mode.
	for _, forbidden := range []string{
		"innerHTML += ",
		"innerHTML = host",
		"innerHTML = call",
		"innerHTML = esc(",
		"alert(",
		"fetch('/api/fritzbox",
		"refs.viewport.addEventListener('pointerup'",
		"refs.viewport.addEventListener('pointermove'",
		"refs.viewport.setPointerCapture(",
	} {
		if strings.Contains(runtime, forbidden) {
			t.Fatalf("fritzbox widget runtime must not contain %q", forbidden)
		}
	}

	charts := readDesktopAssetText(t, "js/desktop/core/widget-fritzbox-charts.js")
	for _, want := range []string{
		"function fritzAreaChartSVG(",
		"function fritzRingSVG(",
		"function fritzGaugeSVG(",
		"function fritzSparkSVG(",
		"function fritzMergeMonitorSamples(",
		"function fritzFormatBits(",
	} {
		if !strings.Contains(charts, want) {
			t.Fatalf("fritzbox chart helpers missing %q", want)
		}
	}

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	if !strings.Contains(shell, "widget.id === 'builtin-fritzbox'") {
		t.Fatal("builtin widget dispatch is missing the builtin-fritzbox branch")
	}
	if !strings.Contains(shell, "renderFritzBoxWidget(container)") {
		t.Fatal("builtin widget dispatch does not call renderFritzBoxWidget")
	}
	if !strings.Contains(shell, "widgetID === 'builtin-fritzbox'") {
		t.Fatal("defaultWidgetBounds is missing the builtin-fritzbox default slot")
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	if !strings.Contains(foundation, "if (id === 'builtin-fritzbox') return t('desktop.widget_fritzbox_title');") {
		t.Fatal("widgetDisplayTitle is missing the builtin-fritzbox mapping")
	}
}

// The widget is seeded hidden (opt-in via the widget drawer) with the shared
// network icon and the same width as the other compact builtin widgets.
func TestDesktopFritzBoxWidgetIsSeeded(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "internal", "desktop", "service.go"))
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	text := string(data)
	idx := strings.Index(text, `ID: "builtin-fritzbox"`)
	if idx < 0 {
		t.Fatal("desktop widget seed is missing builtin-fritzbox")
	}
	entry := text[idx:]
	if end := strings.Index(entry, "}"); end > 0 {
		entry = entry[:end]
	}
	for _, want := range []string{`Icon: "network"`, "Visible: false", "Builtin: true"} {
		if !strings.Contains(entry, want) {
			t.Fatalf("builtin-fritzbox seed entry missing %q in %q", want, entry)
		}
	}
}

// The bundle part order is contractual: chart helpers ship before the runtime,
// both directly after the printer widget in the main desktop bundle, and the
// stylesheet is part of the desktop shell CSS bundle.
func TestDesktopFritzBoxWidgetRuntimeIsBundled(t *testing.T) {
	t.Parallel()

	buildScript, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read build-ui-bundles.js: %v", err)
	}
	text := string(buildScript)
	const chartsPart = "'ui/js/desktop/core/widget-fritzbox-charts.js'"
	const runtimePart = "'ui/js/desktop/core/widget-fritzbox-runtime.js'"
	printerIdx := strings.Index(text, "'ui/js/desktop/core/widget-printer-runtime.js'")
	chartsIdx := strings.Index(text, chartsPart)
	runtimeIdx := strings.Index(text, runtimePart)
	stickyIdx := strings.Index(text, "'ui/js/desktop/core/sticky-notes-runtime.js'")
	if printerIdx < 0 || chartsIdx < 0 || runtimeIdx < 0 || stickyIdx < 0 {
		t.Fatal("build-ui-bundles.js is missing a fritzbox widget bundle part")
	}
	if !(printerIdx < chartsIdx && chartsIdx < runtimeIdx && runtimeIdx < stickyIdx) {
		t.Fatal("fritzbox widget parts must follow widget-printer-runtime.js (charts before runtime) and precede sticky-notes-runtime.js")
	}
	if !strings.Contains(text, "'ui/css/desktop-widget-fritzbox.css'") {
		t.Fatal("desktop shell CSS bundle is missing desktop-widget-fritzbox.css")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, want := range []string{"function fritzAreaChartSVG(", "function renderFritzBoxWidget(container)"} {
		if !strings.Contains(bundle, want) {
			t.Fatalf("main desktop bundle does not contain %q (run npm run build:ui)", want)
		}
	}
	cssBundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	if !strings.Contains(cssBundle, ".vd-fritz {") {
		t.Fatal("desktop shell CSS bundle does not contain the fritzbox widget styles (run npm run build:ui)")
	}
}

// Widget styles follow the shared widget visual language, stay token-based for
// both desktop themes and honor the animation and reduced-motion gates.
func TestDesktopFritzBoxWidgetStyles(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-widget-fritzbox.css")
	for _, want := range []string{
		".vd-fritz {",
		".vd-fritz.is-ready",
		".vd-fritz-track",
		".vd-fritz-dotbtn.is-active",
		".vd-fritz-chart",
		".vd-fritz-gauge",
		".vd-fritz-ring",
		".vd-fritz.is-compact",
		"var(--vd-accent)",
		"var(--vd-coral)",
		"var(--vd-amber)",
		"var(--ds-color-fg-muted)",
		"prefers-reduced-motion",
		"hover: none",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("fritzbox widget stylesheet missing marker %q", want)
		}
	}
	if !strings.Contains(readAllDesktopCSS(t), ".vd-fritz-page:not(.is-active)") {
		t.Fatal("desktop stylesheets do not include the fritzbox widget file")
	}
}

// The widget strings must be translated in every supported desktop locale and
// keep their placeholders intact.
func TestDesktopFritzBoxWidgetI18n(t *testing.T) {
	t.Parallel()

	placeholders := map[string][]string{
		"desktop.widget_fritzbox_connected_for": {"{duration}"},
		"desktop.widget_fritzbox_of_max":        {"{rate}"},
		"desktop.widget_fritzbox_window_label":  {"{span}"},
		"desktop.widget_fritzbox_devices_total": {"{total}"},
		"desktop.widget_fritzbox_more_devices":  {"{count}"},
		"desktop.widget_fritzbox_page_of":       {"{current}", "{total}"},
	}
	keys := []string{
		"desktop.widget_fritzbox_title",
		"desktop.widget_fritzbox_page_connection",
		"desktop.widget_fritzbox_page_traffic",
		"desktop.widget_fritzbox_page_devices",
		"desktop.widget_fritzbox_page_telephony",
		"desktop.widget_fritzbox_online",
		"desktop.widget_fritzbox_offline",
		"desktop.widget_fritzbox_connecting",
		"desktop.widget_fritzbox_download",
		"desktop.widget_fritzbox_upload",
		"desktop.widget_fritzbox_copy_ip",
		"desktop.widget_fritzbox_access_dsl",
		"desktop.widget_fritzbox_access_cable",
		"desktop.widget_fritzbox_access_fiber",
		"desktop.widget_fritzbox_access_ethernet",
		"desktop.widget_fritzbox_access_mobile",
		"desktop.widget_fritzbox_access_other",
		"desktop.widget_fritzbox_no_devices",
		"desktop.widget_fritzbox_no_calls",
		"desktop.widget_fritzbox_missed_today",
		"desktop.widget_fritzbox_tam_new",
		"desktop.widget_fritzbox_disabled_hint",
		"desktop.widget_fritzbox_no_sections",
		"desktop.widget_fritzbox_error",
		"desktop.widget_fritzbox_error_auth",
		"desktop.widget_fritzbox_stale",
		"desktop.widget_fritzbox_prev_page",
		"desktop.widget_fritzbox_next_page",
	}
	for key := range placeholders {
		keys = append(keys, key)
	}

	loadLocale := func(lang string) (string, map[string]string) {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		return path, values
	}
	countFritzKeys := func(values map[string]string) int {
		n := 0
		for key := range values {
			if strings.HasPrefix(key, "desktop.widget_fritzbox_") {
				n++
			}
		}
		return n
	}

	_, en := loadLocale("en")
	enCount := countFritzKeys(en)
	if enCount < len(keys) {
		t.Fatalf("en has only %d fritzbox widget keys", enCount)
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path, values := loadLocale(lang)
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
			for _, ph := range placeholders[key] {
				if !strings.Contains(values[key], ph) {
					t.Fatalf("%s translation for %s lost placeholder %s", path, key, ph)
				}
			}
		}
		if got := countFritzKeys(values); got != enCount {
			t.Fatalf("%s has %d fritzbox widget keys, en has %d", path, got, enCount)
		}
	}
}
