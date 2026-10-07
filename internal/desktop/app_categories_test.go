package desktop

import "testing"

// Every builtin app belongs to one start-menu category, every category except the Store group
// has at least one builtin app, and category icons come from the themed icon catalog.
func TestBuiltinAppsCarryStartMenuCategories(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, category := range DesktopAppCategories() {
		if known[category.ID] {
			t.Fatalf("duplicate start-menu category %q", category.ID)
		}
		known[category.ID] = true
		iconKnown := false
		for _, name := range desktopPreferredIconNames {
			if name == category.Icon {
				iconKnown = true
				break
			}
		}
		if !iconKnown {
			t.Errorf("category %q icon %q is not a preferred desktop icon", category.ID, category.Icon)
		}
	}
	if !known["installed"] {
		t.Fatal("start-menu categories must end with the installed group")
	}

	seen := map[string]int{}
	for _, app := range BuiltinApps() {
		switch {
		case app.Category == "":
			t.Errorf("builtin app %q has no start-menu category", app.ID)
		case app.Category == "installed":
			t.Errorf("builtin app %q must not use the installed group", app.ID)
		case !known[app.Category]:
			t.Errorf("builtin app %q uses unknown category %q", app.ID, app.Category)
		}
		seen[app.Category]++
	}
	for _, category := range DesktopAppCategories() {
		if category.ID != "installed" && seen[category.ID] == 0 {
			t.Errorf("start-menu category %q has no builtin app", category.ID)
		}
	}
}
