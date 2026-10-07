package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

var desktopLiveWallpapers = []string{"silk_flow", "firefly_dusk", "neon_overdrive", "fractal_trip"}

func TestDesktopLiveWallpapersAreSelectableEverywhere(t *testing.T) {
	t.Parallel()

	module := readDesktopAssetText(t, "js/desktop/live-wallpapers.js")
	settings := readDesktopAssetText(t, "js/desktop/apps/settings.js")
	menus := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	spaces := readDesktopAssetText(t, "js/desktop/core/spaces-runtime.js")
	css := readDesktopAssetText(t, "css/desktop-wallpaper-live.css")
	polish := readDesktopAssetText(t, "css/desktop-polish.css")
	allowed := map[string]bool{}
	for _, def := range desktop.DesktopSettingDefinitions() {
		if def.Key == "appearance.wallpaper" {
			for _, value := range def.Values {
				allowed[value] = true
			}
		}
	}
	for _, id := range desktopLiveWallpapers {
		option := "['" + id + "', 'desktop.settings_wallpaper_" + id + "']"
		switch {
		case !allowed[id]:
			t.Errorf("server does not accept wallpaper %q", id)
		case !strings.Contains(module, "  "+id+": { fps:"):
			t.Errorf("live-wallpapers.js has no shader for %q", id)
		case !strings.Contains(settings, option):
			t.Errorf("settings app does not offer %q", id)
		case !strings.Contains(menus, option):
			t.Errorf("desktop context menu does not offer %q", id)
		case !strings.Contains(spaces, "'"+id+"'"):
			t.Errorf("spaces runtime does not know %q", id)
		case !strings.Contains(css, `data-wallpaper="`+id+`"`):
			t.Errorf("wallpaper %q has no CSS fallback", id)
		case !strings.Contains(polish, `.desktop-body[data-wallpaper="`+id+`"] { --vd-ambient-top:`):
			t.Errorf("wallpaper %q has no ambient light", id)
		}
	}
	if !strings.Contains(menus, "options.map(item).concat([{ separator: true }], animated.map(item))") {
		t.Error("context menu must set the animated wallpapers apart")
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		data, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", lang, err)
		}
		for _, id := range desktopLiveWallpapers {
			if strings.TrimSpace(values["desktop.settings_wallpaper_"+id]) == "" {
				t.Errorf("%s.json misses the name of %s", lang, id)
			}
		}
	}
}

func TestDesktopLiveWallpapersStayBehindTheShellAndSpareTheGPU(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	host := strings.Index(html, `id="vd-wallpaper-live"`)
	main := strings.Index(html, `id="main-content"`)
	if host < 0 || main < 0 || host > main {
		t.Fatal("desktop.html must place vd-wallpaper-live before the shell")
	}
	if !strings.Contains(html, `<script type="module" src="/js/desktop/live-wallpapers.js?v={{.BuildVersion}}"></script>`) {
		t.Fatal("desktop.html must load live-wallpapers.js as a versioned module")
	}
	css := readDesktopAssetText(t, "css/desktop-wallpaper-live.css")
	for _, want := range []string{"position: fixed;", "z-index: -1;", "pointer-events: none;", ".vd-wallpaper-live-canvas.is-ready", "@media (prefers-reduced-motion: reduce)"} {
		if !strings.Contains(css, want) {
			t.Errorf("desktop-wallpaper-live.css missing %q", want)
		}
	}
	module := readDesktopAssetText(t, "js/desktop/live-wallpapers.js")
	if !strings.Contains(module, `import { desktopCovered } from "./wallpaper-visibility.js";`) {
		t.Error("live wallpapers must share the City Rain occlusion policy")
	}
	module += readDesktopAssetText(t, "js/desktop/wallpaper-visibility.js")
	for _, want := range []string{
		`window.matchMedia("(prefers-reduced-motion: reduce)")`,
		`document.body.dataset.animations === "false"`,
		`"visibilitychange"`,
		`"webglcontextlost"`,
		`"WEBGL_lose_context"`,
		`"vd-space-hidden"`,
		`"vd-screensaver"`,
		"MAX_PIXELS",
		"function adapt(now)",
		"preserveDrawingBuffer: false",
		`powerPreference: "low-power"`,
		`attributeFilter: ["data-wallpaper", "data-animations"]`,
		`"live-wallpaper:still"`,
		`"live-wallpaper:paused"`,
		`"live-wallpaper:webgl-unavailable"`,
	} {
		if !strings.Contains(module, want) {
			t.Errorf("live-wallpapers.js missing %q", want)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "scripts", "build-ui-bundles.js"))
	if err != nil {
		t.Fatal(err)
	}
	live := strings.Index(string(script), "'ui/css/desktop-wallpaper-live.css'")
	polish := strings.Index(string(script), "'ui/css/desktop-polish.css'")
	if live < 0 || polish < live {
		t.Fatal("desktop-wallpaper-live.css must be bundled before the polish layer")
	}
}
