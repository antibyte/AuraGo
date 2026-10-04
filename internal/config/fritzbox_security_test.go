package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFritzBoxTemplateUsesReadOnlyAndPreservesExplicitWriteGrants(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := yaml.Unmarshal(body, &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.FritzBox.System.ReadOnly || !cfg.FritzBox.Network.ReadOnly || !cfg.FritzBox.Telephony.ReadOnly || !cfg.FritzBox.SmartHome.ReadOnly || !cfg.FritzBox.Storage.ReadOnly || !cfg.FritzBox.TV.ReadOnly {
		t.Fatal("template grants Fritz!Box mutation rights")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("fritzbox:\n  system: {readonly: false}\n  network: {readonly: false}\n  telephony: {readonly: false}\n  smart_home: {readonly: false}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.FritzBox.System.ReadOnly || loaded.FritzBox.Network.ReadOnly || loaded.FritzBox.Telephony.ReadOnly || loaded.FritzBox.SmartHome.ReadOnly {
		t.Fatal("explicit administrator grants overwritten")
	}
}
