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
		"/js/desktop/apps/agent-chat.js",
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
	if !strings.Contains(html, `<script defer src="/js/desktop/main.js?v={{.BuildVersion}}-desktop-20260523-store-romm-buttons"></script>`) {
		t.Fatal("desktop main.js script tag must be cache-busted with BuildVersion")
	}
}

func TestDesktopMainLoaderBumpsCacheAfterRomMStoreLayoutChanges(t *testing.T) {
	t.Parallel()

	main := rawDesktopAssetText(t, "js/desktop/main.js")
	if !strings.Contains(main, "var assetV = v + '-desktop-20260523-store-romm-buttons';") {
		t.Fatal("desktop main loader asset version must be bumped after RomM store layout changes")
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

func TestDesktopAgentChatUsesRegisteredRenderer(t *testing.T) {
	t.Parallel()

	main := rawDesktopAssetText(t, "js/desktop/main.js")
	router := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	agentChat := rawDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	agentChatIndex := strings.Index(main, "'/js/desktop/apps/agent-chat.js?v=' + assetV")
	routingIndex := strings.Index(main, "'/js/desktop/core/menus-and-routing.js?v=' + assetV")
	if agentChatIndex < 0 {
		t.Fatal("desktop main loader must load the agent chat app fragment")
	}
	if routingIndex < 0 {
		t.Fatal("desktop main loader must load the menus/routing fragment")
	}
	if !(agentChatIndex < routingIndex) {
		t.Fatalf("agent chat must load before menus/routing so its window registration executes inside the desktop runtime closure: agent=%d routing=%d", agentChatIndex, routingIndex)
	}
	if strings.Contains(router, "return renderChat(") {
		t.Fatal("desktop router must not call bare renderChat; split app modules should be referenced through stable window app registrations")
	}
	for _, want := range []string{
		"window.AgentChatApp",
		"typeof window.AgentChatApp.render === 'function'",
		"window.AgentChatApp.render(id, context || {})",
	} {
		if !strings.Contains(router, want) {
			t.Fatalf("desktop router missing agent chat renderer marker %q", want)
		}
	}
	for _, want := range []string{
		"window.AgentChatApp = window.AgentChatApp || {}",
		"window.AgentChatApp.render = renderChat",
		"window.renderChat = renderChat",
	} {
		if !strings.Contains(agentChat, want) {
			t.Fatalf("agent chat module missing exported renderer marker %q", want)
		}
	}
}

func TestDesktopModuleLoaderBypassesBrowserCacheForScriptParts(t *testing.T) {
	t.Parallel()

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if !strings.Contains(loader, "fetch(part, { credentials: 'same-origin', cache: 'no-store' })") {
		t.Fatal("desktop module loader must fetch script parts with cache no-store to avoid mixed stale fragments")
	}
}
