package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"
)

// ── MCP Server (Streamable HTTP, JSON-RPC 2.0) ─────────────────────────────
//
// Exposes AuraGo tools as a remote MCP server so external AI agents can
// discover and call them over the network.

const mcpProtocolVersion = "2024-11-05"
const mcpVaultTokenKey = "mcp_server_token"

// ── JSON-RPC 2.0 types ─────────────────────────────────────────────────────

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // may be int, string, or null for notifications
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── MCP protocol result types ───────────────────────────────────────────────

type mcpInitializeResult struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    mcpCapabilities `json:"capabilities"`
	ServerInfo      mcpServerInfo   `json:"serverInfo"`
}

type mcpCapabilities struct {
	Tools *mcpToolsCap `json:"tools,omitempty"`
}

type mcpToolsCap struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type mcpServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type mcpToolSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema"`
}

type mcpToolsListResult struct {
	Tools []mcpToolSchema `json:"tools"`
}

type mcpCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type mcpCallToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ── Handler ─────────────────────────────────────────────────────────────────

func handleMCPEndpoint(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		enabled := s.Cfg.MCPServer.Enabled
		requireAuth := s.Cfg.MCPServer.RequireAuth
		sessionSecret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()

		if !enabled {
			http.Error(w, "MCP server is disabled", http.StatusNotFound)
			return
		}

		// Authenticate
		if requireAuth && !mcpAuthenticate(s, r, sessionSecret) {
			w.Header().Set("WWW-Authenticate", `Bearer`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse JSON-RPC request
		var req mcpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			mcpWriteJSON(w, http.StatusBadRequest, mcpResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &mcpError{Code: -32700, Message: "Parse error"},
			})
			return
		}

		if req.JSONRPC != "2.0" {
			mcpWriteJSON(w, http.StatusOK, mcpResponse{
				JSONRPC: "2.0",
				ID:      parseID(req.ID),
				Error:   &mcpError{Code: -32600, Message: "Invalid Request: jsonrpc must be 2.0"},
			})
			return
		}

		var resp mcpResponse
		resp.JSONRPC = "2.0"
		resp.ID = parseID(req.ID)

		switch req.Method {
		case "initialize":
			resp.Result = mcpInitializeResult{
				ProtocolVersion: mcpProtocolVersion,
				Capabilities:    mcpCapabilities{Tools: &mcpToolsCap{}},
				ServerInfo:      mcpServerInfo{Name: "AuraGo", Version: "1.0.0"},
			}

		case "notifications/initialized":
			// Client acknowledgement — no response required
			return

		case "ping":
			resp.Result = map[string]interface{}{}

		case "tools/list":
			toolSchemas := mcpBuildToolList(s)
			resp.Result = mcpToolsListResult{Tools: toolSchemas}

		case "tools/call":
			resp.Result = mcpCallTool(r.Context(), s, req.Params)

		default:
			resp.Error = &mcpError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
		}

		mcpWriteJSON(w, http.StatusOK, resp)
	}
}

// mcpAuthenticate checks Bearer token or session cookie.
func mcpAuthenticate(s *Server, r *http.Request, sessionSecret string) bool {
	// Check Bearer token
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != "" && s.Vault != nil {
			stored, err := s.Vault.ReadSecret(mcpVaultTokenKey)
			if err == nil && stored != "" && stored == token {
				return true
			}
		}
	}

	// Fall back to session cookie
	if sessionSecret != "" && IsAuthenticated(r, sessionSecret) {
		return true
	}

	return false
}

// mcpBuildToolList converts OpenAI tool schemas to MCP tool schemas,
// filtered by the allowed_tools config.
func mcpBuildToolList(s *Server) []mcpToolSchema {
	s.CfgMu.RLock()
	cfg := s.Cfg
	allowed := cfg.MCPServer.AllowedTools
	s.CfgMu.RUnlock()

	ff := mcpFeatureFlags(cfg)
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	oaiTools := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, s.Logger)

	allowSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowSet[name] = true
	}

	var result []mcpToolSchema
	for _, t := range oaiTools {
		if t.Function == nil {
			continue
		}
		name := t.Function.Name
		// Filter by allowlist (empty = allow all)
		if len(allowSet) > 0 && !allowSet[name] {
			continue
		}

		schema := mcpToolSchema{
			Name:        name,
			Description: t.Function.Description,
		}

		// Convert parameters to MCP inputSchema format
		if t.Function.Parameters != nil {
			schema.InputSchema = t.Function.Parameters
		} else {
			schema.InputSchema = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		result = append(result, schema)
	}
	return result
}

// mcpCallTool dispatches a tools/call request to the agent's tool dispatcher.
func mcpCallTool(ctx context.Context, s *Server, params json.RawMessage) mcpCallToolResult {
	var p mcpCallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcpCallToolResult{
			Content: []mcpContent{{Type: "text", Text: "Invalid parameters: " + err.Error()}},
			IsError: true,
		}
	}

	// Check tool is allowed
	s.CfgMu.RLock()
	cfg := s.Cfg
	allowed := cfg.MCPServer.AllowedTools
	s.CfgMu.RUnlock()

	if len(allowed) > 0 {
		found := false
		for _, name := range allowed {
			if name == p.Name {
				found = true
				break
			}
		}
		if !found {
			return mcpCallToolResult{
				Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Tool %q is not in the allowed list", p.Name)}},
				IsError: true,
			}
		}
	}

	// Build a ToolCall from MCP arguments
	tc := agent.ToolCall{
		IsTool: true,
		Action: p.Name,
	}

	// Marshal arguments into JSON and unmarshal into ToolCall to populate fields
	if len(p.Arguments) > 0 {
		argBytes, _ := json.Marshal(p.Arguments)
		json.Unmarshal(argBytes, &tc)
		// Ensure Action stays correct after unmarshal
		tc.Action = p.Name
		tc.IsTool = true
		tc.Params = p.Arguments
	}

	// Add timeout
	toolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	result := agent.DispatchToolCall(
		toolCtx, tc, cfg, s.Logger, s.LLMClient, s.Vault,
		s.Registry, manifest, s.CronManager, s.MissionManager,
		s.LongTermMem, s.ShortTermMem, s.KG,
		s.InventoryDB, s.InvasionDB, s.CheatsheetDB, s.ImageGalleryDB,
		s.MediaRegistryDB, s.HomepageRegistryDB,
		s.RemoteHub, s.HistoryManager, false, "", s.Guardian, s.LLMGuardian,
		"mcp-server", s.CoAgentRegistry, s.BudgetTracker,
	)

	// Strip the "[Tool Output]\n" prefix if present
	result = strings.TrimPrefix(result, "[Tool Output]\n")
	result = strings.TrimPrefix(result, "[Tool Output]")

	isError := strings.Contains(result, "ERROR") || strings.Contains(result, "[EXECUTION ERROR]")

	return mcpCallToolResult{
		Content: []mcpContent{{Type: "text", Text: result}},
		IsError: isError,
	}
}

func mcpFeatureFlags(cfg *config.Config) agent.ToolFeatureFlags {
	return agent.ToolFeatureFlags{
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		DockerEnabled:            cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		CoAgentEnabled:           false, // MCP clients should not spawn co-agents
		SudoEnabled:              cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker,
		WebhooksEnabled:          cfg.Webhooks.Enabled,
		ProxmoxEnabled:           cfg.Proxmox.Enabled,
		OllamaEnabled:            cfg.Ollama.Enabled,
		TailscaleEnabled:         cfg.Tailscale.Enabled,
		CloudflareTunnelEnabled:  cfg.CloudflareTunnel.Enabled,
		GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
		AnsibleEnabled:           cfg.Ansible.Enabled,
		InvasionControlEnabled:   cfg.InvasionControl.Enabled,
		GitHubEnabled:            cfg.GitHub.Enabled,
		MQTTEnabled:              cfg.MQTT.Enabled,
		AdGuardEnabled:           cfg.AdGuard.Enabled,
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
		HomepageAllowLocalServer: cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		FirewallEnabled:          cfg.Firewall.Enabled && cfg.Runtime.FirewallAccessOK,
		EmailEnabled:             cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		ImageGenerationEnabled:   cfg.ImageGeneration.Enabled,
		RemoteControlEnabled:     cfg.RemoteControl.Enabled,
		MemoryEnabled:            cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
		NotesEnabled:             cfg.Tools.Notes.Enabled,
		MissionsEnabled:          cfg.Tools.Missions.Enabled,
		StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:         cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:               cfg.Tools.WOL.Enabled && cfg.Runtime.BroadcastOK,
		AllowShell:               cfg.Agent.AllowShell,
		AllowPython:              cfg.Agent.AllowPython,
		AllowFilesystemWrite:     cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:     cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:         cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:          cfg.Agent.AllowSelfUpdate,
	}
}

// ── Config API endpoints ────────────────────────────────────────────────────

// handleMCPServerTools returns the list of available tool names for the config UI.
func handleMCPServerTools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		ff := mcpFeatureFlags(cfg)
		manifest := tools.NewManifest(cfg.Directories.ToolsDir)
		oaiTools := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, s.Logger)

		names := make([]string, 0, len(oaiTools))
		for _, t := range oaiTools {
			if t.Function != nil {
				names = append(names, t.Function.Name)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(names)
	}
}

// handleMCPServerToken manages the MCP server Bearer token in the vault.
// GET  → returns masked token (or the full token if freshly generated)
// POST → generates a new token
func handleMCPServerToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Vault == nil {
			http.Error(w, "Vault not configured", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			token, err := s.Vault.ReadSecret(mcpVaultTokenKey)
			if err != nil || token == "" {
				mcpWriteJSON(w, http.StatusOK, map[string]string{"token": ""})
				return
			}
			// Return masked version for display
			masked := token[:4] + "••••••••" + token[len(token)-4:]
			mcpWriteJSON(w, http.StatusOK, map[string]string{"token": masked})

		case http.MethodPost:
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
				return
			}
			token := hex.EncodeToString(b)
			if err := s.Vault.WriteSecret(mcpVaultTokenKey, token); err != nil {
				http.Error(w, "Failed to store token", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[MCP Server] New Bearer token generated and stored in vault")
			mcpWriteJSON(w, http.StatusOK, map[string]string{"token": token})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func mcpWriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func parseID(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var id interface{}
	if err := json.Unmarshal(raw, &id); err != nil {
		return nil
	}
	return id
}
