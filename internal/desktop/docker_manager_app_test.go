package desktop

import "testing"

// The Docker Manager app is a system app that stays hidden until the "docker" capability is met.
func TestBuiltinDockerManagerAppRequiresDockerCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "docker-manager")
	if app.Entry != "builtin://docker-manager" || app.Name != "Docker Manager" || app.Icon != "docker-manager" || app.Category != "system" {
		t.Fatalf("docker-manager manifest = %+v", app)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "docker" {
		t.Fatalf("docker-manager requires = %v, want [docker]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable {
		t.Fatalf("docker-manager must be a visible, permanent builtin: %+v", app)
	}
	iconKnown := false
	for _, name := range desktopPreferredIconNames {
		if name == app.Icon {
			iconKnown = true
		}
	}
	if !iconKnown {
		t.Fatalf("icon %q is not a preferred desktop icon", app.Icon)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"docker": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
