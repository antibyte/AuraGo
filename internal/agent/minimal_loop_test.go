package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type minimalLoopRouteClient struct {
	routes   []llm.ModelRoute
	requests []openai.ChatCompletionRequest
	respond  func(openai.ChatCompletionRequest, int) (openai.ChatCompletionResponse, error)
}

func (c *minimalLoopRouteClient) CandidateRoutes(openai.ChatCompletionRequest) []llm.ModelRoute {
	return append([]llm.ModelRoute(nil), c.routes...)
}

func (c *minimalLoopRouteClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	if c.respond != nil {
		return c.respond(req, len(c.requests))
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "done"},
	}}}, nil
}

func (c *minimalLoopRouteClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming not implemented")
}

func minimalLoopTestRoutes() []llm.ModelRoute {
	return []llm.ModelRoute{
		{ProviderID: "primary", ProviderType: "custom", Model: "primary-model", Primary: true, ContextWindowOverride: 6000, MaxOutputTokensOverride: 512},
		{ProviderID: "fallback", ProviderType: "custom", Model: "fallback-model", ContextWindowOverride: 5000, MaxOutputTokensOverride: 512},
	}
}

func TestPrepareMinimalLoopRequestFitsEveryRouteAndKeepsToolGroupsAtomic(t *testing.T) {
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	messages := make([]openai.ChatCompletionMessage, 0, 20)
	for i := 0; i < 5; i++ {
		callID := "call-" + string(rune('a'+i))
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: strings.Repeat("old user context ", 90)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{
				ID: callID, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`},
			}}},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleTool, ToolCallID: callID, Content: strings.Repeat("large tool result ", 100)},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "old result"},
		)
	}
	messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "latest mandatory task"})
	req := openai.ChatCompletionRequest{Model: "primary-model", Messages: messages, MaxTokens: 512}

	prepared, err := prepareMinimalLoopRequest(context.Background(), cfg, client, &req, MinimalSystemPromptBuilder(nil), nil, logger, newTokenCountCache(512), 0)
	if err != nil {
		t.Fatalf("prepareMinimalLoopRequest: %v", err)
	}
	if len(prepared.Usage) != 2 {
		t.Fatalf("route usage count = %d, want 2", len(prepared.Usage))
	}
	for _, usage := range prepared.Usage {
		if !usage.Fits || usage.TotalTokens > usage.ContextWindow {
			t.Fatalf("route did not fit: %+v", usage)
		}
	}
	if !containsMessage(req.Messages, openai.ChatMessageRoleUser, "latest mandatory task") {
		t.Fatalf("latest task was trimmed: %#v", req.Messages)
	}
	if _, dropped := SanitizeToolMessages(req.Messages); dropped != 0 {
		t.Fatalf("route-aware trim split tool groups; sanitizer would drop %d messages", dropped)
	}
	systemCount := 0
	for _, message := range req.Messages {
		if message.Role == openai.ChatMessageRoleSystem {
			systemCount++
		}
	}
	if systemCount != 1 {
		t.Fatalf("system message count = %d, want 1", systemCount)
	}
}

func TestExecuteMinimalLoopNoToolsWritesFinalPromptLog(t *testing.T) {
	logDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	cfg.Logging.EnablePromptLog = true
	cfg.Logging.LogDir = logDir
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	dispatchCtx := &DispatchContext{Cfg: cfg, LLMClient: client, SessionID: "looper"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, history, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", MinimalSystemPromptBuilder(nil), "finish the task", nil, dispatchCtx, nil, logger, &MinimalLoopOptions{MaxToolRounds: 0})
	if err != nil {
		t.Fatalf("ExecuteMinimalLoop: %v", err)
	}
	if result.Response != "done" || len(client.requests) != 1 {
		t.Fatalf("result=%+v calls=%d", result, len(client.requests))
	}
	if len(client.requests[0].Tools) != 0 || len(history) == 0 {
		t.Fatalf("no-tools request was not preserved: %#v", client.requests[0])
	}
	raw, err := os.ReadFile(filepath.Join(logDir, "prompts.log"))
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	for _, field := range []string{"\"provider\":\"custom\"", "\"prompt_revision\"", "\"builder_prompt_revision\"", "\"tool_catalog_hash\"", "\"build_id\""} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("prompt log missing %s: %s", field, raw)
		}
	}
}

func TestExecuteMinimalLoopRebudgetsNoToolSummary(t *testing.T) {
	tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "test_tool", Parameters: map[string]any{"type": "object"}}}
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	client.respond = func(req openai.ChatCompletionRequest, _ int) (openai.ChatCompletionResponse, error) {
		if len(req.Tools) == 0 {
			return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "summary"}}}}, nil
		}
		return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{ID: "call-loop", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{}`}}},
		}}}}, nil
	}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	dispatchCtx := &DispatchContext{Cfg: cfg, LLMClient: client, SessionID: "looper"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, history, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", MinimalSystemPromptBuilder(nil), "use the tool", []openai.Tool{tool}, dispatchCtx, nil, logger, &MinimalLoopOptions{MaxToolRounds: 1})
	if err != nil {
		t.Fatalf("ExecuteMinimalLoop: %v", err)
	}
	if result.Response != "summary" || len(client.requests) != 3 {
		t.Fatalf("result=%+v calls=%d", result, len(client.requests))
	}
	if len(client.requests[len(client.requests)-1].Tools) != 0 {
		t.Fatal("summary request retained tool schemas")
	}
	if _, dropped := SanitizeToolMessages(history); dropped != 0 {
		t.Fatalf("summary history contains orphaned tool messages: %d", dropped)
	}
}

func TestExecuteMinimalLoopCancelledBeforeSend(t *testing.T) {
	client := &minimalLoopRouteClient{routes: minimalLoopTestRoutes()}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	dispatchCtx := &DispatchContext{Cfg: cfg, LLMClient: client, SessionID: "looper"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := ExecuteMinimalLoop(ctx, client, "primary-model", MinimalSystemPromptBuilder(nil), "cancelled task", nil, dispatchCtx, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), &MinimalLoopOptions{MaxToolRounds: 0})
	if err == nil {
		t.Fatal("expected cancelled minimal loop to fail")
	}
	if len(client.requests) != 0 {
		t.Fatalf("LLM calls = %d, want 0", len(client.requests))
	}
}

func TestExecuteMinimalLoopFailsClosedWhenRequiredToolSchemaCannotFit(t *testing.T) {
	client := &minimalLoopRouteClient{routes: []llm.ModelRoute{{
		ProviderID: "small", ProviderType: "custom", Model: "small-model", Primary: true,
		ContextWindowOverride: 4096, MaxOutputTokensOverride: 1000,
	}}}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 4096
	dispatchCtx := &DispatchContext{Cfg: cfg, LLMClient: client, SessionID: "looper"}
	tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "required_large_tool", Description: strings.Repeat("required schema detail ", 5000),
		Parameters: map[string]any{"type": "object"},
	}}

	_, _, err := ExecuteMinimalLoop(context.Background(), client, "small-model", MinimalSystemPromptBuilder(nil), "use the required tool", []openai.Tool{tool}, dispatchCtx, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), &MinimalLoopOptions{MaxToolRounds: 1})
	if !IsContextBudgetExceeded(err) {
		t.Fatalf("error = %v, want context_budget_exceeded", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("LLM calls = %d, want 0", len(client.requests))
	}
}
