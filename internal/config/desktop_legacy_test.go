package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopLegacyControlLevelLoadsAndIsRemovedOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("# retained\nvirtual_desktop:\n  enabled: true\n  readonly: true\n  control_level: trusted\n  future_setting: preserved\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.ReadOnly {
		t.Fatal("legacy authority changed")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(unchanged, original) {
		t.Fatal("load modified legacy file")
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if bytes.Contains(first, []byte("control_level")) || !bytes.Contains(first, []byte("future_setting: preserved")) || !bytes.Contains(first, []byte("# retained")) {
		t.Fatal("save did not preserve unrelated settings while removing legacy key")
	}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("save is not repeatable")
	}
}
