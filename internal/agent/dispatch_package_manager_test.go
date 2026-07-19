package agent

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func enabledPackageManagerConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Agent.AllowPackageManager = true
	cfg.Agent.SudoEnabled = true
	cfg.Agent.SudoUnrestricted = true
	cfg.PackageManager.Enabled = true
	cfg.PackageManager.AutoDetect = false
	cfg.PackageManager.Override = "apt"
	cfg.PackageManager.AllowInstall = true
	cfg.PackageManager.AllowRemove = true
	cfg.PackageManager.AllowUpgrade = true
	return cfg
}

func packageManagerCall(operation string) ToolCall {
	return ToolCall{
		Action: "package_manager",
		Params: map[string]interface{}{"operation": operation},
	}
}

func TestDispatchPackageManagerPrivilegeGuards(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*config.Config)
		wantSubstr string
	}{
		{
			name: "sudo disabled",
			configure: func(cfg *config.Config) {
				cfg.Agent.SudoEnabled = false
			},
			wantSubstr: "agent.sudo_enabled=true",
		},
		{
			name: "no new privileges",
			configure: func(cfg *config.Config) {
				cfg.Runtime.NoNewPrivileges = true
			},
			wantSubstr: "no-new-privileges is active",
		},
		{
			name: "system writes disabled",
			configure: func(cfg *config.Config) {
				cfg.Agent.SudoUnrestricted = false
			},
			wantSubstr: "agent.sudo_unrestricted=true",
		},
		{
			name: "protect system strict",
			configure: func(cfg *config.Config) {
				cfg.Runtime.ProtectSystemStrict = true
			},
			wantSubstr: "ProtectSystem=strict",
		},
		{
			name:       "vault unavailable",
			configure:  func(*config.Config) {},
			wantSubstr: "vault is not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := enabledPackageManagerConfig()
			tt.configure(cfg)

			got := dispatchPackageManager(packageManagerCall("install"), &DispatchContext{Cfg: cfg})
			if !strings.Contains(got, tt.wantSubstr) {
				t.Fatalf("dispatchPackageManager() = %q, want substring %q", got, tt.wantSubstr)
			}
		})
	}
}

func TestDispatchPackageManagerRejectsDisabledTool(t *testing.T) {
	cfg := enabledPackageManagerConfig()
	cfg.PackageManager.Enabled = false

	got := dispatchPackageManager(packageManagerCall("detect"), &DispatchContext{Cfg: cfg})
	if !strings.Contains(got, "package_manager is disabled") {
		t.Fatalf("dispatchPackageManager() = %q, want disabled-tool guidance", got)
	}
}

func TestDispatchPackageManagerDetectRemainsAvailableWithSystemWriteRestrictions(t *testing.T) {
	cfg := enabledPackageManagerConfig()
	cfg.Agent.SudoEnabled = false
	cfg.Agent.SudoUnrestricted = false
	cfg.Runtime.NoNewPrivileges = true
	cfg.Runtime.ProtectSystemStrict = true

	got := dispatchPackageManager(packageManagerCall("detect"), &DispatchContext{Cfg: cfg})
	for _, want := range []string{`"status":"success"`, `"manager":"apt"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("dispatchPackageManager() = %q, want substring %q", got, want)
		}
	}
}

func TestPackageManagerToolRequiresBothConfigGates(t *testing.T) {
	tests := []struct {
		name         string
		allowAgent   bool
		enableTool   bool
		wantIncluded bool
	}{
		{name: "both disabled"},
		{name: "agent gate only", allowAgent: true},
		{name: "integration gate only", enableTool: true},
		{name: "both enabled", allowAgent: true, enableTool: true, wantIncluded: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Agent.AllowPackageManager = tt.allowAgent
			cfg.PackageManager.Enabled = tt.enableTool

			got := containsName(ToolNamesFromConfig(cfg), "package_manager")
			if got != tt.wantIncluded {
				t.Fatalf("package_manager included = %v, want %v", got, tt.wantIncluded)
			}
		})
	}
}

func TestDispatchSudoReportsProtectSystemRestrictionBeforeVaultLookup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.SudoEnabled = true
	cfg.Agent.SudoUnrestricted = true
	cfg.Runtime.ProtectSystemStrict = true

	got := dispatchShell(ToolCall{
		Action: "execute_sudo",
		Params: map[string]interface{}{"command": "true"},
	}, &DispatchContext{Cfg: cfg})
	if !strings.Contains(got, "ProtectSystem=strict") {
		t.Fatalf("dispatchShell() = %q, want ProtectSystem guidance", got)
	}
}
