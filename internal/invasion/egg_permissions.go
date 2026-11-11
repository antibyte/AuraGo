package invasion

import (
	"encoding/json"
	"fmt"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

// EggPermissions is the complete, non-secret authority of one applied revision.
type EggPermissions struct {
	Version               int      `json:"security_policy_version"`
	AllowedTools          []string `json:"allowed_tools"`
	AllowFilesystemWrite  bool     `json:"allow_filesystem_write"`
	AllowNetworkRequests  bool     `json:"allow_network_requests"`
	AllowUnsandboxedShell bool     `json:"allow_unsandboxed_shell"`
	AllowRemoteShell      bool     `json:"allow_remote_shell"`
	AllowSelfUpdate       bool     `json:"allow_self_update"`
}

func ValidateEggPermissions(egg EggRecord) error {
	selected, err := parseEggAllowedTools(egg.AllowedTools)
	if err != nil {
		return err
	}
	if egg.AllowUnsandboxedShell {
		for _, tool := range selected {
			if tool == "shell" || tool == "execute_shell_command" {
				return nil
			}
		}
		return fmt.Errorf("unsandboxed shell requires an explicit shell tool grant")
	}
	return nil
}

func EggPermissionsJSON(data []byte) (string, error) {
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	if !cfg.EggMode.Enabled {
		return "", fmt.Errorf("not an Egg configuration")
	}
	if err := ValidateEggPolicy(&cfg); err != nil {
		return "", err
	}
	p := EggPermissions{Version: EggSecurityPolicyVersion, AllowedTools: []string{}, AllowFilesystemWrite: cfg.Agent.AllowFilesystemWrite, AllowNetworkRequests: cfg.Agent.AllowNetworkRequests, AllowUnsandboxedShell: cfg.Agent.AllowUnsandboxedShell, AllowRemoteShell: cfg.Agent.AllowRemoteShell, AllowSelfUpdate: cfg.Agent.AllowSelfUpdate}
	if cfg.Agent.AllowShell {
		p.AllowedTools = append(p.AllowedTools, "shell")
	}
	if cfg.Agent.AllowPython {
		p.AllowedTools = append(p.AllowedTools, "python")
	}
	encoded, err := json.Marshal(p)
	return string(encoded), err
}

func ApplyEggPermissions(data []byte, snapshot string) ([]byte, error) {
	var p EggPermissions
	if err := json.Unmarshal([]byte(snapshot), &p); err != nil || p.Version != EggSecurityPolicyVersion || p.AllowedTools == nil {
		return nil, fmt.Errorf("revision has no verified security permissions; review rights and reconfigure the Egg")
	}
	return ApplySafeConfigPatch(data, SafeConfigPatch{AllowedTools: p.AllowedTools, AllowFilesystemWrite: &p.AllowFilesystemWrite, AllowNetworkRequests: &p.AllowNetworkRequests, AllowUnsandboxedShell: &p.AllowUnsandboxedShell, AllowRemoteShell: &p.AllowRemoteShell, AllowSelfUpdate: &p.AllowSelfUpdate})
}
