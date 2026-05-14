package tsnetnode

import (
	"testing"

	"aurago/internal/config"
)

func TestManifestProxyTarget(t *testing.T) {
	cfg := &config.Config{}
	cfg.Manifest.Port = 2099
	cfg.Manifest.HostPort = 2099

	cfg.Runtime.IsDocker = false
	if got := manifestProxyTarget(cfg); got != "http://127.0.0.1:2099" {
		t.Fatalf("manifestProxyTarget(host) = %q, want loopback target", got)
	}

	cfg.Runtime.IsDocker = true
	if got := manifestProxyTarget(cfg); got != "http://manifest:2099" {
		t.Fatalf("manifestProxyTarget(docker) = %q, want Docker service target", got)
	}

	cfg.Runtime.IsDocker = false
	cfg.Manifest.HostPort = 3109
	if got := manifestProxyTarget(cfg); got != "http://127.0.0.1:3109" {
		t.Fatalf("manifestProxyTarget(custom host port) = %q, want published host port", got)
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

func TestManifestTsNetPortUsesHTTPSDefault(t *testing.T) {
	tests := []struct {
		name string
		port int
		want int
	}{
		{name: "empty config", port: 0, want: 443},
		{name: "legacy default", port: 8444, want: 443},
		{name: "custom port", port: 2099, want: 2099},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := manifestTsNetPort(tt.port); got != tt.want {
				t.Fatalf("manifestTsNetPort(%d) = %d, want %d", tt.port, got, tt.want)
			}
		})
	}
}
