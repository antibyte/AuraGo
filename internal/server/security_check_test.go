package server

import (
	"aurago/internal/config"
	"testing"
)

func TestCheckSecurityAddsPhase2Hints(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Auth.Enabled = false
	cfg.CloudflareTunnel.Enabled = true
	cfg.CloudflareTunnel.ExposeWebUI = true

	hints := CheckSecurity(cfg)
	if !hasSecurityHint(hints, "python_no_sandbox") {
		t.Fatalf("expected python_no_sandbox hint, got %#v", hints)
	}
	if !hasSecurityHint(hints, "mcp_public") {
		t.Fatalf("expected mcp_public hint, got %#v", hints)
	}
	if !hasSecurityHint(hints, "critical_public_exposure") {
		t.Fatalf("expected critical_public_exposure hint, got %#v", hints)
	}
}

func TestCheckSecuritySkipsPythonNoSandboxWhenSandboxReady(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Sandbox.Enabled = true
	cfg.Runtime.IsDocker = false

	hints := CheckSecurity(cfg)
	if hasSecurityHint(hints, "python_no_sandbox") {
		t.Fatalf("did not expect python_no_sandbox hint, got %#v", hints)
	}
}

func TestCheckSecurityDoesNotTreatLANHTTPSTailnetAsInternetFacing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.CertMode = "selfsigned"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ServeHTTP = true
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "hash"
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = false
	cfg.Agent.AllowSelfUpdate = true

	if isInternetFacing(cfg) {
		t.Fatal("LAN HTTPS plus tailnet serve should not count as public internet exposure")
	}
	if !isNetworkFacing(cfg) {
		t.Fatal("LAN HTTPS plus tailnet serve should still count as network-facing")
	}

	hints := CheckSecurity(cfg)
	for _, id := range []string{"docker_enabled_no_readonly", "self_update_public", "totp_disabled"} {
		if hasSecurityHint(hints, id) {
			t.Fatalf("did not expect %s for LAN/tailnet-only instance, got %#v", id, hints)
		}
	}
}

func TestCheckSecurityTreatsTailscaleFunnelAsInternetFacing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.CertMode = "selfsigned"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ServeHTTP = true
	cfg.Tailscale.TsNet.Funnel = true
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "hash"
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = false
	cfg.Agent.AllowSelfUpdate = true

	if !isInternetFacing(cfg) {
		t.Fatal("Tailscale Funnel should count as public internet exposure")
	}

	hints := CheckSecurity(cfg)
	for _, id := range []string{"docker_enabled_no_readonly", "self_update_public"} {
		if !hasSecurityHint(hints, id) {
			t.Fatalf("expected %s for Tailscale Funnel exposure, got %#v", id, hints)
		}
	}
}

func TestPublicHostnameLikelyTreatsTailnetAndPrivateHostsAsNonPublic(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"aurago", "aurago.local", "aurago.tailnet-name.ts.net", "192.168.6.238", "100.100.100.100"} {
		if publicHostnameLikely(host) {
			t.Fatalf("host %q should not look public", host)
		}
	}
	for _, host := range []string{"aurago.my-domain.com", "8.8.8.8"} {
		if !publicHostnameLikely(host) {
			t.Fatalf("host %q should look public", host)
		}
	}
}

func hasSecurityHint(hints []SecurityHint, id string) bool {
	for _, hint := range hints {
		if hint.ID == id {
			return true
		}
	}
	return false
}
