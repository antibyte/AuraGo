package desktop

import (
	"reflect"
	"testing"
)

// Chibi Sensei is a builtin AI app behind the "chibi_sensei" capability (the chibi_sensei.enabled switch). Its icon
// is app-owned artwork (logo_path), so it needs no entry in the themed icon sets.
func TestBuiltinChibiSenseiAppRequiresItsCapability(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "chibi-sensei")
	if app.Entry != "builtin://chibi-sensei" || app.Name != "Chibi Sensei" || app.Category != "ai" || app.Runtime != BuiltinRuntime {
		t.Fatalf("chibi-sensei manifest = %+v", app)
	}
	if app.Icon != "chibi-sensei" || app.Metadata["logo_path"] != "/img/desktop-icons/chibi-sensei.svg" || app.Metadata["open_maximized"] != "true" {
		t.Fatalf("chibi-sensei icon = %q, metadata = %v; want the app-owned logo and open_maximized", app.Icon, app.Metadata)
	}
	if len(app.Requires) != 1 || app.Requires[0] != "chibi_sensei" {
		t.Fatalf("chibi-sensei requires = %v, want [chibi_sensei]", app.Requires)
	}
	if want := []string{"files:read", "files:write", "notifications"}; !reflect.DeepEqual(app.Permissions, want) {
		t.Fatalf("chibi-sensei permissions = %v, want %v", app.Permissions, want)
	}
	if !app.Builtin || !app.StartVisible || !app.DockVisible || app.Deletable || app.Internal {
		t.Fatalf("chibi-sensei must be a visible, permanent builtin: %+v", app)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{}); len(got) != 0 {
		t.Fatalf("without the capability the app must stay hidden: %+v", got)
	}
	if got := FilterAvailableApps([]AppManifest{app}, staticCapabilities{"chibi_sensei": true}); len(got) != 1 {
		t.Fatalf("with the capability the app must show: %+v", got)
	}
}
