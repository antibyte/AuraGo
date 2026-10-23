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

func TestNormalizeDetectiveReadOperations(t *testing.T) {
	plain := DetectiveConfig{}
	plain.Normalize()
	if plain.ExtraReadOperations != nil {
		t.Fatalf("empty config gained a map: %v", plain.ExtraReadOperations)
	}
	cfg := DetectiveConfig{ExtraReadOperations: map[string][]string{
		" mcp_call ":    {" archive/read ", "", "archive/read", "  "},
		"composio_call": {},
		"   ":           {"orphan"},
		"web_scraper":   {"read"},
	}}
	cfg.Normalize()
	if got := cfg.ExtraReadOperations; len(got) != 2 {
		t.Fatalf("tools = %v", got)
	}
	if ops := cfg.ExtraReadOperations["mcp_call"]; len(ops) != 1 || ops[0] != "archive/read" {
		t.Fatalf("mcp_call = %v", ops)
	}
	if ops := cfg.ExtraReadOperations["web_scraper"]; len(ops) != 1 || ops[0] != "read" {
		t.Fatalf("web_scraper = %v", ops)
	}
}
