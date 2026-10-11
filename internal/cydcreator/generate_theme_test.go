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
	want := map[string]struct {
		style string
		dark  string
		small string
	}{
		"clear":         {"0", "false", "ui_font_montserrat_14"},
		"night":         {"1", "true", "ui_font_montserrat_14"},
		"terminal":      {"2", "true", "ui_font_jetbrains_mono_14"},
		"sandstorm":     {"0", "false", "ui_font_montserrat_14"},
		"dark_sun":      {"1", "true", "ui_font_montserrat_14"},
		"synthwave":     {"1", "true", "ui_font_montserrat_14"},
		"high_contrast": {"0", "true", "ui_font_montserrat_16"},
	}
	themes, err := Themes()
	if err != nil {
		t.Fatal(err)
	}
	if len(themes) != len(want) {
		t.Fatalf("%d themes, want %d", len(themes), len(want))
	}
	for _, th := range themes {
		w := want[th.ID]
		got := renderTheme(th)
		for _, line := range []string{
			"// Theme \"" + th.ID + "\".\n",
			"static const int ui__button_style = " + w.style + ";",
			"static const bool ui__dark = " + w.dark + ";",
			"ui__colors.font_small = &" + w.small + ";",
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

func TestColorLuma(t *testing.T) {
	for color, want := range map[string]int{"#000000": 0, "#FFFFFF": 255, "#0F1522": 20, "#F4F6F9": 245} {
		if got := colorLuma(color); got != want {
			t.Errorf("colorLuma(%s) = %d, want %d", color, got, want)
		}
	}
}
