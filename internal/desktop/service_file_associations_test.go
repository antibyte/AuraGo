package desktop

import (
	"context"
	"testing"
)

func TestDefaultAppsValidateReferencesAndRetainUnavailableMappings(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	for _, invalid := range []string{`null`, `[]`, `{"TXT":"editor"}`, `{"../txt":"editor"}`, `{"txt":false}`, `{"txt":"does-not-exist"}`} {
		if err := svc.SetSettings(ctx, map[string]string{"files.default_apps": invalid}, SourceUser); err == nil {
			t.Fatalf("accepted invalid association %s", invalid)
		}
	}
	if err := svc.SetSettings(ctx, map[string]string{" files.default_apps ": `{"txt":"does-not-exist"}`}, SourceUser); err == nil {
		t.Fatal("whitespace bypassed application reference validation")
	}
	if _, err := svc.getDB().ExecContext(ctx, `INSERT INTO desktop_settings(key,value,updated_at) VALUES('files.default_apps',?, '')`, `{"old":"removed-app"}`); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetSettings(ctx, map[string]string{"files.default_apps": `{"old":"removed-app","txt":"editor"}`}, SourceUser); err != nil {
		t.Fatalf("preserve old mapping while changing another: %v", err)
	}
	if err := svc.SetSetting(ctx, "files.default_apps", `{"new":"removed-app"}`, SourceUser); err == nil {
		t.Fatal("copied unavailable mapping to a new extension")
	}
	if err := svc.SetSetting(ctx, "files.default_apps", `{}`, SourceUser); err != nil {
		t.Fatalf("explicit reset: %v", err)
	}
}

func TestAccessibilitySettingsRejectUnsupportedValues(t *testing.T) {
	for _, value := range []string{"100", "125", "150"} {
		if err := validateDesktopSetting("accessibility.text_size", value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"0", "200", "125.5", "NaN", "large"} {
		if err := validateDesktopSetting("accessibility.text_size", value); err == nil {
			t.Fatalf("accepted text scale %q", value)
		}
	}
	if err := validateDesktopSetting("accessibility.high_contrast", "false"); err != nil {
		t.Fatal(err)
	}
	if err := validateDesktopSetting("accessibility.high_contrast", "system"); err == nil {
		t.Fatal("accepted unsupported contrast value")
	}
}
