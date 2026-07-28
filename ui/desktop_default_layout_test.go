package ui

import (
	"strings"
	"testing"
)

func TestDesktopFreshInstallWidgetLayoutMatchesDefaultComposition(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	for _, marker := range []string{
		"function defaultWidgetBounds(widget, index)",
		"widgetID === 'builtin-quickchat'",
		"Math.round((workspaceWidth - width) / 2)",
		"widgetID === 'builtin-weather'",
		"widgetID === 'builtin-sysmon'",
		"y: top + 220 + gap",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("fresh-install widget layout missing marker %q", marker)
		}
	}

	autosize := readDesktopAssetText(t, "js/desktop/core/widget-autosize-runtime.js")
	for _, marker := range []string{
		"function alignDefaultBuiltinWidgetStack()",
		"weather.offsetTop + weather.offsetHeight + 8",
	} {
		if !strings.Contains(autosize, marker) {
			t.Fatalf("fresh-install widget stack alignment missing marker %q", marker)
		}
	}

	bundle := readDesktopAssetText(t, "js/desktop/bundles/main.bundle.js")
	for _, marker := range []string{
		"function defaultWidgetBounds(widget, index)",
		"widgetID === 'builtin-quickchat'",
		"widgetID === 'builtin-weather'",
		"widgetID === 'builtin-sysmon'",
		"function alignDefaultBuiltinWidgetStack()",
	} {
		if !strings.Contains(bundle, marker) {
			t.Fatalf("desktop main bundle missing fresh-install layout marker %q", marker)
		}
	}
}

func TestDesktopPetFreshInstallUsesSmallestPickerScale(t *testing.T) {
	t.Parallel()

	foundation := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	runtime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	picker := readDesktopAssetText(t, "js/desktop/apps/pet-picker.js")
	for path, source := range map[string]string{
		"desktop foundation": foundation,
		"pet runtime":        runtime,
		"pet picker":         picker,
	} {
		if !strings.Contains(source, "'pet.scale'") && !strings.Contains(source, "settings['pet.scale']") {
			t.Fatalf("%s does not expose the pet scale setting", path)
		}
		if !strings.Contains(source, "'0.5'") {
			t.Fatalf("%s must use the picker's smallest 0.5x scale as its fallback", path)
		}
	}
	for _, marker := range []string{
		"Math.round(window.innerWidth * 0.57 - width / 2)",
		"Math.round(window.innerHeight - height - 64)",
	} {
		if !strings.Contains(runtime, marker) {
			t.Fatalf("pet runtime missing responsive default placement marker %q", marker)
		}
	}
}
