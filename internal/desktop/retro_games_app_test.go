package desktop

import "testing"

func TestBuiltinRetroGamesAppRequiresCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "retro-games")
	if app.Entry != "builtin://retro-games" || app.Name != "Retro Games" || app.Icon != "arcade" || app.Category != "games" {
		t.Fatalf("retro-games manifest = %+v", app)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "retro_games" {
		t.Fatalf("requires = %v", app.Requires)
	}
	if !app.StartVisible || !app.Builtin {
		t.Fatalf("retro-games must be a visible builtin: %+v", app)
	}
	if target, ok := desktopIconAliases[app.Icon]; !ok || target != "run" {
		t.Fatalf("icon alias %q -> %q, want run", app.Icon, target)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"retro_games": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
