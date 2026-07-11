package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

const (
	desktopChatSessionID               = "virtual-desktop"
	desktopChatMessageSource           = "virtual_desktop_chat"
	homepageStudioMessageSource        = "homepage_studio"
	desktopChatAgentTurnTimeout        = 30 * time.Minute
	desktopChatStreamHeartbeatInterval = 15 * time.Second
)

func handleDesktopChat(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string             `json:"message"`
			Context desktopChatContext `json:"context"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopChatJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Message = strings.TrimSpace(body.Message)
		if body.Message == "" {
			jsonError(w, "Message is required", http.StatusBadRequest)
			return
		}
		if s == nil || s.Cfg == nil {
			jsonError(w, "server not configured", http.StatusInternalServerError)
			return
		}
		unlockSession := lockSessionRequest(desktopChatSessionID)
		defer unlockSession()
		if answer, handled, err := handleDesktopSlashCommand(s, body.Message); handled {
			if err != nil {
				if s.Logger != nil {
					s.Logger.Error("Desktop command execution failed", "error", err)
				}
				jsonError(w, chatCompletionErrorMessage(desktopUILanguage(s), err), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "answer": answer})
			return
		}
		answer := runDesktopAgentChat(r.Context(), s, body.Message, body.Context)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "answer": answer})
	}
}

func handleDesktopChatStream(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Message string             `json:"message"`
			Context desktopChatContext `json:"context"`
		}
		if err := decodeDesktopJSON(w, r, &body, desktopChatJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		body.Message = strings.TrimSpace(body.Message)
		if body.Message == "" {
			jsonError(w, "Message is required", http.StatusBadRequest)
			return
		}

		flusher, canFlush := w.(http.Flusher)
		if !canFlush {
			jsonError(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		if s == nil || s.Cfg == nil {
			sseWriteData(w, "error", "server not configured")
			return
		}

		unlockSession := lockSessionRequest(desktopChatSessionID)
		defer unlockSession()
		if answer, handled, err := handleDesktopSlashCommand(s, body.Message); handled {
			if err != nil {
				sseWriteData(w, "error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), err))
			} else {
				sseWriteData(w, "final_response", answer)
			}
			sseWriteData(w, "done", "done")
			sseWriteDone(w)
			if canFlush {
				flusher.Flush()
			}
			return
		}
		turn, err := prepareDesktopAgentTurn(r.Context(), s, body.Message, body.Context, true)
		if err != nil {
			sseWriteData(w, "error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), err))
			sseWriteData(w, "done", "done")
			sseWriteDone(w)
			if canFlush {
				flusher.Flush()
			}
			return
		}
		broker := &desktopStreamBroker{
			w:        w,
			flusher:  flusher,
			canFlush: canFlush,
		}
		llmCtx, llmCancel := context.WithTimeout(r.Context(), desktopChatAgentTurnTimeout)
		defer llmCancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			sseBroker := NewSSEBrokerAdapterWithSession(s.SSE, desktopChatSessionID)
			combinedBroker := &desktopStreamCombinedBroker{
				stream:       broker,
				sse:          sseBroker,
				shortTermMem: s.ShortTermMem,
				sessionID:    desktopChatSessionID,
			}
			if _, err := agent.ExecuteAgentLoop(llmCtx, turn.req, turn.runCfg, true, combinedBroker); err != nil {
				if llm.IsContextError(err) && llmCtx.Err() != nil {
					return
				}
				combinedBroker.Send("error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), err))
				combinedBroker.SendLLMStreamDone("error")
				combinedBroker.Send("done", "done")
			}
		}()
		heartbeat := time.NewTicker(desktopChatStreamHeartbeatInterval)
		defer heartbeat.Stop()
		streamFinished := false
		for !streamFinished {
			select {
			case <-done:
				streamFinished = true
			case <-r.Context().Done():
				llmCancel()
				<-done
				streamFinished = true
			case <-heartbeat.C:
				if !broker.sendHeartbeat() {
					llmCancel()
					<-done
					streamFinished = true
				}
			}
		}
		broker.mu.Lock()
		alreadyClosed := broker.closed
		broker.closed = true
		broker.mu.Unlock()
		if !alreadyClosed {
			sseWriteDone(w)
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

type desktopStreamBroker struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	canFlush bool
	mu       sync.Mutex
	closed   bool
}

func (b *desktopStreamBroker) sendHeartbeat() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	if err := writeSSEComment(b.w, b.flusher, "heartbeat"); err != nil {
		b.closed = true
		return false
	}
	return true
}

type desktopStreamCombinedBroker struct {
	stream         *desktopStreamBroker
	sse            *SSEBrokerAdapter
	shortTermMem   *memory.SQLiteMemory
	sessionID      string
	hasTurnContent bool
}

func (b *desktopStreamCombinedBroker) Send(event, message string) {
	b.sse.Send(event, message)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	if event == "done" {
		if b.hasTurnContent {
			if answer := latestDesktopAssistantMessage(b.shortTermMem, b.sessionID); answer != "" {
				sseWriteData(b.stream.w, "final_response", answer)
			}
		}
		sseWriteDone(b.stream.w)
		b.stream.closed = true
		if b.stream.canFlush {
			b.stream.flusher.Flush()
		}
		b.stream.mu.Unlock()
		return
	}
	sseWriteData(b.stream.w, event, message)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendJSON(jsonStr string) {
	b.sse.SendJSON(jsonStr)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	fmt.Fprintf(b.stream.w, "data: %s\n\n", jsonStr)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendTyped(eventType string, payload interface{}) bool {
	if b == nil || b.sse == nil || eventType == "" {
		return false
	}
	enriched := enrichPayloadWithSessionID(payload, b.sse.sessionID)
	if !b.sse.SendTyped(eventType, enriched) {
		return false
	}
	msg, err := encodeTypedSSEEvent(eventType, enriched)
	if err != nil {
		return false
	}
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return true
	}
	fmt.Fprintf(b.stream.w, "data: %s\n\n", security.Scrub(msg))
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
	return true
}

func (b *desktopStreamCombinedBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	b.sse.SendLLMStreamDelta(content, toolName, toolID, index, finishReason)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	if strings.TrimSpace(content) != "" {
		b.hasTurnContent = true
	}
	payload := LLMStreamDeltaPayload{
		SessionID:    b.sse.sessionID,
		Content:      content,
		ToolName:     toolName,
		ToolID:       toolID,
		Index:        index,
		FinishReason: finishReason,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "llm_stream_delta", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendLLMStreamDone(finishReason string) {
	b.sse.SendLLMStreamDone(finishReason)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := LLMStreamDonePayload{
		SessionID:    b.sse.sessionID,
		FinishReason: finishReason,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "llm_stream_done", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
	b.sse.SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal, isEstimated, isFinal, source)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := TokenUpdatePayload{
		SessionID:        b.sse.sessionID,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		SessionTotal:     sessionTotal,
		GlobalTotal:      globalTotal,
		IsEstimated:      isEstimated,
		IsFinal:          isFinal,
		Source:           source,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "token_update", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) SendThinkingBlock(provider, content, state string) {
	b.sse.SendThinkingBlock(provider, content, state)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
	}
	payload := ThinkingBlockPayload{
		SessionID: b.sse.sessionID,
		Provider:  provider,
		Content:   content,
		State:     state,
	}
	data, _ := json.Marshal(payload)
	sseWriteJSON(b.stream.w, "thinking_block", data)
	if b.stream.canFlush {
		b.stream.flusher.Flush()
	}
	b.stream.mu.Unlock()
}

func (b *desktopStreamCombinedBroker) Scrub(s string) string {
	return security.Scrub(s)
}

func latestDesktopAssistantMessage(shortTermMem *memory.SQLiteMemory, sessionID string) string {
	if shortTermMem == nil {
		return ""
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "virtual-desktop"
	}
	messages, err := shortTermMem.GetSessionMessages(sessionID)
	if err != nil {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role != "assistant" && role != "agent" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		return security.StripThinkingTags(security.Scrub(content))
	}
	return ""
}

func sseWriteData(w http.ResponseWriter, event, data string) {
	encoded, _ := json.Marshal(map[string]string{"event": event, "detail": data})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
}

func sseWriteJSON(w http.ResponseWriter, event string, jsonPayload []byte) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(jsonPayload, &raw); err == nil {
		evt, _ := json.Marshal(event)
		raw["event"] = evt
		if enriched, err := json.Marshal(raw); err == nil {
			fmt.Fprintf(w, "data: %s\n\n", enriched)
			return
		}
	}
	encoded, _ := json.Marshal(map[string]string{"event": event, "detail": string(jsonPayload)})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
}

func sseWriteDone(w http.ResponseWriter) {
	fmt.Fprint(w, "data: [DONE]\n\n")
}

type desktopChatContext struct {
	Source          string                `json:"source"`
	OriginApp       string                `json:"origin_app,omitempty"`
	HomepageMode    bool                  `json:"homepage_mode,omitempty"`
	Target          string                `json:"target,omitempty"`
	CurrentFile     string                `json:"current_file"`
	CurrentLanguage string                `json:"current_language"`
	CurrentContent  string                `json:"current_content"`
	CursorLine      int                   `json:"cursor_line"`
	CursorColumn    int                   `json:"cursor_column"`
	SelectedText    string                `json:"selected_text"`
	OpenFiles       []string              `json:"open_files"`
	ImageBase64     string                `json:"image_base64,omitempty"`
	WindowContext   *desktopWindowContext `json:"window_context,omitempty"`
}

type desktopWindowContext struct {
	Source     string                         `json:"source,omitempty"`
	AppID      string                         `json:"app_id,omitempty"`
	StoreAppID string                         `json:"store_app_id,omitempty"`
	WindowID   string                         `json:"window_id,omitempty"`
	Label      string                         `json:"label,omitempty"`
	Purpose    string                         `json:"purpose,omitempty"`
	Guide      string                         `json:"guide,omitempty"`
	Resources  []desktopWindowContextResource `json:"resources,omitempty"`
}

type desktopWindowContextResource struct {
	Kind          string `json:"kind,omitempty"`
	Label         string `json:"label,omitempty"`
	Path          string `json:"path,omitempty"`
	ContainerPath string `json:"container_path,omitempty"`
}

func applyDesktopAgentProvider(ctx context.Context, s *Server, cfg *config.Config) llm.ChatClient {
	if s == nil || cfg == nil {
		return nil
	}
	selected := desktopAgentProviderID(ctx, s)
	if selected == "" {
		return s.LLMClient
	}
	provider := cfg.FindProvider(selected)
	if provider == nil {
		return s.LLMClient
	}
	cfg.LLM.Provider = provider.ID
	cfg.LLM.ProviderType = provider.Type
	cfg.LLM.BaseURL = provider.BaseURL
	cfg.LLM.APIKey = provider.APIKey
	cfg.LLM.AccountID = provider.AccountID
	if strings.TrimSpace(provider.Model) != "" {
		cfg.LLM.Model = provider.Model
	}
	return llm.NewClientFromProviderWithConfig(cfg, provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID)
}

func desktopAgentProviderID(ctx context.Context, s *Server) string {
	svc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return ""
	}
	payload, err := svc.Bootstrap(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Settings["agent.provider"])
}

type preparedDesktopAgentTurn struct {
	req    openai.ChatCompletionRequest
	runCfg agent.RunConfig
}

type desktopAgentTurnOptions struct {
	SessionID              string
	MessageSource          string
	AdditionalPrompt       string
	PersistedMessage       string
	VoiceOutputActive      bool
	OnUserMessageInserted  func(messageID int64) error
	PrepareSessionMessages func(messages []memory.HistoryMessage, currentMessageID int64) []memory.HistoryMessage
}

func prepareDesktopAgentTurn(ctx context.Context, s *Server, message string, chatContext desktopChatContext, stream bool) (preparedDesktopAgentTurn, error) {
	messageSource := desktopChatMessageSource
	if isHomepageStudioChatContext(chatContext) {
		messageSource = homepageStudioMessageSource
	}
	return prepareDesktopAgentTurnWithOptions(ctx, s, message, chatContext, stream, desktopAgentTurnOptions{
		SessionID:     desktopChatSessionID,
		MessageSource: messageSource,
	})
}

func prepareDesktopAgentTurnWithOptions(ctx context.Context, s *Server, message string, chatContext desktopChatContext, stream bool, opts desktopAgentTurnOptions) (preparedDesktopAgentTurn, error) {
	var turn preparedDesktopAgentTurn
	if s == nil || s.Cfg == nil {
		return turn, fmt.Errorf("server not configured")
	}
	if s.ShortTermMem == nil {
		return turn, fmt.Errorf("short-term memory not configured")
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		sessionID = desktopChatSessionID
	}
	messageSource := strings.TrimSpace(opts.MessageSource)
	if messageSource == "" {
		messageSource = desktopChatMessageSource
	}

	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	llmClient := applyDesktopAgentProvider(ctx, s, &cfg)

	persistContext := chatContext
	persistContext.ImageBase64 = ""
	desktopPromptContext := strings.TrimSpace(opts.AdditionalPrompt)
	if desktopPromptContext == "" {
		desktopPromptContext = buildDesktopAgentContext(persistContext)
	}
	cfg.Agent.AdditionalPrompt = appendDesktopAdditionalPrompt(cfg.Agent.AdditionalPrompt, desktopPromptContext)

	persistedPrompt := message
	if strings.TrimSpace(opts.PersistedMessage) != "" {
		persistedPrompt = opts.PersistedMessage
	}
	requestPrompt := message
	if strings.TrimSpace(chatContext.ImageBase64) != "" {
		if mainProviderSupportsImageMultimodal(&cfg) {
			requestPrompt = message
		} else {
			requestPrompt = message + "\n\nThe user attached a Camera app photo for this turn, but the selected provider is not configured for multimodal image input. The raw image data is intentionally not stored in chat history."
		}
	}

	if s.Guardian != nil {
		s.Guardian.ScanUserInput(message)
	}
	messageID, err := s.ShortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, persistedPrompt, false, false)
	if err != nil {
		return turn, fmt.Errorf("insert desktop user message: %w", err)
	}
	if opts.OnUserMessageInserted != nil {
		if err := opts.OnUserMessageInserted(messageID); err != nil {
			return turn, fmt.Errorf("desktop user message callback: %w", err)
		}
	}
	touchDesktopChatSessionMetadata(s, sessionID)
	agent.NoteInnerVoiceUserTurn(sessionID)

	sessionMessages, err := s.ShortTermMem.GetSessionMessages(sessionID)
	if err != nil {
		return turn, fmt.Errorf("load desktop session messages: %w", err)
	}
	if opts.PrepareSessionMessages != nil {
		sessionMessages = opts.PrepareSessionMessages(sessionMessages, messageID)
	}

	currentRequestMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: requestPrompt,
	}
	if strings.TrimSpace(chatContext.ImageBase64) != "" && mainProviderSupportsImageMultimodal(&cfg) {
		currentRequestMessage = openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{Type: openai.ChatMessagePartTypeText, Text: requestPrompt},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: "data:image/jpeg;base64," + strings.TrimSpace(chatContext.ImageBase64),
					},
				},
			},
		}
	}

	recentMessages := recentMessagesForRequest(sessionID, "", []openai.ChatCompletionMessage{currentRequestMessage}, nil, sessionMessages)
	recentMessages = replaceDesktopCurrentRequestMessage(recentMessages, persistedPrompt, currentRequestMessage)
	if strings.TrimSpace(chatContext.ImageBase64) == "" {
		for i, msg := range recentMessages {
			recentMessages[i] = promoteUploadedImagesToMultiContent(&cfg, msg, cfg.Directories.WorkspaceDir, s.Logger)
		}
	}
	sanitizedMessages, droppedToolMessages := agent.SanitizeToolMessages(recentMessages)
	if droppedToolMessages > 0 && s.Logger != nil {
		s.Logger.Warn("Sanitized orphaned desktop chat tool messages",
			"session_id", sessionID,
			"dropped", droppedToolMessages,
			"before", len(recentMessages),
			"after", len(sanitizedMessages))
	}

	turn.req = openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: sanitizedMessages,
		Stream:   stream,
	}
	turn.runCfg = buildDesktopRunConfigForSession(s, &cfg, llmClient, sessionID, messageSource)
	if opts.VoiceOutputActive {
		turn.runCfg.VoiceOutputActive = true
	}
	return turn, nil
}

func touchDesktopChatSessionMetadata(s *Server, sessionID string) {
	if s == nil || s.ShortTermMem == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	_ = s.ShortTermMem.UpdateChatSessionPreview(sessionID)
	_ = s.ShortTermMem.TouchChatSession(sessionID)
}

func replaceDesktopCurrentRequestMessage(messages []openai.ChatCompletionMessage, persistedPrompt string, requestMessage openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == openai.ChatMessageRoleUser && messages[i].Content == persistedPrompt {
			out := cloneChatCompletionMessages(messages)
			out[i] = requestMessage
			return out
		}
	}
	return append(cloneChatCompletionMessages(messages), requestMessage)
}

func appendDesktopAdditionalPrompt(base, desktopContext string) string {
	base = strings.TrimSpace(base)
	desktopContext = strings.TrimSpace(desktopContext)
	if desktopContext == "" {
		return base
	}
	if base == "" {
		return desktopContext
	}
	return base + "\n\n" + desktopContext
}

func buildDesktopRunConfig(s *Server, cfg *config.Config, llmClient llm.ChatClient) agent.RunConfig {
	return buildDesktopRunConfigForSession(s, cfg, llmClient, desktopChatSessionID, desktopChatMessageSource)
}

func buildDesktopRunConfigForSession(s *Server, cfg *config.Config, llmClient llm.ChatClient, sessionID, messageSource string) agent.RunConfig {
	if cfg == nil {
		cfg = &config.Config{}
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = desktopChatSessionID
	}
	messageSource = strings.TrimSpace(messageSource)
	if messageSource == "" {
		messageSource = desktopChatMessageSource
	}
	return agent.RunConfig{
		Config:             cfg,
		Logger:             s.Logger,
		LLMClient:          llmClient,
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
		Manifest:           tools.NewManifest(cfg.Directories.ToolsDir),
		CronManager:        s.CronManager,
		MissionManagerV2:   s.MissionManagerV2,
		CoAgentRegistry:    s.CoAgentRegistry,
		BudgetTracker:      s.BudgetTracker,
		DaemonSupervisor:   s.DaemonSupervisor,
		LLMGuardian:        s.LLMGuardian,
		PreparationService: s.PreparationService,
		WorkspaceSearch:    s.WorkspaceSearch,
		SessionID:          sessionID,
		MessageSource:      messageSource,
		VoiceOutputActive:  GetSpeakerMode(),
	}
}

func handleDesktopSlashCommand(s *Server, message string) (string, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(message), "/") {
		return "", false, nil
	}
	if s == nil {
		return "", true, fmt.Errorf("server not configured")
	}
	var promptsDir string
	if s != nil && s.Cfg != nil {
		promptsDir = s.Cfg.Directories.PromptsDir
	}
	cmdCtx := commands.Context{
		STM:              s.ShortTermMem,
		HM:               s.HistoryManager,
		Vault:            s.Vault,
		InventoryDB:      s.InventoryDB,
		BudgetTracker:    s.BudgetTracker,
		Cfg:              s.Cfg,
		PromptsDir:       promptsDir,
		WarningsRegistry: s.WarningsRegistry,
		Lang:             desktopUILanguage(s),
		SessionID:        desktopChatSessionID,
	}
	return commands.Handle(message, cmdCtx)
}

func desktopUILanguage(s *Server) string {
	if s == nil || s.Cfg == nil {
		return "de"
	}
	lang := strings.TrimSpace(s.Cfg.Server.UILanguage)
	if lang == "" {
		return "de"
	}
	return lang
}

func runDesktopAgentChat(ctx context.Context, s *Server, message string, chatContext desktopChatContext) string {
	turn, err := prepareDesktopAgentTurn(ctx, s, message, chatContext, false)
	if err != nil {
		return ""
	}
	broker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, desktopChatSessionID)}
	ctx, cancel := context.WithTimeout(ctx, desktopChatAgentTurnTimeout)
	defer cancel()
	var resp openai.ChatCompletionResponse
	var loopErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		resp, loopErr = agent.ExecuteAgentLoop(ctx, turn.req, turn.runCfg, false, broker)
		if loopErr != nil {
			broker.Send("error_recovery", chatCompletionErrorMessage(desktopUILanguage(s), loopErr))
		}
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return "The desktop agent request timed out."
	}
	answer := strings.TrimSpace(broker.finalResponse)
	if answer == "" && len(resp.Choices) > 0 {
		answer = strings.TrimSpace(resp.Choices[0].Message.Content)
	}
	return security.StripThinkingTags(answer)
}

func buildDesktopAgentPrompt(message string, chatContext desktopChatContext) string {
	context := buildDesktopAgentContext(chatContext)
	if strings.TrimSpace(message) == "" {
		return context
	}
	return strings.TrimSpace(context + "\n\nUser request:\n" + strings.TrimSpace(message))
}

func buildDesktopAgentContext(chatContext desktopChatContext) string {
	if isHomepageStudioChatContext(chatContext) {
		return buildHomepageStudioAgentContext(chatContext)
	}

	var b strings.Builder
	b.WriteString("The user is chatting from AuraGo Virtual Desktop. If they ask for desktop apps, widgets, or files, use the virtual_desktop tool and keep the browser desktop updated.")
	b.WriteString("\n\nNever use file_editor, filesystem, smart_file_read, or other agent_workspace file tools for Virtual Desktop paths. Paths beginning with Apps/ or Widgets/ live in the Virtual Desktop workspace, not agent_workspace/workdir; use virtual_desktop read_file, write_file, install_app, or open_in_app with the same path.")
	b.WriteString("\nFor existing desktop code files, prefer virtual_desktop search_file/read_file_excerpt plus patch_file. Use write_file only when replacing the whole file with complete, non-empty content; never call write_file with omitted or empty content.")
	b.WriteString("\n\nIf the current user request is a short approval or continuation, infer the referenced task from the visible chat history and continue the previous Virtual Desktop task. Do not ask for confirmation again, do not ask the user to repeat the task, and start with the appropriate tool call when files, apps, widgets, documents, or spreadsheets must be changed or opened.")
	b.WriteString("\n\nYou can open files in desktop apps using the virtual_desktop tool with operation \"open_in_app\". Available apps: editor (plain text workspace files), writer (word-processing documents, docx, html), sheets (spreadsheets, xlsx, csv), code-studio (code files, scripts). Code Studio mounts the virtual desktop workspace at /workspace, so Apps/<app_id>/game.js opens as /workspace/Apps/<app_id>/game.js. After editing a generated app, run it with open_app using the generated app id, or open_in_app with the entry path Apps/<app_id>/<entry> so AuraGo can infer the app. After creating or writing a file, proactively open it in the appropriate app so the user can see it immediately. Example: after writing a document, use open_in_app with app_id \"writer\" and path to the file.")
	if windowContext := buildDesktopWindowContextPrompt(chatContext.WindowContext); windowContext != "" {
		b.WriteString("\n\nThe user launched this chat turn from a Virtual Desktop window. Use this metadata only to understand the user's current app context; do not show the raw metadata back unless the user asks.")
		b.WriteString("\n")
		b.WriteString(desktopExternalData("desktop_window_context", windowContext, 12000))
	}
	if chatContext.Source == "code-studio" {
		b.WriteString("\n\nThe user is coding in Code Studio.")
		b.WriteString("\nImportant: Code Studio files live inside the virtual desktop workspace mounted at /workspace, not the homepage workspace and not agent_workspace. Do not use the homepage tool for Code Studio file questions. Prefer the code/content supplied in this prompt; if content is supplied, answer from it without trying to locate the file elsewhere.")
		if strings.TrimSpace(chatContext.CurrentFile) != "" {
			b.WriteString("\nCurrent file:\n")
			b.WriteString(desktopExternalData("desktop_current_file", chatContext.CurrentFile, 2048))
		}
		if strings.TrimSpace(chatContext.CurrentLanguage) != "" {
			b.WriteString("\nLanguage:\n")
			b.WriteString(desktopExternalData("desktop_current_language", chatContext.CurrentLanguage, 128))
		}
		if chatContext.CursorLine > 0 || chatContext.CursorColumn > 0 {
			b.WriteString(fmt.Sprintf("\nCursor: line %d, column %d", chatContext.CursorLine, chatContext.CursorColumn))
		}
		if len(chatContext.OpenFiles) > 0 {
			b.WriteString("\nOpen files:\n")
			b.WriteString(desktopExternalData("desktop_open_files", strings.Join(chatContext.OpenFiles, "\n"), 8192))
		}
		if strings.TrimSpace(chatContext.SelectedText) != "" {
			b.WriteString("\nSelected text:\n")
			b.WriteString(desktopExternalData("desktop_selected_text", chatContext.SelectedText, 24000))
		}
		if strings.TrimSpace(chatContext.SelectedText) == "" && strings.TrimSpace(chatContext.CurrentContent) != "" {
			b.WriteString("\nCurrent file content:\n")
			b.WriteString(desktopExternalData("desktop_current_content", chatContext.CurrentContent, 48000))
		}
	}
	if chatContext.Source == "openscad" {
		b.WriteString("\n\nThe user is working in the OpenSCAD desktop app. For model creation or preview/export requests, generate complete OpenSCAD source and call the native openscad_render tool. Always pass window_id from the Window ID in the desktop window context so the result updates the correct OpenSCAD window. Do not use generic filesystem paths or remote.files for the render path; the OpenSCAD tool writes model.scad, renders preview/export files, and emits an openscad_result event for the app.")
		if strings.TrimSpace(chatContext.CurrentContent) != "" {
			b.WriteString("\nCurrent OpenSCAD source:\n")
			b.WriteString(desktopExternalData("desktop_openscad_source", chatContext.CurrentContent, 48000))
		}
	}
	if chatContext.Source != "code-studio" && chatContext.Source != "openscad" && (strings.TrimSpace(chatContext.CurrentFile) != "" || len(chatContext.OpenFiles) > 0) {
		b.WriteString("\n\nThe user has attached desktop workspace file context. Use the virtual_desktop tool with operation \"read_file\" or the relevant desktop document/workbook tools when you need file contents; do not assume contents from the filename alone.")
		if chatContext.OriginApp == "editor" {
			b.WriteString("\nImportant: This task was launched from the Editor app. If the request asks you to change the attached file, write the result back to the same desktop file with virtual_desktop write_file, then call virtual_desktop open_in_app with app_id \"editor\" and the same path. Do not open Writer for this Editor-origin task unless the user explicitly asks for Writer or a word-processing document.")
		} else if chatContext.OriginApp == "writer" {
			b.WriteString("\nImportant: This task was launched from the Writer app. If the request asks you to change the attached file, write the result back to the same desktop document with office_document or virtual_desktop document operations, then call virtual_desktop open_in_app with app_id \"writer\" and the same path.")
		} else if chatContext.OriginApp == "sheets" {
			b.WriteString("\nImportant: This task was launched from the Sheets app. If the request asks you to change the attached file, write the result back to the same desktop workbook with office_workbook or virtual_desktop workbook operations, then call virtual_desktop open_in_app with app_id \"sheets\" and the same path.")
		}
		if strings.TrimSpace(chatContext.CurrentFile) != "" {
			b.WriteString("\nCurrent desktop file:\n")
			b.WriteString(desktopExternalData("desktop_current_file", chatContext.CurrentFile, 2048))
		}
		if len(chatContext.OpenFiles) > 0 {
			b.WriteString("\nAttached desktop files:\n")
			b.WriteString(desktopExternalData("desktop_open_files", strings.Join(chatContext.OpenFiles, "\n"), 8192))
		}
	}
	if strings.TrimSpace(chatContext.ImageBase64) != "" {
		b.WriteString("\n\nThe user attached a photo taken with the Camera app for this turn. If the provider supports multimodal input, the image is supplied separately as an image_url data URI. Do not store or request the raw image bytes.")
	}
	return b.String()
}

func isHomepageStudioChatContext(chatContext desktopChatContext) bool {
	if chatContext.HomepageMode || strings.EqualFold(strings.TrimSpace(chatContext.Source), "homepage-studio") {
		return true
	}
	if chatContext.WindowContext == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(chatContext.WindowContext.Source), "homepage-studio") ||
		strings.EqualFold(strings.TrimSpace(chatContext.WindowContext.AppID), "homepage-studio") ||
		strings.EqualFold(strings.TrimSpace(chatContext.WindowContext.StoreAppID), "homepage-studio")
}

func buildHomepageStudioAgentContext(chatContext desktopChatContext) string {
	var b strings.Builder
	b.WriteString("The user is working in Homepage Studio, AuraGo's homepage/site editor. Interpret short references like \"the page\" or \"die Seite\" as the current Homepage Studio site, not as a Virtual Desktop widget or app.")
	if target := strings.TrimSpace(chatContext.Target); target != "" {
		b.WriteString("\nTarget: ")
		b.WriteString(target)
	}
	b.WriteString("\nUse homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git for project lifecycle, file edits, checks, deploys, and git history. The legacy homepage tool is acceptable only when focused homepage tools are unavailable.")
	b.WriteString("\nDo not use virtual_desktop apps, widgets, or files for Homepage Studio site changes unless the user explicitly asks to change the Virtual Desktop UI.")
	b.WriteString("\nDo not use generic filesystem, file_editor, execute_shell, or execute_python for homepage workspace files; use homepage workspace tool paths instead.")
	b.WriteString("\nAfter meaningful site changes, refresh or verify the preview and update homepage_registry when available.")
	if windowContext := buildDesktopWindowContextPrompt(chatContext.WindowContext); windowContext != "" {
		b.WriteString("\n\nHomepage Studio launch window metadata follows. Use it only as trusted routing context; do not show the raw metadata back unless the user asks.")
		b.WriteString("\n")
		b.WriteString(desktopExternalData("desktop_window_context", windowContext, 12000))
	}
	if strings.TrimSpace(chatContext.ImageBase64) != "" {
		b.WriteString("\n\nThe user attached a photo taken with the Camera app for this turn. If the provider supports multimodal input, the image is supplied separately as an image_url data URI. Do not store or request the raw image bytes.")
	}
	return b.String()
}

func buildDesktopWindowContextPrompt(windowContext *desktopWindowContext) string {
	if windowContext == nil {
		return ""
	}
	var b strings.Builder
	appendLine := func(label, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(label)
		b.WriteString(": ")
		b.WriteString(value)
	}
	appendLine("Label", windowContext.Label)
	appendLine("App ID", windowContext.AppID)
	appendLine("Store app ID", windowContext.StoreAppID)
	appendLine("Window ID", windowContext.WindowID)
	appendLine("Purpose", windowContext.Purpose)
	appendLine("Guide", windowContext.Guide)
	for i, resource := range windowContext.Resources {
		if i >= 8 {
			break
		}
		if strings.TrimSpace(resource.Kind) == "" &&
			strings.TrimSpace(resource.Label) == "" &&
			strings.TrimSpace(resource.Path) == "" &&
			strings.TrimSpace(resource.ContainerPath) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("Resource")
		if strings.TrimSpace(resource.Label) != "" {
			b.WriteString(" ")
			b.WriteString(strings.TrimSpace(resource.Label))
		}
		b.WriteString(":")
		if strings.TrimSpace(resource.Kind) != "" {
			b.WriteString("\nKind: ")
			b.WriteString(strings.TrimSpace(resource.Kind))
		}
		if strings.TrimSpace(resource.Path) != "" {
			b.WriteString("\nPath: ")
			b.WriteString(strings.TrimSpace(resource.Path))
		}
		if strings.TrimSpace(resource.ContainerPath) != "" {
			b.WriteString("\nContainer path: ")
			b.WriteString(strings.TrimSpace(resource.ContainerPath))
		}
	}
	return b.String()
}

func desktopExternalData(kind, value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if maxBytes > 0 && len(value) > maxBytes {
		value = value[:maxBytes] + "\n[truncated]"
	}
	// Escape nested external_data tags to prevent injection that could break
	// the security wrapper boundary.
	value = strings.ReplaceAll(value, "<external_data>", "&lt;external_data&gt;")
	value = strings.ReplaceAll(value, "</external_data>", "&lt;/external_data&gt;")
	return fmt.Sprintf("<external_data type=%q\u003e\n%s\n</external_data>", kind, value)
}

type desktopReplyBroker struct {
	agent.FeedbackBroker
	finalResponse string
}

func (b *desktopReplyBroker) Send(event, message string) {
	if event == "final_response" {
		b.finalResponse = message
	}
	b.FeedbackBroker.Send(event, message)
}
