package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWidgetDisplayTitle(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, want := range []string{
		"function widgetDisplayTitle(widget)",
		"t('desktop.weather_title')",
		"t('desktop.widget_analog_clock')",
		"t('desktop.widget_quickchat')",
		"t('desktop.widget_sysmon_title')",
		"card.title = widgetDisplayTitle(widget)",
	} {
		if !strings.Contains(foundation, want) {
			t.Fatalf("widget display title missing marker %q", want)
		}
	}
	sigBody := jsFunctionBodyInWindowMenuTest(t, foundation, "function widgetContentSignature(widget)")
	if strings.Contains(sigBody, "widgetDisplayTitle") {
		t.Fatal("widgetContentSignature must keep the stored widget.title, not the translated display title")
	}

	drawer := readDesktopAssetText(t, "js/desktop/core/widget-drawer-runtime.js")
	if !strings.Contains(drawer, "widgetDisplayTitle(widget)") {
		t.Fatal("widget drawer must show widgetDisplayTitle")
	}

	menus := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	for _, want := range []string{
		"${esc(widgetDisplayTitle(widget))}",
		"widget ? widgetDisplayTitle(widget) : btn.dataset.id",
	} {
		if !strings.Contains(menus, want) {
			t.Fatalf("widget manager missing display title marker %q", want)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		if !strings.Contains(text, `"desktop.weather_title"`) {
			t.Fatalf("%s desktop translations missing %q", lang, "desktop.weather_title")
		}
	}
}
