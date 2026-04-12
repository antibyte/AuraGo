package server

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/tools"
)

// toolBridgeRequest is the JSON body for a tool bridge invocation.
type toolBridgeRequest struct {
	Parameters map[string]interface{} `json:"parameters"`
	Timeout    int                    `json:"timeout"` // seconds, default 60, max 300
}

// toolBridgeResponse is the JSON response for a tool bridge invocation.
type toolBridgeResponse struct {
	Status string `json:"status"` // "success" or "error"
	Result string `json:"result"`
}

// toolBridgeNameRegex validates tool names to prevent path traversal.
var toolBridgeNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

const toolBridgeMaxTimeout = 300 // 5 minutes

// handleToolBridgeExecute handles tool invocations from Python skills via the
// internal loopback API. It requires the X-Internal-Token header and restricts
// access to tools listed in config.Tools.PythonToolBridge.AllowedTools.
//
// Endpoint: POST /api/internal/tool-bridge/{toolName}
func handleToolBridgeExecute(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			toolBridgeWriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Loopback-only: verify request comes from localhost
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host != "127.0.0.1" && host != "::1" && !strings.HasPrefix(host, "127.") {
			toolBridgeWriteError(w, http.StatusForbidden, "Tool bridge is loopback-only")
			return
		}

		// Validate internal token (same as background task loopback auth)
		tok := s.internalToken
		reqTok := r.Header.Get("X-Internal-Token")
		if tok == "" || reqTok == "" || !hmac.Equal([]byte(reqTok), []byte(tok)) {
			toolBridgeWriteError(w, http.StatusUnauthorized, "Invalid or missing internal token")
			return
		}

		// Check if tool bridge is enabled
		s.CfgMu.RLock()
		enabled := s.Cfg.Tools.PythonToolBridge.Enabled
		allowedTools := s.Cfg.Tools.PythonToolBridge.AllowedTools
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		if !enabled {
			toolBridgeWriteError(w, http.StatusForbidden, "Python tool bridge is disabled")
			return
		}

		// Extract tool name from URL
		path := strings.TrimPrefix(r.URL.Path, "/api/internal/tool-bridge/")
		toolName := strings.Split(path, "/")[0]
		if toolName == "" {
			toolBridgeWriteError(w, http.StatusBadRequest, "Tool name is required")
			return
		}

		// Validate tool name format
		if !toolBridgeNameRegex.MatchString(toolName) {
			toolBridgeWriteError(w, http.StatusBadRequest, "Invalid tool name format")
			return
		}

		// Check if tool is in the allowed list (strict: empty list = nothing allowed)
		if len(allowedTools) == 0 {
			toolBridgeWriteError(w, http.StatusForbidden, "No tools are allowed via tool bridge (allowed_tools is empty)")
			return
		}
		found := false
		for _, name := range allowedTools {
			if name == toolName {
				found = true
				break
			}
		}
		if !found {
			toolBridgeWriteError(w, http.StatusForbidden, fmt.Sprintf("Tool '%s' is not in the allowed_tools list", toolName))
			return
		}

		// Parse request body
		var req toolBridgeRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			toolBridgeWriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Set timeout default and cap
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 60
		} else if timeout > toolBridgeMaxTimeout {
			timeout = toolBridgeMaxTimeout
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		// Build tool call — only set Params to prevent unintended field overrides.
		tc := agent.ToolCall{
			IsTool: true,
			Action: toolName,
			Params: req.Parameters,
		}

		// Execute tool via dispatch (same pattern as n8n tool execution)
		manifest := tools.NewManifest(cfg.Directories.ToolsDir)
		result := agent.DispatchToolCall(
			ctx, &tc, &agent.DispatchContext{
				Cfg: cfg, Logger: s.Logger, LLMClient: s.LLMClient, Vault: s.Vault,
				Registry: s.Registry, Manifest: manifest, CronManager: s.CronManager,
				MissionManagerV2: s.MissionManagerV2, LongTermMem: s.LongTermMem,
				ShortTermMem: s.ShortTermMem, KG: s.KG,
				InventoryDB: s.InventoryDB, InvasionDB: s.InvasionDB,
				CheatsheetDB: s.CheatsheetDB, ImageGalleryDB: s.ImageGalleryDB,
				MediaRegistryDB: s.MediaRegistryDB, HomepageRegistryDB: s.HomepageRegistryDB,
				ContactsDB: s.ContactsDB, SQLConnectionsDB: s.SQLConnectionsDB,
				SQLConnectionPool: s.SQLConnectionPool, RemoteHub: s.RemoteHub,
				HistoryMgr: s.HistoryManager, Guardian: s.Guardian,
				LLMGuardian: s.LLMGuardian, SessionID: "tool-bridge",
				CoAgentRegistry: s.CoAgentRegistry, BudgetTracker: s.BudgetTracker,
			}, "",
		)

		s.Logger.Info("Tool bridge execution", slog.String("tool", toolName), slog.String("status", toolBridgeResultStatus(result)))

		resp := toolBridgeResponse{
			Result: result,
			Status: toolBridgeResultStatus(result),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func toolBridgeResultStatus(result string) string {
	if strings.HasPrefix(result, "ERROR") || strings.HasPrefix(result, "error:") || strings.HasPrefix(result, "Error:") {
		return "error"
	}
	return "success"
}

func toolBridgeWriteError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(toolBridgeResponse{
		Status: "error",
		Result: message,
	})
}
