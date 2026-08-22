package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"aurago/internal/vaultprompt"

	"aurago/internal/uid"

	"github.com/sashabaranov/go-openai"
)

var (
	followUpDepths        = make(map[string]int)
	muFollowUp            sync.Mutex
	sessionRequestLocks   = make(map[string]*sessionRequestLock)
	muSessionRequestLocks sync.Mutex
)

const firstStartIntroAudioFilename = "brain_in_the _machine.mp3"

func sendFirstStartIntroAudio(broker agent.FeedbackBroker) {
	if broker == nil {
		return
	}
	if sseBroker, ok := broker.(*SSEBrokerAdapter); ok && sseBroker.sse == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"path":     "/files/audio/" + url.PathEscape(firstStartIntroAudioFilename),
		"title":    "Brain in the Machine",
		"autoplay": true,
	})
	if err != nil {
		return
	}
	broker.Send("audio", string(payload))
}

type sessionRequestLock struct {
	mu   sync.Mutex
	refs int
}

func lockSessionRequest(sessionID string) func() {
	return lockSessionRequestWithLogger(sessionID, nil)
}

func sessionRequestActive(sessionID string) bool {
	muSessionRequestLocks.Lock()
	defer muSessionRequestLocks.Unlock()
	lock := sessionRequestLocks[sessionID]
	return lock != nil && lock.refs > 0
}

func lockSessionRequestWithLogger(sessionID string, logger *slog.Logger) func() {
	start := time.Now()
	muSessionRequestLocks.Lock()
	lock := sessionRequestLocks[sessionID]
	if lock == nil {
		lock = &sessionRequestLock{}
		sessionRequestLocks[sessionID] = lock
	}
	lock.refs++
	queued := lock.refs > 1
	queuedRefs := lock.refs
	muSessionRequestLocks.Unlock()
	if queued && logger != nil {
		logger.Info("[SessionLock] Request queued behind active session", "session_id", sessionID, "queued_refs", queuedRefs)
	}
	lock.mu.Lock()
	if logger != nil {
		logger.Debug("[SessionLock] Request acquired session lock", "session_id", sessionID, "wait_ms", time.Since(start).Milliseconds(), "queued", queued)
	}
	return func() {
		lock.mu.Unlock()

		muSessionRequestLocks.Lock()
		lock.refs--
		if lock.refs <= 0 && sessionID != "default" && sessionRequestLocks[sessionID] == lock {
			delete(sessionRequestLocks, sessionID)
		}
		muSessionRequestLocks.Unlock()
	}
}

func cloneChatCompletionMessages(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	return append([]openai.ChatCompletionMessage(nil), messages...)
}

func recentMessagesForRequest(sessionID, missionID string, requestMessages, defaultMessages []openai.ChatCompletionMessage, sessionMessages []memory.HistoryMessage) []openai.ChatCompletionMessage {
	if missionID != "" {
		return cloneChatCompletionMessages(requestMessages)
	}
	if sessionID == "default" {
		return cloneChatCompletionMessages(defaultMessages)
	}

	recentMessages := make([]openai.ChatCompletionMessage, 0, len(sessionMessages))
	for _, m := range sessionMessages {
		if !m.IsInternal {
			recentMessages = append(recentMessages, m.ChatCompletionMessage)
		}
	}
	return recentMessages
}

func requestMessageIsInternal(isFollowUp bool, missionID string) bool {
	return isFollowUp || strings.TrimSpace(missionID) != ""
}

func shouldAppendProspectiveUserMessage(role, missionID string) bool {
	return role == openai.ChatMessageRoleUser && strings.TrimSpace(missionID) == ""
}

func firstStartInitializationMessage() openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
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
	}
}

func chatCompletionMessageSource(isFollowUp bool, missionID string) string {
	if strings.TrimSpace(missionID) != "" {
		return "mission"
	}
	if isFollowUp {
		return "follow_up"
	}
	return "web_chat"
}

func feedbackBrokerForRequest(sse *SSEBroadcaster, sessionID, missionID string, isFollowUp bool) agent.FeedbackBroker {
	if strings.TrimSpace(missionID) != "" || isFollowUp {
		return agent.NoopBroker{}
	}
	return NewSSEBrokerAdapterWithSession(sse, sessionID)
}

type feedbackBrokerOverrideContextKey struct{}

func withFeedbackBrokerOverride(ctx context.Context, broker agent.FeedbackBroker) context.Context {
	if broker == nil {
		return ctx
	}
	return context.WithValue(ctx, feedbackBrokerOverrideContextKey{}, broker)
}

func feedbackBrokerForRequestContext(ctx context.Context, sse *SSEBroadcaster, sessionID, missionID string, isFollowUp bool) agent.FeedbackBroker {
	if ctx != nil {
		if broker, ok := ctx.Value(feedbackBrokerOverrideContextKey{}).(agent.FeedbackBroker); ok && broker != nil {
			return broker
		}
	}
	return feedbackBrokerForRequest(sse, sessionID, missionID, isFollowUp)
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
		isFollowUp, missionID, validInternalHeaders := validateInternalChatHeaders(r, s)
		if !validInternalHeaders {
			writeInvalidInternalChatHeaders(w)
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

		// Check for Follow-Up loop protection. The headers were authenticated
		// above independently of the normal browser/API authentication mode.
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

		if len(req.Messages) == 0 {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_no_messages"), http.StatusBadRequest)
			return
		}

		turnCfg := s.ConfigSnapshot()
		turnLLMClient := s.LLMClient
		speechLabSessionID := "default"
		if missionID != "" {
			speechLabSessionID = "mission-" + missionID
		}
		if chatSessionID := strings.TrimSpace(r.Header.Get("X-Session-ID")); chatSessionID != "" {
			speechLabSessionID = chatSessionID
		}
		lastRequestMessage := req.Messages[len(req.Messages)-1]
		var speechLabReservation *speechLabTurnTokenReservation
		markedSpeechLabInput := false
		if !isFollowUp && missionID == "" {
			speechLabReservation, markedSpeechLabInput = s.speechLabTokens().Reserve(
				strings.TrimSpace(r.Header.Get(speechLabChatTurnTokenHeader)), speechLabSessionID, lastRequestMessage.Content,
			)
		}
		if speechLabReservation != nil {
			defer speechLabReservation.release()
		}
		var speechLabVault config.SecretReader
		if s.Vault != nil {
			speechLabVault = s.Vault
		}
		turnCfg, turnLLMClient, routeErr := speechLabChatTurnRuntime(markedSpeechLabInput, isFollowUp, missionID, turnCfg, turnLLMClient, speechLabVault)
		if routeErr != nil {
			lang := ""
			if cfg := s.ConfigSnapshot(); cfg != nil {
				lang = cfg.Server.UILanguage
			}
			message := i18n.T(lang, "backend.speech_lab_llm_unavailable")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "speech_lab_llm_unavailable", "message": message,
			})
			return
		}

		// Override the model with the configured backend model
		overrideModel := turnCfg.LLM.Model
		if overrideModel != "" {
			s.Logger.Debug("Overriding model", "from", req.Model, "to", overrideModel)
			req.Model = overrideModel
		}

		// 1. Save User Input to Short-Term Memory
		lastUserMsg := req.Messages[len(req.Messages)-1]
		var imageInputErr error
		for _, message := range req.Messages {
			if imageInputErr = validateMainProviderImageParts(turnCfg, message); imageInputErr != nil {
				break
			}
		}
		if imageInputErr == nil {
			imageInputErr = validateCurrentMainProviderImageInput(turnCfg, lastUserMsg)
		}
		if imageInputErr != nil {
			jsonError(w, imageInputErr.Error(), http.StatusBadRequest)
			return
		}
		sessionID := speechLabSessionID
		if lastUserMsg.Role == openai.ChatMessageRoleUser &&
			handlePendingQuestionChatMessage(w, req, sessionID, lastUserMsg.Content, s.Logger) {
			return
		}
		unlockSession := lockSessionRequestWithLogger(sessionID, s.Logger)
		defer unlockSession()

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
				SessionID:        sessionID,
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

		// 2. Rebuild the Context
		// For non-default chat sessions, build context from SQLite instead of HistoryManager
		var recentMessages []openai.ChatCompletionMessage
		if missionID != "" {
			recentMessages = recentMessagesForRequest(sessionID, missionID, req.Messages, nil, nil)
		} else if sessionID == "default" {
			recentMessages = recentMessagesForRequest(sessionID, missionID, req.Messages, s.HistoryManager.GetForLLM(), nil)
		} else {
			sessionMsgs, err := s.ShortTermMem.GetSessionMessages(sessionID)
			if err != nil {
				s.Logger.Error("Failed to load session messages for context", "session_id", sessionID, "error", err)
			} else {
				recentMessages = recentMessagesForRequest(sessionID, missionID, req.Messages, nil, sessionMsgs)
			}
			// Non-default sessions load raw messages from SQLite without the
			// dangling-tool-result filtering that GetForLLM() provides.
			// Apply the same sanitization to prevent API error 2013
			// ("tool result's tool id not found").
			sanitizedMessages, droppedToolMessages := agent.SanitizeToolMessages(recentMessages)
			if droppedToolMessages > 0 {
				s.Logger.Warn("Sanitized orphaned tool messages in non-default session",
					"session_id", sessionID, "dropped", droppedToolMessages,
					"before", len(recentMessages), "after", len(sanitizedMessages))
			}
			recentMessages = sanitizedMessages
		}
		isInternalRequestMessage := requestMessageIsInternal(isFollowUp, missionID)
		if shouldAppendProspectiveUserMessage(lastUserMsg.Role, missionID) {
			// Build the exact prospective history before persistence. The accepted
			// request is committed only after budget preflight succeeds. Generic
			// internal follow-ups still belong in the model request; mission requests
			// already carry their complete request-local history above.
			recentMessages = append(recentMessages, lastUserMsg)
		}

		// Build run configuration for the unified agent loop.
		msgSource := chatCompletionMessageSource(isFollowUp, missionID)
		userIntent := ""
		if !isFollowUp && missionID == "" && lastUserMsg.Role == openai.ChatMessageRoleUser {
			userIntent = lastUserMsg.Content
		}
		runCfg := agent.RunConfig{
			Config:             turnCfg,
			Logger:             s.Logger,
			LLMClient:          turnLLMClient,
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
			WorkspaceSearch:    s.WorkspaceSearch,
			SessionID:          sessionID,
			UserIntent:         userIntent,
			IsMaintenance:      inMaintenance,
			IsMission:          missionID != "",
			MissionID:          missionID,
			MessageSource:      msgSource,
			VoiceOutputActive:  GetSpeakerMode(),
		}
		if msgSource == "web_chat" && !inMaintenance && webVaultSecretPromptAuthenticated(s, r) && vaultSecretPromptWriteEnabled(s) {
			if manager := ensureVaultSecretPrompter(s); manager != nil {
				runCfg.VaultSecretPrompter = manager
				runCfg.VaultSecretTarget = vaultprompt.Target{
					Channel:         "web_chat",
					ClientSessionID: sessionID,
					ConversationID:  sessionID,
				}
			}
		}

		missionToolResultsBefore := missionToolResultCount{}

		finalMessages := append([]openai.ChatCompletionMessage{}, recentMessages...)
		if sessionID == "default" {
			if currentSummary := s.HistoryManager.GetSummary(); currentSummary != "" {
				finalMessages = append([]openai.ChatCompletionMessage{{
					Role:    openai.ChatMessageRoleSystem,
					Content: formatPersistentContextRecap(currentSummary),
				}}, finalMessages...)
			}
		}

		// Include first-start initialization in the prospective request without
		// consuming the one-shot state until budget preflight has accepted it.
		includeFirstStart := false
		if s.IsFirstStart {
			s.muFirstStart.Lock()
			includeFirstStart = !s.firstStartDone
			s.muFirstStart.Unlock()
			if includeFirstStart {
				finalMessages = append(finalMessages, firstStartInitializationMessage())
			}
		}

		// Multimodal promotion (images): convert uploaded attachment references into
		// OpenAI-style MultiContent parts for the outgoing LLM request. We do this
		// here (not in HistoryManager) to avoid bloating persisted history with
		// base64-encoded image data.
		cfg := runCfg.Config
		workspaceDir := runCfg.Config.Directories.WorkspaceDir
		for i := range finalMessages {
			finalMessages[i] = promoteUploadedImagesToMultiContent(cfg, finalMessages[i], workspaceDir, s.Logger)
		}

		req.Messages = finalMessages
		if err := agent.ValidateMinimumRequestBudget(r.Context(), req, runCfg); err != nil {
			if agent.IsContextBudgetExceeded(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "context_budget_exceeded"})
				return
			}
			s.Logger.Error("Chat request budget preflight failed", "error", err)
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.handler_internal_error"), http.StatusInternalServerError)
			return
		}

		// Commit one-shot provenance and persistent state only after the complete
		// prospective request has passed the context guard.
		if speechLabReservation != nil && !speechLabReservation.commit() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "speech_lab_turn_token_conflict"})
			return
		}
		if missionID != "" && s.ShortTermMem != nil {
			var err error
			missionToolResultsBefore, err = resetMissionSessionToolResultBaseline(s.ShortTermMem, sessionID)
			if err != nil {
				s.Logger.Warn("Failed to clear stale mission session context", "session_id", sessionID, "mission_id", missionID, "error", err)
			}
			agent.ClearDiscoverToolsState(sessionID)
		}
		if lastUserMsg.Role == openai.ChatMessageRoleUser && s.Guardian != nil {
			s.Guardian.ScanUserInput(lastUserMsg.Content)
		}
		if lastUserMsg.Role == openai.ChatMessageRoleUser {
			id, err := s.ShortTermMem.InsertMessage(sessionID, lastUserMsg.Role, lastUserMsg.Content, false, isInternalRequestMessage)
			if err != nil {
				s.Logger.Error("Failed to insert user message", "error", err)
			}
			agent.NoteInnerVoiceUserTurn(sessionID)
			if sessionID == "default" && !isInternalRequestMessage && agent.ShouldAppendHistoryMessage(id, err) {
				// Persist raw attachment references; multimodal expansion stays request-local.
				s.HistoryManager.Add(lastUserMsg.Role, lastUserMsg.Content, id, false, false)
			}
			_ = s.ShortTermMem.UpdateChatSessionPreview(sessionID)
			_ = s.ShortTermMem.TouchChatSession(sessionID)
			agent.EnforceSTMPRetentionIfConfigured(s.Cfg, s.ShortTermMem, sessionID, s.Logger)
		}
		if includeFirstStart {
			s.muFirstStart.Lock()
			commitFirstStart := !s.firstStartDone
			if commitFirstStart {
				s.firstStartDone = true
			}
			s.muFirstStart.Unlock()
			if commitFirstStart {
				s.Logger.Info("[FirstStart] Injecting one-time naming prompt")
				sendFirstStartIntroAudio(NewSSEBrokerAdapterWithSession(sse, sessionID))
			}
		}

		// 4. Pass execution to the unified agent loop
		// runCfg is already built above for prompt context flags.

		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
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

			broker := newChatVoiceOutputTrackingBroker(feedbackBrokerForRequestContext(r.Context(), sse, sessionID, missionID, isFollowUp))
			resp, err := agent.ExecuteAgentLoop(r.Context(), req, runCfg, true, broker)
			if err != nil {
				s.Logger.Error("Streamed agent loop failed", "error", err)
				emitStreamedAgentError(w, flusher, broker, sessionID, s.Cfg.Server.UILanguage, err)
				return
			}
			if len(resp.Choices) > 0 {
				maybeEmitChatVoiceOutputFallback(r.Context(), runCfg.Config, s.Logger, runCfg, broker, resp.Choices[0].Message.Content, s.SpeechLab)
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
			broker := newChatVoiceOutputTrackingBroker(feedbackBrokerForRequestContext(r.Context(), sse, sessionID, missionID, isFollowUp))
			resp, err := agent.ExecuteAgentLoop(syncCtx, req, runCfg, false, broker)
			if err != nil {
				s.Logger.Error("Sync agent loop failed", "error", err)
				if agent.IsContextBudgetExceeded(err) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "context_budget_exceeded"})
					return
				}
				// Return a user-visible error as a proper OpenAI response instead of HTTP 500
				errMsg := chatCompletionErrorMessage(s.Cfg.Server.UILanguage, err)
				writeChatCompletionErrorResponse(w, sessionID, errMsg)
				return
			}
			// Scrub any sensitive values from the response content before sending.
			// Also strip reasoning tags and hallucinated RAG placeholders.
			for i := range resp.Choices {
				resp.Choices[i].Message.Content = security.StripThinkingTags(
					security.Scrub(resp.Choices[i].Message.Content),
				)
			}
			if len(resp.Choices) > 0 {
				maybeEmitChatVoiceOutputFallback(r.Context(), runCfg.Config, s.Logger, runCfg, broker, resp.Choices[0].Message.Content, s.SpeechLab)
			}
			if missionID != "" && s.ShortTermMem != nil {
				missionToolResultsAfter := readMissionToolResultCount(s.ShortTermMem, sessionID)
				toolResultDelta := missionToolResultDelta(missionToolResultsBefore, missionToolResultsAfter)
				if toolResultDelta.Known {
					w.Header().Set("X-Aurago-Mission-Tool-Results", strconv.Itoa(toolResultDelta.Value))
				} else {
					s.Logger.Debug("Mission tool result count is unknown", "session_id", sessionID, "mission_id", missionID)
				}
				if len(resp.Choices) > 0 {
					assessment := assessMissionCompletion(resp.Choices[0].Message.Content, toolResultDelta)
					if assessment.Suspicious {
						w.Header().Set("X-Aurago-Mission-Suspicious-Completion", "true")
						w.Header().Set("X-Aurago-Mission-Suspicious-Reason", assessment.Reason)
					}
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
		if manager := currentVaultSecretPrompter(s); manager != nil {
			manager.CancelConversation(sessionID)
		}

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
