package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInfraDefaultsOnAndPreservesExplicitToggle(t *testing.T) {
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if node := yamlLookup(template, "virtual_desktop", "infra", "enabled"); node == nil || node.Value != "true" {
		t.Fatal("config template must explicitly enable Infra")
	}
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"template", string(template), true},
		{"desktop missing", "{}", true},
		{"infra missing", "virtual_desktop:\n  enabled: true\n", true},
		{"enabled missing", "virtual_desktop:\n  infra:\n    read_only: true\n", true},
		{"explicit true", "virtual_desktop:\n  infra:\n    enabled: true\n", true},
		{"explicit false", "virtual_desktop:\n  infra:\n    enabled: false\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.input), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.VirtualDesktop.Infra.Enabled != tc.want {
				t.Fatalf("Infra.Enabled = %v, want %v", cfg.VirtualDesktop.Infra.Enabled, tc.want)
			}
			for _, enabled := range []bool{false, true} {
				cfg.VirtualDesktop.Infra.Enabled = enabled
				if err := cfg.Save(path); err != nil {
					t.Fatal(err)
				}
				cfg, err = Load(path)
				if err != nil {
					t.Fatal(err)
				}
				if cfg.VirtualDesktop.Infra.Enabled != enabled {
					t.Fatalf("saved Infra.Enabled = %v reloaded as %v", enabled, cfg.VirtualDesktop.Infra.Enabled)
				}
				if cfg.VirtualDesktop.Infra.AllowSourceEdit || cfg.VirtualDesktop.Infra.AllowControl {
					t.Fatal("enabling Infra must not grant source writes or device control")
				}
			}
		})
	}
}

func TestInfraConfigurationRequiresExplicitGrants(t *testing.T) {
	for _, input := range []string{"{}", "virtual_desktop:\n  enabled: true\n", "virtual_desktop:\n  infra:\n    enabled: true\n"} {
		var cfg Config
		if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.VirtualDesktop.Infra.AllowSourceEdit || cfg.VirtualDesktop.Infra.AllowControl {
			t.Fatal("Infra source writes and device control must remain opt-in")
		}
	}
	var cfg Config
	if err := yaml.Unmarshal([]byte("virtual_desktop:\n  infra:\n    enabled: true\n    read_only: true\n    allow_source_edit: true\n    allow_control: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	want := InfraConfig{Enabled: true, ReadOnly: true, AllowSourceEdit: true, AllowControl: true}
	if cfg.VirtualDesktop.Infra != want {
		t.Fatalf("YAML grants did not round-trip: %+v", cfg.VirtualDesktop.Infra)
	}
	encoded, err := json.Marshal(cfg.VirtualDesktop)
	if err != nil {
		t.Fatal(err)
	}
	var desktop VirtualDesktopConfig
	if err := json.Unmarshal(encoded, &desktop); err != nil {
		t.Fatal(err)
	}
	if desktop.Infra != want {
		t.Fatalf("JSON grants did not round-trip: %+v", desktop.Infra)
	}
}
