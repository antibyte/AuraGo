package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

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
