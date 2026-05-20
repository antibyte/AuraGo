package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestBuildDesktopAgentPromptKeepsCodeStudioOutOfHomepageWorkspace(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Explain the current file.", desktopChatContext{
		Source:          "code-studio",
		CurrentFile:     "/workspace/hello.go",
		CurrentLanguage: "go",
		CurrentContent:  "package main\n\nfunc main() {}\n",
		OpenFiles:       []string{"/workspace/hello.go"},
	})

	for _, want := range []string{
		"Code Studio files live inside the virtual desktop workspace mounted at /workspace",
		"not the homepage workspace",
		"Do not use the homepage tool for Code Studio file questions",
		"Current file:\n<external_data type=\"desktop_current_file\">\n/workspace/hello.go",
		"Current file content:\n<external_data type=\"desktop_current_content\">\npackage main",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Code Studio prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptPrefersSelectedCodeOverWholeFile(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Explain selected code.", desktopChatContext{
		Source:         "code-studio",
		CurrentFile:    "/workspace/hello.go",
		CurrentContent: "package main\n\nfunc main() {}\n",
		SelectedText:   "func main() {}",
	})

	if !strings.Contains(prompt, "Selected text:\n<external_data type=\"desktop_selected_text\">\nfunc main() {}") {
		t.Fatalf("Code Studio prompt should include selected text, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Current file content:") {
		t.Fatalf("Code Studio prompt should not include whole file when selected text is available:\n%s", prompt)
	}
}

func TestBuildDesktopAgentPromptIncludesDesktopFileContext(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("What should I know about this file?", desktopChatContext{
		Source:      "desktop-file",
		CurrentFile: "Documents/report.md",
		OpenFiles:   []string{"Documents/report.md", "Desktop/notes.txt"},
	})

	for _, want := range []string{
		"The user has attached desktop workspace file context.",
		"Use the virtual_desktop tool",
		"Current desktop file:\n<external_data type=\"desktop_current_file\">\nDocuments/report.md",
		"Attached desktop files:\n<external_data type=\"desktop_open_files\">\nDocuments/report.md\nDesktop/notes.txt",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("desktop file prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptKeepsEditorTasksInEditor(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Please improve this text.", desktopChatContext{
		Source:      "desktop-file",
		OriginApp:   "editor",
		CurrentFile: "Documents/note.txt",
		OpenFiles:   []string{"Documents/note.txt"},
	})

	for _, want := range []string{
		"This task was launched from the Editor app",
		"write the result back to the same desktop file",
		"open_in_app with app_id \"editor\"",
		"Do not open Writer",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("editor-origin prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptKeepsWriterTasksInWriter(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Please polish this document.", desktopChatContext{
		Source:      "desktop-file",
		OriginApp:   "writer",
		CurrentFile: "Documents/report.docx",
		OpenFiles:   []string{"Documents/report.docx"},
	})

	for _, want := range []string{
		"This task was launched from the Writer app",
		"write the result back to the same desktop document",
		"open_in_app with app_id \"writer\"",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("writer-origin prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptKeepsSheetsTasksInSheets(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Please update this table.", desktopChatContext{
		Source:      "desktop-file",
		OriginApp:   "sheets",
		CurrentFile: "Documents/budget.xlsx",
		OpenFiles:   []string{"Documents/budget.xlsx"},
	})

	for _, want := range []string{
		"This task was launched from the Sheets app",
		"write the result back to the same desktop workbook",
		"open_in_app with app_id \"sheets\"",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("sheets-origin prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptForbidsGenericFileToolsForDesktopPaths(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Edit Apps/space-invaders/game.js", desktopChatContext{})

	for _, want := range []string{
		"Never use file_editor",
		"Apps/",
		"Widgets/",
		"virtual_desktop",
		"run it with open_app using the generated app id",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("desktop prompt missing routing guard %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptTurnsShortApprovalsIntoAction(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("continue", desktopChatContext{})

	for _, want := range []string{
		`short approval or continuation`,
		`infer the referenced task from the visible chat history`,
		`continue the previous Virtual Desktop task`,
		`Do not ask for confirmation again`,
		`start with the appropriate tool call`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("desktop prompt missing approval execution guard %q in:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, `"ja"`) || strings.Contains(prompt, `"ok"`) || strings.Contains(prompt, `"go ahead"`) {
		t.Fatalf("desktop prompt should not list language-specific approval examples:\n%s", prompt)
	}
}

func TestDesktopChatHandlersUseDirectAgentLoopForStreaming(t *testing.T) {
	t.Parallel()

	files := []string{"desktop_handlers.go", "desktop_handlers_chat.go"}
	var source string
	for _, name := range files {
		sourceBytes, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		source += string(sourceBytes)
	}
	for _, marker := range []string{
		"runDesktopAgentChat(r.Context(), s, body.Message, body.Context)",
		"prepareDesktopAgentTurn(r.Context(), s, body.Message, body.Context, true)",
		"agent.ExecuteAgentLoop(llmCtx, turn.req, turn.runCfg, true, combinedBroker)",
		"agent.ExecuteAgentLoop(ctx, turn.req, turn.runCfg, false, broker)",
		"context.WithTimeout(ctx, desktopChatAgentTurnTimeout)",
		"lockSessionRequest(desktopChatSessionID)",
		"desktopChatSessionID               = \"virtual-desktop\"",
		"desktopChatMessageSource           = \"virtual_desktop_chat\"",
		"event == \"done\"",
		"latestDesktopAssistantMessage(b.shortTermMem, b.sessionID)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat cancellation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "agent.LoopbackContext") {
		t.Fatal("desktop chat paths must not use LoopbackContext; streaming needs the primary agent loop")
	}
	if strings.Contains(source, "context.WithTimeout(context.Background(), 10*time.Minute)") {
		t.Fatal("desktop chat must not use context.Background for agent loopback timeout")
	}
}

func TestDesktopChatStreamKeepsConnectionAliveDuringLongTurns(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("desktop_handlers_chat.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_handlers_chat.go: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"desktopChatAgentTurnTimeout",
		"30 * time.Minute",
		"desktopChatStreamHeartbeatInterval",
		"15 * time.Second",
		"context.WithTimeout(r.Context(), desktopChatAgentTurnTimeout)",
		"heartbeat := time.NewTicker(desktopChatStreamHeartbeatInterval)",
		"case <-heartbeat.C:",
		"broker.sendHeartbeat()",
		"writeSSEComment(b.w, b.flusher, \"heartbeat\")",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat stream missing long-running connection marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		"context.WithTimeout(r.Context(), 10*time.Minute)",
		"context.WithTimeout(ctx, 10*time.Minute)",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("desktop chat stream still uses short fixed timeout marker %q", forbidden)
		}
	}
}

func TestPrepareDesktopAgentTurnKeepsFullVisibleDesktopHistory(t *testing.T) {
	t.Parallel()

	s := newTestDesktopChatServer(t)
	for i := 0; i < 30; i++ {
		role := openai.ChatMessageRoleUser
		if i%2 == 1 {
			role = openai.ChatMessageRoleAssistant
		}
		if _, err := s.ShortTermMem.InsertMessage(desktopChatSessionID, role, "visible-history-message-"+string(rune('a'+i%26)), false, false); err != nil {
			t.Fatalf("InsertMessage %d: %v", i, err)
		}
	}

	turn, err := prepareDesktopAgentTurn(context.Background(), s, "new request", desktopChatContext{}, true)
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurn: %v", err)
	}
	if !turn.req.Stream {
		t.Fatal("prepared desktop stream request must enable streaming")
	}
	if turn.req.Model != s.Cfg.LLM.Model {
		t.Fatalf("prepared model = %q, want %q", turn.req.Model, s.Cfg.LLM.Model)
	}
	if turn.runCfg.SessionID != desktopChatSessionID {
		t.Fatalf("runCfg.SessionID = %q, want %q", turn.runCfg.SessionID, desktopChatSessionID)
	}
	if turn.runCfg.MessageSource != desktopChatMessageSource {
		t.Fatalf("runCfg.MessageSource = %q, want %q", turn.runCfg.MessageSource, desktopChatMessageSource)
	}
	if len(turn.req.Messages) < 31 {
		t.Fatalf("prepared request kept %d messages, want at least all 30 visible history messages plus current request", len(turn.req.Messages))
	}
}

func TestPrepareDesktopAgentTurnPersistsRawUserMessageOnly(t *testing.T) {
	t.Parallel()

	s := newTestDesktopChatServer(t)
	message := "füge dem Space-Invaders-Spiel Glow hinzu"

	turn, err := prepareDesktopAgentTurn(context.Background(), s, message, desktopChatContext{
		Source:         "code-studio",
		CurrentFile:    "/workspace/Apps/space-invaders/game.js",
		CurrentContent: "console.log('desktop context');",
	}, true)
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurn: %v", err)
	}

	history, err := s.ShortTermMem.GetSessionMessages(desktopChatSessionID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].Content != message {
		t.Fatalf("desktop history persisted %q, want raw user message %q", history[0].Content, message)
	}
	for _, forbidden := range []string{"AuraGo Virtual Desktop", "desktop_user_request", "<external_data"} {
		if strings.Contains(history[0].Content, forbidden) {
			t.Fatalf("desktop history leaked prompt/context marker %q in %q", forbidden, history[0].Content)
		}
	}
	if turn.runCfg.Config == nil || !strings.Contains(turn.runCfg.Config.Agent.AdditionalPrompt, "AuraGo Virtual Desktop") {
		t.Fatalf("desktop routing context must be injected as trusted prompt context, got %q", turn.runCfg.Config.Agent.AdditionalPrompt)
	}
	if !strings.Contains(turn.runCfg.Config.Agent.AdditionalPrompt, `type="desktop_current_content"`) {
		t.Fatalf("desktop file context should remain external_data in trusted prompt context: %q", turn.runCfg.Config.Agent.AdditionalPrompt)
	}
}

func TestPrepareDesktopAgentTurnKeepsRawIntentInCurrentLLMRequest(t *testing.T) {
	t.Parallel()

	s := newTestDesktopChatServer(t)
	message := "starte die App"

	turn, err := prepareDesktopAgentTurn(context.Background(), s, message, desktopChatContext{}, false)
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurn: %v", err)
	}
	if len(turn.req.Messages) == 0 {
		t.Fatal("prepared request has no messages")
	}
	last := turn.req.Messages[len(turn.req.Messages)-1]
	if last.Role != openai.ChatMessageRoleUser || last.Content != message {
		t.Fatalf("last prepared message = role %q content %q, want raw user message %q", last.Role, last.Content, message)
	}
	if strings.Contains(last.Content, "desktop_user_request") || strings.Contains(last.Content, "The user is chatting from AuraGo Virtual Desktop") {
		t.Fatalf("desktop request content still contains prompt wrapper: %q", last.Content)
	}
}

func TestPrepareDesktopAgentTurnSanitizesOrphanedToolMessages(t *testing.T) {
	t.Parallel()

	s := newTestDesktopChatServer(t)
	if _, err := s.ShortTermMem.InsertMessage(desktopChatSessionID, openai.ChatMessageRoleUser, "please run a tool", false, false); err != nil {
		t.Fatalf("InsertMessage user: %v", err)
	}
	if _, err := s.ShortTermMem.InsertMessage(desktopChatSessionID, openai.ChatMessageRoleTool, `{"status":"orphaned"}`, false, false); err != nil {
		t.Fatalf("InsertMessage tool: %v", err)
	}

	turn, err := prepareDesktopAgentTurn(context.Background(), s, "continue", desktopChatContext{}, false)
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurn: %v", err)
	}
	for _, msg := range turn.req.Messages {
		if msg.Role == openai.ChatMessageRoleTool {
			t.Fatalf("prepared request still contains orphaned tool message: %#v", msg)
		}
	}
}

func TestPrepareDesktopAgentTurnDoesNotPersistCameraBase64(t *testing.T) {
	t.Parallel()

	s := newTestDesktopChatServer(t)
	s.Cfg.LLM.Multimodal = true
	s.Cfg.LLM.ProviderType = "openai"
	imagePayload := "abc123desktopcamera"

	turn, err := prepareDesktopAgentTurn(context.Background(), s, "what is in the image?", desktopChatContext{
		ImageBase64: imagePayload,
	}, true)
	if err != nil {
		t.Fatalf("prepareDesktopAgentTurn: %v", err)
	}
	var sawMultimodalImage bool
	for _, msg := range turn.req.Messages {
		if strings.Contains(msg.Content, imagePayload) {
			t.Fatalf("prepared multimodal request should not duplicate camera base64 in text content: %#v", msg)
		}
		for _, part := range msg.MultiContent {
			if part.ImageURL != nil && strings.Contains(part.ImageURL.URL, imagePayload) {
				sawMultimodalImage = true
			}
		}
	}
	if !sawMultimodalImage {
		t.Fatal("prepared multimodal request should carry camera image as image_url data URI")
	}

	history, err := s.ShortTermMem.GetSessionMessages(desktopChatSessionID)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	for _, msg := range history {
		if strings.Contains(msg.Content, imagePayload) {
			t.Fatalf("camera base64 leaked into SQLite history message: %q", msg.Content)
		}
	}
}

func TestDesktopChatStreamEmitsLLMDeltaBeforeDone(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if !req.Stream {
			t.Fatal("desktop stream handler must call the LLM with stream=true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Desktop stream works.\"},\"finish_reason\":null}]}\n\n")
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(upstream.Close)

	s := newTestDesktopChatServer(t)
	s.Cfg.Directories.PromptsDir = testPromptsDir(t)
	s.Cfg.Directories.SkillsDir = t.TempDir()
	s.Cfg.Agent.SystemLanguage = "English"
	s.Cfg.Agent.ContextWindow = 32768
	s.Cfg.CircuitBreaker.LLMTimeoutSeconds = 30
	s.Cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 10
	s.Registry = tools.NewProcessRegistry(s.Logger)

	openaiCfg := openai.DefaultConfig("test-key")
	openaiCfg.BaseURL = upstream.URL + "/v1"
	s.LLMClient = openai.NewClientWithConfig(openaiCfg)

	body := bytes.NewBufferString(`{"message":"hello desktop"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/chat/stream", body)
	handleDesktopChatStream(s).ServeHTTP(rec, req)

	streamBody := rec.Body.String()
	deltaIdx := strings.Index(streamBody, `"event":"llm_stream_delta"`)
	doneIdx := strings.Index(streamBody, "[DONE]")
	if deltaIdx == -1 {
		t.Fatalf("desktop stream did not emit llm_stream_delta before completion:\n%s", streamBody)
	}
	if doneIdx == -1 {
		t.Fatalf("desktop stream did not emit [DONE]:\n%s", streamBody)
	}
	if deltaIdx > doneIdx {
		t.Fatalf("desktop stream emitted [DONE] before llm_stream_delta:\n%s", streamBody)
	}
}

func TestDesktopChatStreamPreservesUTF8AcrossHoldBoundary(t *testing.T) {
	content := "aaaaaü" + strings.Repeat("b", 16)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path: %s", r.URL.Path)
		}
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if !req.Stream {
			t.Fatal("desktop stream handler must call the LLM with stream=true")
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		chunk, err := json.Marshal(map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "test-model",
			"choices": []map[string]interface{}{
				{
					"index":         0,
					"delta":         map[string]string{"content": content},
					"finish_reason": nil,
				},
			},
		})
		if err != nil {
			t.Fatalf("marshal stream chunk: %v", err)
		}
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		fmt.Fprint(w, "data: {\"id\":\"chatcmpl-test\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"test-model\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(upstream.Close)

	s := newTestDesktopChatServer(t)
	s.Cfg.Directories.PromptsDir = testPromptsDir(t)
	s.Cfg.Directories.SkillsDir = t.TempDir()
	s.Cfg.Agent.SystemLanguage = "English"
	s.Cfg.Agent.ContextWindow = 32768
	s.Cfg.CircuitBreaker.LLMTimeoutSeconds = 30
	s.Cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 10
	s.Registry = tools.NewProcessRegistry(s.Logger)

	openaiCfg := openai.DefaultConfig("test-key")
	openaiCfg.BaseURL = upstream.URL + "/v1"
	s.LLMClient = openai.NewClientWithConfig(openaiCfg)

	body := bytes.NewBufferString(`{"message":"hello desktop"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/desktop/chat/stream", body)
	handleDesktopChatStream(s).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/event-stream; charset=utf-8", got)
	}

	var joined strings.Builder
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var payload struct {
			Event   string `json:"event"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload); err == nil && payload.Event == "llm_stream_delta" {
			joined.WriteString(payload.Content)
		}
	}
	got := joined.String()
	if !utf8.ValidString(got) {
		t.Fatalf("joined stream content is invalid UTF-8: %q", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("joined stream content contains replacement characters: %q", got)
	}
	if got != content {
		t.Fatalf("joined stream content = %q, want %q", got, content)
	}
}

func TestLatestDesktopAssistantMessageReturnsLastAssistantReply(t *testing.T) {
	t.Parallel()

	stm, err := memory.NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	sessionID := "virtual-desktop"
	if _, err := stm.InsertMessage(sessionID, "assistant", "first", false, false); err != nil {
		t.Fatalf("InsertMessage first: %v", err)
	}
	if _, err := stm.InsertMessage(sessionID, "user", "next", false, false); err != nil {
		t.Fatalf("InsertMessage user: %v", err)
	}
	if _, err := stm.InsertMessage(sessionID, "assistant", "<think>hidden</think>final answer", false, false); err != nil {
		t.Fatalf("InsertMessage final: %v", err)
	}

	if got := latestDesktopAssistantMessage(stm, sessionID); got != "final answer" {
		t.Fatalf("latestDesktopAssistantMessage = %q, want final answer", got)
	}
}

func newTestDesktopChatServer(t *testing.T) *Server {
	t.Helper()
	logger := slog.Default()
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.Multimodal = false
	cfg.Directories.ToolsDir = t.TempDir()
	cfg.Directories.WorkspaceDir = t.TempDir()
	cfg.Server.UILanguage = "en"
	cfg.CircuitBreaker.MaxToolCalls = 10
	cfg.CircuitBreaker.LLMTimeoutSeconds = 30
	cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 10

	return &Server{
		Cfg:          cfg,
		Logger:       logger,
		SSE:          NewSSEBroadcaster(),
		ShortTermMem: stm,
	}
}

func testPromptsDir(t *testing.T) string {
	t.Helper()
	promptsDir, err := filepath.Abs(filepath.Join("..", "..", "prompts"))
	if err != nil {
		t.Fatalf("resolve prompts dir: %v", err)
	}
	return promptsDir
}

func TestDesktopChatUIRestoresAndClearsVirtualDesktopHistory(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "agent-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"data-chat-clear-history",
		"loadDesktopChatHistory(host)",
		"api('/history?session_id=virtual-desktop')",
		"api('/clear?session_id=virtual-desktop', { method: 'DELETE' })",
		"type=[\"']desktop_user_request",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat history UI missing marker %q", marker)
		}
	}
}

func TestDesktopChatUIHandlesQuestionUserPrompts(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "agent-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"event === 'question_user'",
		"showDesktopQuestionModal(host, normalizeDesktopQuestionPayload(data))",
		"fetch('/api/agent/question-response'",
		"session_id: 'virtual-desktop'",
		"desktop.chat_question_waiting",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat question UI missing marker %q", marker)
		}
	}
}

func TestDesktopChatUIHandlesStreamingFallbackAndDone(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "agent-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"event === 'final_response'",
		"flushStreamingBubble();",
		"event === 'done'",
		"doFinalize();",
		"event === 'token_update'",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop streaming UI missing marker %q", marker)
		}
	}
}

func TestDesktopChatUILaunchContextSupportsFileAutosend(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "agent-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"chat_autosend",
		"submitDesktopChatMessage(host, input.value.trim())",
		"if (context.chat_autosend && input.value.trim() && !state.chatBusy)",
		"if (context.chat_autosend && state.chatBusy)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat autosend UI missing marker %q", marker)
		}
	}

	windowRuntime, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "core", "window-shell-runtime.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop window runtime: %v", err)
	}
	if !strings.Contains(string(windowRuntime), "applyChatLaunchContext(existing.id, context)") {
		t.Fatal("agent-chat launches should reuse the existing chat window and merge launch context")
	}
}

func TestDesktopJSONHandlersUseBodyLimits(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"desktop_handlers.go", "desktop_office_handlers.go", "desktop_looper_handlers.go"} {
		sourceBytes, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		source := string(sourceBytes)
		if name != "desktop_handlers.go" && strings.Contains(source, "json.NewDecoder(r.Body).Decode") {
			t.Fatalf("%s decodes request JSON without desktop body limit helper", name)
		}
	}
	sourceBytes, err := os.ReadFile("desktop_handlers.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_handlers.go: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"const desktopSmallJSONBodyLimit",
		"func decodeDesktopJSON",
		"http.MaxBytesReader(w, r.Body, maxBytes)",
		"decodeDesktopJSON(w, r,",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop body limit helper missing marker %q", marker)
		}
	}
	if strings.Count(source, "json.NewDecoder(r.Body).Decode") != 1 {
		t.Fatal("desktop_handlers.go should use json.NewDecoder(r.Body) only inside decodeDesktopJSON")
	}
}
