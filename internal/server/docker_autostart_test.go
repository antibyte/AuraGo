package server

import (
	"testing"

	"aurago/internal/config"
)

func TestDockerBackedAutoStartsRequireDockerEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Homepage.Enabled = true
	cfg.Homepage.WebServerEnabled = true
	cfg.Homepage.WorkspacePath = t.TempDir()
	cfg.SecurityProxy.Enabled = true

	if homepageDevAutoStartAllowed(cfg) {
		t.Fatal("homepage dev auto-start should be skipped when Docker is disabled")
	}
	if homepageWebServerAutoStartAllowed(cfg) {
		t.Fatal("homepage web server auto-start should be skipped when Docker is disabled")
	}
	if securityProxyAutoStartAllowed(cfg) {
		t.Fatal("security proxy auto-start should be skipped when Docker is disabled")
	}

	cfg.Docker.Enabled = true
	if !homepageDevAutoStartAllowed(cfg) {
		t.Fatal("homepage dev auto-start should be allowed when Docker is enabled")
	}
	if !homepageWebServerAutoStartAllowed(cfg) {
		t.Fatal("homepage web server auto-start should be allowed when Docker is enabled")
	}
	if !securityProxyAutoStartAllowed(cfg) {
		t.Fatal("security proxy auto-start should be allowed when Docker is enabled")
	}
}

func TestCloudflareTunnelAutoStartAvoidsDockerWhenDockerDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.CloudflareTunnel.Enabled = true
	cfg.CloudflareTunnel.AutoStart = true
	cfg.CloudflareTunnel.Mode = "docker"

	if cloudflareTunnelAutoStartAllowed(cfg) {
		t.Fatal("Cloudflare Docker tunnel auto-start should be skipped when Docker is disabled")
	}

	cfg.CloudflareTunnel.Mode = "auto"
	if !cloudflareTunnelAutoStartAllowed(cfg) {
		t.Fatal("Cloudflare auto mode should still be allowed so native mode can be used")
	}
	tunnelCfg := cloudflareTunnelRuntimeConfig(cfg)
	if tunnelCfg.Mode != "native" {
		t.Fatalf("Cloudflare auto mode with Docker disabled resolved to %q, want native", tunnelCfg.Mode)
	}

	cfg.Docker.Enabled = true
	tunnelCfg = cloudflareTunnelRuntimeConfig(cfg)
	if tunnelCfg.Mode != "auto" {
		t.Fatalf("Cloudflare auto mode with Docker enabled resolved to %q, want auto", tunnelCfg.Mode)
	}
}
