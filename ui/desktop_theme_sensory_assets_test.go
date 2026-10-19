package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestNewDesktopThemeCursorsAreNativeAndComplete(t *testing.T) {
	t.Parallel()

	roles := []string{"default", "pointer", "text", "grab", "grabbing", "move", "ew-resize", "ns-resize", "nesw-resize", "nwse-resize", "not-allowed", "wait"}
	themes := []string{"electronic_dreams", "windows_xp", "green_slime", "voxel_world"}
	for _, theme := range themes {
		for _, role := range roles {
			asset := readDesktopAssetText(t, "img/cursors/"+theme+"/"+role+".svg")
			if !strings.Contains(asset, `width="32" height="32" viewBox="0 0 40 40"`) {
				t.Errorf("%s/%s cursor is missing the native SVG dimensions", theme, role)
			}
			if strings.Contains(asset, "<image") || strings.Contains(asset, "foreignObject") {
				t.Errorf("%s/%s cursor must remain self-contained native vector artwork", theme, role)
			}
		}
		for _, assetPath := range []string{"img/cursors/" + theme + "/preview.svg", "img/themes/sounds/" + theme + ".svg"} {
			if !strings.Contains(readDesktopAssetText(t, assetPath), "<svg") {
				t.Errorf("%s is missing its preview artwork", assetPath)
			}
		}
	}

	runtime := readDesktopAssetText(t, "js/desktop/core/cursor-runtime.js")
	if !strings.Contains(runtime, `settingValue('appearance.theme_cursors') !== 'false'`) ||
		!strings.Contains(runtime, "default: [3, 2]") || !strings.Contains(runtime, "hotspot.join(' ') + ', ' + role") {
		t.Fatal("native theme cursors must retain the system fallback and cursor opt-out")
	}
}

func TestGreenSlimeEffectHasBoundedSpringSurfaceAndDetachedDrops(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/theme-fx/green_slime.js")
	for _, marker := range []string{
		"MAX_DEFORMATION = 18", "SURFACE_SAMPLES = 32", "function surfaceTargets(rect)",
		"function followSurface(id, rect, dt)", "const tension = 22", "motionDrives",
		"function detachPeriodicDrop(id, surface)", "dropCount", "currentMaxDisplacement",
		"surfaceNodeCount", "clipOut(ctx, blockers)", "pointer-events:none",
		"observeMaskChanges()", "maskRectsDirty", "maskRefreshCount",
		"ready: !!ctx", "motion,", "paused,", "rafPending: !!raf", "windowCount: records.size",
	} {
		if !strings.Contains(source, marker) {
			t.Errorf("Green Slime effect is missing its bounded spring-surface contract marker %q", marker)
		}
	}
	for _, staleColor := range []string{"rgba(83,255,143", "rgba(27,214,110", "rgba(46,255,126"} {
		if strings.Contains(source, staleColor) {
			t.Errorf("Green Slime effect still uses the superseded mint/neon palette %q", staleColor)
		}
	}
}

func TestWindowsXPIconManifestCoversPreferredDesktopIcons(t *testing.T) {
	t.Parallel()

	var manifest struct {
		Name     string            `json:"name"`
		Version  int               `json:"version"`
		Fallback string            `json:"fallback"`
		Icons    map[string]string `json:"icons"`
		Aliases  map[string]string `json:"aliases"`
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "img/windows-xp/manifest.json")), &manifest); err != nil {
		t.Fatalf("parse Windows XP icon manifest: %v", err)
	}
	if manifest.Name != "AuraGo Windows XP-inspired icon pack" || manifest.Version != 1 || !strings.HasPrefix(manifest.Fallback, "img/windows-xp/icons/") {
		t.Fatalf("unexpected icon pack identity: name=%q version=%d fallback=%q", manifest.Name, manifest.Version, manifest.Fallback)
	}
	preferred := desktop.DesktopIconCatalog(nil).Preferred
	if len(manifest.Icons) != len(preferred) {
		t.Fatalf("manifest has %d icon mappings, backend prefers %d", len(manifest.Icons), len(preferred))
	}
	for _, name := range preferred {
		path, ok := manifest.Icons[name]
		if !ok {
			t.Errorf("Windows XP manifest is missing preferred icon %q", name)
			continue
		}
		if (!strings.HasPrefix(path, "img/windows-xp/icons/") && !strings.HasPrefix(path, "img/papirus/icons/")) || !strings.HasSuffix(path, ".svg") {
			t.Errorf("Windows XP icon %q has invalid local SVG path %q", name, path)
			continue
		}
		if !strings.Contains(readDesktopAssetText(t, path), "<svg") {
			t.Errorf("Windows XP icon %q is missing its SVG asset at %s", name, path)
		}
	}
	if !strings.Contains(readDesktopAssetText(t, manifest.Fallback), "<svg") {
		t.Fatalf("fallback artwork %q is missing", manifest.Fallback)
	}
	for _, name := range []string{"folder", "desktop", "trash", "settings", "home", "documents"} {
		if !strings.HasPrefix(manifest.Icons[name], "img/windows-xp/icons/") {
			t.Errorf("system icon %q must use the original Windows XP vector set, got %q", name, manifest.Icons[name])
		}
	}
	for _, name := range []string{"quakejs", "ruview", "n8n", "node-red", "galaxa-deluxe", "video-studio"} {
		if !strings.HasPrefix(manifest.Icons[name], "img/papirus/icons/") {
			t.Errorf("branded app icon %q should retain its existing identity artwork, got %q", name, manifest.Icons[name])
		}
	}
	for alias, target := range manifest.Aliases {
		if _, ok := manifest.Icons[target]; !ok {
			t.Errorf("Windows XP alias %q points to missing preferred icon %q", alias, target)
		}
	}
}
