package server

import (
	"aurago/internal/config"
	"strings"
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

func TestCheckSecurityNoPasswordTextSaysLocked(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "no_password")
	if hint == nil {
		t.Fatalf("expected no_password hint, got %#v", hints)
	}
	if strings.Contains(strings.ToLower(hint.Description), "bypassed") {
		t.Fatalf("no_password hint should not claim auth is bypassed: %q", hint.Description)
	}
	if !strings.Contains(strings.ToLower(hint.Description), "locked") {
		t.Fatalf("no_password hint should explain access is locked: %q", hint.Description)
	}
}

func TestCheckSecurityWarnsWhenLLMGuardianProviderMissing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.LLMGuardian.Enabled = true
	cfg.LLMGuardian.Provider = "missing-guardian"
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai"}}

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "llm_guardian_provider_missing")
	if hint == nil {
		t.Fatalf("expected llm_guardian_provider_missing hint, got %#v", hints)
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if !strings.Contains(strings.ToLower(hint.Description), "fallback") {
		t.Fatalf("description should mention fallback behavior: %q", hint.Description)
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
	for _, id := range []string{"self_update_public", "totp_disabled"} {
		if hasSecurityHint(hints, id) {
			t.Fatalf("did not expect %s for LAN/tailnet-only instance, got %#v", id, hints)
		}
	}
	if !hasSecurityHint(hints, "docker_enabled_no_readonly") {
		t.Fatalf("expected Docker write-access warning for network-facing LAN/tailnet instance, got %#v", hints)
	}
}

func TestIsNetworkFacingIncludesTailscaleManifestExposure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ExposeManifest = true

	if !isNetworkFacing(cfg) {
		t.Fatal("Tailscale Manifest exposure should count as network-facing")
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
	return findSecurityHint(hints, id) != nil
}

func findSecurityHint(hints []SecurityHint, id string) *SecurityHint {
	for _, hint := range hints {
		if hint.ID == id {
			return &hint
		}
	}
	return nil
}
