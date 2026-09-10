package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopSysWorldRegistrationOpensMaximized(t *testing.T) {
	t.Parallel()

	apps := desktop.BuiltinApps()
	var found *desktop.AppManifest
	for i := range apps {
		if apps[i].ID == "system-world" {
			found = &apps[i]
			break
		}
	}
	if found == nil {
		t.Fatal("desktop.BuiltinApps() missing system-world registration")
	}
	if !found.Builtin || !found.DockVisible || !found.StartVisible {
		t.Fatalf("system-world must be builtin and visible in dock/start, got %+v", found)
	}
	if found.Icon != "network" {
		t.Fatalf("system-world icon = %q, want network", found.Icon)
	}
	if found.Metadata["open_maximized"] != "true" {
		t.Fatalf("system-world must open maximized, metadata = %+v", found.Metadata)
	}
}

func TestDesktopSysWorldLazyAssetsRoutingAndWindowRuntime(t *testing.T) {
	t.Parallel()

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, want := range []string{
		"'system-world'",
		"'/css/desktop-app-sysworld.css'",
		"'/js/desktop/apps/sysworld-data.js'",
		"'/js/desktop/apps/sysworld-hud.js'",
		"'/js/desktop/apps/sysworld.js'",
		"'system-world': ['sysworld']",
	} {
		if !strings.Contains(loader, want) {
			t.Fatalf("desktop app asset registry missing System World marker %q", want)
		}
	}

	routing := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"appId === 'system-world'",
		"window.SysWorldApp",
		"window.SysWorldApp.render",
	} {
		if !strings.Contains(routing, want) {
			t.Fatalf("desktop routing missing System World marker %q", want)
		}
	}

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"'system-world': 'SysWorldApp'",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("desktop foundation missing System World marker %q", want)
		}
	}

	windows := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"'system-world': { width: 1440, height: 900 }",
	} {
		if !strings.Contains(windows, want) {
			t.Fatalf("desktop window runtime missing System World marker %q", want)
		}
	}
}

func TestDesktopSysWorldAppMarkers(t *testing.T) {
	t.Parallel()
	for file, markers := range map[string][]string{
		"js/desktop/apps/sysworld.js":       {"const instances = new Map()", "function dispose(windowId)", "instances.delete(windowId)", "cancelAnimationFrame", "IntersectionObserver", "MutationObserver", "setWindowMenus", "load.abort()", "city.esm.js", "versioned("},
		"js/desktop/apps/sysworld-scene.js": {"from 'three'", "GLTFLoader", "OrbitControls", "createCity", "InstancedMesh", "assetURL", "AbortController", "geoSet.forEach", "matSet.forEach", "forceContextLoss", "webglcontextlost", "requestPointerLock", "setMode", "lastQualityChange", "Math.hypot(forward, right)"},
		"js/desktop/apps/sysworld-data.js":  {"const subscribers = new Set()", "inFlight.has(key)", "generation++", "AuraSSE?.off", "normalizeSystemMetrics", "failed: true", "configured", "/api/dashboard/overview", "/api/knowledge-graph/nodes?limit=300"},
		"js/desktop/apps/sysworld-hud.js":   {"iconMarkup", "'action'", "textContent", "sysworld.city.stale", "sw-map", "sw-source"},
		"css/desktop-app-sysworld.css":      {"--sw-panel: var(--vd-theme-panel-bg", "prefers-reduced-motion", "@container", "pointer:coarse", "focus-visible"},
	} {
		source := readDesktopAssetText(t, file)
		for _, want := range markers {
			if !strings.Contains(source, want) {
				t.Errorf("%s missing %q", file, want)
			}
		}
	}
	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	section := strings.Split(strings.Split(loader, "'system-world': {")[1], "\n        }")[0]
	if strings.Contains(section, "three.min.js") || strings.Contains(section, "OrbitControls.min.js") {
		t.Fatal("City must not load legacy global Three.js")
	}
	scene := readDesktopAssetText(t, "js/desktop/apps/sysworld-scene.js")
	if strings.Contains(scene, "window.THREE") || strings.Contains(scene, "autoRotate") {
		t.Fatal("City must isolate Three.js and never take over an idle camera")
	}
}

func TestDesktopSysWorldTranslations(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.app_system_world",
		"sysworld.agent.busy",
		"sysworld.agent.idle",
		"sysworld.btn.effects",
		"sysworld.btn.graph",
		"sysworld.btn.overview",
		"sysworld.btn.quality",
		"sysworld.cat.ai",
		"sysworld.cat.communication",
		"sysworld.cat.infrastructure",
		"sysworld.cat.monitoring",
		"sysworld.cat.other",
		"sysworld.cat.smarthome",
		"sysworld.cat.storage",
		"sysworld.data_error",
		"sysworld.events",
		"sysworld.events.empty",
		"sysworld.legend",
		"sysworld.loading",
		"sysworld.init_error",
		"sysworld.no_webgl",
		"sysworld.panel.access_count",
		"sysworld.panel.category",
		"sysworld.panel.close",
		"sysworld.panel.disabled",
		"sysworld.panel.enabled",
		"sysworld.panel.image",
		"sysworld.panel.last_run",
		"sysworld.panel.next_run",
		"sysworld.panel.relations",
		"sysworld.panel.restarts",
		"sysworld.panel.schedule",
		"sysworld.panel.state",
		"sysworld.panel.status",
		"sysworld.panel.tokens",
		"sysworld.panel.type",
		"sysworld.panel.hint_keys",
		"sysworld.panel.model",
		"sysworld.panel.ports",
		"sysworld.panel.rank",
		"sysworld.panel.zone",
		"sysworld.kind.coagent",
		"sysworld.kind.container",
		"sysworld.kind.cron",
		"sysworld.kind.daemon",
		"sysworld.kind.integration",
		"sysworld.kind.kgnode",
		"sysworld.kind.mission",
		"sysworld.kind.object",
		"sysworld.kind.tool",
		"sysworld.sec.details",
		"sysworld.sec.relations",
		"sysworld.sec.status",
		"sysworld.time.ago",
		"sysworld.time.in",
		"sysworld.quality.high",
		"sysworld.quality.low",
		"sysworld.quality.medium",
		"sysworld.quality.ultra",
		"sysworld.state.done",
		"sysworld.state.error",
		"sysworld.state.exited",
		"sysworld.state.idle",
		"sysworld.state.paused",
		"sysworld.state.queued",
		"sysworld.state.running",
		"sysworld.state.waiting",
		"sysworld.stats.agent",
		"sysworld.stats.budget",
		"sysworld.stats.cpu",
		"sysworld.stats.memories",
		"sysworld.stats.missions",
		"sysworld.stats.ram",
		"sysworld.stats.uptime",
		"sysworld.zone.agents",
		"sysworld.zone.core",
		"sysworld.zone.graph",
		"sysworld.zone.infra",
		"sysworld.zone.integrations",
		"sysworld.zone.memory",
		"sysworld.zone.missions",
		"sysworld.zone.tools",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			data, err := Content.ReadFile(filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
			if err != nil {
				t.Fatalf("read %s desktop translations: %v", lang, err)
			}
			var values map[string]string
			if err := json.Unmarshal(data, &values); err != nil {
				t.Fatalf("parse %s desktop translations: %v", lang, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(values[key]) == "" {
					t.Fatalf("%s missing non-empty translation for %s", lang, key)
				}
			}
		})
	}
}
