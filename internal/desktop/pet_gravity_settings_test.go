package desktop

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDesktopPetGravitySettingPersistsAndRejectsInvalidWrites(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := Config{Enabled: true, WorkspaceDir: filepath.Join(root, "workspace"), DBPath: filepath.Join(root, "desktop.db")}
	svc := testServiceWithConfig(t, cfg)

	found := false
	for _, def := range DesktopSettingDefinitions() {
		if def.Key != "pet.gravity" {
			continue
		}
		found = true
		if def.Default != "false" {
			t.Fatalf("pet.gravity default = %q, want false", def.Default)
		}
		if len(def.Values) != 2 || def.Values[0] != "true" || def.Values[1] != "false" {
			t.Fatalf("pet.gravity values = %v, want true/false", def.Values)
		}
	}
	if !found {
		t.Fatal("pet.gravity is not a persistable desktop setting")
	}

	for _, value := range []string{"true", "false"} {
		if err := svc.SetSetting(ctx, "pet.gravity", value, SourceUser); err != nil {
			t.Fatalf("set pet.gravity=%s: %v", value, err)
		}
	}
	if err := svc.SetSetting(ctx, "pet.gravity", "maybe", SourceUser); err == nil {
		t.Fatal("pet.gravity accepted an invalid value")
	}

	// This settings fixture has no Docker backend for container cleanup.
	_ = svc.Close()
	reopened := testServiceWithConfig(t, cfg)
	boot, err := reopened.Bootstrap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := boot.Settings["pet.gravity"]; got != "false" {
		t.Fatalf("reopened pet.gravity = %q, want false", got)
	}
}
