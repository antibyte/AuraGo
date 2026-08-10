package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

type rejectingBudgetTestClient struct {
	called bool
}

func (c *rejectingBudgetTestClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.called = true
	return openai.ChatCompletionResponse{}, errors.New("LLM must not be called for an impossible request")
}

func (c *rejectingBudgetTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	c.called = true
	return nil, errors.New("LLM must not be called for an impossible request")
}

func TestHandleChatCompletionsReturnsSanitized413ForImpossibleBudget(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	cfg := &config.Config{}
	cfg.LLM.Provider = "main"
	cfg.LLM.ProviderType = "ollama"
	cfg.LLM.Model = "unknown-budget-test-model"
	cfg.Providers = []config.ProviderEntry{{
		ID:              "main",
		Type:            "ollama",
		BaseURL:         "http://127.0.0.1:11434/v1",
		Model:           cfg.LLM.Model,
		ContextWindow:   8192,
		MaxOutputTokens: 1024,
	}}
	cfg.LLM.BaseURL = cfg.Providers[0].BaseURL
	cfg.SpeechLab = config.SpeechLabConfig{
		Enabled: true, BaseURL: "http://127.0.0.1:8765", ChatInputEnabled: true, ChatLLMProviderID: "main",
	}
	client := &rejectingBudgetTestClient{}
	server := &Server{
		Cfg:            cfg,
		Logger:         logger,
		LLMClient:      client,
		ShortTermMem:   stm,
		HistoryManager: memory.NewEphemeralHistoryManager(),
		IsFirstStart:   true,
	}
	originalFactory := speechLabChatClientFactory
	t.Cleanup(func() { speechLabChatClientFactory = originalFactory })
	speechLabChatClientFactory = func(*config.Config, string, string, string, string) llm.ChatClient { return client }

	const marker = "PROMPT_CONTENT_MUST_NOT_LEAK"
	transcript := marker + strings.Repeat(" token", 12000)
	payload, err := json.Marshal(openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: transcript,
		}},
		MaxTokens: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(payload))
	turnToken := server.speechLabTokens().Issue("default", transcript)
	req.Header.Set(speechLabChatTurnTokenHeader, turnToken)
	rec := httptest.NewRecorder()

	handleChatCompletions(server, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
	if client.called {
		t.Fatal("LLM client was called after the request failed budget preflight")
	}
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	if len(response) != 1 || response["error"] != "context_budget_exceeded" {
		t.Fatalf("response = %#v, want sanitized typed error", response)
	}
	if strings.Contains(rec.Body.String(), marker) {
		t.Fatal("413 response leaked prompt content")
	}
	if messages, err := stm.GetSessionMessages("default"); err != nil || len(messages) != 0 {
		t.Fatalf("rejected request persisted STM messages: count=%d err=%v", len(messages), err)
	}
	if got := len(server.HistoryManager.GetAll()); got != 0 {
		t.Fatalf("rejected request persisted history messages: %d", got)
	}
	if server.firstStartDone {
		t.Fatal("rejected request consumed first-start state")
	}
	if !server.speechLabTokens().Consume(turnToken, "default", transcript) {
		t.Fatal("rejected request consumed Speech Lab provenance token")
	}
}
