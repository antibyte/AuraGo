package desktop

import (
	"context"
	"testing"
)

func TestThemeSensorySettingsPersistIndependently(t *testing.T) {
	t.Parallel()
	svc := testService(t)
	ctx := context.Background()
	for _, sound := range []string{"desktop", "amiga", "cyberpunk", "black_gloss", "crazy", "lollipop", "organic", "electronic_dreams", "windows_xp", "green_slime", "voxel_world", "orbit", "water"} {
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

func TestExperienceThemeSettingsAreAtomicAndPreservePersonalChoices(t *testing.T) {
	t.Parallel()
	svc := testService(t)
	ctx := context.Background()
	if err := svc.SetSettings(ctx, map[string]string{"appearance.wallpaper": "midnight", "sound.theme": "water"}, SourceUser); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"electronic_dreams", "windows_xp", "green_slime", "voxel_world"} {
		icon := "papirus"
		if theme == "windows_xp" {
			icon = "windows_xp"
		}
		if err := svc.SetSettings(ctx, map[string]string{"appearance.theme": theme, "appearance.icon_theme": icon}, SourceUser); err != nil {
			t.Fatal(err)
		}
		boot, err := svc.Bootstrap(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{"appearance.theme": theme, "appearance.icon_theme": icon, "appearance.wallpaper": "midnight", "sound.theme": "water"} {
			if got := boot.Settings[key]; got != want {
				t.Errorf("%s after %s = %q; want %q", key, theme, got, want)
			}
		}
	}
	if err := svc.SetSettings(ctx, map[string]string{"appearance.theme": "windows_xp", "appearance.icon_theme": "invalid-theme"}, SourceUser); err == nil {
		t.Fatal("invalid icon theme must reject the entire update")
	}
	boot, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if boot.Settings["appearance.theme"] != "voxel_world" || boot.Settings["appearance.icon_theme"] != "papirus" {
		t.Fatalf("failed batch changed theme settings: %v", boot.Settings)
	}
}
