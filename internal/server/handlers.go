package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/tools"
	promptsembed "aurago/prompts"

	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

var (
	followUpDepths = make(map[string]int)
	muFollowUp     sync.Mutex
)

// sanitizeFilename sanitizes a filename to prevent path traversal and ensure safe names.
func sanitizeFilename(filename string) string {
	// Get base name only
	base := filepath.Base(filename)

	// Remove any path separators
	base = strings.ReplaceAll(base, "/", "")
	base = strings.ReplaceAll(base, "\\", "")

	// Replace spaces with underscores
	base = strings.ReplaceAll(base, " ", "_")

	// Remove null bytes and control characters
	base = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, base)

	// Ensure no ".." remains (block path traversal)
	for strings.Contains(base, "..") {
		base = strings.ReplaceAll(base, "..", "")
	}

	// Validate against allowlist pattern (alphanumeric, dots, dashes, underscores)
	// If it contains suspicious characters, replace with safe default
	safe := true
	for _, r := range base {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_') {
			safe = false
			break
		}
	}

	if !safe || base == "" || base == "." {
		return "upload.bin"
	}

	return base
}

func handleChatCompletions(s *Server, sse *SSEBroadcaster) http.HandlerFunc {
	// Pre-create manifest once — it caches internally and auto-reloads on file changes
	manifest := tools.NewManifest(s.Cfg.Directories.ToolsDir)
	return func(w http.ResponseWriter, r *http.Request) {
		// Maintenance check: Inform the log but allow interaction via agent loop
		inMaintenance := tools.IsBusy()
		if inMaintenance {
			s.Logger.Info("Processing request in Maintenance Mode")
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit request body to 1 MB to prevent abuse
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.Logger.Error("Failed to decode request body", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		s.Logger.Debug("Received chat completion request", "model", req.Model, "messages_count", len(req.Messages), "stream", req.Stream)

		// Check for Follow-Up loop protection
		isFollowUp := r.Header.Get("X-Internal-FollowUp") == "true"
		followUpKey := "default" // sessionID is resolved later; hardcoded for now
		muFollowUp.Lock()
		if !isFollowUp {
			followUpDepths[followUpKey] = 0 // Reset on real user request
		} else {
			followUpDepths[followUpKey]++
			if followUpDepths[followUpKey] > 10 {
				muFollowUp.Unlock()
				s.Logger.Warn("Blocked follow_up execution to prevent infinite loop", "depth", followUpDepths[followUpKey])
				http.Error(w, `{"error": "Follow-up circuit breaker tripped. Max recursion depth reached."}`, http.StatusTooManyRequests)
				return
			}
		}
		muFollowUp.Unlock()

		// Override the model with the configured backend model
		if s.Cfg.LLM.Model != "" {
			s.Logger.Debug("Overriding model", "from", req.Model, "to", s.Cfg.LLM.Model)
			req.Model = s.Cfg.LLM.Model
		}

		if len(req.Messages) == 0 {
			http.Error(w, "No messages provided", http.StatusBadRequest)
			return
		}

		// 1. Save User Input to Short-Term Memory
		lastUserMsg := req.Messages[len(req.Messages)-1]
		sessionID := "default" // hardcoded until API supports it

		// Guardian: Scan user input for injection patterns (log only, never block)
		if lastUserMsg.Role == openai.ChatMessageRoleUser && s.Guardian != nil {
			s.Guardian.ScanUserInput(lastUserMsg.Content)
		}

		// Phase: Command Interception
		if lastUserMsg.Role == openai.ChatMessageRoleUser && strings.HasPrefix(lastUserMsg.Content, "/") {
			// Intercept Slash Commands
			cmdCtx := commands.Context{
				STM:           s.ShortTermMem,
				HM:            s.HistoryManager,
				Vault:         s.Vault,
				InventoryDB:   s.InventoryDB,
				BudgetTracker: s.BudgetTracker,
				Cfg:           s.Cfg,
				PromptsDir:    s.Cfg.Directories.PromptsDir,
			}
			cmdResult, isCommand, err := commands.Handle(lastUserMsg.Content, cmdCtx)
			if err != nil {
				s.Logger.Error("Command execution failed", "error", err)
				http.Error(w, "Command failed", http.StatusInternalServerError)
				return
			}
			if isCommand {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
					ID:      "cmd-" + uuid.New().String(),
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   "aurago-cmd",
					Choices: []openai.ChatCompletionChoice{
						{
							Index: 0,
							Message: openai.ChatCompletionMessage{
								Role:    openai.ChatMessageRoleAssistant,
								Content: cmdResult,
							},
							FinishReason: openai.FinishReasonStop,
						},
					},
				}); err != nil {
					s.Logger.Error("Failed to encode command response", "error", err)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
				return
			}
		}

		if lastUserMsg.Role == openai.ChatMessageRoleUser {
			id, err := s.ShortTermMem.InsertMessage(sessionID, lastUserMsg.Role, lastUserMsg.Content, false, false)
			if err != nil {
				s.Logger.Error("Failed to insert user message", "error", err)
			}
			if sessionID == "default" {
				s.HistoryManager.Add(lastUserMsg.Role, lastUserMsg.Content, id, false, false)
			}
		}

		// 2. Rebuild the Context
		recentMessages := s.HistoryManager.Get()

		// Phase 33: Recursive Context Compression (Character Based)
		charLimit := s.Cfg.Agent.MemoryCompressionCharLimit
		if s.HistoryManager.TotalChars() >= charLimit {
			if s.HistoryManager.TryLockCompression() {
				// Safety Check: Check if pinned messages exceed 50% of the limit
				pinnedChars := s.HistoryManager.TotalPinnedChars()
				if pinnedChars > charLimit/2 {
					s.Logger.Warn("[Compression] Context overcrowded with pinned messages", "pinned_chars", pinnedChars, "limit", charLimit)
					warningMsg := fmt.Sprintf("WARNING: Pinned messages are consuming %d characters, which is over 50%% of your memory limit (%d). Consider unpinning old information to maintain full context reliability.", pinnedChars, charLimit)
					// Inject warning to agent
					id, err := s.ShortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleSystem, warningMsg, false, false)
					if err != nil {
						s.Logger.Error("Failed to insert compression warning", "error", err)
					}
					s.HistoryManager.Add(openai.ChatMessageRoleSystem, warningMsg, id, false, false)
				}

				// We want to compress about 20% of the limit or at least enough to be under the limit
				targetPruneChars := charLimit / 5
				messagesToSummarize, actualChars := s.HistoryManager.GetOldestMessagesForPruning(targetPruneChars)

				if len(messagesToSummarize) > 0 {
					go func(msgs []memory.HistoryMessage, charsPruned int, existingSummary string) {
						defer s.HistoryManager.UnlockCompression()
						defer func() {
							if r := recover(); r != nil {
								s.Logger.Error("[Compression] Goroutine panic recovered", "error", r)
							}
						}()
						s.Logger.Info("[Compression] Triggering character-based context compression",
							"msg_count", len(msgs), "chars", charsPruned, "limit", charLimit)

						prompt := "Update the following 'Persistent Summary' with the details from the 'Recent Messages' below. Maintain a chronological flow of facts, technical decisions, and user preferences. Ensure metadata is explicitly protected. Result must be a concise briefing.\n\n"
						if existingSummary != "" {
							prompt += "[\"Persistent Summary\"]:\n" + existingSummary + "\n\n"
						}
						prompt += "[\"Recent Messages\"]:\n"
						var dropIDs []int64
						for _, m := range msgs {
							prompt += fmt.Sprintf("[%s]: %s\n\n", m.Role, m.Content)
							dropIDs = append(dropIDs, m.ID)
						}

						summaryReq := openai.ChatCompletionRequest{
							Model: s.Cfg.LLM.Model,
							Messages: []openai.ChatCompletionMessage{
								{Role: openai.ChatMessageRoleSystem, Content: prompt},
							},
							MaxTokens:   1000,
							Temperature: 0.3,
						}

						bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
						defer cancel()

						resp, err := llm.ExecuteWithRetry(bgCtx, s.LLMClient, summaryReq, s.Logger, nil)
						if err != nil {
							s.Logger.Error("[Compression] Background summarization failed", "error", err)
							return
						}

						if len(resp.Choices) > 0 {
							newSummary := resp.Choices[0].Message.Content
							s.HistoryManager.SetSummary(newSummary)
							s.HistoryManager.DropMessages(dropIDs)
							// In SQLite we still delete by count for now, or we could update ShortTermMem to delete by ID list
							// For simplicity and since HistoryManager is the source of truth for active context, we'll stick to this.
							// However, stm.DeleteOldMessages might delete pinned ones if we are not careful.
							// Requirement: "rest weiterhin komprimiert wird".
							// Let's add a DeleteMessagesByID to ShortTermMem too.
							if err := s.ShortTermMem.DeleteMessagesByID(sessionID, dropIDs); err != nil {
								s.Logger.Error("[Compression] Failed to clean up SQLite memory", "error", err)
							}
							s.Logger.Info("[Compression] Background summarization complete and saved",
								"summary_len", len(newSummary), "messages_dropped", len(dropIDs))

							// Archive the LLM-distilled summary to VectorDB so it remains
							// semantically searchable via RAG even after chat resets.
							if s.LongTermMem != nil && !s.LongTermMem.IsDisabled() {
								concept := fmt.Sprintf("Gesprächszusammenfassung %s", time.Now().Format("2006-01-02 15:04"))
								go func(concept, summary string) {
									if _, err := s.LongTermMem.StoreDocument(concept, summary); err != nil {
										s.Logger.Warn("[Compression] VectorDB archive of summary failed", "error", err)
									} else {
										s.Logger.Info("[Compression] Summary archived to VectorDB", "concept", concept)
									}
								}(concept, newSummary)
							}
						}
					}(messagesToSummarize, actualChars, s.HistoryManager.GetSummary())
				} else {
					s.HistoryManager.UnlockCompression()
				}
			}
		}

		// 3. Inject Dynamic Core System Prompt with RAG
		var retrievedMemories string
		if lastUserMsg.Role == openai.ChatMessageRoleUser && lastUserMsg.Content != "" {
			memories, docIDs, err := s.LongTermMem.SearchSimilar(lastUserMsg.Content, 2)
			if err == nil {
				for _, docID := range docIDs {
					_ = s.ShortTermMem.UpdateMemoryAccess(docID)
				}
				if len(memories) > 0 {
					retrievedMemories = strings.Join(memories, "\n---\n")
					s.Logger.Debug("RAG: Retrieved memories", "count", len(memories))
				}
			}
		}

		// Load Core Memory (Semi-Static)
		coreMem := s.ShortTermMem.ReadCoreMemory()

		flags := prompts.ContextFlags{
			IsErrorState:           false,
			RequiresCoding:         false,
			RetrievedMemories:      retrievedMemories,
			SystemLanguage:         s.Cfg.Agent.SystemLanguage,
			CorePersonality:        s.Cfg.Agent.CorePersonality,
			LifeboatEnabled:        s.Cfg.Maintenance.LifeboatEnabled,
			IsMaintenanceMode:      inMaintenance,
			TokenBudget:            s.Cfg.Agent.SystemPromptTokenBudget,
			MessageCount:           len(recentMessages),
			DiscordEnabled:         s.Cfg.Discord.Enabled,
			EmailEnabled:           s.Cfg.Email.Enabled,
			DockerEnabled:          s.Cfg.Docker.Enabled,
			HomeAssistantEnabled:   s.Cfg.HomeAssistant.Enabled,
			WebDAVEnabled:          s.Cfg.WebDAV.Enabled,
			KoofrEnabled:           s.Cfg.Koofr.Enabled,
			ChromecastEnabled:      s.Cfg.Chromecast.Enabled,
			CoAgentEnabled:         s.Cfg.CoAgents.Enabled,
			GoogleWorkspaceEnabled: s.Cfg.Agent.EnableGoogleWorkspace,
			ProxmoxEnabled:         s.Cfg.Proxmox.Enabled,
			OllamaEnabled:          s.Cfg.Ollama.Enabled,
			IsEgg:                  s.Cfg.EggMode.Enabled,
		}
		sysPrompt := prompts.BuildSystemPrompt(s.Cfg.Directories.PromptsDir, flags, coreMem, s.Logger)

		finalMessages := []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		}

		currentSummary := s.HistoryManager.GetSummary()
		if currentSummary != "" {
			finalMessages = append(finalMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + currentSummary,
			})
		}

		finalMessages = append(finalMessages, recentMessages...)

		// First-start: inject a one-time naming prompt so the agent asks the user
		// for a personal name on the very first conversation.
		if s.IsFirstStart {
			s.muFirstStart.Lock()
			if !s.firstStartDone {
				s.firstStartDone = true
				s.muFirstStart.Unlock()
				s.Logger.Info("[FirstStart] Injecting one-time naming prompt")
				finalMessages = append(finalMessages, openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleSystem,
					Content: "[FIRST START INITIALIZATION — ONE TIME ONLY] " +
						"YOU are the AI assistant. YOU do not yet have a name. " +
						"Before responding to the user's message, ask the USER to give YOU (the AI) a personal name. " +
						"Example: 'Bevor wir loslegen – magst du mir einen Namen geben? Oder soll ich mir selbst einen aussuchen?' " +
						"IMPORTANT: You are asking the user to name YOU, the AI — NOT asking them for their own name, " +
						"and NOT offering to give the user a name. " +
						"Wait for the user's answer, then settle on a name for yourself. " +
						"Immediately after the name is decided, save it permanently to core memory " +
						"using the manage_memory tool (operation \"add\", fact: \"My name is <chosen_name>\"). " +
						"Do not skip this step.",
				})
			} else {
				s.muFirstStart.Unlock()
			}
		}

		req.Messages = finalMessages

		// 4. Pass execution to the unified agent loop
		runCfg := agent.RunConfig{
			Config:          s.Cfg,
			Logger:          s.Logger,
			LLMClient:       s.LLMClient,
			ShortTermMem:    s.ShortTermMem,
			HistoryManager:  s.HistoryManager,
			LongTermMem:     s.LongTermMem,
			KG:              s.KG,
			InventoryDB:     s.InventoryDB,
			InvasionDB:      s.InvasionDB,
			Vault:           s.Vault,
			Registry:        s.Registry,
			Manifest:        manifest,
			CronManager:     s.CronManager,
			MissionManager:  s.MissionManager,
			CoAgentRegistry: s.CoAgentRegistry,
			BudgetTracker:   s.BudgetTracker,
			SessionID:       sessionID,
			IsMaintenance:   inMaintenance,
			SurgeryPlan:     "", // UI-driven chats don't currently pass a formal surgery plan
		}

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				s.Logger.Error("Streaming not supported by ResponseWriter")
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}
			// Initial flush to establish SSE connection
			flusher.Flush()

			_, err := agent.ExecuteAgentLoop(r.Context(), req, runCfg, true, sse)
			if err != nil {
				s.Logger.Error("Streamed agent loop failed", "error", err)
				return
			}

			// Conclude SSE stream nicely
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()

		} else {
			resp, err := agent.ExecuteAgentLoop(r.Context(), req, runCfg, false, sse)
			if err != nil {
				s.Logger.Error("Sync agent loop failed", "error", err)
				// Return a user-visible error as a proper OpenAI response instead of HTTP 500
				errMsg := "⚠️ The request timed out — the model did not respond in time. Please try again or switch to a faster model."
				if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
					errMsg = "⚠️ An internal error occurred. Check server logs for details."
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
					ID:      "err-" + sessionID,
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   "aurago",
					Choices: []openai.ChatCompletionChoice{{
						Index: 0,
						Message: openai.ChatCompletionMessage{
							Role:    openai.ChatMessageRoleAssistant,
							Content: errMsg,
						},
						FinishReason: openai.FinishReasonStop,
					}},
				})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}
}

func handleArchiveMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit request body to 10 MB for batch archive uploads
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			s.Logger.Error("Failed to read archive request body", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		trimmed := strings.TrimSpace(string(bodyBytes))

		if strings.HasPrefix(trimmed, "[") {
			var items []memory.ArchiveItem
			if err := json.Unmarshal(bodyBytes, &items); err != nil {
				s.Logger.Error("Failed to decode batch archive request", "error", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			if len(items) == 0 {
				http.Error(w, "Empty batch", http.StatusBadRequest)
				return
			}

			storedIDs, err := s.LongTermMem.StoreBatch(items)
			if err != nil {
				s.Logger.Error("Failed to archive batch", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			for _, id := range storedIDs {
				_ = s.ShortTermMem.UpsertMemoryMeta(id)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "archived": len(items)})
		} else {
			var req memory.ArchiveItem
			if err := json.Unmarshal(bodyBytes, &req); err != nil {
				s.Logger.Error("Failed to decode archive request", "error", err)
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			if req.Concept == "" || req.Content == "" {
				http.Error(w, "Both 'concept' and 'content' are required", http.StatusBadRequest)
				return
			}

			storedIDs, err := s.LongTermMem.StoreDocument(req.Concept, req.Content)
			if err != nil {
				s.Logger.Error("Failed to archive memory", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			for _, id := range storedIDs {
				_ = s.ShortTermMem.UpsertMemoryMeta(id)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "concept": req.Concept})
		}
	}
}

func handleInterrupt(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.Logger.Warn("Stop requested via Web UI")

		agent.InterruptSession("default")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": "Agent interrupted. It will stop after the current step.",
		})
	}
}

// handleUpload receives a multipart file upload and saves it to
// {workspace_dir}/attachments/, returning the agent-visible path.
func handleUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 32 MB max upload size
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, "failed to parse form", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Sanitize filename - prevent path traversal and ensure safe name
		base := sanitizeFilename(header.Filename)

		ts := time.Now().Format("20060102_150405")
		filename := ts + "_" + base

		// Save to {workspace_dir}/attachments/
		attachDir := filepath.Join(s.Cfg.Directories.WorkspaceDir, "attachments")
		if err := os.MkdirAll(attachDir, 0755); err != nil {
			s.Logger.Error("Failed to create attachments dir", "error", err)
			http.Error(w, "failed to create dir", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(attachDir, filename)
		dst, err := os.Create(destPath)
		if err != nil {
			s.Logger.Error("Failed to create upload file", "error", err)
			http.Error(w, "failed to write file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			s.Logger.Error("Failed to write uploaded file", "error", err)
			http.Error(w, "failed to save file", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("File uploaded via Web UI", "filename", filename, "size", header.Size)

		// Return the path the agent should use (relative to project root)
		agentPath := "agent_workspace/workdir/attachments/" + filename

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"path":     agentPath,
			"filename": header.Filename,
		})
	}
}

// handleBudgetStatus returns the current budget status as JSON.
func handleBudgetStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if s.BudgetTracker == nil {
			w.Write([]byte(`{"enabled": false}`))
			return
		}
		w.Write([]byte(s.BudgetTracker.GetStatusJSON()))
	}
}

// handleOpenRouterCredits returns the OpenRouter credit balance as JSON.
func handleOpenRouterCredits(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		provider := strings.ToLower(s.Cfg.LLM.ProviderType)
		if provider != "openrouter" {
			w.Write([]byte(`{"available":false,"reason":"provider is not openrouter"}`))
			return
		}
		credits, err := llm.FetchOpenRouterCredits(s.Cfg.LLM.APIKey, s.Cfg.LLM.BaseURL)
		if err != nil {
			s.Logger.Error("Failed to fetch OpenRouter credits", "error", err)
			w.Write([]byte(fmt.Sprintf(`{"available":true,"error":%q}`, err.Error())))
			return
		}
		data, _ := json.Marshal(map[string]interface{}{
			"available":    true,
			"balance":      credits.Balance,
			"usage":        credits.Usage,
			"limit":        credits.Limit,
			"is_free_tier": credits.IsFreeTier,
		})
		w.Write(data)
	}
}

// isCorePersonality reports whether name is a built-in persona shipped in the
// embedded FS. Core personas are read-only and must not be overwritten or deleted.
func isCorePersonality(name string) bool {
	_, err := promptsembed.FS.Open("personalities/" + name + ".md")
	return err == nil
}

// PersonalityEntry describes a single persona for the API response.
type PersonalityEntry struct {
	Name string `json:"name"`
	Core bool   `json:"core"`
}

func handleListPersonalities(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Seed with embedded core personalities (always present in binary).
		profiles := []PersonalityEntry{}
		seen := map[string]bool{}
		if embFiles, err := promptsembed.FS.ReadDir("personalities"); err == nil {
			for _, f := range embFiles {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
					n := strings.TrimSuffix(f.Name(), ".md")
					profiles = append(profiles, PersonalityEntry{Name: n, Core: true})
					seen[n] = true
				}
			}
		}

		// Add user-created personalities from disk (not already in embedded set).
		personalitiesDir := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities")
		if files, err := os.ReadDir(personalitiesDir); err == nil {
			for _, f := range files {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
					n := strings.TrimSuffix(f.Name(), ".md")
					if !seen[n] {
						profiles = append(profiles, PersonalityEntry{Name: n, Core: false})
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active":        s.Cfg.Agent.CorePersonality,
			"personalities": profiles,
		})
	}
}

func handlePersonalityState(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.Agent.PersonalityEngine {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"enabled": false})
			return
		}

		traits, err := s.ShortTermMem.GetTraits()
		if err != nil {
			s.Logger.Error("Failed to get personality traits", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		mood := s.ShortTermMem.GetCurrentMood()
		trigger := s.ShortTermMem.GetLastMoodTrigger()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": true,
			"mood":    string(mood),
			"trigger": trigger,
			"traits":  traits,
		})
	}
}

func handleUpdatePersonality(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		if req.ID == "" {
			http.Error(w, "Personality ID is required", http.StatusBadRequest)
			return
		}

		// Verify existence — accept personality from disk or from embedded binary.
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", req.ID+".md")
		if _, err := os.Stat(profilePath); os.IsNotExist(err) {
			if !isCorePersonality(req.ID) {
				http.Error(w, "Personality not found", http.StatusNotFound)
				return
			}
		}

		// Update config
		s.Cfg.Agent.CorePersonality = req.ID

		// Save config
		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			configPath = "config.yaml"
		}
		if err := s.Cfg.Save(configPath); err != nil {
			s.Logger.Error("Failed to save config", "error", err)
			http.Error(w, "Failed to persist configuration", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("Core personality updated", "id", req.ID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "active": req.ID})
	}
}

// handlePersonalityFeedback allows the user to send reward/punishment signals
// via mood buttons (thumbs up, thumbs down, angry) to adjust personality traits.
func handlePersonalityFeedback(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.Agent.PersonalityEngine {
			http.Error(w, "Personality engine is disabled", http.StatusBadRequest)
			return
		}

		var req struct {
			Type string `json:"type"` // "positive", "negative", "angry"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		type traitDelta struct {
			trait string
			delta float64
		}

		var deltas []traitDelta
		var mood memory.Mood
		var trigger string

		switch req.Type {
		case "positive":
			deltas = []traitDelta{
				{memory.TraitConfidence, 0.05},
				{memory.TraitAffinity, 0.05},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodFocused
			trigger = "user positive feedback (thumbs up)"
		case "negative":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.03},
				{memory.TraitAffinity, -0.03},
				{memory.TraitThoroughness, 0.02},
			}
			mood = memory.MoodCautious
			trigger = "user negative feedback (thumbs down)"
		case "angry":
			deltas = []traitDelta{
				{memory.TraitConfidence, -0.06},
				{memory.TraitAffinity, -0.06},
				{memory.TraitEmpathy, 0.04},
			}
			mood = memory.MoodCautious
			trigger = "user angry feedback"
		case "laughing":
			deltas = []traitDelta{
				{memory.TraitAffinity, 0.05},
				{memory.TraitCreativity, 0.03},
				{memory.TraitEmpathy, 0.02},
			}
			mood = memory.MoodPlayful
			trigger = "user laughing feedback"
		case "crying":
			deltas = []traitDelta{
				{memory.TraitEmpathy, 0.08},
				{memory.TraitConfidence, -0.05},
				{memory.TraitLoneliness, 0.05},
			}
			mood = memory.MoodCautious
			trigger = "user crying feedback"
		case "amazed":
			deltas = []traitDelta{
				{memory.TraitCuriosity, 0.08},
				{memory.TraitCreativity, 0.05},
			}
			mood = memory.MoodCurious
			trigger = "user amazed feedback"
		default:
			http.Error(w, "Invalid feedback type. Use: positive, negative, angry, laughing, crying, amazed", http.StatusBadRequest)
			return
		}

		for _, d := range deltas {
			if err := s.ShortTermMem.UpdateTrait(d.trait, d.delta); err != nil {
				s.Logger.Error("Failed to update trait", "trait", d.trait, "error", err)
			}
		}

		if err := s.ShortTermMem.LogMood(mood, trigger); err != nil {
			s.Logger.Error("Failed to log mood", "error", err)
		}

		s.Logger.Info("Personality feedback applied", "type", req.Type, "mood", string(mood))

		// Return updated state
		traits, _ := s.ShortTermMem.GetTraits()
		currentMood := s.ShortTermMem.GetCurrentMood()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"type":   req.Type,
			"mood":   string(currentMood),
			"traits": traits,
		})
	}
}

// isValidPersonalityName checks that a personality name is safe (no path traversal, no special chars).
func isValidPersonalityName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// handleGetPersonalityContent returns the markdown body and parsed meta of a personality file.
// GET /api/config/personality-files?name=NAME
func handleGetPersonalityContent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			http.Error(w, "Invalid personality name", http.StatusBadRequest)
			return
		}
		// Try disk first (user override), then fall back to embedded binary.
		var data []byte
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", name+".md")
		if d, err := os.ReadFile(profilePath); err == nil {
			data = d
		} else if d, err := promptsembed.FS.ReadFile("personalities/" + name + ".md"); err == nil {
			data = d
		} else {
			http.Error(w, "Personality not found", http.StatusNotFound)
			return
		}

		// Split YAML front matter from body
		type metaFields struct {
			Volatility               float64 `json:"volatility"`
			EmpathyBias              float64 `json:"empathy_bias"`
			ConflictResponse         string  `json:"conflict_response"`
			LonelinessSusceptibility float64 `json:"loneliness_susceptibility"`
			TraitDecayRate           float64 `json:"trait_decay_rate"`
		}
		meta := metaFields{
			Volatility:               1.0,
			EmpathyBias:              1.0,
			ConflictResponse:         "neutral",
			LonelinessSusceptibility: 1.0,
			TraitDecayRate:           1.0,
		}
		body := strings.TrimSpace(string(data))

		if strings.HasPrefix(body, "---") {
			// Find closing ---
			rest := body[3:]
			if idx := strings.Index(rest, "\n---"); idx != -1 {
				yamlPart := strings.TrimSpace(rest[:idx])
				body = strings.TrimSpace(rest[idx+4:])

				// Parse relevant fields with simple line scanning
				for _, line := range strings.Split(yamlPart, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "volatility:") {
						fmt.Sscanf(strings.TrimPrefix(line, "volatility:"), " %f", &meta.Volatility)
					} else if strings.HasPrefix(line, "empathy_bias:") {
						fmt.Sscanf(strings.TrimPrefix(line, "empathy_bias:"), " %f", &meta.EmpathyBias)
					} else if strings.HasPrefix(line, "loneliness_susceptibility:") {
						fmt.Sscanf(strings.TrimPrefix(line, "loneliness_susceptibility:"), " %f", &meta.LonelinessSusceptibility)
					} else if strings.HasPrefix(line, "trait_decay_rate:") {
						fmt.Sscanf(strings.TrimPrefix(line, "trait_decay_rate:"), " %f", &meta.TraitDecayRate)
					} else if strings.HasPrefix(line, "conflict_response:") {
						val := strings.Trim(strings.TrimPrefix(line, "conflict_response:"), " \"'")
						if val != "" {
							meta.ConflictResponse = val
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": name,
			"body": body,
			"meta": meta,
		})
	}
}

// handleSavePersonalityFile creates or updates a personality file.
// POST /api/config/personality-files  body: {"name":"...", "content":"..."}
func handleSavePersonalityFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		if !isValidPersonalityName(req.Name) {
			http.Error(w, "Invalid personality name: use letters, digits, - and _ only (max 64 chars)", http.StatusBadRequest)
			return
		}
		// Core personas shipped with the binary are read-only.
		if isCorePersonality(req.Name) {
			http.Error(w, "Core personality '"+req.Name+"' is read-only and cannot be modified.", http.StatusForbidden)
			return
		}
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", req.Name+".md")
		if err := os.WriteFile(profilePath, []byte(req.Content), 0644); err != nil {
			s.Logger.Error("Failed to write personality file", "name", req.Name, "error", err)
			http.Error(w, "Failed to save personality file", http.StatusInternalServerError)
			return
		}
		s.Logger.Info("Personality file saved", "name", req.Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "name": req.Name})
	}
}

// handleDeletePersonalityFile removes a personality file.
// DELETE /api/config/personality-files?name=NAME
func handleDeletePersonalityFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if !isValidPersonalityName(name) {
			http.Error(w, "Invalid personality name", http.StatusBadRequest)
			return
		}
		// Core personas are read-only — they live in the embedded binary.
		if isCorePersonality(name) {
			http.Error(w, "Core personality '"+name+"' is read-only and cannot be deleted.", http.StatusForbidden)
			return
		}
		// Prevent deleting the currently active personality
		if strings.EqualFold(name, s.Cfg.Agent.CorePersonality) {
			http.Error(w, "Cannot delete the currently active personality", http.StatusConflict)
			return
		}
		profilePath := filepath.Join(s.Cfg.Directories.PromptsDir, "personalities", name+".md")
		if err := os.Remove(profilePath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "Personality not found", http.StatusNotFound)
			} else {
				s.Logger.Error("Failed to delete personality file", "name", name, "error", err)
				http.Error(w, "Failed to delete personality", http.StatusInternalServerError)
			}
			return
		}
		s.Logger.Info("Personality file deleted", "name", name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
