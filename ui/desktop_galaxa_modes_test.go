package ui

import (
	"strings"
	"testing"
)

func TestGalaxaModesModuleRegistered(t *testing.T) {
	t.Parallel()
	loader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
	for _, marker := range []string{
		"/js/desktop/apps/galaxa-modes.js",
		"/js/desktop/apps/galaxa-audio-core.js",
		"/js/desktop/apps/galaxa-audio-sfx.js",
		"/js/desktop/apps/galaxa-audio-music.js",
		"/js/desktop/apps/galaxa-entities-combat.js",
		"/js/desktop/apps/galaxa-render-world.js",
	} {
		if !strings.Contains(loader, marker) {
			t.Fatalf("module loader missing %q", marker)
		}
	}
}

func TestGalaxaSettingsIncludesModeCycle(t *testing.T) {
	t.Parallel()
	deluxe := readEmbeddedText(t, "js/desktop/apps/galaxa-deluxe.js")
	for _, marker := range []string{
		"key: 'mode'",
		"'gauntlet'",
		"'hyperdrive'",
		"'mirror'",
		"GC.createModes(gameCtx)",
	} {
		if !strings.Contains(deluxe, marker) {
			t.Fatalf("galaxa-deluxe missing mode marker %q", marker)
		}
	}
}

func TestGalaxaModeSFXRespectMute(t *testing.T) {
	t.Parallel()
	sfx := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-sfx.js")
	for _, marker := range []string{
		"screenShatter() { if (ctx.G.muted) return;",
		"bulletTimeEnter() { if (ctx.G.muted) return;",
		"modeSelect() { if (ctx.G.muted) return;",
		"gauntletWave() { if (ctx.G.muted) return;",
	} {
		if !strings.Contains(sfx, marker) {
			t.Fatalf("galaxa audio sfx missing mute guard %q", marker)
		}
	}
}

func TestGalaxaModeThemesExist(t *testing.T) {
	t.Parallel()
	music := readEmbeddedText(t, "js/desktop/apps/galaxa-audio-music.js")
	for _, theme := range []string{"gauntlet:", "hyperdrive:", "mirror:"} {
		if !strings.Contains(music, theme) {
			t.Fatalf("galaxa music missing theme %q", theme)
		}
	}
}
