package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallAndListPets(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	files := map[string][]byte{
		"pet.json":         []byte(`{"id":"test-pet","displayName":"Test Pet","spritesheetPath":"spritesheet.webp"}`),
		"spritesheet.webp": []byte("RIFF\x00\x00\x00\x00WEBPVP8 "),
	}
	if err := svc.InstallPet(ctx, "test-pet", files); err != nil {
		t.Fatalf("InstallPet: %v", err)
	}

	pets, err := svc.ListPets(ctx)
	if err != nil {
		t.Fatalf("ListPets: %v", err)
	}
	if len(pets) < 1 {
		t.Fatalf("expected at least one pet, got %d", len(pets))
	}
	found := false
	for _, p := range pets {
		if p.ID == "test-pet" {
			found = true
			if p.DisplayName != "Test Pet" {
				t.Fatalf("display name = %q, want Test Pet", p.DisplayName)
			}
		}
	}
	if !found {
		t.Fatalf("test-pet not found in %v", pets)
	}

	pet, err := svc.GetPet(ctx, "test-pet")
	if err != nil {
		t.Fatalf("GetPet: %v", err)
	}
	if pet.ID != "test-pet" {
		t.Fatalf("GetPet returned %q, want test-pet", pet.ID)
	}
}

func TestSetActivePet(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	if err := svc.SetActivePet(ctx, "openpets-default"); err != nil {
		t.Fatalf("SetActivePet: %v", err)
	}
	active, err := svc.GetActivePetID(ctx)
	if err != nil {
		t.Fatalf("GetActivePetID: %v", err)
	}
	if active != "openpets-default" {
		t.Fatalf("active pet = %q, want openpets-default", active)
	}
}

func TestDeletePetClearsActive(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	files := map[string][]byte{
		"pet.json":         []byte(`{"id":"deleteme","displayName":"Delete Me","spritesheetPath":"spritesheet.webp"}`),
		"spritesheet.webp": []byte("RIFF\x00\x00\x00\x00WEBPVP8 "),
	}
	if err := svc.InstallPet(ctx, "deleteme", files); err != nil {
		t.Fatalf("InstallPet: %v", err)
	}
	if err := svc.SetActivePet(ctx, "deleteme"); err != nil {
		t.Fatalf("SetActivePet: %v", err)
	}
	if err := svc.DeletePet(ctx, "deleteme"); err != nil {
		t.Fatalf("DeletePet: %v", err)
	}
	active, err := svc.GetActivePetID(ctx)
	if err != nil {
		t.Fatalf("GetActivePetID: %v", err)
	}
	if active != "" {
		t.Fatalf("active pet = %q, want empty after delete", active)
	}
}

func TestInstallBundledDefaultPet(t *testing.T) {
	root := t.TempDir()
	if err := InstallBundledDefaultPet(root, defaultPetSpritesheet); err != nil {
		t.Fatalf("InstallBundledDefaultPet: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Pets", "openpets-default", "pet.json")); err != nil {
		t.Fatalf("default pet.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "Pets", "openpets-default", "spritesheet.webp")); err != nil {
		t.Fatalf("default spritesheet missing: %v", err)
	}
}

func TestServiceRepairsBrokenDefaultPetSeed(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	dbPath := filepath.Join(t.TempDir(), "desktop.db")
	cfg := Config{
		Enabled:            true,
		WorkspaceDir:       root,
		DBPath:             dbPath,
		MaxFileSizeMB:      1,
		AllowGeneratedApps: true,
		AllowAgentControl:  true,
		ControlLevel:       ControlConfirmDestructive,
	}
	svc := testServiceWithConfig(t, cfg)
	ctx := context.Background()
	spritesheetPath := filepath.Join(root, petsDirName, "openpets-default", "spritesheet.webp")
	if err := os.Remove(spritesheetPath); err != nil {
		t.Fatalf("remove default spritesheet fixture: %v", err)
	}
	_ = svc.Close()

	reopened := testServiceWithConfig(t, cfg)
	if _, err := os.Stat(spritesheetPath); err != nil {
		t.Fatalf("default spritesheet was not repaired: %v", err)
	}
	pets, err := reopened.ListPets(ctx)
	if err != nil {
		t.Fatalf("ListPets after repair: %v", err)
	}
	for _, pet := range pets {
		if pet.ID == "openpets-default" {
			return
		}
	}
	t.Fatalf("default pet missing after repair: %+v", pets)
}

func TestParsePetScale(t *testing.T) {
	if ParsePetScale("1.5") != 1.5 {
		t.Fatalf("ParsePetScale(1.5) failed")
	}
	if ParsePetScale("foo") != 1.0 {
		t.Fatalf("ParsePetScale(foo) should fallback to 1.0")
	}
	if ParsePetScale("5.0") != 1.0 {
		t.Fatalf("ParsePetScale(5.0) should clamp to 1.0")
	}
}
