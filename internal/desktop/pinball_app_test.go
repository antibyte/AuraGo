package desktop

import "testing"

func TestBuiltinPinballApp(t *testing.T) {
	for _, app := range BuiltinApps() {
		if app.ID != "pinball" {
			continue
		}
		if app.Name != "AuraPinball" || app.Category != "games" || app.Runtime != BuiltinRuntime || app.Entry != "builtin://pinball" {
			t.Fatalf("invalid Pinball registration: %+v", app)
		}
		if app.Metadata["logo_path"] != "/img/desktop-icons/pinball.svg" || app.Metadata["open_maximized"] != "true" || len(app.Requires) != 0 || len(app.Permissions) != 0 {
			t.Fatalf("invalid Pinball presentation or policy: %+v", app)
		}
		return
	}
	t.Fatal("missing built-in Pinball app")
}
