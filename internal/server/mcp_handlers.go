package server

import (
	"aurago/internal/config"
	"encoding/json"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

// handleMCPServers dispatches GET / PUT for /api/mcp-servers.
func handleMCPServers(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetMCPServers(s, w, r)
		case http.MethodPut:
			handlePutMCPServers(s, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	var incoming []config.MCPServer
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	if configPath == "" {
		http.Error(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	// Read raw YAML, update mcp.servers key, write back
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.Logger.Error("Failed to read config for mcp-servers update", "error", err)
		http.Error(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		s.Logger.Error("Failed to parse config for mcp-servers update", "error", err)
		http.Error(w, "Failed to parse config", http.StatusInternalServerError)
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
	rawCfg["mcp"] = mcpSection

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		s.Logger.Error("Failed to marshal config after mcp-servers update", "error", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		s.Logger.Error("Failed to write config after mcp-servers update", "error", err)
		http.Error(w, "Failed to write config", http.StatusInternalServerError)
		return
	}

	// Hot-reload
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		s.Logger.Error("[MCPServers] Hot-reload failed", "error", loadErr)
		http.Error(w, "Saved but reload failed: "+loadErr.Error(), http.StatusInternalServerError)
		return
	}
	savedPath := s.Cfg.ConfigPath
	*s.Cfg = *newCfg
	s.Cfg.ConfigPath = savedPath
	s.Cfg.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Unlock()

	s.Logger.Info("[MCPServers] Updated", "count", len(incoming))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(incoming),
	})
}
