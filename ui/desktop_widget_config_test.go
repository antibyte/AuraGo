package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopWidgetConfigPersistence(t *testing.T) {
	t.Parallel()

	shell := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, want := range []string{
		"function widgetConfig(widget)",
		"function persistWidgetRecord(",
		"function persistWidgetConfig(",
		"function toggleWidgetAutoSize(",
		"renderWeatherWidget(container, widget)",
		"async function persistWeatherLocation(loc, imported)",
		"await persistWidgetConfig(record, { location: loc }, { skipReload: true })",
		"record.config.location",
		"localStorage.getItem(STORAGE_KEY)",
		"localStorage.removeItem(STORAGE_KEY)",
		"if (!imported)",
		"localStorage.setItem(STORAGE_KEY, JSON.stringify(loc))",
	} {
		if !strings.Contains(shell, want) {
			t.Fatalf("desktop widget config persistence missing marker %q", want)
		}
	}

	weatherBody := jsFunctionBodyInWindowMenuTest(t, shell, "async function renderWeatherWidget(container, widget)")
	if strings.Contains(weatherBody, "localStorage.setItem(STORAGE_KEY, JSON.stringify(location))") {
		t.Fatal("weather widget must not write vd-weather-location as the primary store")
	}
	if !strings.Contains(weatherBody, "fromConfig || imported || DEFAULT_WEATHER_LOCATION") {
		t.Fatal("weather widget must load config.location, then import localStorage, then default")
	}

	menus := readDesktopAssetText(t, "js/desktop/core/menus-and-routing.js")
	menuBody := jsFunctionBodyInWindowMenuTest(t, menus, "function showWidgetContextMenu(event, widget)")
	for _, want := range []string{
		"desktop.widget_auto_size",
		"icon: autoSize ? 'check-square' : 'square'",
		"disabled: desktopReadonly()",
		"action: () => toggleWidgetAutoSize(widget)",
	} {
		if !strings.Contains(menuBody, want) {
			t.Fatalf("widget context menu auto-size toggle missing marker %q", want)
		}
	}

	runtime := readDesktopAssetText(t, "js/desktop/core/widget-autosize-runtime.js")
	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	for _, source := range []string{runtime, foundation} {
		if !strings.Contains(source, "widget.config.auto_size") {
			t.Fatal("widgetShouldAutoSize must read widget.config.auto_size")
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		text := rawDesktopAssetText(t, filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json")))
		if !strings.Contains(text, `"desktop.widget_auto_size"`) {
			t.Fatalf("%s desktop translations missing %q", lang, "desktop.widget_auto_size")
		}
	}
}
