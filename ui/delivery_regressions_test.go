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
		"const CACHE_SCHEMA_VERSION",
		"new URL(self.location.href).searchParams.get('v')",
		"return request.url;",
		"url.origin !== self.location.origin",
		"!isStaticAsset(url)",
		"request.headers.has('Range')",
		"event.respondWith(fetch(request))",
		"try { await cache.put(key, response.clone()); } catch (_) { }",
		"key.startsWith('aurago-') && key !== STATIC_CACHE",
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
		"window.addEventListener('pageshow', function (event)",
		"event.persisted",
		"!window.AuraSSE.isConnected()",
		"window.AuraSSE.connect();",
	} {
		if !strings.Contains(shared, marker) {
			t.Fatalf("shared lifecycle is missing BFCache contract marker %q", marker)
		}
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
