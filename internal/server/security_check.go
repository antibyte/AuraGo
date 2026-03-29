package server

import (
	"fmt"
	"os"
	"strings"

	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

// Severity levels for security hints.
const (
	SevCritical = "critical"
	SevWarning  = "warning"
	SevInfo     = "info"
)

// SecurityHint describes a single security issue found in the current configuration.
// AutoFixable hints carry a FixPatch that ApplyHardening can merge into config.yaml.
type SecurityHint struct {
	ID          string                 `json:"id"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	AutoFixable bool                   `json:"auto_fixable"`
	FixPatch    map[string]interface{} `json:"-"` // applied server-side only, never serialised
}

// isInternetFacing returns true when the server is reachable from outside localhost.
// This influences whether auth_disabled is critical vs. just a warning.
func isInternetFacing(cfg *config.Config) bool {
	if cfg.Server.HTTPS.Enabled {
		return true
	}
	h := cfg.Server.Host
	if h == "0.0.0.0" || h == "" {
		return true
	}
	if cfg.Tailscale.TsNet.Enabled {
		return true
	}
	return false
}

// CheckSecurity evaluates the current config and returns a list of security hints.
// The returned slice is ordered from most to least severe.
func CheckSecurity(cfg *config.Config) []SecurityHint {
	var hints []SecurityHint
	facing := isInternetFacing(cfg)
	dangerCount := countDangerZoneCapabilities(cfg)

	// 1. auth_disabled
	if !cfg.Auth.Enabled {
		sev := SevWarning
		desc := "Authentication is disabled. Any visitor with network access can use the agent."
		if facing {
			sev = SevCritical
			desc = "Authentication is disabled but the server is reachable from the internet. " +
				"Anyone can read your conversations, execute tools, and access all integrations."
		}
		hints = append(hints, SecurityHint{
			ID: "auth_disabled", Severity: sev,
			Title:       "Authentication disabled",
			Description: desc,
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"auth": map[string]interface{}{"enabled": true}},
		})
	}

	// 2. no_password — auth enabled but no password hash configured
	if cfg.Auth.Enabled && cfg.Auth.PasswordHash == "" {
		hints = append(hints, SecurityHint{
			ID: "no_password", Severity: SevCritical,
			Title: "No password configured",
			Description: "Login guard is enabled but no password has been set. " +
				"Authentication is effectively bypassed — set a password in the Login Guard section.",
			AutoFixable: false,
		})
	}

	// 2b. critical_public_exposure — public instance with weak perimeter and multiple powerful capabilities
	if facing &&
		(!cfg.Server.HTTPS.Enabled || !cfg.Auth.Enabled || cfg.Auth.PasswordHash == "") &&
		dangerCount >= 2 {
		hints = append(hints, SecurityHint{
			ID: "critical_public_exposure", Severity: SevCritical,
			Title: "Public instance is broadly exposed",
			Description: "The server is reachable beyond localhost, key perimeter protections are missing, " +
				"and multiple high-risk capabilities are enabled. Lock down access before using AuraGo on this network.",
			AutoFixable: false,
		})
	}

	// 3. no_https — internet-facing without TLS
	if facing && !cfg.Server.HTTPS.Enabled &&
		cfg.Server.Host != "127.0.0.1" && cfg.Server.Host != "localhost" {
		hints = append(hints, SecurityHint{
			ID: "no_https", Severity: SevWarning,
			Title: "No HTTPS configured",
			Description: "The server appears to be accessible from outside localhost but is running " +
				"plain HTTP. Credentials and session cookies are transmitted unencrypted.",
			AutoFixable: false,
		})
	}

	// 4. n8n_no_token — n8n endpoint without Bearer token
	if cfg.N8n.Enabled && !cfg.N8n.RequireToken {
		hints = append(hints, SecurityHint{
			ID: "n8n_no_token", Severity: SevCritical,
			Title: "n8n API: token authentication disabled",
			Description: "The n8n integration endpoint is enabled without Bearer-token authentication. " +
				"Any caller can trigger agent tasks, read memories, and create missions.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"n8n": map[string]interface{}{"require_token": true}},
		})
	}

	// 5. a2a_no_auth — A2A dedicated server port open without any auth
	if cfg.A2A.Server.Port > 0 && !cfg.A2A.Auth.APIKeyEnabled && !cfg.A2A.Auth.BearerEnabled {
		hints = append(hints, SecurityHint{
			ID: "a2a_no_auth", Severity: SevWarning,
			Title: "A2A server running without authentication",
			Description: fmt.Sprintf(
				"The Agent-to-Agent server is listening on a dedicated port (%d) with no API key or Bearer token configured.",
				cfg.A2A.Server.Port,
			),
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"a2a": map[string]interface{}{"auth": map[string]interface{}{"api_key_enabled": true}}},
		})
	}

	// 6. insecure_ssh_key — SSH host key verification disabled
	if cfg.RemoteControl.SSHInsecureHostKey {
		hints = append(hints, SecurityHint{
			ID: "insecure_ssh_key", Severity: SevWarning,
			Title: "SSH host key verification disabled",
			Description: "remote_control.ssh_insecure_host_key is true. SSH connections accept any host key — " +
				"this enables man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"remote_control": map[string]interface{}{"ssh_insecure_host_key": false}},
		})
	}

	// 7. danger_all_public — all danger-zone capabilities enabled on a public server
	if facing &&
		cfg.Agent.AllowShell &&
		cfg.Agent.AllowPython &&
		cfg.Agent.AllowFilesystemWrite &&
		cfg.Agent.AllowNetworkRequests &&
		cfg.Agent.AllowRemoteShell {
		hints = append(hints, SecurityHint{
			ID: "danger_all_public", Severity: SevWarning,
			Title: "All danger-zone capabilities enabled on public server",
			Description: "Shell execution, Python, filesystem write, network requests, and remote shell are all enabled. " +
				"Consider disabling capabilities you don't need and enabling the LLM Guardian.",
			AutoFixable: false,
		})
	}

	// 7b. python_no_sandbox — Python is enabled without an effective sandbox backend
	if cfg.Agent.AllowPython && !pythonSandboxReady(cfg) {
		sev := SevWarning
		if facing {
			sev = SevCritical
		}
		hints = append(hints, SecurityHint{
			ID: "python_no_sandbox", Severity: sev,
			Title: "Python execution without effective sandbox",
			Description: "Python execution is enabled, but the isolated sandbox backend is not effectively available. " +
				"Enable the sandbox or disable Python execution for this environment.",
			AutoFixable: false,
		})
	}

	// 7c. mcp_public — MCP exposed on an internet-facing instance
	if facing && cfg.Agent.AllowMCP && cfg.MCP.Enabled {
		hints = append(hints, SecurityHint{
			ID: "mcp_public", Severity: SevWarning,
			Title: "MCP enabled on an externally reachable instance",
			Description: "Model Context Protocol is enabled while the server is reachable beyond localhost. " +
				"Expose it only behind authentication and transport security, or disable MCP if not needed.",
			AutoFixable: false,
		})
	}

	// 8. meshcentral_insecure — TLS verification disabled
	if cfg.MeshCentral.Enabled && cfg.MeshCentral.Insecure {
		hints = append(hints, SecurityHint{
			ID: "meshcentral_insecure", Severity: SevWarning,
			Title: "MeshCentral: TLS verification disabled",
			Description: "meshcentral.insecure is true. TLS certificate errors are ignored, " +
				"enabling potential man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"meshcentral": map[string]interface{}{"insecure": false}},
		})
	}

	// 9. proxmox_insecure — TLS verification disabled
	if cfg.Proxmox.Enabled && cfg.Proxmox.Insecure {
		hints = append(hints, SecurityHint{
			ID: "proxmox_insecure", Severity: SevWarning,
			Title: "Proxmox: TLS verification disabled",
			Description: "proxmox.insecure is true. TLS certificate errors are ignored, " +
				"enabling potential man-in-the-middle attacks.",
			AutoFixable: true,
			FixPatch:    map[string]interface{}{"proxmox": map[string]interface{}{"insecure": false}},
		})
	}

	// 10. totp_disabled — internet-facing with password-only auth
	if cfg.Auth.Enabled && cfg.Auth.PasswordHash != "" && !cfg.Auth.TOTPEnabled && facing {
		hints = append(hints, SecurityHint{
			ID: "totp_disabled", Severity: SevInfo,
			Title: "Two-factor authentication not enabled",
			Description: "The server is internet-facing but only password authentication is active. " +
				"Enable TOTP 2FA in the Login Guard section.",
			AutoFixable: false,
		})
	}

	return hints
}

// ApplyHardening merges the fix patches for the given hint IDs into config.yaml and hot-reloads.
// Returns the list of hint IDs that were actually applied.
func ApplyHardening(s *Server, ids []string) ([]string, error) {
	wantedSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		wantedSet[id] = true
	}

	s.CfgMu.RLock()
	hints := CheckSecurity(s.Cfg)
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	combined := map[string]interface{}{}
	applied := []string{}
	for _, h := range hints {
		if !h.AutoFixable || !wantedSet[h.ID] || len(h.FixPatch) == 0 {
			continue
		}
		deepMerge(combined, h.FixPatch, "")
		applied = append(applied, h.ID)
	}
	if len(applied) == 0 {
		return nil, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if rawCfg == nil {
		rawCfg = map[string]interface{}{}
	}
	deepMerge(rawCfg, combined, "")

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	// Validate before writing
	var validateCfg config.Config
	if err := yaml.Unmarshal(out, &validateCfg); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Hot-reload: use the same pattern as handleUpdateConfig
	s.CfgMu.Lock()
	if newCfg, loadErr := config.Load(configPath); loadErr == nil {
		newCfg.ApplyVaultSecrets(s.Vault)
		newCfg.ResolveProviders()
		newCfg.ApplyOAuthTokens(s.Vault)
		newCfg.Runtime = s.Cfg.Runtime
		savedPath := s.Cfg.ConfigPath
		*s.Cfg = *newCfg
		s.Cfg.ConfigPath = savedPath
	}
	s.CfgMu.Unlock()

	s.Logger.Info("[Security] Hardening applied", "ids", strings.Join(applied, ", "))
	return applied, nil
}

func countDangerZoneCapabilities(cfg *config.Config) int {
	count := 0
	for _, enabled := range []bool{
		cfg.Agent.AllowShell,
		cfg.Agent.AllowPython,
		cfg.Agent.AllowFilesystemWrite,
		cfg.Agent.AllowNetworkRequests,
		cfg.Agent.AllowRemoteShell,
		cfg.Agent.AllowSelfUpdate,
		cfg.Agent.AllowMCP && cfg.MCP.Enabled,
		cfg.Agent.SudoEnabled,
	} {
		if enabled {
			count++
		}
	}
	return count
}

func pythonSandboxReady(cfg *config.Config) bool {
	if !cfg.Sandbox.Enabled {
		return false
	}
	if cfg.Runtime.IsDocker && !cfg.Runtime.DockerSocketOK {
		return false
	}
	return true
}
