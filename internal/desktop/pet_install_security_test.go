package desktop

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/testutil"
)

func TestCleanPetRelPathRejectsWindowsDeviceNames(t *testing.T) {
	// Validate serial device names as strings only; integration fixtures use NUL.
	for _, name := range []string{"NUL", "nul.txt", "CON", "aux.png", "PRN", "CONIN$", "CONOUT$", "COM1", "com9.txt", "LPT1", "LPT².png", "COM¹", "x/NUL/y", "x:stream", `C:\pets`, "x/../..", "../pet.json", "x.", "x ", "bad\x00name"} {
		if clean, ok := cleanPetRelPath(name); ok {
			t.Errorf("accepted %q as %q", name, clean)
		}
	}
	for _, name := range []string{"sprites/sheet.webp", "./pet.json", "sprites/extra/../sheet.webp", "company.png", "com10.txt"} {
		if _, ok := cleanPetRelPath(name); !ok {
			t.Errorf("rejected valid path %q", name)
		}
	}
}

func TestInstallPetPreflightsAllEntries(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()
	id := "preflight-pet"
	original := []byte(`{"displayName":"Original","spritesheetPath":"sprites/sheet.webp"}`)
	if err := svc.InstallPet(ctx, id, map[string][]byte{"pet.json": original, "sprites/sheet.webp": []byte("sheet")}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"../escape", "NUL", "x:stream", "pet.json/child", "sprites", "./pet.json"} {
		t.Run(invalid, func(t *testing.T) {
			update := map[string][]byte{"pet.json": []byte("changed"), "sprites/sheet.webp": []byte("changed"), invalid: []byte("invalid")}
			if err := svc.InstallPet(ctx, id, update); err == nil {
				t.Fatal("invalid package accepted")
			}
			got, err := os.ReadFile(filepath.Join(svc.Config().WorkspaceDir, petsDirName, id, "pet.json"))
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("validation changed existing manifest: %q, %v", got, err)
			}
			if err := svc.InstallPet(ctx, "new-pet", update); err == nil {
				t.Fatal("invalid new package accepted")
			}
			if _, err := os.Lstat(filepath.Join(svc.Config().WorkspaceDir, petsDirName, "new-pet")); !os.IsNotExist(err) {
				t.Fatalf("invalid package created destination: %v", err)
			}
		})
	}
}

func TestPetZipRejectsEveryInvalidEntry(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode os.FileMode
	}{
		{"../escape", 0600}, {"NUL", 0600}, {"x:stream", 0600}, {"../bad/", os.ModeDir | 0700}, {"linked", os.ModeSymlink | 0600}, {"pet.json", 0600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			zw := zip.NewWriter(&data)
			for i, name := range []string{"pet.json", "spritesheet.webp", tc.name} {
				h := &zip.FileHeader{Name: name}
				h.SetMode(0600)
				if i == 2 {
					h.SetMode(tc.mode)
				}
				w, err := zw.CreateHeader(h)
				if err != nil {
					t.Fatal(err)
				}
				if !h.Mode().IsDir() {
					if _, err := w.Write([]byte("fixture")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := extractPetZip(bytes.NewReader(data.Bytes()), int64(data.Len())); err == nil {
				t.Fatal("invalid zip entry accepted")
			}
		})
	}
}

func TestInstallPetRejectsLinkedTargets(t *testing.T) {
	svc := testService(t)
	workspace := svc.Config().WorkspaceDir
	for _, kind := range []string{"external", "internal"} {
		t.Run(kind, func(t *testing.T) {
			target := t.TempDir()
			if kind == "internal" {
				target = filepath.Join(workspace, "target")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(target, "pet.json")
			if err := os.WriteFile(marker, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			id := "linked-" + kind
			testutil.DirectoryLink(t, filepath.Join(workspace, petsDirName, id), target)
			if err := svc.InstallPet(context.Background(), id, map[string][]byte{"pet.json": []byte("changed")}); err == nil {
				t.Fatal("linked pet accepted")
			}
			id += "-nested"
			if err := os.Mkdir(filepath.Join(workspace, petsDirName, id), 0700); err != nil {
				t.Fatal(err)
			}
			testutil.DirectoryLink(t, filepath.Join(workspace, petsDirName, id, "nested"), target)
			if err := svc.InstallPet(context.Background(), id, map[string][]byte{"nested/pet.json": []byte("changed")}); err == nil {
				t.Fatal("linked parent accepted")
			}
			got, err := os.ReadFile(marker)
			if err != nil || string(got) != "untouched" {
				t.Fatalf("link target changed: %q, %v", got, err)
			}
		})
	}
}

func TestPetWriteRejectsDirectorySwappedAfterPreflight(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := preflightPetWrite(root, "nested/pet.json"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "nested"), filepath.Join(dir, "old")); err != nil {
		t.Fatal(err)
	}
	testutil.DirectoryLink(t, filepath.Join(dir, "nested"), outside)
	if parent, err := openPetWriteDirectory(root, "nested"); err == nil {
		parent.Close()
		t.Fatal("swapped directory accepted")
	}
}

func TestPetZipValidRoundTrip(t *testing.T) {
	svc := testService(t)
	var data bytes.Buffer
	zw := zip.NewWriter(&data)
	for name, body := range map[string]string{"pet.json": `{"spritesheetPath":"sprites/sheet.webp"}`, "sprites/sheet.webp": "sheet"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := svc.InstallPetFromZip(context.Background(), "valid-zip", bytes.NewReader(data.Bytes()), int64(data.Len())); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPet(context.Background(), "valid-zip"); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	svc.cfg.ReadOnly = true
	svc.mu.Unlock()
	if err := svc.InstallPetFromZip(context.Background(), "denied-zip", bytes.NewReader(data.Bytes()), int64(data.Len())); err == nil {
		t.Fatal("read-only zip install accepted")
	}
}
