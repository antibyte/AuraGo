package invasion

import (
	"aurago/internal/config"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// allowedToolsWhitelist is the set of tool IDs that can be used in safe reconfigure patches.
var allowedToolsWhitelist = map[string]bool{
	"shell":                 true,
	"execute_shell_command": true,
	"python":                true,
	"python_execute":        true,
}

// GenerateEggConfig produces a minimal config.yaml for a deployed egg worker.
// It derives settings from the master config and the egg/nest records.
func GenerateEggConfig(masterCfg *config.Config, egg EggRecord, nest NestRecord, sharedKey, masterURL, eggMasterKey string) ([]byte, error) {
	cfg := make(map[string]interface{})

	// ── Server ──
	cfg["server"] = map[string]interface{}{
		"host":       "0.0.0.0",
		"port":       egg.EggPort,
		"master_key": eggMasterKey,
	}

	// ── LLM — either inherit from master or use egg's own ──
	if egg.InheritLLM {
		// SECURITY NOTE: The master's API key is included in the config YAML.
		// For Docker deployments the config is copied into the container as a file
		// (not an env var) to prevent exposure via "docker inspect". For SSH
		// deployments the config is written with restricted permissions on the
		// remote host. The egg host must be considered trusted — a compromised
		// egg can extract the key from its config file.
		cfg["llm"] = map[string]interface{}{
			"provider":             masterCfg.LLM.ProviderType,
			"base_url":             masterCfg.LLM.BaseURL,
			"api_key":              masterCfg.LLM.APIKey,
			"model":                masterCfg.LLM.Model,
			"use_native_functions": masterCfg.LLM.UseNativeFunctions,
			"temperature":          masterCfg.LLM.Temperature,
			"structured_outputs":   masterCfg.LLM.StructuredOutputs,
		}
	} else {
		llmSection := map[string]interface{}{
			"provider":             egg.Provider,
			"base_url":             egg.BaseURL,
			"model":                egg.Model,
			"use_native_functions": true,
			"temperature":          0.7,
			"structured_outputs":   false,
		}
		// API key will be empty here — it must be injected from vault or sent via secret channel
		if egg.APIKeyRef != "" {
			llmSection["api_key"] = "" // placeholder, egg will use vault
		}
		cfg["llm"] = llmSection
	}

	// ── Directories (standard paths) ──
	cfg["directories"] = map[string]interface{}{
		"data_dir":      "./data",
		"workspace_dir": "./agent_workspace",
		"tools_dir":     "./agent_workspace/tools",
		"prompts_dir":   "./prompts",
		"skills_dir":    "./agent_workspace/skills",
		"vectordb_dir":  "./data/vectordb",
	}

	// ── SQLite ──
	cfg["sqlite"] = map[string]interface{}{
		"short_term_path": "./data/short_term.db",
		"long_term_path":  "./data/long_term.db",
		"inventory_path":  "./data/inventory.db",
		"invasion_path":   "./data/invasion.db",
	}

	// ── Agent — worker mode, no personality ──
	// Derive permission flags from the egg's allowed_tools configuration.
	// If no tools are specified (empty = default set), grant shell and python.
	allowShell := true
	allowPython := true
	if egg.AllowedTools != "" {
		// Parse the JSON array of allowed tool IDs
		var tools []string
		if err := json.Unmarshal([]byte(egg.AllowedTools), &tools); err == nil && len(tools) > 0 {
			allowShell = false
			allowPython = false
			for _, t := range tools {
				switch strings.TrimSpace(t) {
				case "shell", "execute_shell_command":
					allowShell = true
				case "python", "python_execute":
					allowPython = true
				}
			}
		}
	}
	cfg["agent"] = map[string]interface{}{
		"system_language":            masterCfg.Agent.SystemLanguage,
		"personality_engine":         false,
		"personality_engine_v2":      false,
		"core_personality":           "neutral",
		"system_prompt_token_budget": 6000,
		"context_window":             masterCfg.Agent.ContextWindow,
		"show_tool_results":          true,
		"workflow_feedback":          false,
		"debug_mode":                 false,
		"user_profiling":             false,
		"allow_shell":                allowShell,
		"allow_python":               allowPython,
		"allow_filesystem_write":     true,
		"allow_network_requests":     true,
		"allow_remote_shell":         false,
		"allow_self_update":          false,
		"sudo_enabled":               false,
	}

	// ── Google Workspace — disabled for eggs ──
	cfg["google_workspace"] = map[string]interface{}{
		"enabled": false,
	}

	// ── Circuit Breaker ──
	cfg["circuit_breaker"] = map[string]interface{}{
		"max_tool_calls":      15,
		"llm_timeout_seconds": 600,
		"retry_intervals":     []string{"10s", "2m", "10m"},
	}

	// ── Disable all integrations ──
	cfg["telegram"] = map[string]interface{}{"telegram_user_id": 0}
	cfg["discord"] = map[string]interface{}{"enabled": false}
	cfg["email"] = map[string]interface{}{"enabled": false}
	cfg["home_assistant"] = map[string]interface{}{"enabled": false}
	cfg["docker"] = map[string]interface{}{"enabled": false}
	cfg["chromecast"] = map[string]interface{}{"enabled": false}
	cfg["co_agents"] = map[string]interface{}{"enabled": false}
	cfg["invasion_control"] = map[string]interface{}{"enabled": false}
	cfg["webdav"] = map[string]interface{}{"enabled": false}
	cfg["koofr"] = map[string]interface{}{"enabled": false}
	cfg["rocketchat"] = map[string]interface{}{"enabled": false}
	cfg["webhooks"] = map[string]interface{}{"enabled": false}
	cfg["proxmox"] = map[string]interface{}{"enabled": false}
	cfg["ollama"] = map[string]interface{}{"enabled": false}
	cfg["tailscale"] = map[string]interface{}{"enabled": false}
	cfg["ansible"] = map[string]interface{}{"enabled": false}

	// ── Web Config (minimal, for API access from master) ──
	cfg["web_config"] = map[string]interface{}{"enabled": false}

	// ── Auth (disabled — egg uses shared key auth) ──
	cfg["auth"] = map[string]interface{}{"enabled": false}

	// ── Logging ──
	cfg["logging"] = map[string]interface{}{
		"log_dir":         "./log",
		"enable_file_log": true,
	}

	// ── Maintenance ──
	cfg["maintenance"] = map[string]interface{}{
		"enabled":          false,
		"lifeboat_enabled": false,
	}

	// ── EggMode (the critical section) ──
	eggModeCfg := map[string]interface{}{
		"enabled":    true,
		"master_url": masterURL,
		"shared_key": sharedKey,
		"egg_id":     egg.ID,
		"nest_id":    nest.ID,
	}
	// When master uses self-signed TLS, the egg must skip certificate verification.
	if masterCfg.Server.HTTPS.Enabled && masterCfg.Server.HTTPS.CertMode == "selfsigned" {
		eggModeCfg["tls_skip_verify"] = true
	}
	cfg["egg_mode"] = eggModeCfg

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal egg config: %w", err)
	}

	return data, nil
}

// ApplySafeConfigPatch applies a SafeConfigPatch to an existing egg config YAML.
// It returns the modified config YAML bytes. The original config is not modified.
// Only whitelisted fields are changed; all other sections remain untouched.
func ApplySafeConfigPatch(originalYAML []byte, patch SafeConfigPatch) ([]byte, error) {
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(originalYAML, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse existing config: %w", err)
	}

	// ── LLM section ──
	if patch.InheritLLM != nil {
		// Note: InheritLLM is stored on the EggRecord, not in the config YAML.
		// When InheritLLM is toggled, the caller should regenerate the full config
		// via GenerateEggConfig instead of patching. We still allow the flag here
		// for audit purposes but it has no effect on the YAML directly.
	}

	if patch.Provider != nil {
		if llm, ok := cfg["llm"].(map[string]interface{}); ok {
			llm["provider"] = *patch.Provider
		}
	}
	if patch.BaseURL != nil {
		if llm, ok := cfg["llm"].(map[string]interface{}); ok {
			llm["base_url"] = *patch.BaseURL
		}
	}
	if patch.Model != nil {
		if llm, ok := cfg["llm"].(map[string]interface{}); ok {
			llm["model"] = *patch.Model
		}
	}

	// ── Agent section — runtime flags ──
	if patch.AllowFilesystemWrite != nil {
		if agent, ok := cfg["agent"].(map[string]interface{}); ok {
			agent["allow_filesystem_write"] = *patch.AllowFilesystemWrite
		}
	}
	if patch.AllowNetworkRequests != nil {
		if agent, ok := cfg["agent"].(map[string]interface{}); ok {
			agent["allow_network_requests"] = *patch.AllowNetworkRequests
		}
	}
	if patch.AllowRemoteShell != nil {
		if agent, ok := cfg["agent"].(map[string]interface{}); ok {
			agent["allow_remote_shell"] = *patch.AllowRemoteShell
		}
	}
	if patch.AllowSelfUpdate != nil {
		if agent, ok := cfg["agent"].(map[string]interface{}); ok {
			agent["allow_self_update"] = *patch.AllowSelfUpdate
		}
	}

	// ── Allowed tools → derive allow_shell / allow_python ──
	if len(patch.AllowedTools) > 0 {
		allowShell := false
		allowPython := false
		for _, t := range patch.AllowedTools {
			switch strings.TrimSpace(t) {
			case "shell", "execute_shell_command":
				allowShell = true
			case "python", "python_execute":
				allowPython = true
			}
		}
		if agent, ok := cfg["agent"].(map[string]interface{}); ok {
			agent["allow_shell"] = allowShell
			agent["allow_python"] = allowPython
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched config: %w", err)
	}
	return data, nil
}

// ResolveMasterURL derives the WebSocket URL the egg should connect to,
// based on the nest's route configuration and the master's server settings.
func ResolveMasterURL(masterCfg *config.Config, nest NestRecord) string {
	scheme := "ws"
	masterPort := masterCfg.Server.Port
	if masterPort == 0 {
		masterPort = 8080
	}

	// Use wss:// when the master has HTTPS enabled.
	if masterCfg.Server.HTTPS.Enabled {
		scheme = "wss"
		if masterCfg.Server.HTTPS.HTTPSPort > 0 {
			masterPort = masterCfg.Server.HTTPS.HTTPSPort
		} else {
			masterPort = 443
		}
	}

	// For local Docker deployments the egg runs inside a container and cannot
	// reach the host via 0.0.0.0 (the bind address). Use host.docker.internal
	// which is the standard gateway hostname on Docker Desktop (Windows/macOS)
	// and also available on recent Docker versions on Linux.
	// Fall back to the Docker default bridge gateway (172.17.0.1) for older
	// Linux setups where host.docker.internal is not automatically set.
	if nest.DeployMethod == "docker_local" && nest.Route != "custom" && nest.Route != "ssh_tunnel" {
		host := nest.Host
		if host == "" || host == "0.0.0.0" {
			host = "host.docker.internal"
		}
		return fmt.Sprintf("%s://%s:%d/api/invasion/ws", scheme, host, masterPort)
	}

	switch nest.Route {
	case "tailscale", "wireguard", "direct":
		// Use nest host or fall back to master config
		host := nest.Host
		if host == "" || host == "0.0.0.0" {
			host = masterCfg.Server.Host
		}
		// Guard against the master bind address being used as a target
		if host == "0.0.0.0" {
			host = "localhost"
		}
		return fmt.Sprintf("%s://%s:%d/api/invasion/ws", scheme, host, masterPort)

	case "ssh_tunnel":
		// Egg will reach master via a forwarded local port
		// RouteConfig should contain {"tunnel_port": 8080}
		return fmt.Sprintf("%s://localhost:%d/api/invasion/ws", scheme, masterPort)

	case "custom":
		// RouteConfig contains the full URL
		if nest.RouteConfig != "" {
			return nest.RouteConfig
		}
		return fmt.Sprintf("%s://%s:%d/api/invasion/ws", scheme, nest.Host, masterPort)

	default:
		host := nest.Host
		if host == "" || host == "0.0.0.0" {
			host = masterCfg.Server.Host
		}
		if host == "0.0.0.0" {
			host = "localhost"
		}
		return fmt.Sprintf("%s://%s:%d/api/invasion/ws", scheme, host, masterPort)
	}
}
