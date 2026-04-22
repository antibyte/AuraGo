package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/security"
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		enabled := mcpServerEnabled(s.Cfg)
		requireAuth := mcpServerRequireAuth(s.Cfg)
		sessionSecret := s.Cfg.Auth.SessionSecret
		mainAuthEnabled := s.Cfg.Auth.Enabled
		s.CfgMu.RUnlock()

		if !enabled {
			jsonError(w, "MCP server is disabled", http.StatusNotFound)
			return
		}

		// Authenticate the caller.
		// The /mcp route bypasses the main auth middleware so that external MCP clients
		// (e.g. Claude Desktop, Cursor) can authenticate via Bearer token without needing
		// a browser session cookie.
		//
		// Auth rules:
		//   - requireAuth=true  → always check Bearer token / session (existing)
		//   - requireAuth=false AND main auth disabled → open access (home-lab default)
		//   - requireAuth=false AND main auth enabled  → fallback to session cookie so that
		//     the endpoint is not reachable from the internet without any credential
		needsAuth := requireAuth || mainAuthEnabled
		if needsAuth && !mcpAuthenticate(s, r, sessionSecret) {
			w.Header().Set("WWW-Authenticate", `Bearer`)
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse JSON-RPC request (limit body to 10 MB for safety)
		var req mcpRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(&req); err != nil {
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
	allowed := mcpEffectiveAllowedTools(cfg)
	s.CfgMu.RUnlock()

	allowSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowSet[name] = true
	}

	var result []mcpToolSchema
	for _, t := range mcpBuildToolCatalog(s) {
		name := t.Name
		// Filter by allowlist (empty = allow all)
		if len(allowSet) > 0 && !allowSet[name] {
			continue
		}
		result = append(result, t)
	}
	return result
}

// mcpCallTool dispatches a tools/call request to the agent's tool dispatcher.
func mcpCallTool(ctx context.Context, s *Server, params json.RawMessage) mcpCallToolResult {
	var p mcpCallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.Logger.Error("MCP tool call received invalid parameters", "error", err)
		return mcpCallToolResult{
			Content: []mcpContent{{Type: "text", Text: "Invalid parameters"}},
			IsError: true,
		}
	}

	if !mcpToolAvailable(s, p.Name) {
		return mcpCallToolResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Tool %q is not available in the current MCP runtime", p.Name)}},
			IsError: true,
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
		// Scan marshaled arguments for prompt injection before dispatch
		if s.Guardian != nil {
			if scan := s.Guardian.ScanForInjection(string(argBytes)); scan.Level >= security.ThreatHigh {
				s.Logger.Warn("[MCP] Prompt injection detected in tool arguments — blocking execution",
					"tool", p.Name, "level", scan.Level, "patterns", scan.Patterns)
				return mcpCallToolResult{
					Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Security threat detected in tool arguments (level: %s). Execution blocked.", scan.Level)}},
					IsError: true,
				}
			}
		}
		json.Unmarshal(argBytes, &tc)
		// Ensure Action stays correct after unmarshal
		tc.Action = p.Name
		tc.IsTool = true
		tc.Params = p.Arguments
	}

	// Add timeout
	toolCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	result := agent.DispatchToolCall(
		toolCtx, &tc, &agent.DispatchContext{
			Cfg: cfg, Logger: s.Logger, LLMClient: s.LLMClient, Vault: s.Vault,
			Registry: s.Registry, Manifest: manifest, CronManager: s.CronManager,
			MissionManagerV2: s.MissionManagerV2, LongTermMem: s.LongTermMem,
			ShortTermMem: s.ShortTermMem, KG: s.KG,
			InventoryDB: s.InventoryDB, InvasionDB: s.InvasionDB,
			CheatsheetDB: s.CheatsheetDB, ImageGalleryDB: s.ImageGalleryDB,
			MediaRegistryDB: s.MediaRegistryDB, HomepageRegistryDB: s.HomepageRegistryDB,
			ContactsDB: s.ContactsDB, PlannerDB: s.PlannerDB, SQLConnectionsDB: s.SQLConnectionsDB,
			SQLConnectionPool: s.SQLConnectionPool, RemoteHub: s.RemoteHub,
			HistoryMgr: s.HistoryManager, Guardian: s.Guardian,
			LLMGuardian: s.LLMGuardian, SessionID: "mcp-server",
			CoAgentRegistry: s.CoAgentRegistry, BudgetTracker: s.BudgetTracker,
		}, "",
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

func mcpToolAvailable(s *Server, toolName string) bool {
	// Use the catalog directly instead of mcpBuildToolList which also
	// filters by allowed-tools — we check availability before the allowlist
	// filter so the error message is accurate.
	for _, tool := range mcpBuildToolCatalog(s) {
		if tool.Name == toolName {
			return true
		}
	}
	return false
}

func mcpFeatureFlags(s *Server) agent.ToolFeatureFlags {
	cfg := s.Cfg
	return agent.ToolFeatureFlags{
		HomeAssistantEnabled:         cfg.HomeAssistant.Enabled,
		DockerEnabled:                cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		CoAgentEnabled:               false, // MCP clients should not spawn co-agents
		SudoEnabled:                  cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker && !cfg.Runtime.NoNewPrivileges,
		WebhooksEnabled:              cfg.Webhooks.Enabled,
		ProxmoxEnabled:               cfg.Proxmox.Enabled,
		OllamaEnabled:                cfg.Ollama.Enabled,
		TailscaleEnabled:             cfg.Tailscale.Enabled,
		WebScraperEnabled:            cfg.Tools.WebScraper.Enabled,
		CloudflareTunnelEnabled:      cfg.CloudflareTunnel.Enabled,
		GoogleWorkspaceEnabled:       cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:              cfg.OneDrive.Enabled,
		VirusTotalEnabled:            cfg.VirusTotal.Enabled,
		GolangciLintEnabled:          cfg.GolangciLint.Enabled,
		AnsibleEnabled:               cfg.Ansible.Enabled,
		InvasionControlEnabled:       cfg.InvasionControl.Enabled,
		GitHubEnabled:                cfg.GitHub.Enabled,
		MQTTEnabled:                  cfg.MQTT.Enabled,
		AdGuardEnabled:               cfg.AdGuard.Enabled,
		MCPEnabled:                   cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:               cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		MeshCentralEnabled:           cfg.MeshCentral.Enabled,
		HomepageEnabled:              cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
		HomepageAllowLocalServer:     cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:               cfg.Netlify.Enabled,
		FirewallEnabled:              cfg.Firewall.Enabled && cfg.Runtime.FirewallAccessOK,
		EmailEnabled:                 cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		ImageGenerationEnabled:       cfg.ImageGeneration.Enabled,
		RemoteControlEnabled:         cfg.RemoteControl.Enabled,
		DiscordEnabled:               cfg.Discord.Enabled,
		TelegramEnabled:              cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0,
		SQLConnectionsEnabled:        cfg.SQLConnections.Enabled && s.SQLConnectionsDB != nil && s.SQLConnectionPool != nil,
		MemoryEnabled:                cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:        cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:          cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:             cfg.Tools.Scheduler.Enabled,
		NotesEnabled:                 cfg.Tools.Notes.Enabled,
		MissionsEnabled:              cfg.Tools.Missions.Enabled,
		StopProcessEnabled:           cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:             cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled:     cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:                   cfg.Tools.WOL.Enabled && cfg.Runtime.BroadcastOK,
		AllowShell:                   cfg.Agent.AllowShell,
		AllowPython:                  cfg.Agent.AllowPython,
		AllowFilesystemWrite:         cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:         cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:             cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:              cfg.Agent.AllowSelfUpdate,
		FritzBoxSystemEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
		FritzBoxNetworkEnabled:       cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
		FritzBoxTelephonyEnabled:     cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
		FritzBoxSmartHomeEnabled:     cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
		FritzBoxStorageEnabled:       cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
		FritzBoxTVEnabled:            cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
		ContactsEnabled:              cfg.Tools.Contacts.Enabled,
		PlannerEnabled:               cfg.Tools.Planner.Enabled,
		MediaConversionEnabled:       cfg.Tools.MediaConversion.Enabled,
		PythonSecretInjectionEnabled: cfg.Tools.PythonSecretInjection.Enabled,
	}
}

// ── Config API endpoints ────────────────────────────────────────────────────

// handleMCPServerTools returns the list of available tool names for the config UI.
func handleMCPServerTools(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		catalog := mcpBuildToolCatalog(s)
		names := make([]string, 0, len(catalog))
		for _, t := range catalog {
			if t.Name != "" {
				names = append(names, t.Name)
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
			jsonError(w, "Vault not configured", http.StatusServiceUnavailable)
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
			masked := maskToken(token)
			mcpWriteJSON(w, http.StatusOK, map[string]string{"token": masked})

		case http.MethodPost:
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				jsonError(w, "Failed to generate token", http.StatusInternalServerError)
				return
			}
			token := hex.EncodeToString(b)
			if err := s.Vault.WriteSecret(mcpVaultTokenKey, token); err != nil {
				jsonError(w, "Failed to store token", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[MCP Server] New Bearer token generated and stored in vault")
			mcpWriteJSON(w, http.StatusOK, map[string]string{"token": token})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// maskToken returns a masked representation of a secret token.
// Safely handles tokens shorter than 8 characters.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "••••••••"
	}
	return token[:4] + "••••••••" + token[len(token)-4:]
}

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
