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

func hasSecurityHint(hints []SecurityHint, id string) bool {
	for _, hint := range hints {
		if hint.ID == id {
			return true
		}
	}
	return false
}
