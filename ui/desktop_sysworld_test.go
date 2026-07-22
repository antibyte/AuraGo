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
		"'/js/vendor/three.min.js'",
		"'/js/vendor/OrbitControls.min.js'",
		"'/js/desktop/apps/sysworld-effects.js'",
		"'/js/desktop/apps/sysworld-scene.js'",
		"'/js/desktop/apps/sysworld-core.js'",
		"'/js/desktop/apps/sysworld-orbit.js'",
		"'/js/desktop/apps/sysworld-graph.js'",
		"'/js/desktop/apps/sysworld-fleet.js'",
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
		"'system-world': true",
	} {
		if !strings.Contains(windows, want) {
			t.Fatalf("desktop window runtime missing System World marker %q", want)
		}
	}
}

func TestDesktopSysWorldAppMarkers(t *testing.T) {
	t.Parallel()

	app := readDesktopAssetText(t, "js/desktop/apps/sysworld.js")
	for _, want := range []string{
		"window.SysWorldApp = { render, dispose }",
		"const instances = new Map()",
		"function render(container, windowId, context = {})",
		"function dispose(windowId)",
		"instances.delete(windowId)",
		"aurago.desktop.sysworld.quality",
		"cancelAnimationFrame",
		"/api/dashboard/overview",
		"/api/dashboard/system",
		"/api/dashboard/memory",
		"/api/dashboard/activity",
		"/api/missions/v2",
		"/api/dashboard/tool-stats",
		"/api/containers",
		"/api/daemons",
		"/api/knowledge-graph/nodes",
		"/api/knowledge-graph/edges",
		"/api/personality/state",
		"/api/budget",
		"function normalizeSystemMetrics",
		"function applySystemMetrics",
		"usage_percent",
		"used_percent",
		"reg('system_metrics'",
		"sse.off(type, inst.sseHandlers[type])",
		"sysworld.no_webgl",
		"tickAmbientFx",
		"inst.effectsEnabled === false",
		"zoneAnchor",
		"cycleFocus",
		"function relTime",
		"updateSelLabel",
		"autoRotate",
		"showSelLabel",
		"inst.focused",
		"inst.follow",
		"clearFollow",
		"updateFollowTarget",
	} {
		if !strings.Contains(app, want) {
			t.Fatalf("System World entry missing implementation marker %q", want)
		}
	}

	modules := map[string][]string{
		"js/desktop/apps/sysworld-effects.js": {"window.SysWorld", "NS.createFx", "NS.PALETTE", "glowTexture", "comet", "beam", "sparkle", "tween", "textSprite", "hoverRing", "selectBeacon", "clearBeacon"},
		"js/desktop/apps/sysworld-scene.js":   {"window.SysWorld", "NS.createStage", "NS.LAYOUT", "THREE.OrbitControls", "flyTo", "introFlight", "sysworld-dust", "sysworld-aurora"},
		"js/desktop/apps/sysworld-core.js":    {"window.SysWorld", "NS.createCore", "setMood", "setMemory", "memoryFlash", "punch", "sysworld-core-halo"},
		"js/desktop/apps/sysworld-orbit.js":   {"window.SysWorld", "NS.createOrbit", "setIntegrations", "pickables", "satellitePosition", "textSprite", "categoryGeo"},
		"js/desktop/apps/sysworld-graph.js":   {"window.SysWorld", "NS.createGraph", "build", "expand", "setVisible", "pickables", "highlightNeighbors"},
		"js/desktop/apps/sysworld-fleet.js":   {"window.SysWorld", "NS.createFleet", "setMissions", "setCoAgents", "setTools", "setInfra", "flashTool", "textSprite", "tickGeo", "finGeo", "gearGeo", "plateEdgeGeo", "containerName"},
		"js/desktop/apps/sysworld-hud.js":     {"window.SysWorld", "NS.createHud", "showPanel", "showTooltip", "setStats", "setLegend", "showSelLabel", "positionSelLabel", "hideSelLabel", "data-sw-zone", "onZoneHover"},
	}
	for file, markers := range modules {
		body := readDesktopAssetText(t, file)
		for _, want := range markers {
			if !strings.Contains(body, want) {
				t.Fatalf("%s missing marker %q", file, want)
			}
		}
	}

	css := readDesktopAssetText(t, "css/desktop-app-sysworld.css")
	for _, want := range []string{
		".sysworld",
		".sysworld-canvas",
		".sysworld-hud",
		".sw-stats",
		".sw-actions",
		".sw-legend",
		".sw-events",
		".sw-tooltip",
		".sw-info",
		".sw-loading",
		".sw-fallback",
		".sw-sel-label",
		".sw-section",
		".sw-bar-fill",
		".sw-pill",
		"sysworld-panel-sheen",
		"repeating-linear-gradient",
		"prefers-reduced-motion",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("System World CSS missing marker %q", want)
		}
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
