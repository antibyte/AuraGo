package ui

import (
	"strings"
	"testing"
)

func TestDesktopHTMLLoadsFragmentedAppsOnlyThroughMainLoader(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	mainLoader := rawDesktopAssetText(t, "js/desktop/main.js")
	mainBundle := readDesktopAssetText(t, "js/desktop/main.js")
	for _, part := range []string{
		"/js/desktop/apps/editor-filemenu.js",
		"/js/desktop/apps/planning-gallery-music.js",
		"/js/desktop/apps/quickconnect-launchpad-chat.js",
	} {
		if strings.Contains(html, `src="`+part) {
			t.Fatalf("desktop.html must not load bundle fragment %s directly", part)
		}
		if !strings.Contains(mainBundle, `/* ui`+part+` */`) {
			t.Fatalf("desktop main bundle must include fragment %s", part)
		}
	}
	if strings.Contains(mainBundle, "/js/desktop/apps/calendar.js") {
		t.Fatal("desktop main loader must not load calendar outside the desktop runtime closure")
	}
	if !strings.Contains(mainLoader, "loadBundle('main', '/js/desktop/bundles/main.bundle.js')") {
		t.Fatal("desktop main loader must load the prebuilt main bundle")
	}
	if !strings.Contains(html, `<script defer src="/js/desktop/main.js?v={{.BuildVersion}}"></script>`) {
		t.Fatal("desktop main.js script tag must be cache-busted with BuildVersion")
	}
	if !strings.Contains(html, `rel="preload" href="/js/desktop/bundles/main.bundle.js?v={{.BuildVersion}}"`) {
		t.Fatal("desktop.html must preload main.bundle.js with BuildVersion")
	}
}

func TestDesktopMainLoaderBumpsCacheAfterWindowAIContext(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	main := rawDesktopAssetText(t, "js/desktop/main.js")
	if !strings.Contains(html, "/js/desktop/main.js?v={{.BuildVersion}}") {
		t.Fatal("desktop main loader script tag must be cache-busted with BuildVersion")
	}
	if !strings.Contains(main, "/js/desktop/bundles/main.bundle.js") {
		t.Fatal("desktop main loader must point at the prebuilt main bundle")
	}
}

func TestDesktopCSSBumpsCacheAfterCalculatorLayoutFix(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `/css/desktop-shell.bundle.css?v={{.BuildVersion}}`) {
		t.Fatal("desktop shell CSS link must be cache-busted with BuildVersion")
	}

	css := rawDesktopAssetText(t, "css/desktop-shell.bundle.css")
	if !strings.Contains(css, `/* ui/css/desktop-windows.css */`) {
		t.Fatal("desktop shell CSS bundle must include desktop windows CSS")
	}
}

func TestDesktopMainBundleFragmentsKeepNormalizeZIndexBoundary(t *testing.T) {
	t.Parallel()

	main := readDesktopAssetText(t, "js/desktop/main.js")
	windowInteractions := rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js")
	menuRuntime := strings.TrimLeft(rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js"), "\ufeff\r\n\t ")
	if !strings.Contains(main, "/* ui/js/desktop/core/window-interactions-runtime.js */") {
		t.Fatal("desktop main loader must load the window interaction runtime chunk")
	}
	for _, marker := range []string{
		"function normalizeWindowZIndexes()",
		"wins.forEach((win, i) =>",
		"state.z = wins.length * 10;",
	} {
		if !strings.Contains(windowInteractions, marker) {
			t.Fatalf("window interaction runtime missing normalize z-index marker %q", marker)
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

	main := readDesktopAssetText(t, "js/desktop/main.js")
	planningIndex := strings.Index(main, "/* ui/js/desktop/apps/planning-gallery-music.js */")
	quickConnectIndex := strings.Index(main, "/* ui/js/desktop/apps/quickconnect-launchpad-chat.js */")
	sdkIndex := strings.Index(main, "/* ui/js/desktop/core/sdk-events-bootstrap.js */")
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

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	router := rawDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	agentChat := rawDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	if !strings.Contains(loader, "'/js/desktop/apps/agent-chat.js'") {
		t.Fatal("desktop module loader must lazy-load the agent chat app fragment")
	}
	if strings.Contains(router, "return renderChat(") {
		t.Fatal("desktop router must not call bare renderChat; split app modules should be referenced through stable window app registrations")
	}
	for _, want := range []string{
		"window.AgentChatApp",
		"typeof window.AgentChatApp.render === 'function'",
		"window.AgentChatApp.render(id, Object.assign({}, context || {}, { __desktopRuntime:",
		"contentEl, esc, desktopText, iconMarkup, api, loadBootstrap, showDesktopNotification",
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
	if strings.Contains(agentChat, "const host = contentEl(id)") {
		t.Fatal("agent chat module is lazy-loaded outside the desktop runtime closure and must not depend on contentEl")
	}
	if !strings.Contains(agentChat, "agentChatContentEl(id)") {
		t.Fatal("agent chat module must resolve its window content host independently")
	}
}

func TestDesktopModuleLoaderBypassesBrowserCacheForScriptParts(t *testing.T) {
	t.Parallel()

	loader := rawDesktopAssetText(t, "js/desktop/core/module-loader.js")
	if strings.Contains(loader, "(0, eval)") || strings.Contains(loader, "response.text()") {
		t.Fatal("desktop module loader must use prebuilt script tags instead of fetch/eval script parts")
	}
	if !strings.Contains(loader, "versionedURL(url)") {
		t.Fatal("desktop module loader must cache-bust lazy bundle URLs")
	}
	if !strings.Contains(loader, "modulePromises.delete(cacheKey);") {
		t.Fatal("desktop module loader must drop failed script bundle promises so retries can recover")
	}

	html := readDesktopAssetText(t, "desktop.html")
	if !strings.Contains(html, `/js/desktop/core/module-loader.js?v={{.BuildVersion}}`) {
		t.Fatal("desktop module-loader.js script tag must be cache-busted with BuildVersion")
	}
	if !strings.Contains(loader, "APP_I18N_SECTIONS") || !strings.Contains(loader, "loadAppI18nSections") {
		t.Fatal("desktop module loader must support lazy app i18n sections")
	}
}
