package desktop

import (
	"regexp"
	"slices"
	"testing"
)

func TestDesktopThemeIDsAreValid(t *testing.T) {
	t.Parallel()

	idPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	ids := DesktopThemeIDs()
	if len(ids) == 0 {
		t.Fatal("DesktopThemeIDs returned no themes")
	}
	if ids[0] != DefaultDesktopTheme {
		t.Fatalf("first theme = %q, want the default %q", ids[0], DefaultDesktopTheme)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !idPattern.MatchString(id) {
			t.Errorf("theme id %q does not match %s", id, idPattern)
		}
		// The shared CSS already keys [data-theme="light"] and [data-theme="dark"] on the app-wide colour mode.
		if id == "light" || id == "dark" {
			t.Errorf("theme id %q is reserved by the shared CSS", id)
		}
		if seen[id] {
			t.Errorf("duplicate theme id %q", id)
		}
		seen[id] = true
	}

	// Every call hands out its own slice, so a caller cannot rewrite the list for everyone else.
	first, second := DesktopThemeIDs(), DesktopThemeIDs()
	if !slices.Equal(first, second) {
		t.Fatalf("DesktopThemeIDs differs between calls: %v vs %v", first, second)
	}
	if &first[0] == &second[0] {
		t.Fatal("DesktopThemeIDs returned slices that share one backing array")
	}
	first[0] = "mutated"
	if second[0] != DefaultDesktopTheme || DesktopThemeIDs()[0] != DefaultDesktopTheme {
		t.Fatal("mutating a returned slice changed later results")
	}
}

func TestDesktopThemeSettingAcceptsEveryTheme(t *testing.T) {
	t.Parallel()

	for _, id := range DesktopThemeIDs() {
		if err := validateDesktopSetting("appearance.theme", id); err != nil {
			t.Errorf("appearance.theme %q rejected: %v", id, err)
		}
	}
	if err := validateDesktopSetting("appearance.theme", "citrus"); err == nil {
		t.Error("appearance.theme accepted the unknown theme \"citrus\"")
	}
	if got := DesktopSettingDefaults()["appearance.theme"]; got != DefaultDesktopTheme {
		t.Errorf("default appearance.theme = %q, want %q", got, DefaultDesktopTheme)
	}
}
