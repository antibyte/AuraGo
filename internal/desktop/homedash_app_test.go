package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

// HomeDash is a comms app shown only while the server reports the "homedash" capability. Its icon is app-owned artwork (logo_path), like
// ECalc's, so it needs no entry in the themed icon sets.
func TestBuiltinHomeDashAppIsACommsAppGatedByItsCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "homedash")
	if app.Entry != "builtin://homedash" || app.Name != "HomeDash" || app.Category != "comms" || app.Runtime != BuiltinRuntime {
		t.Fatalf("homedash manifest = %+v", app)
	}
	if app.Icon != "homedash" || app.Metadata["logo_path"] != "/img/desktop-icons/homedash.svg" {
		t.Fatalf("homedash icon = %q, metadata = %v", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "homedash" {
		t.Fatalf("homedash requires = %v, want [homedash]", app.Requires)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("homedash must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"homedash": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "ui", "img", "desktop-icons", "homedash.svg")); err != nil {
		t.Fatalf("icon artwork missing: %v", err)
	}
}
