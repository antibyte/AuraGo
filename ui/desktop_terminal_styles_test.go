package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopTerminalStyleCatalog(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/terminal-styles.js")
	for _, want := range []string{
		"window.TerminalStyles = {",
		"ids:",
		"normalize:",
		"load:",
		"save:",
		"profile:",
		"applyXterm:",
		"'modern'",
		"'amber'",
		"'green'",
		"'apple2'",
		"'commodore64'",
		"'ibm3278'",
		"'vintage'",
		"'mono-green'",
		"'transparent-green'",
		"aurago.desktop.terminal.style",
		"return 'modern'",
		"phosphor:",
		"term.options.theme = profile.theme",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("terminal-styles.js missing %q", want)
		}
	}
	if strings.Contains(source, "crt-shader") || strings.Contains(source, "cool-retro-term") {
		t.Fatal("terminal-styles.js must not reference Chat CRT shaders or cool-retro-term sources")
	}
}

func TestDesktopTerminalStyleI18n(t *testing.T) {
	t.Parallel()

	keys := []string{
		"desktop.terminal_audio",
		"desktop.terminal_audio_off",
		"desktop.terminal_audio_on",
		"desktop.terminal_style",
		"desktop.terminal_style_amber",
		"desktop.terminal_style_apple2",
		"desktop.terminal_style_commodore64",
		"desktop.terminal_style_green",
		"desktop.terminal_style_ibm3278",
		"desktop.terminal_style_modern",
		"desktop.terminal_style_mono_green",
		"desktop.terminal_style_transparent_green",
		"desktop.terminal_style_vintage",
	}
	english := map[string]string{
		"desktop.terminal_audio":                   "Key click",
		"desktop.terminal_audio_off":               "Key click off",
		"desktop.terminal_audio_on":                "Key click on",
		"desktop.terminal_style":                     "Style",
		"desktop.terminal_style_amber":             "Amber",
		"desktop.terminal_style_apple2":            "Apple II",
		"desktop.terminal_style_commodore64":       "Commodore 64",
		"desktop.terminal_style_green":             "Green phosphor",
		"desktop.terminal_style_ibm3278":           "IBM 3278",
		"desktop.terminal_style_modern":            "Modern",
		"desktop.terminal_style_mono_green":        "Monochrome green",
		"desktop.terminal_style_transparent_green": "Transparent green",
		"desktop.terminal_style_vintage":           "Vintage",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := "lang/desktop/" + lang + ".json"
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range keys {
			got := strings.TrimSpace(values[key])
			if got == "" {
				t.Fatalf("%s missing non-empty %s", path, key)
			}
			if lang != "en" && got == english[key] && (key == "desktop.terminal_audio" || key == "desktop.terminal_style") {
				t.Fatalf("%s must not copy the English %s string", path, key)
			}
		}
		if lang == "de" {
			if strings.Contains(values["desktop.terminal_audio_on"], "ae") || strings.Contains(values["desktop.terminal_style_green"], "ue") {
				t.Fatal("German terminal style strings must use real umlauts")
			}
			if values["desktop.terminal_style"] == "Style" {
				t.Fatal("German desktop.terminal_style must be translated")
			}
		}
	}
}
