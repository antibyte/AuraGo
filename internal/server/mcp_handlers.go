package server

import (
	"aurago/internal/config"
	"aurago/internal/tools"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	initExternalMCPManager     = tools.InitMCPManager
	shutdownExternalMCPManager = tools.ShutdownMCPManager
)

func syncExternalMCPRuntime(cfg *config.Config, logger *slog.Logger) {
	shutdownExternalMCPManager()
	if cfg == nil || logger == nil {
		return
	}
	if !cfg.Agent.AllowMCP || !cfg.MCP.Enabled || len(cfg.MCP.Servers) == 0 {
		return
	}
	mcpConfigs := make([]tools.MCPServerConfig, len(cfg.MCP.Servers))
	for i, srv := range cfg.MCP.Servers {
		mcpConfigs[i] = tools.MCPServerConfig{
			Name:    srv.Name,
			Command: srv.Command,
			Args:    srv.Args,
			Env:     srv.Env,
			Enabled: srv.Enabled,
		}
	}
	initExternalMCPManager(mcpConfigs, logger)
}

// handleMCPServers dispatches GET / PUT for /api/mcp-servers.
func handleMCPServers(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetMCPServers(s, w, r)
		case http.MethodPut:
			handlePutMCPServers(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleGetMCPServers returns the MCP servers list.
func handleGetMCPServers(s *Server, w http.ResponseWriter, _ *http.Request) {
	s.CfgMu.RLock()
	servers := s.Cfg.MCP.Servers
	s.CfgMu.RUnlock()

	if servers == nil {
		servers = []config.MCPServer{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

// handlePutMCPServers saves a new MCP servers array to config.yaml and hot-reloads.
func handlePutMCPServers(s *Server, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var incoming []config.MCPServer
	var enabledOverride *bool
	trimmed := strings.TrimSpace(string(body))
	switch {
	case trimmed == "":
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	case strings.HasPrefix(trimmed, "["):
		if err := json.Unmarshal(body, &incoming); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	default:
		var payload struct {
			Enabled *bool              `json:"enabled"`
			Servers []config.MCPServer `json:"servers"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		incoming = payload.Servers
		enabledOverride = payload.Enabled
	}
	if incoming == nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	if configPath == "" {
		jsonError(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	// Read raw YAML, update mcp.servers key, write back
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.Logger.Error("Failed to read config for mcp-servers update", "error", err)
		jsonError(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		s.Logger.Error("Failed to parse config for mcp-servers update", "error", err)
		jsonError(w, "Failed to parse config", http.StatusInternalServerError)
		return
	}

	// Ensure mcp section exists
	mcpSection, ok := rawCfg["mcp"].(map[string]interface{})
	if !ok {
		mcpSection = map[string]interface{}{}
	}

	// Build servers list for YAML
	serversList := make([]interface{}, len(incoming))
	for i, srv := range incoming {
		m := map[string]interface{}{
			"name":    srv.Name,
			"command": srv.Command,
			"enabled": srv.Enabled,
		}
		if len(srv.Args) > 0 {
			m["args"] = srv.Args
		}
		if len(srv.Env) > 0 {
			m["env"] = srv.Env
		}
		serversList[i] = m
	}
	mcpSection["servers"] = serversList
	if enabledOverride != nil {
		mcpSection["enabled"] = *enabledOverride
	}
	rawCfg["mcp"] = mcpSection

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		s.Logger.Error("Failed to marshal config after mcp-servers update", "error", err)
		jsonError(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		s.Logger.Error("Failed to write config after mcp-servers update", "error", err)
		jsonError(w, "Failed to write config", http.StatusInternalServerError)
		return
	}

	// Hot-reload
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		s.Logger.Error("[MCPServers] Hot-reload failed", "error", loadErr)
		jsonError(w, "Saved but reload failed", http.StatusInternalServerError)
		return
	}
	savedPath := s.Cfg.ConfigPath
	*s.Cfg = *newCfg
	s.Cfg.ConfigPath = savedPath
	s.Cfg.ApplyVaultSecrets(s.Vault)
	s.Cfg.ApplyOAuthTokens(s.Vault)
	syncExternalMCPRuntime(s.Cfg, s.Logger)
	s.CfgMu.Unlock()

	s.Logger.Info("[MCPServers] Updated", "count", len(incoming))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(incoming),
	})
}
