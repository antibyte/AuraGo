package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/llm"
	"aurago/internal/tools"
	openai "github.com/sashabaranov/go-openai"
)

type mailEventTestClient struct {
	requests []openai.ChatCompletionRequest
}

func (c *mailEventTestClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonStop, Message: openai.ChatCompletionMessage{Role: "assistant", Content: "No actionable new mail. <done/>"}}}}, nil
}
func (c *mailEventTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, fmt.Errorf("unexpected stream")
}

func TestMailIngressNeverInheritsDefaultConversation(t *testing.T) {
	s := newOperatorCommandTestServer(t)
	var logs bytes.Buffer
	s.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.Directories.PromptsDir = testPromptsDir(t)
	s.Cfg.Directories.ToolsDir = t.TempDir()
	s.Cfg.Directories.SkillsDir = t.TempDir()
	s.Cfg.Agent.SystemLanguage = "English"
	s.Cfg.Agent.ContextWindow = 65536
	s.Cfg.CircuitBreaker.MaxToolCalls = 4
	s.Registry = tools.NewProcessRegistry(s.Logger)
	s.Cfg.LLM.Model = "gpt-4o-mini"
	s.Cfg.LLM.ProviderType = "openai"
	client := &mailEventTestClient{}
	s.LLMClient = client
	const oldTask = "PRIVATE_ORDO_TASK_SENTINEL: continue updating old project reports"
	s.HistoryManager.Add("user", oldTask, 1, false, false)
	s.HistoryManager.CurrentSummary = oldTask
	const mail = "[EMAIL NOTIFICATION] fixture account: newsletter UID 42"
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"`+mail+`"}]}`))
	req.RemoteAddr = "127.0.0.1:43210"
	req.Header.Set("X-Internal-FollowUp", "true")
	req.Header.Set("X-Internal-Token", s.internalToken)
	req.Header.Set("X-External-Source", "email")
	req.Header.Set("X-Session-ID", "default")
	rec := httptest.NewRecorder()
	handleChatCompletions(s, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(client.requests) == 0 {
		t.Fatalf("mail did not reach fixture model: %s logs=%s", rec.Body.String(), logs.String())
	}
	for _, request := range client.requests {
		raw, _ := json.Marshal(request.Messages)
		if strings.Contains(string(raw), oldTask) {
			t.Fatal("default conversation leaked into mail event")
		}
	}
	foundPolicy := false
	for _, message := range client.requests[0].Messages {
		foundPolicy = foundPolicy || strings.Contains(message.Content, "[MAIL EVENT]")
	}
	if !foundPolicy {
		t.Fatal("independent mail instructions missing")
	}
	messages, err := s.ShortTermMem.GetSessionMessages(tools.MailEventSessionID(mail))
	if err != nil || len(messages) == 0 {
		t.Fatalf("isolated event was not persisted: %v", err)
	}
}
