package desktop

import "testing"

func TestBuiltinSynthStudioAppRegistration(t *testing.T) {
	app := testFindApp(t, BuiltinApps(), "synth-studio")
	if app.Name != "Synth Studio" || app.Entry != "builtin://synth-studio" || app.Icon != "synth-studio" {
		t.Fatalf("unexpected Synth Studio registration: %+v", app)
	}
	if app.Category != "creative" {
		t.Fatalf("category = %q, want creative", app.Category)
	}
	if app.Metadata["open_maximized"] != "true" || app.Metadata["logo_path"] != "/img/desktop-icons/synth-studio.svg" {
		t.Fatalf("unexpected window or icon metadata: %+v", app.Metadata)
	}
	permissions := map[string]bool{}
	for _, permission := range app.Permissions {
		permissions[permission] = true
	}
	for _, permission := range []string{"files:read", "files:write"} {
		if !permissions[permission] {
			t.Errorf("Synth Studio is missing permission %q", permission)
		}
	}
}
