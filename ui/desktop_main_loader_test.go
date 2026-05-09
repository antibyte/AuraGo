package ui

import (
	"strings"
	"testing"
)

func TestDesktopHTMLLoadsFragmentedAppsOnlyThroughMainLoader(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	main := rawDesktopAssetText(t, "js/desktop/main.js")
	for _, part := range []string{
		"/js/desktop/apps/settings-calculator.js",
		"/js/desktop/apps/planning-gallery-music.js",
		"/js/desktop/apps/quickconnect-launchpad-chat.js",
	} {
		if strings.Contains(html, `src="`+part) {
			t.Fatalf("desktop.html must not load bundle fragment %s directly", part)
		}
		if !strings.Contains(main, `'`+part+`?v=' + assetV`) {
			t.Fatalf("desktop main loader must load bundle fragment %s with cache busting", part)
		}
	}
	if strings.Contains(main, "/js/desktop/apps/calendar.js") {
		t.Fatal("desktop main loader must not load calendar outside the desktop runtime closure")
	}
	if !strings.Contains(html, `<script defer src="/js/desktop/main.js?v={{.BuildVersion}}-desktop-20260509i"></script>`) {
		t.Fatal("desktop main.js script tag must be cache-busted with BuildVersion")
	}
}

func TestDesktopMainBundleFragmentsKeepNormalizeZIndexBoundary(t *testing.T) {
	t.Parallel()

	windowRuntime := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	menuRuntime := strings.TrimLeft(rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js"), "\ufeff\r\n\t ")
	for _, marker := range []string{
		"function normalizeWindowZIndexes()",
		"wins.forEach((win, i) =>",
		"state.z = wins.length * 10;",
	} {
		if !strings.Contains(windowRuntime, marker) {
			t.Fatalf("window shell runtime missing normalize z-index marker %q", marker)
		}
	}
	if strings.HasPrefix(menuRuntime, "state.z =") || strings.HasPrefix(menuRuntime, "wins.forEach") {
		t.Fatal("menus runtime must not start with a dangling normalizeWindowZIndexes function body")
	}
	if !strings.HasPrefix(menuRuntime, "function isEditableTarget") {
		t.Fatal("menus runtime must start at the context-menu runtime boundary")
	}
}

func TestDesktopMainEmbedsCalendarInsideRuntimeClosure(t *testing.T) {
	t.Parallel()

	main := rawDesktopAssetText(t, "js/desktop/main.js")
	planningIndex := strings.Index(main, "'/js/desktop/apps/planning-gallery-music.js?v=' + assetV")
	quickConnectIndex := strings.Index(main, "'/js/desktop/apps/quickconnect-launchpad-chat.js?v=' + assetV")
	sdkIndex := strings.Index(main, "'/js/desktop/core/sdk-events-bootstrap.js?v=' + assetV")
	for name, index := range map[string]int{
		"planning-gallery-music":      planningIndex,
		"quickconnect-launchpad-chat": quickConnectIndex,
		"sdk-events-bootstrap":        sdkIndex,
	} {
		if index < 0 {
			t.Fatalf("desktop main loader missing %s module", name)
		}
	}
	if !(planningIndex < quickConnectIndex && quickConnectIndex < sdkIndex) {
		t.Fatalf("desktop main loader must keep split app continuations before sdk bootstrap: planning=%d quickconnect=%d sdk=%d", planningIndex, quickConnectIndex, sdkIndex)
	}

	sdk := rawDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	calendarIndex := strings.Index(sdk, "async function renderCalendar(id)")
	initIndex := strings.Index(sdk, "async function init()")
	closeIndex := strings.LastIndex(sdk, "})();")
	if calendarIndex < 0 {
		t.Fatal("sdk-events-bootstrap must embed renderCalendar inside the desktop runtime closure")
	}
	if !(calendarIndex < initIndex && initIndex < closeIndex) {
		t.Fatalf("renderCalendar must be inside the runtime closure before init: calendar=%d init=%d close=%d", calendarIndex, initIndex, closeIndex)
	}
}
