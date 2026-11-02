package desktop

import "testing"

// Storymaker is a builtin creative app behind the "storymaker" capability (the storymaker.enabled switch). Its icon is
// app-owned artwork (logo_path), like Synth Studio's and ECalc's, so it needs no entry in the themed icon sets.
func TestBuiltinStorymakerAppRequiresItsCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "storymaker")
	if app.Entry != "builtin://storymaker" || app.Name != "Storymaker" || app.Category != "creative" || app.Runtime != BuiltinRuntime {
		t.Fatalf("storymaker manifest = %+v", app)
	}
	if app.Icon != "storymaker" || app.Metadata["logo_path"] != "/img/desktop-icons/storymaker.svg" || app.Metadata["open_maximized"] != "true" {
		t.Fatalf("storymaker icon = %q, metadata = %v; want the app-owned logo and open_maximized", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "storymaker" {
		t.Fatalf("storymaker requires = %v, want [storymaker]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("storymaker must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"storymaker": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
