package tsnetnode

import (
	"testing"

	"aurago/internal/config"
)

func TestManifestProxyTarget(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Port = 2099

	cfg.Runtime.IsDocker = false
	if got := manifestProxyTarget(cfg); got != "http://127.0.0.1:2099" {
		t.Fatalf("manifestProxyTarget(host) = %q, want loopback target", got)
	}

	cfg.Runtime.IsDocker = true
	if got := manifestProxyTarget(cfg); got != "http://manifest:2099" {
		t.Fatalf("manifestProxyTarget(docker) = %q, want Docker service target", got)
	}
}

func TestManifestHostnameDefault(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tailscale.TsNet.Hostname = "aurago"
	m := NewManager(cfg, nil)

	if got := m.effectiveManifestHostname(); got != "aurago-manifest" {
		t.Fatalf("effectiveManifestHostname() = %q, want aurago-manifest", got)
	}

	cfg.Tailscale.TsNet.ManifestHostname = "custom-manifest"
	if got := m.effectiveManifestHostname(); got != "custom-manifest" {
		t.Fatalf("effectiveManifestHostname() = %q, want custom-manifest", got)
	}
}
