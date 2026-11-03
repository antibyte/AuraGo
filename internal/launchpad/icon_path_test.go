package launchpad

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIconPathsKeepDeletionInsideOwnedDirectory(t *testing.T) {
	data := filepath.Join(t.TempDir(), "data")
	if err := os.MkdirAll(filepath.Join(data, iconDir), 0700); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"tokens.json", "../data-sibling/marker.png", "launchpad_icons/../database.png", "launchpad_icons/nested/file.png"} {
		target := filepath.Join(data, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := DeleteIcon(data, relative); err == nil {
			t.Errorf("accepted %q", relative)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("modified protected fixture %q: %v", relative, err)
		}
	}
	for _, relative := range []string{"launchpad_icons/icon.png", `launchpad_icons\icon.webp`} {
		name, err := localIconName(relative)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(data, iconDir, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := DeleteIcon(data, relative); err != nil {
			t.Fatal(err)
		}
	}
	if err := DeleteIcon(data, "https://example.invalid/icon.png"); err != nil {
		t.Fatal(err)
	}
}

func TestIconPathsRejectSymlinks(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outside, "icon.png")
	if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(data, iconDir)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := DeleteIcon(data, "launchpad_icons/icon.png"); err == nil {
		t.Fatal("accepted a symlinked icon directory")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
}

func TestLinkWritesValidateIconPaths(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "launchpad.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	link := LaunchpadLink{Title: "Fixture", URL: "https://example.invalid"}
	link.ID, err = Create(db, link)
	if err != nil {
		t.Fatal(err)
	}
	for _, icon := range []string{"tokens.json", "../data-sibling/marker.png", "launchpad_icons/icon.p", "launchpad_icons/../icon.png"} {
		link.IconPath = icon
		if _, err := Create(db, link); err == nil {
			t.Errorf("Create accepted %q", icon)
		}
		if err := Update(db, link); err == nil {
			t.Errorf("Update accepted %q", icon)
		}
	}
	link.IconPath = "https://example.invalid/icon.png"
	if err := Update(db, link); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".p", ".jp", ".g", "."} {
		if allowedIconExtension(ext) {
			t.Errorf("accepted partial extension %q", ext)
		}
	}
}
