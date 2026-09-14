package ui

import (
	"os"
	"strings"
	"testing"
)

func assetStructureIndex(source, fragment string) int {
	source = strings.Join(strings.Fields(normalizeAssetText([]byte(source))), " ")
	fragment = strings.Join(strings.Fields(fragment), " ")
	if fragment == "" {
		return 0
	}
	return strings.Index(source, fragment)
}

func containsAssetStructure(source, fragment string) bool {
	if strings.TrimSpace(fragment) == "" {
		return true
	}
	return assetStructureIndex(source, fragment) >= 0
}

func TestServiceWorkerKeepsPushHandlersAndSecureStaticCachePolicy(t *testing.T) {
	t.Parallel()

	sw := normalizeAssetText(mustReadUIFile(t, "sw.js"))
	for _, marker := range []string{
		"self.addEventListener('push'",
		"self.addEventListener('notificationclick'",
		"self.addEventListener('notificationclose'",
		"new URL(rawTarget, self.location.origin)",
		"target.origin === self.location.origin",
		"const CACHE_SCHEMA_VERSION = '4'",
		"if (!url.searchParams.get('v')) return;",
		"new URL(self.location.href).searchParams.get('v')",
		"return request.url;",
		"url.origin !== self.location.origin",
		"!isStaticAsset(url)",
		"request.headers.has('Range')",
		"event.respondWith(fetch(request))",
		"try { await cache.put(key, response.clone()); } catch (_) { }",
		"key.startsWith('aurago-') && key !== STATIC_CACHE",
		"CORE_ASSETS.map(url => cache.add(url).catch(() => {}))",
	} {
		if !strings.Contains(sw, marker) {
			t.Fatalf("service worker is missing delivery contract marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"const HTML_CACHE",
		"cache.put(request.url, networkResponse.clone())",
		"`${url.origin}${url.pathname}`",
	} {
		if strings.Contains(sw, forbidden) {
			t.Fatalf("service worker must not keep unsafe cache behavior %q", forbidden)
		}
	}
	shared := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	if strings.Count(shared, "updateViaCache: 'none'") != 2 {
		t.Fatal("service worker registration and retry must both bypass the HTTP cache")
	}
}

func TestSharedLifecycleOrdersDisposerBeforeSSEAndPreservesBFCache(t *testing.T) {
	t.Parallel()

	shared := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	disposer := strings.Index(shared, "window.AuraDisposer = (function ()")
	sse := strings.Index(shared, "window.AuraSSE = (function ()")
	if disposer < 0 || sse < 0 || disposer > sse {
		t.Fatalf("AuraDisposer must be defined before AuraSSE: disposer=%d sse=%d", disposer, sse)
	}
	for _, marker := range []string{
		"window.addEventListener('pagehide', function (event)",
		"if (event.persisted) return;",
		"window.addEventListener('beforeunload'",
		"document.addEventListener('visibilitychange', _resume)",
		"window.addEventListener('pageshow', _resume)",
		"window.addEventListener('online', _resume)",
		"document.addEventListener('freeze', _suspend)",
		"_es.readyState === EventSource.OPEN",
		"X-AuraGo-Asset-Set",
	} {
		if !strings.Contains(shared, marker) {
			t.Fatalf("shared lifecycle is missing PWA resume contract marker %q", marker)
		}
	}
}

func TestPWAManifestHasStableStartURL(t *testing.T) {
	t.Parallel()

	manifest := normalizeAssetText(mustReadUIFile(t, "site.webmanifest"))
	for _, marker := range []string{
		`"start_url": "/"`,
		`"scope": "/"`,
		`"id": "/"`,
		`"display": "standalone"`,
		`"lang": "en"`,
		`"description":`,
		`"purpose": "any"`,
		`"purpose": "maskable"`,
		`"shortcuts"`,
		`"url": "/"`,
		`"url": "/desktop"`,
	} {
		if !strings.Contains(manifest, marker) {
			t.Fatalf("PWA manifest is missing installable start contract %q", marker)
		}
	}
}

func TestPWAHTMLDeclaresAppleWebAppCapable(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read ui directory: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		html := normalizeAssetText(mustReadUIFile(t, entry.Name()))
		if !strings.Contains(html, `href="/site.webmanifest"`) {
			continue
		}
		checked++
		for _, marker := range []string{
			`name="apple-mobile-web-app-capable"`,
			`name="mobile-web-app-capable"`,
			`name="apple-mobile-web-app-title"`,
		} {
			if !strings.Contains(html, marker) {
				t.Fatalf("%s is missing PWA install meta %q", entry.Name(), marker)
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected HTML pages that link the web manifest")
	}
}

func TestPWARegistrationRetriesOneTransientScriptFetchFailure(t *testing.T) {
	t.Parallel()

	shared := normalizeAssetText(mustReadUIFile(t, "js/shared/shared-core.js"))
	for _, marker := range []string{
		"const swURL = serviceWorkerURL()",
		"navigator.serviceWorker.register(swURL, { updateViaCache: 'none' })",
		"setTimeout(resolve, 1500)",
		"registered after retry",
		"initial error:",
		"if (!('serviceWorker' in navigator)) {",
		"name: 'apple-mobile-web-app-capable'",
		"name: 'mobile-web-app-capable'",
	} {
		if !strings.Contains(shared, marker) {
			t.Fatalf("PWA registration retry is missing %q", marker)
		}
	}
	registerAt := strings.Index(shared, "navigator.serviceWorker.register(swURL, { updateViaCache: 'none' })")
	pushGateAt := strings.LastIndex(shared, "!('PushManager' in window)")
	if registerAt < 0 || pushGateAt < 0 || pushGateAt < registerAt {
		t.Fatal("service worker registration must run even when PushManager is missing")
	}
}

func TestBundleBuilderCheckModeIsReadOnlyAndBundlesShipRuntimeSources(t *testing.T) {
	t.Parallel()

	buildBytes, err := os.ReadFile("../scripts/build-ui-bundles.js")
	if err != nil {
		t.Fatalf("read bundle builder: %v", err)
	}
	build := normalizeAssetText(buildBytes)
	for _, marker := range []string{
		"const checkOnly = process.argv.includes('--check');",
		"value.replace(/\\r\\n?/g, '\\n')",
		"const expected = Buffer.from(content, 'utf8');",
		"!existing.equals(expected)",
		"process.exitCode = 1",
	} {
		if !strings.Contains(build, marker) {
			t.Fatalf("bundle builder is missing deterministic check marker %q", marker)
		}
	}

	chatBundle := normalizeAssetText(mustReadUIFile(t, "js/chat/bundles/chat-runtime.bundle.js"))
	for _, marker := range []string{
		"/* ui/js/chat/modules/smart-scroller.js */",
		"window.SmartScroller = SmartScroller;",
	} {
		if !strings.Contains(chatBundle, marker) {
			t.Fatalf("chat runtime bundle is missing SmartScroller marker %q", marker)
		}
	}
	desktopBundle := normalizeAssetText(mustReadUIFile(t, "js/desktop/bundles/main.bundle.js"))
	for _, marker := range []string{
		"/* ui/js/desktop/core/pet-runtime.js */",
		"syncBootstrap: syncPetBootstrap",
	} {
		if !strings.Contains(desktopBundle, marker) {
			t.Fatalf("desktop main bundle is missing Pet marker %q", marker)
		}
	}
}
