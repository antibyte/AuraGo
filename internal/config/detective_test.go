package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectiveConfigurationRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("detective:\n  enabled: true\n  readonly: false\n  profiles:\n    quick: {seconds: 300, tools: 40, iterations: 60, tokens: 50000}\n  extra_read_operations:\n    mcp_call: [archive/read]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Detective.Profiles["maximum"] = DetectiveProfile{Seconds: 3600, Tools: 250, Iterations: 320}
	if err = cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Detective.Enabled || cfg.Detective.ReadOnly || cfg.Detective.Profiles["quick"].Tokens != 50000 || cfg.Detective.Profiles["maximum"].Iterations != 320 || cfg.Detective.ExtraReadOperations["mcp_call"][0] != "archive/read" {
		t.Fatalf("lost research config: %+v", cfg.Detective)
	}
}
