package desktopstore

import (
	"context"
	"testing"

	"aurago/internal/desktop"
)

// Every Store app lands in a real start-menu category, never in the "installed" catch-all, and an
// entry that fronts a builtin app agrees with that app's category.
func TestStoreCatalogEntriesCarryStartMenuCategories(t *testing.T) {
	known := map[string]bool{}
	for _, category := range desktop.DesktopAppCategories() {
		known[category.ID] = true
	}
	builtinCategories := map[string]string{}
	for _, app := range desktop.BuiltinApps() {
		builtinCategories[app.ID] = app.Category
	}
	for _, entry := range DefaultCatalog() {
		t.Run(entry.ID, func(t *testing.T) {
			switch {
			case entry.Category == "":
				t.Fatalf("catalog entry %q has no start-menu category", entry.ID)
			case entry.Category == "installed":
				t.Fatalf("catalog entry %q must use a real category, not the installed group", entry.ID)
			case !known[entry.Category]:
				t.Fatalf("catalog entry %q uses unknown category %q", entry.ID, entry.Category)
			}
			if builtin, ok := builtinCategories[entry.DesktopAppID]; ok && builtin != entry.Category {
				t.Fatalf("catalog entry %q fronts builtin %q with category %q, but declares %q", entry.ID, entry.DesktopAppID, builtin, entry.Category)
			}
		})
	}
}

func TestInstallOperationCopiesCategoryToDesktopManifest(t *testing.T) {
	ctx := context.Background()
	desktopAdapter := &fakeDesktopAdapter{}
	svc := newTestService(t, &fakeDockerAdapter{}, desktopAdapter, &fakeLaunchpadAdapter{}, fixedPorts(18090))

	op, err := svc.StartInstall(ctx, InstallRequest{AppID: "quakejs-rootless", BindMode: BindModeLocal})
	if err != nil {
		t.Fatalf("start install: %v", err)
	}
	if err := svc.RunOperation(ctx, op.ID); err != nil {
		t.Fatalf("run install: %v", err)
	}
	if desktopAdapter.installed.Category != "games" {
		t.Fatalf("desktop manifest category = %q, want games", desktopAdapter.installed.Category)
	}
}
