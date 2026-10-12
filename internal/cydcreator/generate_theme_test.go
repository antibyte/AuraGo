package cydcreator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderThemeMatchesReference(t *testing.T) {
	th, err := ThemeByID("night")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(referenceDir, "src", "generated", "ui_theme.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if got := renderTheme(th); got != string(want) {
		t.Fatalf("ui_theme.cpp:\n%s", firstDiff(string(want), got))
	}
}

func TestRenderThemeSubstitutions(t *testing.T) {
	type row struct {
		style, dark                string
		small, normal, large, mono string
	}
	want := map[string]row{
		"clear":         {"0", "false", "ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_jetbrains_mono_16"},
		"night":         {"1", "true", "ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_jetbrains_mono_16"},
		"terminal":      {"2", "true", "ui_font_jetbrains_mono_14", "ui_font_jetbrains_mono_16", "ui_font_jetbrains_mono_20", "ui_font_jetbrains_mono_16"},
		"sandstorm":     {"0", "false", "ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_jetbrains_mono_16"},
		"dark_sun":      {"1", "true", "ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_jetbrains_mono_16"},
		"synthwave":     {"1", "true", "ui_font_montserrat_14", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_jetbrains_mono_16"},
		"high_contrast": {"0", "true", "ui_font_montserrat_16", "ui_font_montserrat_20", "ui_font_montserrat_24", "ui_font_jetbrains_mono_20"},
	}
	themes, err := Themes()
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != len(want) {
		t.Fatalf("%d themes, want %d", len(themes), len(want))
	}
	for _, th := range themes {
		w, ok := want[th.ID]
		if !ok {
			t.Fatalf("theme %q has no expectation row; add one", th.ID)
		}
		got := renderTheme(th)
		for _, line := range []string{
			"// Theme \"" + th.ID + "\".\n",
			"static const int ui__button_style = " + w.style + ";",
			"static const bool ui__dark = " + w.dark + ";",
			"ui__colors.font_small = &" + w.small + ";",
			"ui__colors.font_normal = &" + w.normal + ";",
			"ui__colors.font_large = &" + w.large + ";",
			"ui__colors.font_mono = &" + w.mono + ";",
			"ui__colors.accent = " + cHexColor(th.Tokens.Accent) + ";",
		} {
			if !strings.Contains(got, line) {
				t.Errorf("%s: ui_theme.cpp lacks %q", th.ID, line)
			}
		}
		if strings.Contains(got, "{{") {
			t.Errorf("%s: unreplaced placeholder", th.ID)
		}
	}
}

func TestRenderThemeInvalidTokens(t *testing.T) {
	// A zero Theme must still render compilable text: no empty value after
	// "=", black colours (the cHexColor fallback), button style filled.
	got := renderTheme(Theme{})
	if strings.Contains(got, "= ;") {
		t.Errorf("renderTheme(Theme{}) emits an empty value:\n%s", got[:strings.Index(got, "static lv_style_t")])
	}
	for _, line := range []string{
		"static const int ui__button_style = 0;",
		"static const bool ui__dark = true;",
		"ui__colors.bg = lv_color_hex(0x000000);",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("renderTheme(Theme{}) lacks %q", line)
		}
	}
	if strings.Contains(got, "{{") {
		t.Error("renderTheme(Theme{}): unreplaced placeholder")
	}

	// The theme id only reaches a comment: it must not break out of the line.
	got = renderTheme(Theme{ID: "a\nint evil;\r\\"})
	if !strings.Contains(got, "// Theme \"a int evil;\".\n") {
		t.Errorf("theme id was not sanitised for the comment:\n%s", got[:strings.Index(got, "#include")])
	}

	// The dark flag follows the luma threshold (127 dark, 128 light) and uses
	// the same colour validation as the emitted colour constant.
	for bg, dark := range map[string]string{
		"#7F7F7F": "true", "#808080": "false", "#FFFFFF": "false",
		"FFFFFF": "true", "#FFFFFFFF": "true", "#FFF": "true", "": "true",
	} {
		th := Theme{ID: "x", ButtonStyle: "filled"}
		th.Tokens.Bg = bg
		if got := renderTheme(th); !strings.Contains(got, "static const bool ui__dark = "+dark+";") {
			t.Errorf("bg %q: want ui__dark = %s", bg, dark)
		}
	}
}

func TestColorLuma(t *testing.T) {
	for color, want := range map[string]int{
		"#000000": 0, "#FFFFFF": 255, "#0F1522": 20, "#F4F6F9": 245,
		"#7F7F7F": 127, "#808080": 128, "#ffffff": 255,
		// Not exactly #RRGGBB: black, like cHexColor.
		"": 0, "FFFFFF": 0, "#FFF": 0, "#FFFFFFFF": 0, "#GGGGGG": 0, "0xFFFFFF": 0, "#+FFFFF": 0, " #FFFFFF": 0,
	} {
		if got := colorLuma(color); got != want {
			t.Errorf("colorLuma(%q) = %d, want %d", color, got, want)
		}
	}
}
