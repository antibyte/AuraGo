package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPersonaBoosterDefaultsAndRepair(t *testing.T) {
	cases := []struct {
		name        string
		yaml        string
		enabled     bool
		intensity   string
		spontaneous bool
		cooldown    int
	}{
		{"missing section", "personality:\n  core_personality: punk\n", true, "strong", true, 20},
		{"invalid intensity", "personality:\n  persona_booster:\n    intensity: extreme\n", true, "strong", true, 20},
		{"explicit values", "personality:\n  persona_booster:\n    enabled: false\n    intensity: WILD\n    spontaneous: false\n    cooldown_minutes: 0\n", false, "wild", false, 0},
		{"cooldown clamp", "personality:\n  persona_booster:\n    cooldown_minutes: 5000\n", true, "strong", true, 1440},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			got := cfg.Personality.PersonaBooster
			if got.Enabled != tc.enabled || got.Intensity != tc.intensity || got.Spontaneous != tc.spontaneous || got.CooldownMinutes != tc.cooldown {
				t.Fatalf("persona_booster = %+v", got)
			}
		})
	}
}
