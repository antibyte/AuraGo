package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsExplicitInvalidRuViewSettings(t *testing.T) {
	for _, setting := range []string{"mode: invalid", "source: invalid", "http_port: -1", "udp_port: 70000", "storage_mb: -1"} {
		t.Run(setting, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("ruview:\n  "+setting+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || !strings.Contains(strings.ToLower(err.Error()), "ruview") {
				t.Fatalf("explicit invalid RuView setting accepted or wrong error: %v", err)
			}
		})
	}
}

func TestNormalizeRuViewConfigFillsOnlyUnsetValues(t *testing.T) {
	var c RuViewConfig
	NormalizeRuViewConfig(&c)
	want := DefaultRuViewConfig()
	if c.Mode != want.Mode || c.Source != want.Source || c.HTTPPort != want.HTTPPort || c.UDPPort != want.UDPPort || c.StorageMB != want.StorageMB || c.UDPAllow == nil {
		t.Fatalf("normalized empty config = %#v, want the defaults of %#v", c, want)
	}
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatalf("normalized empty config is invalid: %v", err)
	}

	c = RuViewConfig{Mode: " \t", Source: " "}
	NormalizeRuViewConfig(&c)
	if c.Mode != want.Mode || c.Source != want.Source {
		t.Fatalf("blank mode/source normalized to %q/%q", c.Mode, c.Source)
	}

	c = RuViewConfig{Mode: " External ", Source: "WIFI"}
	NormalizeRuViewConfig(&c)
	if c.Mode != "external" || c.Source != "wifi" {
		t.Fatalf("padded mixed-case mode/source normalized to %q/%q", c.Mode, c.Source)
	}

	explicit := RuViewConfig{Mode: "kubernetes", Source: "radio", HTTPPort: 80, UDPPort: 70000, StorageMB: 1, UDPAllow: []string{"8.8.8.8"}}
	c = explicit
	c.UDPAllow = append([]string(nil), explicit.UDPAllow...)
	NormalizeRuViewConfig(&c)
	if c.Mode != explicit.Mode || c.Source != explicit.Source || c.HTTPPort != explicit.HTTPPort || c.UDPPort != explicit.UDPPort || c.StorageMB != explicit.StorageMB || len(c.UDPAllow) != 1 {
		t.Fatalf("explicit values were replaced: %#v", c)
	}
	for _, invalid := range []RuViewConfig{
		{Mode: "kubernetes"},
		{Source: "radio"},
		{HTTPPort: 80},
		{UDPPort: 70000},
		{StorageMB: 1},
	} {
		NormalizeRuViewConfig(&invalid)
		if ValidateRuViewConfig(invalid, false) == nil {
			t.Errorf("accepted explicit invalid value %#v", invalid)
		}
	}
}

// Config.Save writes the whole ruview section. A Config that was not built by
// Load carries empty RuView values; reloading that file must select defaults.
func TestLoadRuViewWithoutExplicitModeRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  ui_language: en\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{}).Save(path); err != nil {
		t.Fatalf("Save(empty config): %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !yamlHasPath(raw, "ruview", "mode") {
		t.Fatalf("fixture did not write ruview.mode:\n%s", raw)
	}
	want := DefaultRuViewConfig()
	for _, step := range []string{"first load", "reload after save"} {
		cfg, err := Load(path)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		got := cfg.RuView
		if got.Mode != want.Mode || got.Source != want.Source || got.HTTPPort != want.HTTPPort || got.UDPPort != want.UDPPort || got.StorageMB != want.StorageMB || got.UDPAllow == nil {
			t.Fatalf("%s: ruview = %#v, want the defaults of %#v", step, got, want)
		}
		if err := cfg.Save(path); err != nil {
			t.Fatalf("%s: Save: %v", step, err)
		}
	}
}

func TestLoadRuViewAcceptsPaddedMixedCaseModeAndSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ruview:\n  mode: \" External \"\n  source: WIFI\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load(padded mixed-case values): %v", err)
	}
	if cfg.RuView.Mode != "external" || cfg.RuView.Source != "wifi" {
		t.Fatalf("mode/source = %q/%q, want external/wifi", cfg.RuView.Mode, cfg.RuView.Source)
	}
}

func TestRuViewDefaultsAndNetworkBoundary(t *testing.T) {
	c := DefaultRuViewConfig()
	if c.Enabled || c.AgentAccess || !c.ReadOnly {
		t.Fatal("unsafe defaults")
	}
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"0.0.0.0", "127.0.0.1", "169.254.169.254", "8.8.8.8", "::1", "10.1.2.3/4"} {
		c.UDPEnabled = true
		c.UDPBind = address
		c.UDPAllow = []string{"10.1.2.3"}
		if ValidateRuViewConfig(c, false) == nil {
			t.Errorf("accepted bind %s", address)
		}
	}
	c.UDPBind = "192.168.1.2"
	c.UDPAllow = []string{"192.168.0.0/16"}
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	c.UDPAllow = []string{"10.0.0.0/7"}
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("accepted oversized CIDR")
	}
	c = DefaultRuViewConfig()
	c.Mode = "external"
	c.Enabled = true
	c.URL = "http://127.0.0.1:3022"
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("implicit LAN grant")
	}
	c.AllowPrivate = true
	if err := ValidateRuViewConfig(c, false); err != nil {
		t.Fatal(err)
	}
	c.URL = "http://user:secret@example.com"
	if ValidateRuViewConfig(c, false) == nil {
		t.Fatal("credentials in URL")
	}
}
