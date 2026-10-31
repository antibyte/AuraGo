package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestKaggleDefaultsAndSecretOmission(t *testing.T) {
	c := DefaultKaggleConfig()
	if c.Enabled || !c.ReadOnly || c.Local.Enabled || c.MaxTransferMB != 2048 || c.MaxStorageMB != 10240 || c.Local.MemoryMB != 4096 || c.Local.IdleMinutes != 15 {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	c.Token = "fixture-secret"
	c.LegacyKey = "fixture-secret"
	b, _ := yaml.Marshal(c)
	if strings.Contains(string(b), "fixture-secret") {
		t.Fatal("secret persisted in YAML")
	}
	b, _ = json.Marshal(c)
	if strings.Contains(string(b), "fixture-secret") {
		t.Fatal("secret exposed in JSON")
	}
}
