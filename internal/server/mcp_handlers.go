package server

import (
	"aurago/internal/config"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"aurago/internal/tools"
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

var (
	initExternalMCPManager     = tools.InitMCPManager
	shutdownExternalMCPManager = tools.ShutdownMCPManager
)

func syncExternalMCPRuntime(cfg *config.Config, vault config.SecretReader, logger *slog.Logger) {
	shutdownExternalMCPManager()
	if cfg == nil || logger == nil {
		return
	}
	if !cfg.Agent.AllowMCP || !cfg.MCP.Enabled || len(cfg.MCP.Servers) == 0 {
		return
	}
	initExternalMCPManager(buildRuntimeMCPConfigs(cfg, vault, logger), logger)
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

func mcpConfigPath(s *Server) string {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.ConfigPath
}

func persistMCPSectionUpdate(s *Server, mutate func(map[string]interface{}) error) error {
	configPath := mcpConfigPath(s)
	if configPath == "" {
		return os.ErrInvalid
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return err
	}

	mcpSection, ok := rawCfg["mcp"].(map[string]interface{})
	if !ok {
		mcpSection = map[string]interface{}{}
	}
	if err := mutate(mcpSection); err != nil {
		return err
	}
	rawCfg["mcp"] = mcpSection

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}
	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		return err
	}

	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		return loadErr
	}
	savedPath := s.Cfg.ConfigPath
	*s.Cfg = *newCfg
	s.Cfg.ConfigPath = savedPath
	s.Cfg.ApplyVaultSecrets(s.Vault)
	s.Cfg.ApplyOAuthTokens(s.Vault)
	runtimeMCPConfigs := buildRuntimeMCPConfigs(s.Cfg, s.Vault, s.Logger)
	shutdownExternalMCPManager()
	if s.Cfg.Agent.AllowMCP && s.Cfg.MCP.Enabled && len(runtimeMCPConfigs) > 0 {
		initExternalMCPManager(runtimeMCPConfigs, s.Logger)
	}
	s.CfgMu.Unlock()
	return nil
}

// handlePutMCPServers saves a new MCP servers array to config.yaml and hot-reloads.
func handlePutMCPServers(s *Server, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
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

	// Build servers list for YAML
	err = persistMCPSectionUpdate(s, func(mcpSection map[string]interface{}) error {
		serversList := make([]interface{}, len(incoming))
		for i, srv := range incoming {
			m := map[string]interface{}{
				"name":    srv.Name,
				"command": srv.Command,
				"enabled": srv.Enabled,
				"runtime": strings.TrimSpace(srv.Runtime),
			}
			if len(srv.Args) > 0 {
				m["args"] = srv.Args
			}
			if len(srv.Env) > 0 {
				m["env"] = srv.Env
			}
			if strings.TrimSpace(srv.DockerImage) != "" {
				m["docker_image"] = strings.TrimSpace(srv.DockerImage)
			}
			if strings.TrimSpace(srv.DockerCommand) != "" {
				m["docker_command"] = strings.TrimSpace(srv.DockerCommand)
			}
			if srv.AllowLocalFallback {
				m["allow_local_fallback"] = true
			}
			if strings.TrimSpace(srv.HostWorkdir) != "" {
				m["host_workdir"] = strings.TrimSpace(srv.HostWorkdir)
			}
			if strings.TrimSpace(srv.ContainerWorkdir) != "" {
				m["container_workdir"] = strings.TrimSpace(srv.ContainerWorkdir)
			}
			serversList[i] = m
		}
		mcpSection["servers"] = serversList
		if enabledOverride != nil {
			mcpSection["enabled"] = *enabledOverride
		}
		return nil
	})
	if err != nil {
		s.Logger.Error("Failed to marshal config after mcp-servers update", "error", err)
		jsonError(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("[MCPServers] Updated", "count", len(incoming))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(incoming),
	})
}

func handleMCPSecrets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetMCPSecrets(s, w, r)
		case http.MethodPut:
			handlePutMCPSecrets(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGetMCPSecrets(s *Server, w http.ResponseWriter, _ *http.Request) {
	s.CfgMu.RLock()
	cfgCopy := *s.Cfg
	s.CfgMu.RUnlock()

	statuses := buildMCPSecretStatuses(&cfgCopy, s.Vault)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"secrets": statuses,
	})
}

func handlePutMCPSecrets(s *Server, w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Secrets []struct {
			Alias       string `json:"alias"`
			Label       string `json:"label"`
			Description string `json:"description"`
			Value       string `json:"value"`
			DeleteValue bool   `json:"delete_value"`
		} `json:"secrets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	normalized := make([]config.MCPSecret, 0, len(payload.Secrets))
	writeValues := make(map[string]string)
	deleteValues := make(map[string]bool)
	seen := make(map[string]bool)
	for _, secret := range payload.Secrets {
		alias := normalizeMCPSecretAlias(secret.Alias)
		if alias == "" || !mcpAliasRe.MatchString(alias) || seen[alias] {
			continue
		}
		seen[alias] = true
		normalized = append(normalized, config.MCPSecret{
			Alias:       alias,
			Label:       strings.TrimSpace(secret.Label),
			Description: strings.TrimSpace(secret.Description),
		})
		if strings.TrimSpace(secret.Value) != "" {
			writeValues[alias] = secret.Value
		}
		if secret.DeleteValue {
			deleteValues[alias] = true
		}
	}

	s.CfgMu.RLock()
	existing := append([]config.MCPSecret(nil), s.Cfg.MCP.Secrets...)
	s.CfgMu.RUnlock()

	if err := persistMCPSectionUpdate(s, func(mcpSection map[string]interface{}) error {
		secretsList := make([]interface{}, 0, len(normalized))
		for _, secret := range normalized {
			entry := map[string]interface{}{
				"alias": secret.Alias,
			}
			if secret.Label != "" {
				entry["label"] = secret.Label
			}
			if secret.Description != "" {
				entry["description"] = secret.Description
			}
			secretsList = append(secretsList, entry)
		}
		mcpSection["secrets"] = secretsList
		return nil
	}); err != nil {
		s.Logger.Error("Failed to save MCP secrets metadata", "error", err)
		jsonError(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	currentAliases := make(map[string]bool, len(normalized))
	for _, secret := range normalized {
		currentAliases[secret.Alias] = true
	}
	for _, secret := range existing {
		alias := normalizeMCPSecretAlias(secret.Alias)
		if alias != "" && !currentAliases[alias] {
			deleteValues[alias] = true
		}
	}
	for alias, value := range writeValues {
		if err := s.Vault.WriteSecret(mcpSecretVaultKey(alias), value); err != nil {
			s.Logger.Error("Failed to write MCP secret to vault", "alias", alias, "error", err)
			jsonError(w, "Failed to store secret in vault", http.StatusInternalServerError)
			return
		}
	}
	for alias := range deleteValues {
		if err := s.Vault.DeleteSecret(mcpSecretVaultKey(alias)); err != nil {
			s.Logger.Error("Failed to delete MCP secret from vault", "alias", alias, "error", err)
			jsonError(w, "Failed to delete secret from vault", http.StatusInternalServerError)
			return
		}
	}

	s.CfgMu.Lock()
	s.Cfg.MCP.Secrets = normalized
	runtimeMCPConfigs := buildRuntimeMCPConfigs(s.Cfg, s.Vault, s.Logger)
	shutdownExternalMCPManager()
	if s.Cfg.Agent.AllowMCP && s.Cfg.MCP.Enabled && len(runtimeMCPConfigs) > 0 {
		initExternalMCPManager(runtimeMCPConfigs, s.Logger)
	}
	s.CfgMu.Unlock()

	handleGetMCPSecrets(s, w, r)
}

func handleMCPPreferences(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetMCPPreferences(s, w, r)
		case http.MethodPut:
			handlePutMCPPreferences(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleGetMCPPreferences(s *Server, w http.ResponseWriter, _ *http.Request) {
	s.CfgMu.RLock()
	preferences := s.Cfg.MCP.PreferredCapabilities
	s.CfgMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preferences)
}

func handlePutMCPPreferences(s *Server, w http.ResponseWriter, r *http.Request) {
	var preferences config.MCPPreferredCapabilities
	if err := json.NewDecoder(r.Body).Decode(&preferences); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := persistMCPSectionUpdate(s, func(mcpSection map[string]interface{}) error {
		mcpSection["preferred_capabilities"] = map[string]interface{}{
			"web_search": map[string]interface{}{
				"server": strings.TrimSpace(preferences.WebSearch.Server),
				"tool":   strings.TrimSpace(preferences.WebSearch.Tool),
			},
			"vision": map[string]interface{}{
				"server": strings.TrimSpace(preferences.Vision.Server),
				"tool":   strings.TrimSpace(preferences.Vision.Tool),
			},
		}
		return nil
	})
	if err != nil {
		s.Logger.Error("Failed to save MCP preferred capabilities", "error", err)
		jsonError(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	s.Logger.Info("[MCPPreferences] Updated",
		"web_search_server", preferences.WebSearch.Server,
		"web_search_tool", preferences.WebSearch.Tool,
		"vision_server", preferences.Vision.Server,
		"vision_tool", preferences.Vision.Tool,
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

func handleMCPRuntimeServers(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		servers, err := tools.MCPListServers(s.Logger)
		if err != nil {
			servers = []map[string]interface{}{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"servers": servers,
		})
	}
}

func handleMCPRuntimeTools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		serverName := strings.TrimSpace(r.URL.Query().Get("server"))
		if serverName == "" {
			jsonError(w, "server query parameter is required", http.StatusBadRequest)
			return
		}
		mcpTools, err := tools.MCPListTools(serverName, s.Logger)
		if err != nil {
			s.Logger.Warn("[MCPRuntime] Failed to list tools", "server", serverName, "error", err)
			mcpTools = []tools.MCPToolInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"tools":  mcpTools,
		})
	}
}
