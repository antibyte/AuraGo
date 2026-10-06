package desktop

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var expectedBundledDefaultPetIDs = []string{"openpets-default", "snoopy", "clippit", "tux", "wall-e", "dobby", "aurago-neutral", "aurago-servant", "aurago-professional", "aurago-mistress", "aurago-thinker", "aurago-evil", "aurago-secretary", "aurago-psycho", "aurago-punk", "aurago-friend", "aurago-mcp", "aurago-terminator"}

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

func TestInstallBundledDefaultPets(t *testing.T) {
	root := t.TempDir()
	if err := InstallBundledDefaultPets(root); err != nil {
		t.Fatalf("InstallBundledDefaultPets: %v", err)
	}

	for _, id := range expectedBundledDefaultPetIDs {
		t.Run(id, func(t *testing.T) {
			petDir := filepath.Join(root, "Pets", id)
			if _, err := os.Stat(filepath.Join(petDir, "pet.json")); err != nil {
				t.Fatalf("%s pet.json missing: %v", id, err)
			}
			if _, err := os.Stat(filepath.Join(petDir, "spritesheet.webp")); err != nil {
				t.Fatalf("%s spritesheet missing: %v", id, err)
			}
			pet, err := getPetInDir(root, id)
			if err != nil {
				t.Fatalf("getPetInDir(%s): %v", id, err)
			}
			if pet.ID != id {
				t.Fatalf("pet ID = %q, want %q", pet.ID, id)
			}
		})
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

func TestServiceBootstrapRepairsEmptyPetWorkspace(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	root := svc.Config().WorkspaceDir
	petDir := filepath.Join(root, petsDirName, "openpets-default")
	if err := os.RemoveAll(petDir); err != nil {
		t.Fatalf("remove default pet fixture: %v", err)
	}

	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	for _, pet := range bootstrap.Pets {
		if pet.ID == "openpets-default" {
			if _, err := os.Stat(filepath.Join(petDir, "spritesheet.webp")); err != nil {
				t.Fatalf("default spritesheet was not repaired: %v", err)
			}
			return
		}
	}
	t.Fatalf("default pet missing from bootstrap after repair: %+v", bootstrap.Pets)
}

func TestServiceBootstrapIncludesAllBundledDefaultPets(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	bootstrap, err := svc.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	seen := make(map[string]bool)
	for _, pet := range bootstrap.Pets {
		seen[pet.ID] = true
	}
	for _, id := range expectedBundledDefaultPetIDs {
		if !seen[id] {
			t.Fatalf("bundled pet %q missing from bootstrap: %+v", id, bootstrap.Pets)
		}
	}
}

// writePetFixture creates Pets/<id>/pet.json with the given spritesheet path
// and returns the pet directory.
func writePetFixture(t *testing.T, workspaceDir, id, spritesheet string) string {
	t.Helper()
	petDir := filepath.Join(workspaceDir, petsDirName, id)
	if err := os.MkdirAll(petDir, 0o700); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(PetJSON{DisplayName: "Fixture " + id, SpritesheetPath: spritesheet})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(petDir, "pet.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	return petDir
}

func writePetSheetFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), 0o600); err != nil {
		t.Fatal(err)
	}
}

// linkPetDirForTest links link to the directory target: a symlink where the
// OS allows one, else (Windows without the symlink privilege) a junction.
func linkPetDirForTest(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	} else if runtime.GOOS != "windows" {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// Paths are test-owned temporary directories, passed as separate args.
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junctions unavailable: %v %s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

func assertPetErrorHidesHostPaths(t *testing.T, err error, hidden ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, h := range hidden {
		if h != "" && (strings.Contains(msg, h) || strings.Contains(msg, filepath.ToSlash(h))) {
			t.Fatalf("error must not disclose host path %q: %v", h, err)
		}
	}
}

func TestGetPetRejectsEscapingSpritesheetAndID(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	ws := svc.Config().WorkspaceDir
	outside := t.TempDir()
	outsideSheet := filepath.Join(outside, "hostname")
	writePetSheetFixture(t, outsideSheet)

	// A valid spritesheet next to each manifest proves that only the
	// spritesheet path form causes the rejection.
	cases := []struct {
		id          string
		spritesheet string
	}{
		{"traversal-root", "../../../etc/hostname"},
		{"traversal-outside", ""}, // filled below: relative path to outsideSheet
		{"inner-traversal", "sprites/../../traversal-outside/spritesheet.webp"},
		{"absolute", outsideSheet},
		{"rooted", "/spritesheet.webp"},
		{"backslash", `sprites\spritesheet.webp`},
		{"colon", "spritesheet.webp:stream"},
		{"drive", "C:spritesheet.webp"},
		{"dot", "."},
		{"dir", "sprites"},
	}
	for i := range cases {
		petDir := filepath.Join(ws, petsDirName, cases[i].id)
		if cases[i].spritesheet == "" {
			rel, err := filepath.Rel(petDir, outsideSheet)
			if err != nil {
				t.Skipf("no relative path to outside fixture: %v", err)
			}
			cases[i].spritesheet = filepath.ToSlash(rel)
		}
		writePetFixture(t, ws, cases[i].id, cases[i].spritesheet)
		writePetSheetFixture(t, filepath.Join(petDir, "spritesheet.webp"))
		writePetSheetFixture(t, filepath.Join(petDir, "sprites", "spritesheet.webp"))
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			_, err := svc.GetPet(ctx, tc.id)
			if err == nil {
				t.Fatalf("spritesheet path %q must be rejected", tc.spritesheet)
			}
			assertPetErrorHidesHostPaths(t, err, ws, outside)
			if strings.Contains(err.Error(), "etc") || strings.Contains(err.Error(), "hostname") {
				t.Fatalf("error must not echo the manifest path: %v", err)
			}
		})
	}

	for _, id := range []string{"../other", "..", "Upper", "a/b", `a\b`, "", ".hidden"} {
		_, err := svc.GetPet(ctx, id)
		if err == nil {
			t.Fatalf("pet id %q outside the pattern must be rejected", id)
		}
		assertPetErrorHidesHostPaths(t, err, ws, outside)
	}

	pets, err := svc.ListPets(ctx)
	if err != nil {
		t.Fatalf("ListPets: %v", err)
	}
	for _, pet := range pets {
		for _, tc := range cases {
			if pet.ID == tc.id {
				t.Fatalf("ListPets must skip pet %q with spritesheet %q", tc.id, tc.spritesheet)
			}
		}
	}
}

func TestGetPetRejectsLinkedSpritesheets(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	ws := svc.Config().WorkspaceDir
	outside := t.TempDir()
	outsideSheet := filepath.Join(outside, "spritesheet.webp")
	writePetSheetFixture(t, outsideSheet)

	t.Run("file symlink", func(t *testing.T) {
		petDir := writePetFixture(t, ws, "file-link", "spritesheet.webp")
		if err := os.Symlink(outsideSheet, filepath.Join(petDir, "spritesheet.webp")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := svc.GetPet(ctx, "file-link")
		assertPetErrorHidesHostPaths(t, err, ws, outside)
	})

	t.Run("in-pet file symlink", func(t *testing.T) {
		petDir := writePetFixture(t, ws, "inner-link", "alias.webp")
		writePetSheetFixture(t, filepath.Join(petDir, "spritesheet.webp"))
		if err := os.Symlink("spritesheet.webp", filepath.Join(petDir, "alias.webp")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := svc.GetPet(ctx, "inner-link")
		assertPetErrorHidesHostPaths(t, err, ws, outside)
	})

	t.Run("directory link", func(t *testing.T) {
		petDir := writePetFixture(t, ws, "dir-link", "linked/spritesheet.webp")
		linkPetDirForTest(t, outside, filepath.Join(petDir, "linked"))
		_, err := svc.GetPet(ctx, "dir-link")
		assertPetErrorHidesHostPaths(t, err, ws, outside)
	})

	t.Run("linked pet directory", func(t *testing.T) {
		target := t.TempDir()
		writePetFixture(t, target, "real", "spritesheet.webp")
		writePetSheetFixture(t, filepath.Join(target, petsDirName, "real", "spritesheet.webp"))
		if _, err := getPetInDir(target, "real"); err != nil {
			t.Fatalf("fixture pet must load in its own workspace: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(ws, petsDirName), 0o700); err != nil {
			t.Fatal(err)
		}
		linkPetDirForTest(t, filepath.Join(target, petsDirName, "real"), filepath.Join(ws, petsDirName, "pet-link"))
		_, err := svc.GetPet(ctx, "pet-link")
		assertPetErrorHidesHostPaths(t, err, ws, target)
	})
}

func TestGetPetResolvesValidSpritesheetInsidePetDir(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	ws := svc.Config().WorkspaceDir

	cases := []struct {
		id, spritesheet, want string
	}{
		{"nested-sheet", "sprites/sheet.webp", "sprites/sheet.webp"},
		{"dotted-sheet", "./sprites/./sheet.webp", "sprites/sheet.webp"},
		{"inner-up-sheet", "sprites/extra/../sheet.webp", "sprites/sheet.webp"},
		{"padded-sheet", "  sprites/sheet.webp  ", "sprites/sheet.webp"},
		{"default-sheet", "", "spritesheet.webp"},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			petDir := writePetFixture(t, ws, tc.id, tc.spritesheet)
			writePetSheetFixture(t, filepath.Join(petDir, filepath.FromSlash(tc.want)))
			pet, err := svc.GetPet(ctx, tc.id)
			if err != nil {
				t.Fatalf("GetPet: %v", err)
			}
			if pet.ID != tc.id || pet.Spritesheet != tc.want {
				t.Fatalf("pet = %+v, want id %q spritesheet %q", pet, tc.id, tc.want)
			}
		})
	}

	t.Run("missing", func(t *testing.T) {
		writePetFixture(t, ws, "missing-sheet", "sprites/none.webp")
		_, err := svc.GetPet(ctx, "missing-sheet")
		assertPetErrorHidesHostPaths(t, err, ws)
	})
	t.Run("missing manifest", func(t *testing.T) {
		if err := os.MkdirAll(filepath.Join(ws, petsDirName, "no-manifest"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := svc.GetPet(ctx, "no-manifest")
		assertPetErrorHidesHostPaths(t, err, ws)
	})
}

func TestBundledPetIDsMatchPetIDPattern(t *testing.T) {
	for _, pet := range bundledDefaultPets() {
		if !petIDPattern.MatchString(pet.Manifest.ID) {
			t.Fatalf("bundled pet id %q does not match the pet id pattern", pet.Manifest.ID)
		}
	}
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
