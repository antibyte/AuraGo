package ui

import (
	"os"
	"strings"
	"testing"
)

func TestDesktopPetRuntimeSynchronizesBootstrapChanges(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	for _, marker := range []string{
		"function syncPetBootstrap(payload)",
		"state.bootstrap.active_pet_id = payload.active_pet_id || '';",
		"settings['pet.active_id'] = state.bootstrap.active_pet_id;",
		"case 'pet_changed':\n                syncPetBootstrap(event.payload);\n                loadPet();",
		"syncBootstrap: syncPetBootstrap",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("desktop pet runtime is missing bootstrap sync marker %q", marker)
		}
	}
	for _, marker := range []string{
		"const pet = pets.find(p => p.id === id);",
		"return pet || pets[0] || null;",
		"spriteEl.style.setProperty('--pet-row-y', '-' + (def.row * PET_FRAME_H) + 'px');",
		"spriteEl.style.setProperty('--pet-frame-end-x', '-' + (def.frames * PET_FRAME_W) + 'px');",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("desktop pet runtime is missing visibility fallback marker %q", marker)
		}
	}

	picker := readDesktopAssetText(t, "js/desktop/apps/pet-picker.js")
	for _, marker := range []string{
		"const body = await api('/api/desktop/pets?action=activate'",
		"syncPetBootstrap({ active_pet_id: body.active_pet_id || id });",
		"const body = await api('/api/desktop/settings'",
		"syncPetBootstrap({ settings: body.settings || { [key]: value } });",
	} {
		if !strings.Contains(picker, marker) {
			t.Fatalf("desktop pet picker is missing runtime sync marker %q", marker)
		}
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"function syncPetBootstrap(payload)",
		"syncBootstrap: syncPetBootstrap",
		"return pet || pets[0] || null;",
		"spriteEl.style.setProperty('--pet-row-y', '-' + (def.row * PET_FRAME_H) + 'px');",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle is missing runtime sync marker %q", marker)
		}
	}
}

func TestDesktopPetAssetURLsPreserveNestedSpritePaths(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"js/desktop/core/pet-runtime.js",
		"js/desktop/apps/pet-picker.js",
	} {
		source := readDesktopAssetText(t, path)
		for _, marker := range []string{
			"function petAssetURL(id, relPath)",
			".split('/')",
			".map(encodeURIComponent)",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s is missing nested pet asset URL marker %q", path, marker)
			}
		}
		if strings.Contains(source, "encodeURIComponent(pet.spritesheet") {
			t.Fatalf("%s must encode pet spritesheet path segment-by-segment, not as one escaped path", path)
		}
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	if !strings.Contains(bundle, "function petAssetURL(id, relPath)") {
		t.Fatalf("desktop main bundle is missing pet asset URL helper")
	}
}

func TestDesktopPetLayerSitsAboveWindowsByDefault(t *testing.T) {
	t.Parallel()

	base := readDesktopAssetText(t, "css/desktop-base.css")
	if !strings.Contains(base, "--vd-z-pet: 650;") {
		t.Fatalf("desktop base CSS must define --vd-z-pet above windows and dock but below menus")
	}

	petCSS := readDesktopAssetText(t, "css/desktop-pet.css")
	layerBody := desktopExactCSSRuleBody(t, petCSS, ".vd-pet-layer")
	if !strings.Contains(layerBody, "z-index: var(--vd-z-pet, 650);") {
		t.Fatalf("desktop pet layer should use --vd-z-pet, got %s", layerBody)
	}

	bundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, marker := range []string{
		"--vd-z-pet: 650;",
		"z-index: var(--vd-z-pet, 650);",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop shell CSS bundle is missing pet layer marker %q", marker)
		}
	}
}

func TestDesktopPetRuntimeHydratesMissingBootstrapPets(t *testing.T) {
	t.Parallel()

	runtime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	for _, marker := range []string{
		"let petRuntimeInitialized = false;",
		"let petCatalogHydration = null;",
		"async function hydratePetCatalog()",
		"api('/api/desktop/pets')",
		"api('/api/desktop/settings')",
		"hydratePetCatalog();",
		"if (petRuntimeInitialized) return;",
		"petRuntimeInitialized = true;",
		"initPetRuntime();",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("desktop pet runtime is missing bootstrap hydration marker %q", marker)
		}
	}

	picker := readDesktopAssetText(t, "js/desktop/apps/pet-picker.js")
	if !strings.Contains(picker, "syncPetBootstrap({ pets, active_pet_id: activeId, settings });\n                if (window.PetRuntime && typeof window.PetRuntime.load === 'function') window.PetRuntime.load();") {
		t.Fatal("desktop pet picker must re-render the runtime after loading pets/settings")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"let petRuntimeInitialized = false;",
		"let petCatalogHydration = null;",
		"async function hydratePetCatalog()",
		"api('/api/desktop/pets')",
		"api('/api/desktop/settings')",
		"hydratePetCatalog();",
		"if (petRuntimeInitialized) return;",
		"petRuntimeInitialized = true;",
		"initPetRuntime();",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle is missing pet hydration marker %q", marker)
		}
	}
}

func TestDesktopPetRuntimeLoadsEarlyAndStartsItself(t *testing.T) {
	t.Parallel()

	buildScriptBytes, err := os.ReadFile("../scripts/build-ui-bundles.js")
	if err != nil {
		t.Fatalf("read desktop bundle build script: %v", err)
	}
	buildScript := string(buildScriptBytes)
	foundationIdx := strings.Index(buildScript, "'ui/js/desktop/core/desktop-foundation.js'")
	petIdx := strings.Index(buildScript, "'ui/js/desktop/core/pet-runtime.js'")
	windowIdx := strings.Index(buildScript, "'ui/js/desktop/core/window-shell-runtime.js'")
	if foundationIdx < 0 || petIdx < 0 || windowIdx < 0 {
		t.Fatal("desktop bundle source list is missing expected core runtime entries")
	}
	if !(foundationIdx < petIdx && petIdx < windowIdx) {
		t.Fatalf("pet runtime must load immediately after desktop foundation; foundation=%d pet=%d window=%d", foundationIdx, petIdx, windowIdx)
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	foundationMarker := strings.Index(bundle, "/* ui/js/desktop/core/desktop-foundation.js */")
	petMarker := strings.Index(bundle, "/* ui/js/desktop/core/pet-runtime.js */")
	windowMarker := strings.Index(bundle, "/* ui/js/desktop/core/window-shell-runtime.js */")
	if foundationMarker < 0 || petMarker < 0 || windowMarker < 0 {
		t.Fatal("desktop main bundle is missing expected core runtime markers")
	}
	if !(foundationMarker < petMarker && petMarker < windowMarker) {
		t.Fatalf("desktop main bundle must load pet runtime immediately after foundation; foundation=%d pet=%d window=%d", foundationMarker, petMarker, windowMarker)
	}
}

func TestDesktopPetAnimationUsesRuntimePixelOffsets(t *testing.T) {
	t.Parallel()

	petCSS := readDesktopAssetText(t, "css/desktop-pet.css")
	for _, marker := range []string{
		"--pet-row-y: 0px;",
		"--pet-frame-end-x: -1152px;",
		"background-position: 0 var(--pet-row-y);",
		"to { background-position: var(--pet-frame-end-x) var(--pet-row-y); }",
	} {
		if !strings.Contains(petCSS, marker) {
			t.Fatalf("desktop pet CSS is missing stable animation marker %q", marker)
		}
	}
	if strings.Contains(petCSS, "calc(var(--pet-row)") || strings.Contains(petCSS, "calc(var(--pet-frames)") {
		t.Fatal("desktop pet CSS must not rely on unsupported calc() multiplication for sprite offsets")
	}

	bundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	if !strings.Contains(bundle, "background-position: 0 var(--pet-row-y);") {
		t.Fatal("desktop shell CSS bundle is missing stable pet background-position marker")
	}
	if strings.Contains(bundle, "calc(var(--pet-row)") || strings.Contains(bundle, "calc(var(--pet-frames)") {
		t.Fatal("desktop shell CSS bundle must not rely on unsupported calc() multiplication for pet sprite offsets")
	}
}

func TestDesktopPetAnimationSurvivesReducedMotionWhenEnabled(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"css/desktop-pet.css",
		"css/desktop-shell.bundle.css",
	} {
		source := readDesktopAssetText(t, path)
		for _, marker := range []string{
			"@media (prefers-reduced-motion: reduce)",
			".desktop-body[data-animations=\"true\"] .vd-pet-sprite",
			"animation-duration: var(--pet-duration) !important;",
			"animation-iteration-count: var(--pet-iterations) !important;",
		} {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s must keep desktop pet sprite frames animated under reduced motion when AuraGo animations are enabled; missing %q", path, marker)
			}
		}
	}
}

func TestDesktopPetReloadsAfterBootstrapRefresh(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, marker := range []string{
		"function refreshPetRuntime()",
		"window.PetRuntime.load();",
		"renderDesktop();\n            refreshPetRuntime();",
	} {
		if !strings.Contains(foundation, marker) {
			t.Fatalf("desktop foundation is missing pet refresh marker %q", marker)
		}
	}

	events := readDesktopAssetText(t, "js/desktop/core/sdk-events-bootstrap.js")
	if !strings.Contains(events, "renderDesktop();\n            refreshPetRuntime();\n            return;") {
		t.Fatal("desktop welcome event must refresh pet runtime after replacing bootstrap state")
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"function refreshPetRuntime()",
		"renderDesktop();\n            refreshPetRuntime();",
		"renderDesktop();\n            refreshPetRuntime();\n            return;",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle is missing pet bootstrap refresh marker %q", marker)
		}
	}
}
