package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
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

		grant, ok := s.bridgeGrant(r, true)
		if !ok {
			toolBridgeWriteError(w, http.StatusUnauthorized, "Missing, expired or revoked run capability")
			return
		}

		// Check if tool bridge is enabled
		cfg := s.ConfigSnapshot()
		if cfg == nil || !cfg.Tools.PythonToolBridge.Enabled {
			toolBridgeWriteError(w, http.StatusForbidden, "Python tool bridge is disabled")
			return
		}

		// Extract tool name from URL
		path := strings.TrimPrefix(r.URL.Path, "/api/internal/tool-bridge/")
		toolName := path
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
		if len(cfg.Tools.PythonToolBridge.AllowedTools) == 0 {
			toolBridgeWriteError(w, http.StatusForbidden, "No tools are allowed via tool bridge (allowed_tools is empty)")
			return
		}
		found := false
		for _, name := range cfg.Tools.PythonToolBridge.AllowedTools {
			if name == toolName {
				found = true
				break
			}
		}
		if !found {
			toolBridgeWriteError(w, http.StatusForbidden, fmt.Sprintf("Tool '%s' is not in the allowed_tools list", toolName))
			return
		}
		if _, allowed := grant.tools[toolName]; !allowed {
			toolBridgeWriteError(w, http.StatusForbidden, "Tool is outside the originating run capability")
			return
		}

		// Parse request body
		var req toolBridgeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			toolBridgeWriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			toolBridgeWriteError(w, http.StatusBadRequest, "Expected one request object")
			return
		}

		// Optional: per-tool extra allowlists
		if toolName == "sql_query" {
			// Require an explicit allowlist of connection names for SQL usage from Python skills.
			allowedConnections := cfg.Tools.PythonToolBridge.AllowedSQLConnections
			if len(allowedConnections) == 0 {
				toolBridgeWriteError(w, http.StatusForbidden, "SQL tool bridge access is blocked (allowed_sql_connections is empty)")
				return
			}
			connName, _ := req.Parameters["connection_name"].(string)
			connName = strings.TrimSpace(connName)
			if connName == "" {
				toolBridgeWriteError(w, http.StatusBadRequest, "connection_name is required for sql_query")
				return
			}
			if _, ok := grant.sql[connName]; !ok {
				toolBridgeWriteError(w, http.StatusForbidden, "SQL connection is outside the originating run capability")
				return
			}
			okConn := false
			for _, n := range allowedConnections {
				if n == connName {
					okConn = true
					break
				}
			}
			if !okConn {
				toolBridgeWriteError(w, http.StatusForbidden, "SQL connection is not allowed for tool bridge")
				return
			}
		}

		// Set timeout default and cap
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 60
		} else if timeout > toolBridgeMaxTimeout {
			timeout = toolBridgeMaxTimeout
		}

		ctx, cancel := context.WithTimeout(grant.ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		stop := context.AfterFunc(r.Context(), cancel)
		defer stop()
		ctx = budget.WithTracking(ctx, grant.dc.BudgetTracker, "tool_bridge")

		// Build tool call — only set Params to prevent unintended field overrides.
		tc := agent.ToolCall{
			IsTool: true,
			Action: toolName,
			Params: req.Parameters,
		}

		// Preserve the original session, grants and live-authorization snapshot.
		dc := grant.dc
		result := agent.DispatchToolCallResult(ctx, &tc, &dc, dc.UserContext)
		status := "success"
		if result.Status != agent.ToolResultSuccess {
			status = "error"
		}
		if s.Logger != nil {
			s.Logger.Info("Tool bridge execution", "tool", toolName, "status", result.Status)
		}

		resp := toolBridgeResponse{
			Result: result.Output,
			Status: status,
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
