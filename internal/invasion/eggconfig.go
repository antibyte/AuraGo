package invasion

import (
	"aurago/internal/config"
	"fmt"

	"gopkg.in/yaml.v3"
)

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
		"allow_shell":                true,
		"allow_python":               true,
		"allow_filesystem_write":     true,
		"allow_network_requests":     true,
		"allow_remote_shell":         false,
		"allow_self_update":          false,
		"sudo_enabled":               false,
		"enable_google_workspace":    false,
	}

	// ── Circuit Breaker ──
	cfg["circuit_breaker"] = map[string]interface{}{
		"max_tool_calls":      15,
		"llm_timeout_seconds": 180,
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
	cfg["egg_mode"] = map[string]interface{}{
		"enabled":    true,
		"master_url": masterURL,
		"shared_key": sharedKey,
		"egg_id":     egg.ID,
		"nest_id":    nest.ID,
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal egg config: %w", err)
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

	switch nest.Route {
	case "tailscale", "wireguard", "direct":
		// Use nest host or fall back to master config
		host := nest.Host
		if host == "" {
			host = masterCfg.Server.Host
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
		return fmt.Sprintf("%s://%s:%d/api/invasion/ws", scheme, nest.Host, masterPort)
	}
}
