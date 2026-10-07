package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIntegrationTemplateSectionsUseSupportedFieldsAndSafeDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var sections map[string]any
	if err := yaml.Unmarshal(data, &sections); err != nil {
		t.Fatal(err)
	}
	var cfg Config
	for name, target := range map[string]any{
		"a2a": &cfg.A2A, "email_accounts": &cfg.EmailAccounts, "firewall": &cfg.Firewall,
		"golangci_lint": &cfg.GolangciLint, "heartbeat": &cfg.Heartbeat, "journal": &cfg.Journal, "security_proxy": &cfg.SecurityProxy,
	} {
		value, exists := sections[name]
		if !exists {
			t.Fatalf("reference is missing %s", name)
		}
		encoded, err := yaml.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(encoded))
		decoder.KnownFields(true)
		if err := decoder.Decode(target); err != nil {
			t.Fatalf("unsupported fields in %s: %v", name, err)
		}
	}
	if cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled || cfg.Firewall.Enabled || cfg.GolangciLint.Enabled || cfg.Heartbeat.Enabled || cfg.SecurityProxy.Enabled || len(cfg.EmailAccounts) != 0 {
		t.Fatal("reference silently enables an optional integration")
	}
	if cfg.Firewall.Mode != "readonly" || !cfg.A2A.Auth.APIKeyEnabled {
		t.Fatal("reference weakens operator safety defaults")
	}
	if cfg.Heartbeat.DayTimeWindow.Interval != "1h" || cfg.Heartbeat.NightTimeWindow.Interval != "4h" || !cfg.Journal.AutoEntries || !cfg.Journal.DailySummary {
		t.Fatal("reference differs from loader schedule/journal defaults")
	}
}
