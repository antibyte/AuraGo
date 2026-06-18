package ui

import (
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
	if !strings.Contains(base, "--vd-z-pet: 220;") {
		t.Fatalf("desktop base CSS must define --vd-z-pet between windows and dock")
	}

	petCSS := readDesktopAssetText(t, "css/desktop-pet.css")
	layerBody := desktopExactCSSRuleBody(t, petCSS, ".vd-pet-layer")
	if !strings.Contains(layerBody, "z-index: var(--vd-z-pet);") {
		t.Fatalf("desktop pet layer should use --vd-z-pet, got %s", layerBody)
	}

	bundle := readDesktopAssetText(t, "css/desktop-shell.bundle.css")
	for _, marker := range []string{
		"--vd-z-pet: 220;",
		"z-index: var(--vd-z-pet);",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop shell CSS bundle is missing pet layer marker %q", marker)
		}
	}
}
