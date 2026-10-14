package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Retro-Net is on by default: the template writes the grant explicitly as true
// and a config without the key loads it as true. An explicit false stays off,
// and Config.Save writes the value explicitly so a switched-off grant survives
// a reload.
func TestVirtualDesktopRetroNetToggleDefaultsOnAndPersists(t *testing.T) {
	t.Parallel()

	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	if !yamlHasPath(template, "virtual_desktop", "retronet_enabled") {
		t.Fatal("config_template.yaml must write virtual_desktop.retronet_enabled explicitly")
	}
	if node := yamlLookup(template, "virtual_desktop", "retronet_enabled"); node == nil || node.Value != "true" {
		t.Fatalf("config_template.yaml must enable Retro-Net, got %+v", node)
	}
	templateCopy := filepath.Join(t.TempDir(), "template.yaml")
	if err := os.WriteFile(templateCopy, template, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(templateCopy)
	if err != nil {
		t.Fatalf("Load(template copy): %v", err)
	}
	if !cfg.VirtualDesktop.RetroNetEnabled {
		t.Fatal("the template must load with Retro-Net on")
	}

	for _, tc := range []struct {
		name, yaml string
		want       bool
	}{
		{"key missing", "server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n", true},
		{"section missing", "server:\n  ui_language: en\n", true},
		{"explicit false", "server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n  retronet_enabled: false\n", false},
		{"explicit true", "server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n  retronet_enabled: true\n", true},
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(tc.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("%s: Load: %v", tc.name, err)
		}
		if cfg.VirtualDesktop.RetroNetEnabled != tc.want {
			t.Fatalf("%s: RetroNetEnabled = %v, want %v", tc.name, cfg.VirtualDesktop.RetroNetEnabled, tc.want)
		}
	}

	for _, value := range []bool{false, true} {
		savedPath := filepath.Join(t.TempDir(), "saved.yaml")
		if err := os.WriteFile(savedPath, []byte("server:\n  ui_language: en\nvirtual_desktop:\n  enabled: true\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		toSave, err := Load(savedPath)
		if err != nil {
			t.Fatalf("load test config: %v", err)
		}
		toSave.VirtualDesktop.RetroNetEnabled = value
		if err := toSave.Save(savedPath); err != nil {
			t.Fatalf("Save(%v): %v", value, err)
		}
		raw, err := os.ReadFile(savedPath)
		if err != nil {
			t.Fatal(err)
		}
		if !yamlHasPath(raw, "virtual_desktop", "retronet_enabled") {
			t.Fatalf("Save(%v) did not write virtual_desktop.retronet_enabled:\n%s", value, raw)
		}
		var saved struct {
			VirtualDesktop struct {
				RetroNetEnabled bool `yaml:"retronet_enabled"`
			} `yaml:"virtual_desktop"`
		}
		if err := yaml.Unmarshal(raw, &saved); err != nil {
			t.Fatalf("parse saved config: %v", err)
		}
		if saved.VirtualDesktop.RetroNetEnabled != value {
			t.Fatalf("Save persisted retronet_enabled=%v, want %v:\n%s", saved.VirtualDesktop.RetroNetEnabled, value, raw)
		}
		reloaded, err := Load(savedPath)
		if err != nil {
			t.Fatalf("Load(saved %v): %v", value, err)
		}
		if reloaded.VirtualDesktop.RetroNetEnabled != value {
			t.Fatalf("a saved retronet_enabled=%v reloads as %v", value, reloaded.VirtualDesktop.RetroNetEnabled)
		}
	}
}
