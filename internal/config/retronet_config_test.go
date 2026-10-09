package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Retro-Net dials public services from the server, so the grant is written
// explicitly as false in the template, loads as false when absent and is kept
// by Config.Save.
func TestVirtualDesktopRetroNetToggleDefaultsOffAndPersists(t *testing.T) {
	t.Parallel()

	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	if !yamlHasPath(template, "virtual_desktop", "retronet_enabled") {
		t.Fatal("config_template.yaml must write virtual_desktop.retronet_enabled explicitly")
	}
	templateCopy := filepath.Join(t.TempDir(), "template.yaml")
	if err := os.WriteFile(templateCopy, template, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(templateCopy)
	if err != nil {
		t.Fatalf("Load(template copy): %v", err)
	}
	if cfg.VirtualDesktop.RetroNetEnabled {
		t.Fatal("the template enables Retro-Net; it must default to false")
	}

	minimal := filepath.Join(t.TempDir(), "minimal.yaml")
	if err := os.WriteFile(minimal, []byte("server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(minimal)
	if err != nil {
		t.Fatalf("Load(minimal): %v", err)
	}
	if cfg.VirtualDesktop.RetroNetEnabled {
		t.Fatal("a config without retronet_enabled must load with Retro-Net off")
	}

	savedPath := filepath.Join(t.TempDir(), "saved.yaml")
	if err := os.WriteFile(savedPath, []byte("server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	toSave := &Config{}
	toSave.VirtualDesktop.RetroNetEnabled = true
	if err := toSave.Save(savedPath); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		VirtualDesktop struct {
			RetroNetEnabled bool `yaml:"retronet_enabled"`
		} `yaml:"virtual_desktop"`
	}
	if err := yaml.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if !saved.VirtualDesktop.RetroNetEnabled {
		t.Fatalf("Save did not persist virtual_desktop.retronet_enabled:\n%s", raw)
	}
}
