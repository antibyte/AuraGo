package ui

import (
	"os"
	"path/filepath"
	"regexp"
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
		"/js/desktop/apps/galaxa-entities-weapons.js",
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

func TestGalaxaSplitModulesUseCtxExports(t *testing.T) {
	t.Parallel()

	// Helpers moved onto ctx during the Galaxa file split must not be called bare
	// from sibling modules (ReferenceError at runtime).
	ctxOnly := []string{
		"getParticle",
		"recycleParticles",
		"renderFlame",
	}

	appsDir := filepath.Join("js", "desktop", "apps")
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		t.Fatal(err)
	}

	funcDefRe := regexp.MustCompile(`\bfunction\s+([A-Za-z_$][\w$]*)\s*\(`)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "galaxa") || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(appsDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		locals := map[string]bool{}
		for _, m := range funcDefRe.FindAllStringSubmatch(text, -1) {
			locals[m[1]] = true
		}

		lines := strings.Split(text, "\n")
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "//") || strings.HasPrefix(trim, "*") {
				continue
			}
			for _, sym := range ctxOnly {
				if locals[sym] {
					continue
				}
				if !strings.Contains(line, sym+"(") {
					continue
				}
				if strings.Contains(line, "ctx."+sym+"(") || strings.Contains(line, "function "+sym+"(") {
					continue
				}
				t.Errorf("%s:%d bare %s() call; use ctx.%s() after module split", entry.Name(), i+1, sym, sym)
			}
		}
	}
}
