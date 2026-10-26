package ui

import (
	"regexp"
	"strings"
	"testing"
)

// The WP4 tools live in three modules that the loader already lists. These tests guard the wiring (which tool is registered
// where, shortcuts, file size, CSS prefix); the behaviour is covered by scripts/test-pixel-tools.mjs.

func pixelToolsSources(t *testing.T) (brush, pro, transform string) {
	t.Helper()
	return normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-brush.js")),
		normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-tools-pro.js")),
		normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-transform.js"))
}

func TestPixelToolsModulesRegisterAndStaySmall(t *testing.T) {
	brush, pro, transform := pixelToolsSources(t)
	for name, source := range map[string]string{"brush": brush, "tools-pro": pro, "transform": transform} {
		if !strings.Contains(source, "Pixel.registerModule('"+name+"'") {
			t.Fatalf("pixel-%s.js should register itself as module %q", name, name)
		}
		// TestVirtualDesktopFirstPartyJSFilesStayBelowLineBudget allows fewer than 1100 lines per desktop script.
		if lines := strings.Count(source, "\n"); lines >= 1100 {
			t.Fatalf("pixel-%s.js has %d lines, want < 1100", name, lines)
		}
	}
}

func TestPixelToolsRegisterEveryWP4Tool(t *testing.T) {
	brush, pro, transform := pixelToolsSources(t)
	for _, id := range []string{"brush", "pencil", "airbrush", "eraser"} {
		if !strings.Contains(brush, "Brush.defineStrokeTool({ id: '"+id+"'") {
			t.Fatalf("pixel-brush.js should define the stroke tool %q", id)
		}
	}
	for _, id := range []string{"smudge", "blur-brush", "dodge-burn", "color-replace", "clone-stamp", "healing", "spot-heal", "red-eye", "hand", "zoom", "gradient", "fill"} {
		if !strings.Contains(pro, "id: '"+id+"'") && !strings.Contains(pro, "['clone-stamp', 'healing']") {
			t.Fatalf("pixel-tools-pro.js should register the tool %q", id)
		}
	}
	for _, id := range []string{"rounded-rect", "polygon", "star", "rectangle", "ellipse", "line", "arrow"} {
		if !strings.Contains(pro, "'"+id+"': [") && !strings.Contains(pro, "        "+id+": [") {
			t.Fatalf("pixel-tools-pro.js should define the shape tool %q", id)
		}
	}
	for _, id := range []string{"free-transform", "crop", "move"} {
		if !strings.Contains(transform, "id: '"+id+"'") {
			t.Fatalf("pixel-transform.js should define the tool %q", id)
		}
	}
	// Engine features the spec asks for.
	for _, want := range []string{"createPath", "symmetryPoints", "openSession", "'destination-out'", "'source-atop'", "createRadialGradient", "getCoalescedEvents", "pressureFactors", "BracketLeft", "pristine"} {
		if !strings.Contains(brush, want) {
			t.Fatalf("pixel-brush.js is missing %q", want)
		}
	}
	if strings.Contains(brush, "shadowBlur") {
		t.Fatal("the brush engine draws real radial-falloff stamps, never shadowBlur")
	}
	// Optional dependency on the inpaint op is by feature detection.
	if !strings.Contains(pro, "ops['inpaint.fill']") || !strings.Contains(pro, "diffusionFill") {
		t.Fatal("spot heal should use inpaint.fill when present and fall back to a diffusion fill")
	}
	for _, want := range []string{"snapBox", "scaleDrag", "cropDrag", "'#ff2bd6'", "Ctrl+Alt+T", "startCrop", "applyCrop", "cancelCrop", "transformDocument"} {
		if !strings.Contains(transform, want) {
			t.Fatalf("pixel-transform.js is missing %q", want)
		}
	}
}

func TestPixelToolShortcutsAreUnique(t *testing.T) {
	_, pro, transform := pixelToolsSources(t)
	state := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-state.js"))
	builtin := map[string]string{}
	for _, m := range regexp.MustCompile(`'([a-z-]+)': '([A-Za-z+]+)'`).FindAllStringSubmatch(state[strings.Index(state, "const TOOL_SHORTCUTS"):], -1) {
		builtin[strings.ToLower(m[2])] = m[1]
	}
	// Free transform: its Edit menu item owns Ctrl+Alt+T (and Ctrl+T through shortcuts); the tool and the command carry no key.
	seen := map[string]bool{}
	for _, source := range []string{pro, transform} {
		for _, m := range regexp.MustCompile(`shortcut: '([A-Za-z+]+)'`).FindAllStringSubmatch(source, -1) {
			key := strings.ToLower(m[1])
			if key == "ctrl+t" {
				continue
			}
			if seen[key] {
				t.Fatalf("shortcut %s is registered twice by the WP4 tools", m[1])
			}
			if other, ok := builtin[key]; ok {
				t.Fatalf("shortcut %s collides with the built-in tool %s", m[1], other)
			}
			seen[key] = true
		}
	}
	for _, key := range []string{"c", "h", "z", "y", "j", "shift+j", "shift+b", "shift+e", "shift+r", "shift+o", "shift+s"} {
		if !seen[key] {
			t.Fatalf("no WP4 tool registers the shortcut %q", key)
		}
	}
}

func TestPixelToolsCSSIsPrefixedAndSelfContained(t *testing.T) {
	css := normalizePixelAsset(readDesktopAssetText(t, "css/pixel-tools-pro.css"))
	selector := regexp.MustCompile(`(?m)^([^{}/@][^{}]*)\{`)
	class := regexp.MustCompile(`\.([A-Za-z_][\w-]*)`)
	for _, m := range selector.FindAllStringSubmatch(css, -1) {
		for _, c := range class.FindAllStringSubmatch(m[1], -1) {
			if !strings.HasPrefix(c[1], "pixel-") {
				t.Fatalf("pixel-tools-pro.css class %q is not prefixed pixel- (selector %q)", c[1], strings.TrimSpace(m[1]))
			}
		}
	}
	for _, want := range []string{".pixel-tool-rail", ".pixel-opt-wrap", ".pixel-brush-panel", ".pixel-brush-presets"} {
		if !strings.Contains(css, want) {
			t.Fatalf("pixel-tools-pro.css should style %s", want)
		}
	}
	if strings.Contains(css, "cc-") {
		t.Fatal("never use the cc- class prefix (cookie blockers hide it)")
	}
}

func TestPixelToolsKeepLegacyCropEntryPoints(t *testing.T) {
	_, _, transform := pixelToolsSources(t)
	view := normalizePixelAsset(readDesktopAssetText(t, "js/desktop/apps/pixel-view.js"))
	// The Transform panel and the Image menu start the crop tool through startCrop(); Apply / Cancel live in the tool's
	// options bar, so the panel carries no (dead) buttons of its own. The crop tool answers all three runtime methods.
	if !strings.Contains(view, "this.startCrop()") {
		t.Fatal("pixel-view.js should keep calling this.startCrop()")
	}
	if strings.Contains(view, "data-crop-actions") || strings.Contains(view, "apply-crop") {
		t.Fatal("the Transform panel must not show Apply / Cancel crop buttons that the crop tool never enables")
	}
	for _, want := range []string{"startCrop: function startCrop()", "applyCrop: function applyCropRuntime()", "cancelCrop: function cancelCropRuntime()"} {
		if !strings.Contains(transform, want) {
			t.Fatalf("pixel-transform.js should override %s", want)
		}
	}
}
