package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/i18n"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"aurago/internal/uid"

	"github.com/sashabaranov/go-openai"
)

var (
	followUpDepths        = make(map[string]int)
	muFollowUp            sync.Mutex
	sessionRequestLocks   = make(map[string]*sync.Mutex)
	muSessionRequestLocks sync.Mutex
)

func lockSessionRequest(sessionID string) func() {
	muSessionRequestLocks.Lock()
	lock := sessionRequestLocks[sessionID]
	if lock == nil {
		lock = &sync.Mutex{}
		sessionRequestLocks[sessionID] = lock
	}
	muSessionRequestLocks.Unlock()
	lock.Lock()
	return func() {
		lock.Unlock()
		// Remove per-mission entries after use so the map does not grow unboundedly.
		if sessionID != "default" {
			muSessionRequestLocks.Lock()
			delete(sessionRequestLocks, sessionID)
			muSessionRequestLocks.Unlock()
		}
	}
}

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

func isActiveContentExtension(filename string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".html", ".htm", ".js", ".mjs", ".svg", ".xml", ".xhtml":
		return true
	default:
		return false
	}
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
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		// Limit request body to 1 MB to prevent abuse
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.Logger.Error("Failed to decode request body", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_bad_request"), http.StatusBadRequest)
			return
		}

		s.Logger.Debug("Received chat completion request", "model", req.Model, "messages_count", len(req.Messages), "stream", req.Stream)

		// Check for Follow-Up loop protection
		isFollowUp := r.Header.Get("X-Internal-FollowUp") == "true"
		missionID := r.Header.Get("X-Mission-ID")
		followUpKey := "default"
		if missionID != "" {
			followUpKey = "mission-" + missionID
		}
		muFollowUp.Lock()
		if !isFollowUp {
			delete(followUpDepths, followUpKey) // cleanup on real user request
		} else {
			followUpDepths[followUpKey]++
			if followUpDepths[followUpKey] > 10 {
				muFollowUp.Unlock()
				s.Logger.Warn("Blocked follow_up execution to prevent infinite loop", "depth", followUpDepths[followUpKey], "key", followUpKey)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_followup_circuit_breaker"), http.StatusTooManyRequests)
				return
			}
		}
		muFollowUp.Unlock()
		// Decrement the follow-up depth when this request is done; clean up mission entries.
		if isFollowUp && followUpKey != "default" {
			defer func() {
				muFollowUp.Lock()
				if followUpDepths[followUpKey] > 0 {
					followUpDepths[followUpKey]--
				}
				if followUpDepths[followUpKey] == 0 {
					delete(followUpDepths, followUpKey)
				}
				muFollowUp.Unlock()
			}()
		}

		// Override the model with the configured backend model
		s.CfgMu.RLock()
		overrideModel := s.Cfg.LLM.Model
		s.CfgMu.RUnlock()
		if overrideModel != "" {
			s.Logger.Debug("Overriding model", "from", req.Model, "to", overrideModel)
			req.Model = overrideModel
		}

		if len(req.Messages) == 0 {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_no_messages"), http.StatusBadRequest)
			return
		}

		// 1. Save User Input to Short-Term Memory
		lastUserMsg := req.Messages[len(req.Messages)-1]
		sessionID := "default"
		if missionID != "" {
			sessionID = "mission-" + missionID
		}
		// Support chat session switching via X-Session-ID header
		if chatSessionID := r.Header.Get("X-Session-ID"); chatSessionID != "" {
			sessionID = chatSessionID
		}
		unlockSession := lockSessionRequest(sessionID)
		defer unlockSession()

		// Guardian: Scan user input for injection patterns (log only, never block)
		if lastUserMsg.Role == openai.ChatMessageRoleUser && s.Guardian != nil {
			s.Guardian.ScanUserInput(lastUserMsg.Content)
		}

		// Phase: Command Interception
		if lastUserMsg.Role == openai.ChatMessageRoleUser && strings.HasPrefix(lastUserMsg.Content, "/") {
			// Intercept Slash Commands
			cmdCtx := commands.Context{
				STM:              s.ShortTermMem,
				HM:               s.HistoryManager,
				Vault:            s.Vault,
				InventoryDB:      s.InventoryDB,
				BudgetTracker:    s.BudgetTracker,
				Cfg:              s.Cfg,
				PromptsDir:       s.Cfg.Directories.PromptsDir,
				WarningsRegistry: s.WarningsRegistry,
				Lang:             s.Cfg.Server.UILanguage,
			}
			cmdResult, isCommand, err := commands.Handle(lastUserMsg.Content, cmdCtx)
			if err != nil {
				s.Logger.Error("Command execution failed", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_command_failed"), http.StatusInternalServerError)
				return
			}
			if isCommand {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
					ID:      "cmd-" + uid.New(),
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
					jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_internal_error"), http.StatusInternalServerError)
				}
				return
			}
		}

		if lastUserMsg.Role == openai.ChatMessageRoleUser {
			id, err := s.ShortTermMem.InsertMessage(sessionID, lastUserMsg.Role, lastUserMsg.Content, false, false)
			if err != nil {
				s.Logger.Error("Failed to insert user message", "error", err)
			}
			agent.NoteInnerVoiceUserTurn(sessionID)
			if sessionID == "default" {
				// Persist the raw text message (including attachment paths) so we
				// don't bloat history.json with base64-encoded images. Multimodal
				// promotion happens only when building the outgoing LLM request.
				s.HistoryManager.Add(lastUserMsg.Role, lastUserMsg.Content, id, false, false)
			}
			// Update session preview and touch timestamp
			_ = s.ShortTermMem.UpdateChatSessionPreview(sessionID)
			_ = s.ShortTermMem.TouchChatSession(sessionID)
		}

		// 2. Rebuild the Context
		// For non-default chat sessions, build context from SQLite instead of HistoryManager
		var recentMessages []openai.ChatCompletionMessage
		if sessionID == "default" {
			recentMessages = s.HistoryManager.GetForLLM()
		} else {
			sessionMsgs, err := s.ShortTermMem.GetSessionMessages(sessionID)
			if err != nil {
				s.Logger.Error("Failed to load session messages for context", "session_id", sessionID, "error", err)
			} else {
				for _, m := range sessionMsgs {
					if !m.IsInternal {
						recentMessages = append(recentMessages, m.ChatCompletionMessage)
					}
				}
			}
			// Non-default sessions load raw messages from SQLite without the
			// dangling-tool-result filtering that GetForLLM() provides.
			// Apply the same sanitization to prevent API error 2013
			// ("tool result's tool id not found").
			if sanitized, dropped := agent.SanitizeToolMessages(recentMessages); dropped > 0 {
				s.Logger.Warn("Sanitized orphaned tool messages in non-default session",
					"session_id", sessionID, "dropped", dropped,
					"before", len(recentMessages), "after", len(sanitized))
				recentMessages = sanitized
			}
		}

		// Phase 33: Recursive Context Compression (Character Based)
		// Only applies to the default session which uses HistoryManager
		charLimit := s.Cfg.Agent.MemoryCompressionCharLimit
		if sessionID == "default" && s.HistoryManager.TotalChars() >= charLimit {
			if ok, release := s.HistoryManager.TryLockCompression(); ok {
				// Do NOT defer release() here — ownership transfers to the goroutine below.
				// If no goroutine is spawned, release() is called explicitly at the end of the
				// else branch.  Using defer here would fire when the HTTP handler returns
				// (~seconds after the agent loop finishes), which is far BEFORE the goroutine's
				// LLM summarisation call completes (up to 2 minutes).  That premature unlock
				// allowed a second request to start another compression round whose snapshot
				// included the just-finished agent response, silently deleting it from
				// HistoryManager and making it disappear on the next page reload.

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
					go func(msgs []memory.HistoryMessage, charsPruned int, existingSummary string, releaseFn func()) {
						defer releaseFn() // Goroutine owns the lock; released when summarisation completes
						defer func() {
							if r := recover(); r != nil {
								s.Logger.Error("[Compression] Goroutine panic recovered", "error", r)
							}
						}()
						compressionClient, compressionModel := llm.ResolveHelperBackedClient(s.Cfg, s.LLMClient, s.Cfg.LLM.Model)
						llmSource := "main"
						if compressionModel != s.Cfg.LLM.Model {
							llmSource = "helper"
						}
						s.Logger.Info("[Compression] Triggering character-based context compression",
							"msg_count", len(msgs), "chars", charsPruned, "limit", charLimit, "llm", llmSource, "model", compressionModel)

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
							Model: compressionModel,
							Messages: []openai.ChatCompletionMessage{
								{Role: openai.ChatMessageRoleUser, Content: prompt},
							},
							MaxTokens:   1000,
							Temperature: 0.3,
						}

						bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
						defer cancel()

						resp, err := llm.ExecuteWithRetry(bgCtx, compressionClient, summaryReq, s.Logger, nil)
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
					}(messagesToSummarize, actualChars, s.HistoryManager.GetSummary(), release)
				} else {
					// No messages to compress — release the lock immediately.
					release()
				}
			}
		}

		// Build run configuration for the unified agent loop.
		msgSource := "web_chat"
		if missionID != "" {
			msgSource = "mission"
		}
		runCfg := agent.RunConfig{
			Config:             s.Cfg,
			Logger:             s.Logger,
			LLMClient:          s.LLMClient,
			ShortTermMem:       s.ShortTermMem,
			HistoryManager:     s.HistoryManager,
			LongTermMem:        s.LongTermMem,
			KG:                 s.KG,
			InventoryDB:        s.InventoryDB,
			InvasionDB:         s.InvasionDB,
			CheatsheetDB:       s.CheatsheetDB,
			ImageGalleryDB:     s.ImageGalleryDB,
			MediaRegistryDB:    s.MediaRegistryDB,
			HomepageRegistryDB: s.HomepageRegistryDB,
			ContactsDB:         s.ContactsDB,
			PlannerDB:          s.PlannerDB,
			SQLConnectionsDB:   s.SQLConnectionsDB,
			SQLConnectionPool:  s.SQLConnectionPool,
			RemoteHub:          s.RemoteHub,
			Vault:              s.Vault,
			Registry:           s.Registry,
			Manifest:           manifest,
			CronManager:        s.CronManager,
			MissionManagerV2:   s.MissionManagerV2,
			CoAgentRegistry:    s.CoAgentRegistry,
			BudgetTracker:      s.BudgetTracker,
			DaemonSupervisor:   s.DaemonSupervisor,
			LLMGuardian:        s.LLMGuardian,
			PreparationService: s.PreparationService,
			SessionID:          sessionID,
			IsMaintenance:      inMaintenance,
			IsMission:          missionID != "",
			MissionID:          missionID,
			MessageSource:      msgSource,
			VoiceOutputActive:  GetSpeakerMode(),
		}

		missionToolResultsBefore := 0
		if missionID != "" && s.ShortTermMem != nil {
			if count, err := s.ShortTermMem.CountInternalToolResultMessages(sessionID); err == nil {
				missionToolResultsBefore = count
			} else {
				s.Logger.Debug("Failed to read pre-run mission tool result count", "session_id", sessionID, "error", err)
			}
		}

		finalMessages := append([]openai.ChatCompletionMessage{}, recentMessages...)
		if sessionID == "default" {
			if currentSummary := s.HistoryManager.GetSummary(); currentSummary != "" {
				finalMessages = append([]openai.ChatCompletionMessage{{
					Role:    openai.ChatMessageRoleSystem,
					Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + currentSummary,
				}}, finalMessages...)
			}
		}

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

		// Multimodal promotion (images): convert uploaded attachment references into
		// OpenAI-style MultiContent parts for the outgoing LLM request. We do this
		// here (not in HistoryManager) to avoid bloating persisted history with
		// base64-encoded image data.
		s.CfgMu.RLock()
		cfg := s.Cfg
		workspaceDir := s.Cfg.Directories.WorkspaceDir
		s.CfgMu.RUnlock()
		for i := range finalMessages {
			finalMessages[i] = promoteUploadedImagesToMultiContent(cfg, finalMessages[i], workspaceDir, s.Logger)
		}

		req.Messages = finalMessages

		// 4. Pass execution to the unified agent loop
		// runCfg is already built above for prompt context flags.

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			flusher, ok := w.(http.Flusher)
			if !ok {
				s.Logger.Error("Streaming not supported by ResponseWriter")
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_streaming_unsupported"), http.StatusInternalServerError)
				return
			}
			// Initial flush to establish SSE connection
			flusher.Flush()

			broker := NewSSEBrokerAdapterWithSession(sse, sessionID)
			_, err := agent.ExecuteAgentLoop(r.Context(), req, runCfg, true, broker)
			if err != nil {
				s.Logger.Error("Streamed agent loop failed", "error", err)
				if llm.IsImageNotSupportedError(err) {
					errMsg := i18n.T(s.Cfg.Server.UILanguage, "backend.handler_image_not_supported")
					broker.SendLLMStreamDelta(errMsg, "", "", 0, "")
					broker.SendLLMStreamDone("stop")
				}
				return
			}

			// Conclude SSE stream nicely
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			flusher.Flush()

		} else {
			// Use a detached context for sync requests so a client disconnect
			// does not abort an in-progress tool chain (e.g. mid-execution after
			// the agent already started hatching an egg or running a command).
			syncCtx, syncCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer syncCancel()
			broker := NewSSEBrokerAdapterWithSession(sse, sessionID)
			resp, err := agent.ExecuteAgentLoop(syncCtx, req, runCfg, false, broker)
			if err != nil {
				s.Logger.Error("Sync agent loop failed", "error", err)
				if llm.IsImageNotSupportedError(err) {
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
								Content: i18n.T(s.Cfg.Server.UILanguage, "backend.handler_image_not_supported"),
							},
							FinishReason: openai.FinishReasonStop,
						}},
					})
					return
				}
				// Return a user-visible error as a proper OpenAI response instead of HTTP 500
				errMsg := i18n.T(s.Cfg.Server.UILanguage, "backend.handler_timeout_error")
				if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "context canceled") {
					errMsg = i18n.T(s.Cfg.Server.UILanguage, "backend.handler_sync_error")
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
			// Scrub any sensitive values from the response content before sending.
			// Also strip reasoning tags and hallucinated RAG placeholders.
			for i := range resp.Choices {
				resp.Choices[i].Message.Content = security.StripThinkingTags(
					security.Scrub(resp.Choices[i].Message.Content),
				)
			}
			if missionID != "" && s.ShortTermMem != nil {
				missionToolResultsAfter := missionToolResultsBefore
				if count, err := s.ShortTermMem.CountInternalToolResultMessages(sessionID); err == nil {
					missionToolResultsAfter = count
				} else {
					s.Logger.Debug("Failed to read post-run mission tool result count", "session_id", sessionID, "error", err)
				}
				toolResultDelta := missionToolResultsAfter - missionToolResultsBefore
				if toolResultDelta < 0 {
					toolResultDelta = 0
				}
				w.Header().Set("X-Aurago-Mission-Tool-Results", strconv.Itoa(toolResultDelta))
				if len(resp.Choices) > 0 && missionResponseLooksIncomplete(resp.Choices[0].Message.Content, toolResultDelta) {
					w.Header().Set("X-Aurago-Mission-Suspicious-Completion", "true")
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}
}

func handleArchiveMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		// Limit request body to 10 MB for batch archive uploads
		r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			s.Logger.Error("Failed to read archive request body", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_bad_request"), http.StatusBadRequest)
			return
		}

		trimmed := strings.TrimSpace(string(bodyBytes))

		if strings.HasPrefix(trimmed, "[") {
			var items []memory.ArchiveItem
			if err := json.Unmarshal(bodyBytes, &items); err != nil {
				s.Logger.Error("Failed to decode batch archive request", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_bad_request"), http.StatusBadRequest)
				return
			}

			if len(items) == 0 {
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_empty_batch"), http.StatusBadRequest)
				return
			}

			storedIDs, err := s.LongTermMem.StoreBatch(items)
			if err != nil {
				s.Logger.Error("Failed to archive batch", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_internal_error"), http.StatusInternalServerError)
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
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_bad_request"), http.StatusBadRequest)
				return
			}

			if req.Concept == "" || req.Content == "" {
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_concept_content_required"), http.StatusBadRequest)
				return
			}

			storedIDs, err := s.LongTermMem.StoreDocument(req.Concept, req.Content)
			if err != nil {
				s.Logger.Error("Failed to archive memory", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_internal_error"), http.StatusInternalServerError)
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
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		// Determine which session to interrupt.
		// Prefer X-Session-ID header, fall back to JSON body, then "default".
		sessionID := "default"
		if sid := r.Header.Get("X-Session-ID"); sid != "" {
			sessionID = sid
		} else {
			var body struct {
				SessionID string `json:"session_id"`
			}
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.SessionID != "" {
					sessionID = body.SessionID
				}
			}
		}

		s.Logger.Warn("Stop requested via Web UI", "session_id", sessionID)

		agent.InterruptSession(sessionID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "success",
			"message": i18n.T(s.Cfg.Server.UILanguage, "backend.handler_agent_interrupted"),
		})
	}
}

// handleUpload receives a multipart file upload and saves it to
// {workspace_dir}/attachments/, returning the agent-visible path.
func handleUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		// 32 MB max upload size
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_failed_parse_form"), http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_missing_file"), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Sanitize filename - prevent path traversal and ensure safe name
		base := sanitizeFilename(header.Filename)
		if isActiveContentExtension(base) {
			jsonError(w, "Uploads with active content extensions are not allowed", http.StatusBadRequest)
			return
		}

		ts := time.Now().Format("20060102_150405")
		filename := ts + "_" + uid.New() + "_" + base

		// Save to {workspace_dir}/attachments/
		attachDir := filepath.Join(s.Cfg.Directories.WorkspaceDir, "attachments")
		if err := os.MkdirAll(attachDir, 0755); err != nil {
			s.Logger.Error("Failed to create attachments dir", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_failed_create_dir"), http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(attachDir, filename)
		dst, err := os.Create(destPath)
		if err != nil {
			s.Logger.Error("Failed to create upload file", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_failed_write_file"), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			s.Logger.Error("Failed to write uploaded file", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_failed_save_file"), http.StatusInternalServerError)
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

		var apiKey, baseURL string
		found := false

		// Only check the currently active main/helper LLM paths here.
		// This endpoint is polled by the UI and should not emit auth warnings just
		// because some unrelated provider entry somewhere else in the config uses
		// OpenRouter. The explicit /credits command can still do broader fallback.
		if strings.ToLower(s.Cfg.LLM.ProviderType) == "openrouter" && s.Cfg.LLM.APIKey != "" {
			apiKey = s.Cfg.LLM.APIKey
			baseURL = s.Cfg.LLM.BaseURL
			found = true
		} else if strings.ToLower(s.Cfg.LLM.HelperProviderType) == "openrouter" && s.Cfg.LLM.HelperAPIKey != "" {
			// Check helper LLM
			apiKey = s.Cfg.LLM.HelperAPIKey
			baseURL = s.Cfg.LLM.HelperBaseURL
			found = true
		}

		// Trim whitespace defensively — vault values can occasionally have trailing
		// newlines, which would send an empty Bearer token to OpenRouter.
		apiKey = strings.TrimSpace(apiKey)
		if !found || apiKey == "" {
			w.Write([]byte(`{"available":false,"reason":"provider is not openrouter"}`))
			return
		}

		credits, err := llm.FetchOpenRouterCredits(apiKey, baseURL)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "HTTP 401") || strings.Contains(errStr, "HTTP 403") {
				// Auth failure — key is present but invalid/revoked. Log once as WARN
				// (not ERROR) so the log is not spammed on every dashboard refresh.
				s.Logger.Warn("[OpenRouter] Credit check auth failed — verify the API key stored in the vault for the OpenRouter provider", "error", err)
				w.Write([]byte(`{"available":false,"reason":"auth_failed"}`))
			} else {
				s.Logger.Error("Failed to fetch OpenRouter credits", "error", err)
				w.Write([]byte(fmt.Sprintf(`{"available":true,"error":"%s"}`, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_credits_fetch_error"))))
			}
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
