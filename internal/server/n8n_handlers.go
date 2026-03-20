package server

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

// N8n Vault Key for token storage
const n8nVaultTokenKey = "n8n_api_token"

// N8n scopes
const (
	N8nScopeRead     = "n8n:read"
	N8nScopeChat     = "n8n:chat"
	N8nScopeTools    = "n8n:tools"
	N8nScopeMemory   = "n8n:memory"
	N8nScopeMissions = "n8n:missions"
	N8nScopeAdmin    = "n8n:admin"
)

// N8nEvent types for webhooks
const (
	N8nEventAgentResponse    = "agent.response"
	N8nEventAgentToolCall    = "agent.tool_call"
	N8nEventAgentError       = "agent.error"
	N8nEventMemoryStored     = "memory.stored"
	N8nEventMissionCompleted = "mission.completed"
)

// n8n isolated session store — keeps n8n conversations separate from the main HistoryManager.
const (
	n8nSessionTTL     = 4 * time.Hour
	n8nSessionMaxMsgs = 50
)

var (
	n8nSessionMu   sync.Mutex
	n8nSessions    = make(map[string][]openai.ChatCompletionMessage)
	n8nSessionLast = make(map[string]time.Time)
	n8nRateMu      sync.Mutex
	n8nRateWindows = make(map[string][]time.Time)
)

// n8nChatRequest represents a chat request from n8n
type n8nChatRequest struct {
	Message       string   `json:"message"`
	SessionID     string   `json:"session_id,omitempty"`
	SystemPrompt  string   `json:"system_prompt,omitempty"`
	Tools         []string `json:"tools,omitempty"`
	ContextWindow int      `json:"context_window,omitempty"`
	Stream        bool     `json:"stream,omitempty"`
}

// n8nChatResponse represents a chat response to n8n
type n8nChatResponse struct {
	Response   string        `json:"response"`
	SessionID  string        `json:"session_id"`
	ToolCalls  []n8nToolCall `json:"tool_calls,omitempty"`
	TokensUsed int           `json:"tokens_used"`
	DurationMs int64         `json:"duration_ms"`
}

type n8nToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result,omitempty"`
}

// n8nToolExecuteRequest represents a direct tool execution request
type n8nToolExecuteRequest struct {
	Parameters map[string]interface{} `json:"parameters"`
	Async      bool                   `json:"async,omitempty"`
	Timeout    int                    `json:"timeout,omitempty"`
}

// n8nToolExecuteResponse represents a tool execution response
type n8nToolExecuteResponse struct {
	Result string `json:"result"`
	Status string `json:"status"`
	TaskID string `json:"task_id,omitempty"`
	Error  string `json:"error,omitempty"`
}

// n8nMemorySearchRequest represents a memory search request
type n8nMemorySearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Type   string `json:"type,omitempty"` // short_term, long_term, knowledge_graph
	Filter string `json:"filter,omitempty"`
}

// n8nMemoryStoreRequest represents a memory store request
type n8nMemoryStoreRequest struct {
	Content  string                 `json:"content"`
	Type     string                 `json:"type"` // short_term, long_term, core
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// n8nMissionRequest represents a mission creation request
type n8nMissionRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Steps       []map[string]interface{} `json:"steps,omitempty"`
	Trigger     string                   `json:"trigger,omitempty"` // manual, webhook, schedule
	RunNow      bool                     `json:"run_now,omitempty"`
}

// n8nWebhookPayload represents a webhook event to send to n8n
type n8nWebhookPayload struct {
	Event     string                 `json:"event"`
	Timestamp string                 `json:"timestamp"`
	SessionID string                 `json:"session_id,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature"`
}

// handleN8nStatus returns the n8n integration status and capabilities
func handleN8nStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg.N8n
		s.CfgMu.RUnlock()

		if !cfg.Enabled {
			http.Error(w, "n8n integration is disabled", http.StatusNotFound)
			return
		}

		// Status also requires auth — it reveals tool inventory and config.
		if !n8nAuthorize(s, w, r, N8nScopeRead) {
			return
		}

		response := map[string]interface{}{
			"status":       "ok",
			"version":      "1.0.0",
			"capabilities": []string{"chat", "tools", "memory", "missions", "webhooks"},
			"config": map[string]interface{}{
				"readonly":       cfg.ReadOnly,
				"allowed_tools":  cfg.AllowedTools,
				"allowed_events": cfg.AllowedEvents,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleN8nChat handles chat requests from n8n
func handleN8nChat(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeChat) {
			return
		}

		var req n8nChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			n8nWriteError(w, http.StatusBadRequest, "Invalid request body", "parse_error")
			return
		}

		if req.Message == "" {
			n8nWriteError(w, http.StatusBadRequest, "Message is required", "validation_error")
			return
		}

		start := time.Now()

		// Generate session ID if not provided
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = generateSessionID()
		}

		// Set context window default
		contextWindow := req.ContextWindow
		if contextWindow <= 0 {
			contextWindow = 10
		}

		// Build system prompt
		systemPrompt := req.SystemPrompt
		if systemPrompt == "" {
			systemPrompt = buildDefaultSystemPrompt(s.Cfg)
		}

		// Filter tools if specified
		var allowedTools []string
		s.CfgMu.RLock()
		if len(s.Cfg.N8n.AllowedTools) > 0 {
			allowedTools = s.Cfg.N8n.AllowedTools
		} else if len(req.Tools) > 0 {
			allowedTools = req.Tools
		}
		readonly := s.Cfg.N8n.ReadOnly
		s.CfgMu.RUnlock()

		ctx := context.Background()

		manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
		ff := buildFeatureFlags(s.Cfg)
		toolSchemas := agent.BuildNativeToolSchemas(s.Cfg.Directories.SkillsDir, manifest, ff, s.Logger)

		if len(allowedTools) > 0 {
			filteredSchemas := make([]openai.Tool, 0)
			allowedSet := make(map[string]bool)
			for _, name := range allowedTools {
				allowedSet[name] = true
			}
			for _, schema := range toolSchemas {
				if schema.Function != nil && allowedSet[schema.Function.Name] {
					filteredSchemas = append(filteredSchemas, schema)
				}
			}
			toolSchemas = filteredSchemas
		}

		// Build messages from the isolated n8n session store — does NOT touch the main HistoryManager.
		messages := []openai.ChatCompletionMessage{
			{Role: "system", Content: systemPrompt},
		}
		messages = append(messages, n8nGetSessionMessages(sessionID, contextWindow)...)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    "user",
			Content: req.Message,
		})

		// Multi-turn tool execution loop — run up to maxToolRounds before returning.
		var toolCalls []n8nToolCall
		var finalContent string
		totalTokens := 0
		const maxToolRounds = 5
		for range maxToolRounds {
			llmResp, tokens, err := callLLMWithTools(ctx, s, messages, toolSchemas)
			if err != nil {
				s.Logger.Error("[n8n] LLM chat failed", "error", err)
				n8nWriteError(w, http.StatusInternalServerError, "Agent processing failed", "agent_error")
				return
			}
			totalTokens += tokens

			if len(llmResp.ToolCalls) == 0 {
				// LLM returned a text answer — done.
				finalContent = llmResp.Content
				break
			}

			// Append the assistant message that requests tool calls.
			messages = append(messages, *llmResp)

			// Execute each tool and feed results back into the conversation.
			for _, tc := range llmResp.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)

				toolCall := n8nToolCall{
					Name:      tc.Function.Name,
					Arguments: args,
				}

				result := "Tool execution disabled (readonly mode)"
				if !readonly {
					agentTC := agent.ToolCall{
						IsTool: true,
						Action: tc.Function.Name,
						Params: args,
					}
					result = agent.DispatchToolCall(
						ctx, agentTC, s.Cfg, s.Logger, s.LLMClient, s.Vault,
						s.Registry, manifest, s.CronManager, s.MissionManagerV2,
						s.LongTermMem, s.ShortTermMem, s.KG,
						s.InventoryDB, s.InvasionDB, s.CheatsheetDB, s.ImageGalleryDB,
						s.MediaRegistryDB, s.HomepageRegistryDB,
						s.RemoteHub, s.HistoryManager, false, "", s.Guardian, s.LLMGuardian,
						"n8n", s.CoAgentRegistry, s.BudgetTracker, "",
					)
				}
				toolCall.Result = result
				toolCalls = append(toolCalls, toolCall)
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
		}

		// Persist the exchange in the isolated n8n session store.
		n8nStoreSessionMessages(sessionID, req.Message, finalContent)

		duration := time.Since(start).Milliseconds()

		n8nSendWebhook(s, N8nEventAgentResponse, sessionID, map[string]interface{}{
			"message":     req.Message,
			"response":    finalContent,
			"tool_calls":  toolCalls,
			"tokens_used": totalTokens,
		})

		resp := n8nChatResponse{
			Response:   finalContent,
			SessionID:  sessionID,
			ToolCalls:  toolCalls,
			TokensUsed: totalTokens,
			DurationMs: duration,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handleN8nToolsList returns available tools
func handleN8nToolsList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeRead) {
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		manifest := tools.NewManifest(cfg.Directories.ToolsDir)
		ff := buildFeatureFlags(cfg)
		toolSchemas := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, s.Logger)

		// Filter by allowed tools if configured
		allowedTools := cfg.N8n.AllowedTools
		if len(allowedTools) > 0 {
			allowedSet := make(map[string]bool)
			for _, name := range allowedTools {
				allowedSet[name] = true
			}
			filtered := make([]openai.Tool, 0)
			for _, schema := range toolSchemas {
				if schema.Function != nil && allowedSet[schema.Function.Name] {
					filtered = append(filtered, schema)
				}
			}
			toolSchemas = filtered
		}

		// Convert to n8n format
		type toolInfo struct {
			Name        string      `json:"name"`
			Description string      `json:"description"`
			Parameters  interface{} `json:"parameters"`
		}

		tools := make([]toolInfo, 0, len(toolSchemas))
		for _, schema := range toolSchemas {
			if schema.Function != nil {
				params := schema.Function.Parameters
				if params == nil {
					params = map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					}
				}
				tools = append(tools, toolInfo{
					Name:        schema.Function.Name,
					Description: schema.Function.Description,
					Parameters:  params,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tools": tools,
		})
	}
}

// handleN8nToolExecute handles direct tool execution
func handleN8nToolExecute(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeTools) {
			return
		}

		// Extract tool name from URL
		path := strings.TrimPrefix(r.URL.Path, "/api/n8n/tools/")
		toolName := strings.Split(path, "/")[0]
		if toolName == "" {
			n8nWriteError(w, http.StatusBadRequest, "Tool name is required", "validation_error")
			return
		}

		// Check readonly mode
		s.CfgMu.RLock()
		readonly := s.Cfg.N8n.ReadOnly
		allowedTools := s.Cfg.N8n.AllowedTools
		s.CfgMu.RUnlock()

		if readonly {
			n8nWriteError(w, http.StatusForbidden, "Tool execution disabled (readonly mode)", "readonly")
			return
		}

		// Check if tool is allowed
		if len(allowedTools) > 0 {
			found := false
			for _, name := range allowedTools {
				if name == toolName {
					found = true
					break
				}
			}
			if !found {
				n8nWriteError(w, http.StatusForbidden, "Tool not in allowed list", "not_allowed")
				return
			}
		}

		var req n8nToolExecuteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			n8nWriteError(w, http.StatusBadRequest, "Invalid request body", "parse_error")
			return
		}

		// Set timeout default
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 60
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		// Build tool call
		tc := agent.ToolCall{
			IsTool: true,
			Action: toolName,
			Params: req.Parameters,
		}

		// Marshal params for ToolCall fields
		if len(req.Parameters) > 0 {
			argBytes, _ := json.Marshal(req.Parameters)
			json.Unmarshal(argBytes, &tc)
			tc.Action = toolName
			tc.IsTool = true
			tc.Params = req.Parameters
		}

		// Execute tool
		manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
		result := agent.DispatchToolCall(
			ctx, tc, s.Cfg, s.Logger, s.LLMClient, s.Vault,
			s.Registry, manifest, s.CronManager, s.MissionManagerV2,
			s.LongTermMem, s.ShortTermMem, s.KG,
			s.InventoryDB, s.InvasionDB, s.CheatsheetDB, s.ImageGalleryDB,
			s.MediaRegistryDB, s.HomepageRegistryDB,
			s.RemoteHub, s.HistoryManager, false, "", s.Guardian, s.LLMGuardian,
			"n8n", s.CoAgentRegistry, s.BudgetTracker, "",
		)

		// Send webhook
		n8nSendWebhook(s, N8nEventAgentToolCall, "", map[string]interface{}{
			"tool_name":  toolName,
			"parameters": req.Parameters,
			"result":     result,
		})

		status := "success"
		if strings.Contains(result, "ERROR") || strings.Contains(result, "error") {
			status = "error"
		}

		resp := n8nToolExecuteResponse{
			Result: result,
			Status: status,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handleN8nMemorySearch searches agent memory
func handleN8nMemorySearch(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeMemory) {
			return
		}

		var req n8nMemorySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			n8nWriteError(w, http.StatusBadRequest, "Invalid request body", "parse_error")
			return
		}

		if req.Query == "" {
			n8nWriteError(w, http.StatusBadRequest, "Query is required", "validation_error")
			return
		}

		if req.Limit <= 0 {
			req.Limit = 10
		}

		results := make([]map[string]interface{}, 0)

		switch req.Type {
		case "short_term":
			// Search short term memory (SQLite)
			history := s.HistoryManager.GetAll()
			for _, msg := range history {
				content := ""
				if msg.ChatCompletionMessage.Content != "" {
					content = msg.ChatCompletionMessage.Content
				}
				if strings.Contains(strings.ToLower(content), strings.ToLower(req.Query)) {
					results = append(results, map[string]interface{}{
						"type":    "short_term",
						"role":    msg.ChatCompletionMessage.Role,
						"content": content,
					})
					if len(results) >= req.Limit {
						break
					}
				}
			}

		case "long_term":
			// Search long term memory (Vector DB)
			if s.LongTermMem != nil {
				contents, distances, err := s.LongTermMem.SearchSimilar(req.Query, req.Limit, "tool_guides")
				if err == nil {
					for i, content := range contents {
						distance := ""
						if i < len(distances) {
							distance = distances[i]
						}
						results = append(results, map[string]interface{}{
							"type":     "long_term",
							"content":  content,
							"distance": distance,
						})
					}
				}
			}

		case "knowledge_graph":
			// Query knowledge graph - returns formatted string
			if s.KG != nil {
				searchResult := s.KG.Search(req.Query)
				results = append(results, map[string]interface{}{
					"type":    "knowledge_graph",
					"content": searchResult,
				})
			}

		default:
			// Search all: short_term then long_term then knowledge_graph.
			for _, msg := range s.HistoryManager.GetAll() {
				content := msg.ChatCompletionMessage.Content
				if !msg.IsInternal && strings.Contains(strings.ToLower(content), strings.ToLower(req.Query)) {
					results = append(results, map[string]interface{}{
						"type":    "short_term",
						"role":    msg.ChatCompletionMessage.Role,
						"content": content,
					})
					if len(results) >= req.Limit {
						break
					}
				}
			}
			if s.LongTermMem != nil && len(results) < req.Limit {
				contents, distances, err := s.LongTermMem.SearchSimilar(req.Query, req.Limit-len(results), "tool_guides")
				if err == nil {
					for i, content := range contents {
						distance := ""
						if i < len(distances) {
							distance = distances[i]
						}
						results = append(results, map[string]interface{}{
							"type":     "long_term",
							"content":  content,
							"distance": distance,
						})
					}
				}
			}
			if s.KG != nil {
				if searchResult := s.KG.Search(req.Query); searchResult != "" {
					results = append(results, map[string]interface{}{
						"type":    "knowledge_graph",
						"content": searchResult,
					})
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"query":   req.Query,
			"type":    req.Type,
			"results": results,
			"count":   len(results),
		})
	}
}

// handleN8nMemoryStore stores information in memory
func handleN8nMemoryStore(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeMemory) {
			return
		}

		// Check readonly mode
		s.CfgMu.RLock()
		readonly := s.Cfg.N8n.ReadOnly
		s.CfgMu.RUnlock()

		if readonly {
			n8nWriteError(w, http.StatusForbidden, "Memory storage disabled (readonly mode)", "readonly")
			return
		}

		var req n8nMemoryStoreRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			n8nWriteError(w, http.StatusBadRequest, "Invalid request body", "parse_error")
			return
		}

		if req.Content == "" {
			n8nWriteError(w, http.StatusBadRequest, "Content is required", "validation_error")
			return
		}

		var stored bool
		var err error

		switch req.Type {
		case "short_term":
			// Add to history
			err = s.HistoryManager.Add("system", req.Content, 0, false, true)
			stored = err == nil

		case "long_term":
			// Store in the vector DB; fall back to conversation history if unavailable.
			if s.LongTermMem != nil {
				_, err = s.LongTermMem.StoreDocument(req.Content, req.Content)
			} else {
				err = s.HistoryManager.Add("system", req.Content, 0, false, true)
			}
			stored = err == nil

		case "core":
			// Store in core memory - use short term as fallback
			err = s.HistoryManager.Add("system", req.Content, 0, true, true)
			stored = err == nil

		default:
			n8nWriteError(w, http.StatusBadRequest, "Invalid memory type", "validation_error")
			return
		}

		if err != nil {
			s.Logger.Error("[n8n] Memory store failed", "error", err)
			n8nWriteError(w, http.StatusInternalServerError, "Failed to store memory", "storage_error")
			return
		}

		// Send webhook
		n8nSendWebhook(s, N8nEventMemoryStored, "", map[string]interface{}{
			"type":     req.Type,
			"content":  req.Content,
			"metadata": req.Metadata,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stored":  stored,
			"type":    req.Type,
			"content": req.Content,
		})
	}
}

// handleN8nMissionCreate creates and optionally runs a mission
func handleN8nMissionCreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !n8nCheckEnabled(s, w) {
			return
		}

		if !n8nAuthorize(s, w, r, N8nScopeMissions) {
			return
		}

		// Check readonly mode
		s.CfgMu.RLock()
		readonly := s.Cfg.N8n.ReadOnly
		s.CfgMu.RUnlock()

		if readonly {
			n8nWriteError(w, http.StatusForbidden, "Mission creation disabled (readonly mode)", "readonly")
			return
		}

		var req n8nMissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			n8nWriteError(w, http.StatusBadRequest, "Invalid request body", "parse_error")
			return
		}

		if req.Name == "" {
			n8nWriteError(w, http.StatusBadRequest, "Mission name is required", "validation_error")
			return
		}

		if s.MissionManagerV2 == nil {
			n8nWriteError(w, http.StatusServiceUnavailable, "Mission manager not available", "service_unavailable")
			return
		}

		execType := tools.ExecutionManual
		var triggerType tools.TriggerType
		switch req.Trigger {
		case "schedule":
			execType = tools.ExecutionScheduled
		case "webhook":
			execType = tools.ExecutionTriggered
			triggerType = tools.TriggerWebhook
		}

		prompt := req.Description
		if prompt == "" {
			prompt = req.Name
		}

		m := &tools.MissionV2{
			ID:            "n8n_" + generateSessionID(),
			Name:          req.Name,
			Prompt:        prompt,
			ExecutionType: execType,
			TriggerType:   triggerType,
			Priority:      "medium",
			Enabled:       true,
			Status:        tools.MissionStatusIdle,
			CreatedAt:     time.Now(),
		}

		if err := s.MissionManagerV2.Create(m); err != nil {
			s.Logger.Error("[n8n] Mission create failed", "error", err)
			n8nWriteError(w, http.StatusInternalServerError, "Failed to create mission", "create_error")
			return
		}
		s.Logger.Info("[n8n] Mission created", "name", m.Name, "id", m.ID)

		executionID := ""
		if req.RunNow {
			if err := s.MissionManagerV2.RunNow(m.ID); err != nil {
				s.Logger.Warn("[n8n] Mission RunNow failed", "error", err, "id", m.ID)
			} else {
				executionID = "exec_" + generateSessionID()
				s.Logger.Info("[n8n] Mission queued for execution", "mission_id", m.ID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mission_id":   m.ID,
			"execution_id": executionID,
			"status":       "created",
			"run_now":      req.RunNow,
		})
	}
}

// handleN8nToken manages the n8n API token
func handleN8nToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Vault == nil {
			http.Error(w, "Vault not configured", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			token, err := s.Vault.ReadSecret(n8nVaultTokenKey)
			if err != nil || token == "" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"token": ""})
				return
			}
			// Return masked version
			masked := token[:4] + "••••••••" + token[len(token)-4:]
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"token": masked})

		case http.MethodPost:
			// Generate new token
			rawToken, err := generateN8nToken()
			if err != nil {
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
				return
			}
			if err := s.Vault.WriteSecret(n8nVaultTokenKey, rawToken); err != nil {
				http.Error(w, "Failed to store token", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[n8n] New API token generated")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"token": rawToken})

		case http.MethodDelete:
			if err := s.Vault.DeleteSecret(n8nVaultTokenKey); err != nil {
				http.Error(w, "Failed to delete token", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[n8n] API token deleted")
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// Helper functions

func n8nCheckEnabled(s *Server, w http.ResponseWriter) bool {
	s.CfgMu.RLock()
	enabled := s.Cfg.N8n.Enabled
	s.CfgMu.RUnlock()

	if !enabled {
		http.Error(w, "n8n integration is disabled", http.StatusNotFound)
		return false
	}
	return true
}

func n8nAuthenticate(s *Server, r *http.Request, requiredScope string) bool {
	s.CfgMu.RLock()
	requireToken := s.Cfg.N8n.RequireToken
	mainAuthEnabled := s.Cfg.Auth.Enabled
	sessionSecret := s.Cfg.Auth.SessionSecret
	allowedScopes := s.Cfg.N8n.Scopes
	s.CfgMu.RUnlock()

	// If token auth is not required and global auth is off, only enforce scope restrictions.
	if !requireToken && !mainAuthEnabled {
		return n8nScopeAllowed(allowedScopes, requiredScope)
	}

	// Check Bearer token.
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != "" && s.Vault != nil {
			stored, err := s.Vault.ReadSecret(n8nVaultTokenKey)
			if err == nil && stored != "" && stored == token {
				return n8nScopeAllowed(allowedScopes, requiredScope)
			}
		}
	}

	// Fall back to session cookie.
	if sessionSecret != "" && IsAuthenticated(r, sessionSecret) {
		return n8nScopeAllowed(allowedScopes, requiredScope)
	}

	return false
}

// n8nScopeAllowed reports whether requiredScope is permitted by the configured scope list.
// An empty list permits all scopes (backwards-compatible default).
func n8nScopeAllowed(scopes []string, required string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, s := range scopes {
		if s == required || s == N8nScopeAdmin {
			return true
		}
	}
	return false
}

// n8nCheckRateLimit enforces RateLimitRPS for n8n endpoints using a per-token sliding window.
func n8nCheckRateLimit(s *Server, r *http.Request) bool {
	s.CfgMu.RLock()
	rps := s.Cfg.N8n.RateLimitRPS
	s.CfgMu.RUnlock()

	if rps <= 0 {
		return true
	}

	// Key on the first 8 chars of the Bearer token, or fall back to the client IP.
	key := ClientIP(r)
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		t := strings.TrimPrefix(auth, "Bearer ")
		if len(t) >= 8 {
			key = "tok:" + t[:8]
		}
	}

	now := time.Now()
	cutoff := now.Add(-time.Second)

	n8nRateMu.Lock()
	defer n8nRateMu.Unlock()

	ts := n8nRateWindows[key]
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	ts = ts[i:]

	if len(ts) >= rps {
		n8nRateWindows[key] = ts
		return false
	}
	n8nRateWindows[key] = append(ts, now)
	return true
}

// n8nAuthorize combines auth check, scope enforcement and rate limiting.
// It writes the appropriate error response and returns false when the request should be rejected.
func n8nAuthorize(s *Server, w http.ResponseWriter, r *http.Request, scope string) bool {
	if !n8nAuthenticate(s, r, scope) {
		n8nWriteError(w, http.StatusUnauthorized, "Unauthorized", "invalid_token")
		return false
	}
	if !n8nCheckRateLimit(s, r) {
		n8nWriteError(w, http.StatusTooManyRequests, "Rate limit exceeded", "rate_limited")
		return false
	}
	return true
}

func n8nWriteError(w http.ResponseWriter, status int, message string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":  message,
		"code":   code,
		"status": status,
	})
}

func generateN8nToken() (string, error) {
	// Generate a secure random token: n8n_ + 32 hex chars
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "n8n_" + hex.EncodeToString(b), nil
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func buildDefaultSystemPrompt(cfg *config.Config) string {
	return fmt.Sprintf("You are %s, an AI assistant. Be helpful and concise.", cfg.Agent.CorePersonality)
}

func buildFeatureFlags(cfg *config.Config) agent.ToolFeatureFlags {
	return agent.ToolFeatureFlags{
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		DockerEnabled:            cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		CoAgentEnabled:           false,
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

func n8nSendWebhook(s *Server, event string, sessionID string, data map[string]interface{}) {
	s.CfgMu.RLock()
	webhookURL := s.Cfg.N8n.WebhookBaseURL
	allowedEvents := s.Cfg.N8n.AllowedEvents
	s.CfgMu.RUnlock()

	if webhookURL == "" {
		return
	}

	// Check if event is allowed
	allowed := false
	for _, e := range allowedEvents {
		if e == event {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}

	// Build payload
	payload := n8nWebhookPayload{
		Event:     event,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		SessionID: sessionID,
		Data:      data,
	}

	// Compute HMAC over the full payload (event+timestamp+session_id+data) to prevent
	// tampering with any field, not just the data object.
	if s.Vault != nil {
		secret, _ := s.Vault.ReadSecret(n8nVaultTokenKey)
		if secret != "" {
			// Marshal the payload without the signature field first.
			type payloadForSig struct {
				Event     string                 `json:"event"`
				Timestamp string                 `json:"timestamp"`
				SessionID string                 `json:"session_id,omitempty"`
				Data      map[string]interface{} `json:"data"`
			}
			sigData, _ := json.Marshal(payloadForSig{
				Event:     payload.Event,
				Timestamp: payload.Timestamp,
				SessionID: payload.SessionID,
				Data:      payload.Data,
			})
			h := hmac.New(sha256.New, []byte(secret))
			h.Write(sigData)
			payload.Signature = hex.EncodeToString(h.Sum(nil))
		}
	}

	// Send async with a bounded timeout so the goroutine cannot leak.
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		jsonPayload, _ := json.Marshal(payload)
		resp, err := client.Post(
			webhookURL+"/webhook/aurago/"+event,
			"application/json",
			strings.NewReader(string(jsonPayload)),
		)
		if err != nil {
			s.Logger.Warn("[n8n] Failed to send webhook", "error", err)
			return
		}
		defer resp.Body.Close()
	}()
}

// callLLMWithTools calls the LLM with tools using the agent's LLM client
func callLLMWithTools(
	ctx context.Context,
	s *Server,
	messages []openai.ChatCompletionMessage,
	tools []openai.Tool,
) (*openai.ChatCompletionMessage, int, error) {
	// Create chat completion request
	req := openai.ChatCompletionRequest{
		Model:       s.Cfg.LLM.Model,
		Messages:    messages,
		Tools:       makeToolsPtr(tools),
		Temperature: float32(s.Cfg.LLM.Temperature),
	}

	// Call LLM
	resp, err := s.LLMClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	if len(resp.Choices) == 0 {
		return nil, 0, fmt.Errorf("no response from LLM")
	}

	return &resp.Choices[0].Message, resp.Usage.TotalTokens, nil
}

func makeToolsPtr(tools []openai.Tool) []openai.Tool {
	if len(tools) == 0 {
		return nil
	}
	return tools
}

// n8nGetSessionMessages returns recent messages from the isolated n8n session store.
func n8nGetSessionMessages(sessionID string, window int) []openai.ChatCompletionMessage {
	n8nSessionMu.Lock()
	defer n8nSessionMu.Unlock()
	n8nPurgeExpiredSessionsLocked()
	msgs := n8nSessions[sessionID]
	if len(msgs) > window {
		msgs = msgs[len(msgs)-window:]
	}
	n8nSessionLast[sessionID] = time.Now()
	return append([]openai.ChatCompletionMessage(nil), msgs...)
}

// n8nStoreSessionMessages persists the user/assistant exchange in the isolated session store.
func n8nStoreSessionMessages(sessionID, userMsg, assistantMsg string) {
	n8nSessionMu.Lock()
	defer n8nSessionMu.Unlock()
	msgs := n8nSessions[sessionID]
	msgs = append(msgs, openai.ChatCompletionMessage{
		Role:    "user",
		Content: userMsg,
	})
	if assistantMsg != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    "assistant",
			Content: assistantMsg,
		})
	}
	if len(msgs) > n8nSessionMaxMsgs {
		msgs = msgs[len(msgs)-n8nSessionMaxMsgs:]
	}
	n8nSessions[sessionID] = msgs
	n8nSessionLast[sessionID] = time.Now()
}

// n8nPurgeExpiredSessionsLocked removes sessions older than n8nSessionTTL.
// Caller must hold n8nSessionMu.
func n8nPurgeExpiredSessionsLocked() {
	cutoff := time.Now().Add(-n8nSessionTTL)
	for id, last := range n8nSessionLast {
		if last.Before(cutoff) {
			delete(n8nSessions, id)
			delete(n8nSessionLast, id)
		}
	}
}
