package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloudflarePortsValidateBeforeNormalization(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(fmt.Sprintf("cloudflare_tunnel:\n  metrics_port: %d\n", port)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "metrics_port") {
			t.Fatalf("invalid port %d accepted: %v", port, err)
		}
	}
	for _, port := range []int{0, 1, 80, 65535} {
		if err := ValidateCloudflareTunnelPorts(port, 0, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidateCloudflareTunnelPorts(18080, 18080, nil); err == nil {
		t.Fatal("shared port accepted")
	}
}

func TestCloudflarePortsFollowActiveListenersAndMissingDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Homepage.WebServerPort = 3000
	cfg.CloudflareTunnel.MetricsPort = 3000
	if err := ValidateCloudflareTunnelConfig(cfg); err != nil {
		t.Fatal("inactive Homepage was treated as a listener", err)
	}
	cfg.Homepage.WebServerEnabled = true
	if err := ValidateCloudflareTunnelConfig(cfg); err == nil {
		t.Fatal("active Homepage conflict missed")
	}
	cfg.CloudflareTunnel.MetricsPort = 8080
	if err := ValidateCloudflareTunnelConfig(cfg); err == nil {
		t.Fatal("HTTP conflict missed")
	}
	cfg.Server.HTTPS.Enabled = true
	if err := ValidateCloudflareTunnelConfig(cfg); err == nil {
		t.Fatal("implicit internal loopback conflict missed")
	}
	cfg.CloudflareTunnel.LoopbackPort = 18080
	cfg.CloudflareTunnel.MetricsPort = 443
	if err := ValidateCloudflareTunnelConfig(cfg); err == nil {
		t.Fatal("default HTTPS conflict missed")
	}
	cfg.Server.HTTPS.HTTPPort = 80
	cfg.CloudflareTunnel.MetricsPort = 80
	if err := ValidateCloudflareTunnelConfig(cfg); err == nil {
		t.Fatal("HTTP redirect conflict missed")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("cloudflare_tunnel:\n  enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CloudflareTunnel.AutoStart || loaded.CloudflareTunnel.ExposeWebUI || loaded.CloudflareTunnel.ExposeHomepage {
		t.Fatal("missing boolean grants became enabled")
	}
}
