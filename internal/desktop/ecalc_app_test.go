package desktop

import "testing"

// ECalc is a plain builtin developer app: no capability gate, it is always listed. Its icon is
// app-owned artwork (logo_path), like Synth Studio's, so it needs no entry in the themed icon sets.
func TestBuiltinECalcAppIsAnAlwaysVisibleDeveloperApp(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "ecalc")
	if app.Entry != "builtin://ecalc" || app.Name != "ECalc" || app.Category != "dev" || app.Runtime != BuiltinRuntime {
		t.Fatalf("ecalc manifest = %+v", app)
	}
	if app.Icon != "ecalc" || app.Metadata["logo_path"] != "/img/desktop-icons/ecalc.svg" {
		t.Fatalf("ecalc icon = %q, metadata = %v; want the app-owned logo /img/desktop-icons/ecalc.svg", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 0 {
		t.Fatalf("ecalc must not require a capability, requires = %v", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("ecalc must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 1 {
		t.Fatalf("ecalc must show without any capability: %+v", got)
	}
}
