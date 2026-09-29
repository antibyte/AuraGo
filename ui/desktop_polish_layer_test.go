package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDesktopPolishLayerLoadsLast(t *testing.T) {
	t.Parallel()

	scriptBytes, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatalf("read build script: %v", err)
	}
	script := string(scriptBytes)
	chrome := strings.Index(script, "'ui/css/desktop-chrome.css'")
	polish := strings.Index(script, "'ui/css/desktop-polish.css'")
	if chrome < 0 || polish < chrome {
		t.Fatalf("desktop-polish.css must be bundled after desktop-chrome.css")
	}
	rest := script[polish+len("'ui/css/desktop-polish.css'"):]
	if end, next := strings.Index(rest, "]"), strings.Index(rest, "'ui/css/"); end < 0 || (next >= 0 && next < end) {
		t.Fatalf("desktop-polish.css must be the last part of the desktop shell CSS bundle")
	}
	if !strings.Contains(script, "'ui/js/desktop/core/polish-runtime.js'") {
		t.Fatalf("build-ui-bundles.js must include polish-runtime.js")
	}
}

func TestDesktopPolishMotionIsGated(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-polish.css")
	for _, want := range []string{"@media (prefers-reduced-motion: reduce)", "@media (prefers-reduced-transparency: reduce)"} {
		if !strings.Contains(css, want) {
			t.Fatalf("desktop-polish.css must handle %s", want)
		}
	}
	rules := regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1)
	for _, rule := range rules {
		selector, body := strings.TrimSpace(rule[1]), rule[2]
		if !strings.Contains(body, "animation:") || strings.Contains(body, "animation: none") {
			continue
		}
		if !strings.Contains(selector, `:not([data-animations="false"])`) {
			t.Errorf("animation in %q must be gated by data-animations", selector)
		}
	}
	js := readDesktopAssetText(t, "js/desktop/core/polish-runtime.js")
	if !strings.Contains(js, "!animationsEnabled()") {
		t.Fatalf("polish-runtime.js must not ring the bell with animations off")
	}
}

func TestDesktopPolishCoversEveryWallpaper(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-polish.css")
	base := readDesktopAssetText(t, "css/desktop-base.css")
	seen := map[string]bool{"aurora": true} // the default wallpaper uses the .desktop-body defaults
	for _, match := range regexp.MustCompile(`\.desktop-body\[data-wallpaper="([a-z_]+)"\]`).FindAllStringSubmatch(base, -1) {
		id := match[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		if !strings.Contains(css, `.desktop-body[data-wallpaper="`+id+`"] { --vd-ambient-top:`) {
			t.Errorf("wallpaper %q has no ambient light in desktop-polish.css; photos: run python scripts/wallpaper-ambient.py", id)
		}
	}
}

func TestDesktopPolishPointerLightTargetsMatch(t *testing.T) {
	t.Parallel()

	js := readDesktopAssetText(t, "js/desktop/core/polish-runtime.js")
	css := readDesktopAssetText(t, "css/desktop-polish.css")
	match := regexp.MustCompile(`POINTER_LIGHT_TARGETS = '([^']+)'`).FindStringSubmatch(js)
	if match == nil {
		t.Fatalf("polish-runtime.js must declare POINTER_LIGHT_TARGETS")
	}
	list := regexp.MustCompile(`:is\(([^)]+)\):hover`).FindAllStringSubmatch(css, -1)
	if len(list) == 0 {
		t.Fatalf("desktop-polish.css must style the pointer light targets")
	}
	for _, rule := range list {
		if rule[1] != match[1] {
			t.Errorf("pointer light selector %q differs from POINTER_LIGHT_TARGETS %q", rule[1], match[1])
		}
	}
	if !strings.Contains(js, "addEventListener('pointermove', trackPointerLight, { passive: true })") {
		t.Fatalf("pointer light must listen passively")
	}
}
