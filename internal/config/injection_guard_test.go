package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInjectionGuardConfiguration(t *testing.T) {
	for _, action := range []string{"", "quarantine", "datamark", "allow"} {
		var cfg Config
		if err := yaml.Unmarshal([]byte("guardian:\n  injection_guard:\n    action: '"+action+"'\n"), &cfg); err != nil {
			t.Fatal(err)
		}
		err := ValidateInjectionGuardConfig(&cfg)
		if (err != nil) != (action == "allow") {
			t.Fatalf("action=%q: %v", action, err)
		}
		if cfg.Guardian.InjectionGuard.Enabled {
			t.Fatal("existing config enabled optional model")
		}
		if action == "" && cfg.Guardian.InjectionGuard.Action != "quarantine" {
			t.Fatal("missing safe default")
		}
	}
	var cfg Config
	if err := yaml.Unmarshal([]byte("guardian:\n  injection_guard:\n    enabled: true\n    action: datamark\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var next Config
	if yaml.Unmarshal(data, &next) != nil || !next.Guardian.InjectionGuard.Enabled || next.Guardian.InjectionGuard.Action != "datamark" {
		t.Fatal("roundtrip")
	}
}
