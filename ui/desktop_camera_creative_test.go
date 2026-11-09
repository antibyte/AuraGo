package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var cameraCreativeKeys = []string{
	"desktop.camera_tools",
	"desktop.camera_photo_only",
	"desktop.camera_look",
	"desktop.camera_look_natural",
	"desktop.camera_look_mono",
	"desktop.camera_look_warm",
	"desktop.camera_look_cool",
	"desktop.camera_look_vintage",
	"desktop.camera_look_unavailable",
	"desktop.camera_zoom",
	"desktop.camera_zoom_in",
	"desktop.camera_zoom_out",
	"desktop.camera_ratio",
	"desktop.camera_ratio_auto",
	"desktop.camera_pause",
	"desktop.camera_resume",
	"desktop.camera_paused",
	"desktop.camera_pause_unsupported",
	"desktop.camera_retry",
	"desktop.camera_hint_tools",
}

var cameraCreativeLocales = []string{
	"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh",
}

func TestDesktopCameraCreativeContracts(t *testing.T) {
	t.Parallel()

	camera := readDesktopAssetText(t, "js/desktop/apps/camera.js")
	for _, marker := range []string{
		"window.CameraCreative",
		"creative.sourceRect()",
		"creative.lookFilter()",
		"creative.setContext",
		"creative.menuItems()",
		"creative.hintPairs",
		"creative.dispose()",
		"function togglePause()",
		"state.pauseSupported",
		"data-action=\"retry\"",
		"aria-pressed",
		"is-paused",
		"desktop.camera_paused",
	} {
		if !strings.Contains(camera, marker) {
			t.Fatalf("camera.js missing camera creative marker %q", marker)
		}
	}

	creative := readDesktopAssetText(t, "js/desktop/apps/camera-creative.js")
	for _, marker := range []string{
		"function sourceRect",
		"canvasFilterSupported",
		"ResizeObserver",
		"window.CameraCreative",
		"desktop.camera_look_unavailable",
		"desktop.camera_pause_unsupported",
		"dispose:",
	} {
		if !strings.Contains(creative, marker) {
			t.Fatalf("camera-creative.js missing marker %q", marker)
		}
	}

	loader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	first := strings.Index(loader, "'/js/desktop/apps/camera-creative.js'")
	second := strings.Index(loader, "'/js/desktop/apps/camera.js'")
	if first < 0 || second < 0 || first > second {
		t.Fatal("module-loader must load camera-creative.js before camera.js")
	}
}

func TestDesktopCameraCreativeLocales(t *testing.T) {
	t.Parallel()

	english := readCameraCreativeLocale(t, "en")
	distinctive := []string{
		"desktop.camera_tools",
		"desktop.camera_photo_only",
		"desktop.camera_look_unavailable",
		"desktop.camera_pause_unsupported",
		"desktop.camera_retry",
	}
	for _, locale := range cameraCreativeLocales {
		values := readCameraCreativeLocale(t, locale)
		for _, key := range cameraCreativeKeys {
			value, ok := values[key]
			if !ok || strings.TrimSpace(value) == "" {
				t.Fatalf("%s locale is missing a translation for %q", locale, key)
			}
		}
		if locale == "en" {
			continue
		}
		for _, key := range distinctive {
			if values[key] == english[key] {
				t.Fatalf("%s locale reuses the English text for %q", locale, key)
			}
		}
	}

	german := readCameraCreativeLocale(t, "de")
	if !strings.Contains(german["desktop.camera_look_mono"], "ß") ||
		!strings.Contains(german["desktop.camera_look_cool"], "ü") ||
		!strings.Contains(german["desktop.camera_look_unavailable"], "unterstützt") {
		t.Fatal("German camera creative strings must use real umlauts and eszett")
	}
}

func TestDesktopCameraCreativeStyles(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/camera.css")
	for _, marker := range []string{
		".camera-cc-panel",
		".camera-cc-mask",
		".camera-cc-look",
		".camera-cc-pause-glyph",
		".camera-error-retry",
		".camera-app.is-paused .camera-rec-dot",
		"flex-wrap: wrap;",
		".camera-app [hidden]",
		"display: none !important;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("camera.css missing marker %q", marker)
		}
	}
	reduced := css[strings.LastIndex(css, "@media (prefers-reduced-motion: reduce)"):]
	if !strings.Contains(reduced, ".camera-cc-panel") {
		t.Fatal("reduced-motion rules must cover the creative panel")
	}
}

func readCameraCreativeLocale(t *testing.T, locale string) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("lang", "desktop", locale+".json"))
	if err != nil {
		t.Fatalf("read %s locale: %v", locale, err)
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("parse %s locale: %v", locale, err)
	}
	return values
}
