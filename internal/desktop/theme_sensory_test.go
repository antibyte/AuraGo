package desktop

import (
	"context"
	"testing"
)

func TestThemeSensorySettingsPersistIndependently(t *testing.T) {
	t.Parallel()
	svc := testService(t)
	ctx := context.Background()
	for _, sound := range []string{"desktop", "amiga", "cyberpunk", "black_gloss", "crazy", "lollipop", "organic", "water"} {
		if err := svc.SetSetting(ctx, "sound.theme", sound, SourceUser); err != nil {
			t.Fatalf("save sound %s: %v", sound, err)
		}
	}
	for key, value := range map[string]string{"appearance.theme_cursors": "false", "sound.enabled": "false", "appearance.theme": "organic"} {
		if err := svc.SetSetting(ctx, key, value, SourceUser); err != nil {
			t.Fatalf("save %s: %v", key, err)
		}
	}
	boot, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"appearance.theme_cursors": "false", "sound.enabled": "false", "sound.theme": "water", "appearance.theme": "organic"} {
		if got := boot.Settings[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if err := svc.SetSetting(ctx, "appearance.theme_cursors", "custom-url", SourceUser); err == nil {
		t.Fatal("cursor setting accepted an unsupported value")
	}
}
