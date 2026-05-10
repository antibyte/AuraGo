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
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"aurago/internal/tools"
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
		unlockSession := lockSessionRequest("virtual-desktop")
		defer unlockSession()
		answer := runDesktopAgentChat(r.Context(), s, body.Message, body.Context)
		w.Header().Set("Content-Type", "application/json")
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
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		if s == nil || s.Cfg == nil {
			sseWriteData(w, "error", "server not configured")
			return
		}

		unlockSession := lockSessionRequest("virtual-desktop")
		defer unlockSession()

		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()
		llmClient := applyDesktopAgentProvider(r.Context(), s, &cfg)
		sessionID := "virtual-desktop"
		runCfg := agent.RunConfig{
			Config:             &cfg,
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
			SessionID:          sessionID,
			MessageSource:      "virtual_desktop_chat",
			VoiceOutputActive:  GetSpeakerMode(),
		}
		prompt := buildDesktopAgentPrompt(body.Message, body.Context)
		broker := &desktopStreamBroker{
			w:        w,
			flusher:  flusher,
			canFlush: canFlush,
		}
		llmCtx, llmCancel := context.WithTimeout(r.Context(), 10*time.Minute)
		defer llmCancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			sseBroker := NewSSEBrokerAdapterWithSession(s.SSE, sessionID)
			combinedBroker := &desktopStreamCombinedBroker{
				stream: broker,
				sse:    sseBroker,
			}
			agent.LoopbackContext(llmCtx, runCfg, prompt, combinedBroker)
		}()
		select {
		case <-done:
		case <-r.Context().Done():
			llmCancel()
			<-done
		}
		broker.mu.Lock()
		broker.closed = true
		broker.mu.Unlock()
		sseWriteDone(w)
		if canFlush {
			flusher.Flush()
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

type desktopStreamCombinedBroker struct {
	stream *desktopStreamBroker
	sse    *SSEBrokerAdapter
}

func (b *desktopStreamCombinedBroker) Send(event, message string) {
	b.sse.Send(event, message)
	b.stream.mu.Lock()
	if b.stream.closed {
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

func (b *desktopStreamCombinedBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
	b.sse.SendLLMStreamDelta(content, toolName, toolID, index, finishReason)
	b.stream.mu.Lock()
	if b.stream.closed {
		b.stream.mu.Unlock()
		return
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
	Source          string   `json:"source"`
	CurrentFile     string   `json:"current_file"`
	CurrentLanguage string   `json:"current_language"`
	CurrentContent  string   `json:"current_content"`
	CursorLine      int      `json:"cursor_line"`
	CursorColumn    int      `json:"cursor_column"`
	SelectedText    string   `json:"selected_text"`
	OpenFiles       []string `json:"open_files"`
	ImageBase64     string   `json:"image_base64,omitempty"`
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
	return llm.NewClientFromProviderDetails(provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID)
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

func runDesktopAgentChat(ctx context.Context, s *Server, message string, chatContext desktopChatContext) string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	cfg := *s.Cfg
	s.CfgMu.RUnlock()
	llmClient := applyDesktopAgentProvider(ctx, s, &cfg)
	sessionID := "virtual-desktop"
	runCfg := agent.RunConfig{
		Config:             &cfg,
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
		SessionID:          sessionID,
		MessageSource:      "virtual_desktop_chat",
		VoiceOutputActive:  GetSpeakerMode(),
	}
	prompt := buildDesktopAgentPrompt(message, chatContext)
	broker := &desktopReplyBroker{FeedbackBroker: NewSSEBrokerAdapterWithSession(s.SSE, sessionID)}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		agent.LoopbackContext(ctx, runCfg, prompt, broker)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return "The desktop agent request timed out."
	}
	return strings.TrimSpace(broker.finalResponse)
}

func buildDesktopAgentPrompt(message string, chatContext desktopChatContext) string {
	var b strings.Builder
	b.WriteString("The user is chatting from AuraGo Virtual Desktop. If they ask for desktop apps, widgets, or files, use the virtual_desktop tool and keep the browser desktop updated.")
	b.WriteString("\n\nYou can open files in desktop apps using the virtual_desktop tool with operation \"open_in_app\". Available apps: writer (documents, docx, html, md, txt), sheets (spreadsheets, xlsx, csv), code-studio (code files, scripts). After creating or writing a file, proactively open it in the appropriate app so the user can see it immediately. Example: after writing a document, use open_in_app with app_id \"writer\" and path to the file.")
	if chatContext.Source == "code-studio" {
		b.WriteString("\n\nThe user is coding in Code Studio.")
		b.WriteString("\nImportant: Code Studio files live inside the dedicated Code Studio container workspace, not the homepage workspace and not agent_workspace. Do not use the homepage tool for Code Studio file questions. Prefer the code/content supplied in this prompt; if content is supplied, answer from it without trying to locate the file elsewhere.")
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
	if chatContext.Source != "code-studio" && (strings.TrimSpace(chatContext.CurrentFile) != "" || len(chatContext.OpenFiles) > 0) {
		b.WriteString("\n\nThe user has attached desktop workspace file context. Use the virtual_desktop tool with operation \"read_file\" or the relevant desktop document/workbook tools when you need file contents; do not assume contents from the filename alone.")
		if strings.TrimSpace(chatContext.CurrentFile) != "" {
			b.WriteString("\nCurrent desktop file:\n")
			b.WriteString(desktopExternalData("desktop_current_file", chatContext.CurrentFile, 2048))
		}
		if len(chatContext.OpenFiles) > 0 {
			b.WriteString("\nAttached desktop files:\n")
			b.WriteString(desktopExternalData("desktop_open_files", strings.Join(chatContext.OpenFiles, "\n"), 8192))
		}
	}
	b.WriteString("\n\nUser request:\n")
	b.WriteString(desktopExternalData("desktop_user_request", message, 12000))
	if strings.TrimSpace(chatContext.ImageBase64) != "" {
		b.WriteString("\n\nThe user has attached a photo taken with the Camera app. The image is provided as base64-encoded JPEG data below. Describe and analyze what you see in the image.\n")
		b.WriteString(desktopExternalData("desktop_camera_image_base64", chatContext.ImageBase64, 614400))
	}
	return b.String()
}

func desktopExternalData(kind, value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if maxBytes > 0 && len(value) > maxBytes {
		value = value[:maxBytes] + "\n[truncated]"
	}
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
