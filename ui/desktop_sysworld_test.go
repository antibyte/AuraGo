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
		"js/desktop/apps/sysworld.js":                  {"const instances = new Map()", "function dispose(windowId)", "instances.delete(windowId)", "cancelAnimationFrame", "IntersectionObserver", "MutationObserver", "setWindowMenus", "load.abort()", "city.esm.js", "versioned(", "/api/desktop/system-world/memory-artifacts", "cache:'no-store'", "artifacts.request?.abort()", "setHologram(", "inst.setPhoto", "city?.capture()", "URL.revokeObjectURL", "removeEventListener('keydown',photoKeys,true)", "sw-ready", "city.intro()", "onSoundFrame:", "inst.sound?.setTier?.(", "inst.sound?.frame?.("},
		"js/desktop/apps/sysworld-scene.js":            {"from 'three'", "GLTFLoader", "OrbitControls", "createCity", "InstancedMesh", "assetURL", "AbortController", "geoSet.forEach", "matSet.forEach", "forceContextLoss", "webglcontextlost", "requestPointerLock", "setMode", "lastQualityChange", "Math.hypot(forward, right)", "createMemoryHologram", "createAtmosphere", "createDrones", "obstaclesFrom", "hologram.dispose()", "atmosphere.dispose()", "drones.dispose()", "createSurfaces", "surfaces.dispose()", "surfaces.prepare(n)", "options.onHover", "options.onCut", "options.onProgress", "capture()", "intro()", "experience.zoom(", "experience.scoping()", "createWindows()", "windows.adopt(n)", "kitFor(id, catalog3)", "kitCatalog(catalog,catalog3)", "createLobbyRender(", "lobbyRender.adopt(n)", "kit3:catalog3", "furnishing().map(", "districts,placements,kitPlacements)", "createHarbour(", "createGulls(", "createParticles(", "surfaces.setWind(", "NEAR_HEIGHT", "createSoundFrame(", "options.onSoundFrame?.(", "createFurnitureSets(", "export function easePitch(", "aviationPoints(", "experience.lobby()", "windows.setEnvironment("},
		"js/desktop/apps/sysworld-experience.js":       {"'sky-deck'", "'sky-lift'", "'telescope'", "function rideSky", "function updateScope", "skyWalkable(", "options.onScope?.(null)", "'hover-pack'", "function startHover", "options.onFlight?.(null)", "function seat()", "createLobbyDoors()", "lobbyAirspace(", "lobbyGround(x,z)!==null", "furnishingPlaces(options.kit3)", "function doorValue(", "lobby:()=>lobby", "traffic.relocate(b,camera.position)"},
		"js/desktop/apps/sysworld-hover.js":            {"export function createHoverFlight", "export const hoverLimits", "land(instant", "cancel(){"},
		"js/desktop/apps/sysworld-layout.js":           {"export const skyDeck", "export function skyWalkable", "export const hoverPads"},
		"js/desktop/apps/sysworld-surfaces.js":         {"export function createSurfaces", "onBeforeCompile", "customProgramCacheKey", "export function windowSeeds", "Promise.allSettled", "texture.source.data?.close?.()", "#define SW_LEVEL ", "SW_SWAY"},
		"js/desktop/apps/sysworld-navigation.js":       {"export function createNavigator", "export function obstaclesFrom", "export function streetRoute", "a.state = 'turn'", "firstConflict", "laneClear", "sizes[p.asset]"},
		"js/desktop/apps/sysworld-hologram.js":         {"export function createMemoryHologram", "export function sanitizeArtifacts", "fillText", "AdditiveBlending", "setReducedMotion(value)"},
		"js/desktop/apps/sysworld-atmosphere.js":       {"export function createAtmosphere", "createCinema", "AdditiveBlending", "smoothstep(0.99955,0.99982,mu)", "setTier(", "setAviation(", "setFlash(", "dispose()", "lampLevel", "p=instanceMatrix*p;", "export function windAt(", "export function seaHeight(", "export function coneFade(", "export function roofTop(", "export function aviationPoints(", "attribute float fade;"},
		"js/desktop/apps/sysworld-windows.js":          {"export function createWindows", "export function windowLighting", "export const WINDOW_MATERIAL", "onBeforeCompile", "customProgramCacheKey", "#define WIN_LEVEL ", "USE_INSTANCING", "vWinRoom=vec2(uv.x,1.0-uv.y)", "dispose()", "export const WINDOW_TONES", "export const WINDOW_ENV", "vec3 winTone(float h)"},
		"js/desktop/apps/sysworld-cinema.js":           {"export function createCinema", "ShaderPass", "letterbox", "clamp(uv,0.0,1.0)", "dispose()"},
		"js/desktop/apps/sysworld-audio.js":            {"export function createCityAmbience", "createConvolver", "clearTimeout(eventTimer)", "thunder(", "setMood(", "createDynamicsCompressor", "export const LIMITER", "createSoundSpace(", "zoneWeights(", "aurago.desktop.sysworld.volume", "createSoundBeds(", "createSoundMovers(", "createRoomTones(", "createSoundRecordings("},
		"js/desktop/apps/sysworld-zones.js":            {"export function zoneWeights(", "export function surfaceAt(", "export function doorState(", "export const ROOMS", "export const SURFACE_KINDS", "export function createZoneTracker("},
		"js/desktop/apps/sysworld-sound-space.js":      {"export function createSoundSpace(", "export function glassLaw(", "export const ROOM_SPACES", "createConvolver()"},
		"js/desktop/apps/sysworld-sound-beds.js":       {"export function createSoundBeds(", "export function leafLevel(", "seaHeight("},
		"js/desktop/apps/sysworld-sound-movers.js":     {"export function createSoundMovers(", "export function tramLaw(", "craftPoses(", "import {gullPose,FLOCK}", "rollBands", "passGain"},
		"js/desktop/apps/sysworld-sound-rooms.js":      {"export function createRoomTones(", "export const ROOM_TONES"},
		"js/desktop/apps/sysworld-sound-frame.js":      {"export function createSoundFrame(", "windAt("},
		"js/desktop/apps/sysworld-sound-recordings.js": {"export function createSoundRecordings(", "export const RECORDINGS", "export function recordingLevel(", "export const SHOTS", "export const SHOT_IDS"},
		"js/desktop/apps/sysworld-furniture-lod.js":    {"export function createFurnitureSets(", "export const FURNITURE_LOD", "export function nearFlag("},
		"js/desktop/apps/sysworld-foley.js":            {"export function createCityFoley", "export const STEP_VOICES", "surfaceAt(", "export const PAIRS", "PAIR.sample"},
		"js/desktop/apps/sysworld-ground.js":           {"export function streetProfile", "export function streetModules", "export function pedestrianGraph", "export function carriagewayFree", "export const kerb", "export function onCarriageway"},
		"js/desktop/apps/sysworld-lobbies.js":          {"export const LOBBIES", "export const PODIUMS", "export function lobbyGround", "export function stepFree", "export function lobbySolids", "export function createLobbyDoors", "export function lobbyGraph", "export function treadCentre(", "export const FOOT_HALF"},
		"js/desktop/apps/sysworld-lobby-render.js":     {"export function createLobbyRender", "export const PANE_MATERIAL", "depthWrite:false", "getObjectByName('lobby')", "door_(\x5cd+)_([lr])", "dispose()", "setLighting("},
		"js/desktop/apps/sysworld-society.js":          {"export function residentShape", "snap:treadCentre"},
		"js/desktop/apps/sysworld-places.js":           {"streetPoints"},
		"js/desktop/apps/sysworld-footing.js":          {"export function springStep", "export function solveLeg", "export function adaptFeet", "export function findRig", "export function soleGaps(", "snap=null"},
		"js/desktop/apps/sysworld-contact.js":          {"export function createContactShadows", "dispose()"},
		"js/desktop/apps/sysworld-drones.js":           {"export function createDrones", "CatmullRomCurve3", "/^rotor_/", "dispose()"},
		"js/desktop/apps/sysworld-data.js":             {"const subscribers = new Set()", "inFlight.has(key)", "generation++", "AuraSSE?.off", "normalizeSystemMetrics", "failed: true", "configured", "/api/desktop/integrations/dashboard/overview", "/api/desktop/integrations/knowledge-graph/nodes?limit=300"},
		"js/desktop/apps/sysworld-hud.js":              {"iconMarkup", "'action'", "textContent", "sysworld.city.stale", "sw-map", "sw-source", "new ResizeObserver", "observer?.disconnect()", "'progressbar'", "sysworld.city.tour_step", "orient(", "drawSpark", "inst.reducedMotion()", "NS.map", "sw-map-camera", "sw-rail-toggle", "aria-expanded", "sw-scope", "sysworld.city.scope_idle", "flight(state)", "sysworld.city.flight_landing", "sysworld.city.flight_up", "sysworld.city.flight_down", "moveKey(key,false)", "e.preventDefault();button.setPointerCapture", "addEventListener('mousedown',e=>e.preventDefault())"},
		"css/desktop-app-sysworld.css":                 {"--sw-panel: var(--vd-theme-panel-bg", "prefers-reduced-motion", "@container", "pointer:coarse", "focus-visible", ".sysworld.sw-photo", ".sw-pin[data-pin=\"off\"]", ".sw-compass", ".sw-loading.sw-done", ".sw-map-board", ".sw-rail.sw-open", "data-mode=\"map\"", ".sw-scope-lens", ".sysworld.sw-scoping", ".sw-flight", ".sysworld.sw-flying", ".sw-flight-key"},
		"js/desktop/apps/sysworld-furnishing.js":       {"export function furnishing()", "export function violations(", "export const SIZES", "export function furnishingPlaces(", "export function neonSigns()", "export const SURFACES", "export const NEAR"},
		"js/desktop/apps/sysworld-parts.js":            {"export function createPartInstances("},
		"js/desktop/apps/sysworld-harbour.js":          {"export function createHarbour(", "export function craftPoses(", "HEAVE*seaHeight("},
		"js/desktop/apps/sysworld-gulls.js":            {"export function createGulls(", "'wing_l'"},
		"js/desktop/apps/sysworld-particles.js":        {"export function createParticles(", "export function steamVents()", "STEAM_MAX"},
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
	atmosphere := readDesktopAssetText(t, "js/desktop/apps/sysworld-atmosphere.js")
	if strings.Contains(atmosphere, "PlaneGeometry(150") {
		t.Fatal("moon must be a sky-dome disc, not a billboard quad")
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
		"sysworld.city.photo",
		"sysworld.city.photo_exit",
		"sysworld.city.photo_hint",
		"sysworld.city.photo_save",
		"sysworld.city.tour_step",
		"sysworld.compass.e",
		"sysworld.compass.n",
		"sysworld.compass.s",
		"sysworld.compass.w",
		"sysworld.city.scope",
		"sysworld.city.scope_hint",
		"sysworld.city.scope_idle",
		"sysworld.world.interact_telescope",
		"sysworld.world.interact_unscope",
		"sysworld.world.interact_skyup",
		"sysworld.world.interact_skydown",
		"sysworld.world.interact_skycall",
		"sysworld.world.skydeck",
		"sysworld.world.interact_hoverpack",
		"sysworld.world.interact_land",
		"sysworld.city.flight_title",
		"sysworld.city.flight_altitude",
		"sysworld.city.flight_speed",
		"sysworld.city.flight_landing",
		"sysworld.city.flight_hint",
		"sysworld.city.flight_up",
		"sysworld.city.flight_down",
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
