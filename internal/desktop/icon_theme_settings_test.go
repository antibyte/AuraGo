package desktop

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDesktopAdditionalIconSettingsPersistAndRejectInvalidWrites(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := Config{Enabled: true, WorkspaceDir: filepath.Join(root, "workspace"), DBPath: filepath.Join(root, "desktop.db")}
	svc := testServiceWithConfig(t, cfg)
	for _, def := range DesktopSettingDefinitions() {
		if def.Key != "appearance.icon_theme" && def.Key != "appearance.tela_folder_color" && def.Key != "appearance.fluent_folder_color" {
			continue
		}
		for _, value := range def.Values {
			if err := svc.SetSetting(ctx, def.Key, value, SourceUser); err != nil {
				t.Fatalf("%s=%s: %v", def.Key, value, err)
			}
		}
	}
	want := map[string]string{"appearance.icon_theme": "buuf_nestort", "appearance.tela_folder_color": "dracula", "appearance.fluent_folder_color": "teal"}
	if err := svc.SetSettings(ctx, want, SourceUser); err != nil {
		t.Fatal(err)
	}
	for key := range want {
		if err := svc.SetSettings(ctx, map[string]string{key: "../../invalid", "appearance.accent": "orange"}, SourceUser); err == nil {
			t.Fatalf("accepted invalid %s", key)
		}
	}
	// This settings fixture has no Docker backend for container cleanup.
	_ = svc.Close()
	reopened := testServiceWithConfig(t, cfg)
	boot, err := reopened.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range want {
		if boot.Settings[key] != value {
			t.Fatalf("reopened %s = %q; want %q", key, boot.Settings[key], value)
		}
	}
	if boot.Settings["appearance.accent"] != "teal" {
		t.Fatal("failed atomic settings write changed another setting")
	}
	for _, theme := range []string{"papirus", "whitesur", "windows_xp", "tela", "candy", "fluent", "buuf_nestort"} {
		got, err := NormalizeDesktopIconName(theme+":folder", "test")
		if err != nil || got != "folder" {
			t.Fatalf("prefix %s: %q, %v", theme, got, err)
		}
	}
}
