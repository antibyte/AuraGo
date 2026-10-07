package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

type usageRegressionStream struct {
	chunks           []openai.ChatCompletionStreamResponse
	terminal         error
	index            int
	terminalReturned bool
}

func (s *usageRegressionStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if s.index < len(s.chunks) {
		chunk := s.chunks[s.index]
		s.index++
		return chunk, nil
	}
	if s.terminal != nil && !s.terminalReturned {
		s.terminalReturned = true
		return openai.ChatCompletionStreamResponse{}, s.terminal
	}
	return openai.ChatCompletionStreamResponse{}, io.EOF
}

func (*usageRegressionStream) Close() error { return nil }

type usageRegressionClient struct {
	routes    []llm.ModelRoute
	streams   []*usageRegressionStream
	requests  []openai.ChatCompletionRequest
	responses []openai.ChatCompletionResponse
}

func (c *usageRegressionClient) CandidateRoutes(openai.ChatCompletionRequest) []llm.ModelRoute {
	return append([]llm.ModelRoute(nil), c.routes...)
}

func (c *usageRegressionClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	if len(c.responses) == 0 {
		return openai.ChatCompletionResponse{}, errors.New("no scripted response remains")
	}
	response := c.responses[0]
	c.responses = c.responses[1:]
	return response, nil
}

func (c *usageRegressionClient) CreateChatCompletionStream(_ context.Context, req openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	c.requests = append(c.requests, req)
	if len(c.streams) == 0 {
		return nil, errors.New("no scripted stream remains")
	}
	stream := c.streams[0]
	c.streams = c.streams[1:]
	return stream, nil
}

type usageRegressionBroker struct {
	updates    int
	prompt     int
	completion int
	total      int
	estimated  bool
}

func (*usageRegressionBroker) Send(string, string)                                    {}
func (*usageRegressionBroker) SendJSON(string)                                        {}
func (*usageRegressionBroker) SendLLMStreamDelta(string, string, string, int, string) {}
func (*usageRegressionBroker) SendLLMStreamDone(string)                               {}
func (b *usageRegressionBroker) SendTokenUpdate(prompt, completion, total, _, _ int, estimated, _ bool, _ string) {
	b.updates++
	b.prompt += prompt
	b.completion += completion
	b.total += total
	b.estimated = b.estimated || estimated
}
func (*usageRegressionBroker) SendThinkingBlock(string, string, string) {}

func usageRegressionChunk(model string, usage *openai.Usage, delta openai.ChatCompletionStreamChoiceDelta, finish openai.FinishReason) openai.ChatCompletionStreamResponse {
	chunk := openai.ChatCompletionStreamResponse{Model: model, Usage: usage}
	if delta.Content != "" || delta.ReasoningContent != "" || delta.Role != "" || len(delta.ToolCalls) > 0 || finish != "" {
		chunk.Choices = []openai.ChatCompletionStreamChoice{{Index: 0, Delta: delta, FinishReason: finish}}
	}
	return chunk
}

func usageRegressionRoutes() []llm.ModelRoute {
	return []llm.ModelRoute{{ProviderID: "usage-fixture", ProviderType: "custom", Model: "primary-model", Primary: true, ContextWindowOverride: 32768, MaxOutputTokensOverride: 2048}}
}

func newUsageRegressionTracker(t *testing.T, cfg *config.Config) *budget.Tracker {
	t.Helper()
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 100
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	t.Cleanup(tracker.Flush)
	return tracker
}

func TestStreamingResponseAssemblesAndAccountsPartialToolOutput(t *testing.T) {
	for _, tc := range []struct {
		name           string
		usageChunks    []*openai.Usage
		wantPrompt     int
		wantCompletion int
		wantEstimate   bool
	}{
		{name: "prompt only estimates observed text reasoning and tool call", usageChunks: []*openai.Usage{{PromptTokens: 29}, {PromptTokens: 29}}, wantPrompt: 29, wantEstimate: true},
		{name: "repeated cumulative provider components are not summed", usageChunks: []*openai.Usage{{PromptTokens: 29}, {PromptTokens: 29, CompletionTokens: 4, TotalTokens: 33}, {PromptTokens: 29, CompletionTokens: 4, TotalTokens: 33}}, wantPrompt: 29, wantCompletion: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toolIndex := 0
			tool := openai.ToolCall{Index: &toolIndex, ID: "call-1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "test_tool", Arguments: `{"x":1}`}}
			chunks := []openai.ChatCompletionStreamResponse{
				usageRegressionChunk("actual-route-model", nil, openai.ChatCompletionStreamChoiceDelta{Role: "assistant", Content: "visible", ReasoningContent: "reasoning", ToolCalls: []openai.ToolCall{tool}}, ""),
			}
			for _, usage := range tc.usageChunks {
				chunks = append(chunks, usageRegressionChunk("actual-route-model", usage, openai.ChatCompletionStreamChoiceDelta{}, ""))
			}
			chunks = append(chunks, usageRegressionChunk("actual-route-model", nil, openai.ChatCompletionStreamChoiceDelta{}, openai.FinishReasonToolCalls))
			client := &usageRegressionClient{routes: usageRegressionRoutes(), streams: []*usageRegressionStream{{chunks: chunks}}}
			retries := 0
			result := handleStreamingResponse(context.Background(), openai.ChatCompletionRequest{Model: "primary-model", Stream: true}, client, false, defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), &NoopBroker{}, AgentTelemetryScope{}, func() {}, time.Second, &retries, true)
			if result.err != nil || result.promptTokens != tc.wantPrompt || result.completionTokens <= 0 || result.totalTokens != result.promptTokens+result.completionTokens {
				t.Fatalf("stream usage = prompt %d completion %d total %d source %q estimated %v err=%v", result.promptTokens, result.completionTokens, result.totalTokens, result.tokenSource, result.usedFallbackEstimate, result.err)
			}
			if tc.wantEstimate {
				wantCompletion := estimateTokensForModel("visiblereasoningtest_tool{\"x\":1}", "actual-route-model")
				if result.completionTokens != wantCompletion || !result.usedFallbackEstimate {
					t.Fatalf("completion estimate = %d estimated=%v, want exact observed payload estimate %d", result.completionTokens, result.usedFallbackEstimate, wantCompletion)
				}
			} else if result.completionTokens != tc.wantCompletion || result.totalTokens != 33 || result.usedFallbackEstimate {
				t.Fatalf("provider usage was changed or estimated: %+v", result)
			}
			if len(result.resp.Choices) != 1 {
				t.Fatalf("assembled choices = %d, want one", len(result.resp.Choices))
			}
			message := result.resp.Choices[0].Message
			if message.Content != "visible" || message.ReasoningContent != "reasoning" || len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Arguments != `{"x":1}` {
				t.Fatalf("stream output was not assembled: %+v", message)
			}
			if result.resp.Model != "actual-route-model" || result.resp.Usage.PromptTokens != tc.wantPrompt || result.resp.Usage.TotalTokens != result.totalTokens {
				t.Fatalf("normalized response usage/model = %+v / %q", result.resp.Usage, result.resp.Model)
			}
		})
	}
}

func TestAgentLoopBooksStreamReceiveCancellationOnce(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "stream-usage-cancel", "web_chat")
	defer cleanup()
	runCfg.SuppressTurnSideEffects = true
	runCfg.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
	runCfg.Config.CircuitBreaker.LLMTimeoutSeconds = 10
	runCfg.Config.CircuitBreaker.LLMStreamChunkTimeoutSeconds = 5
	tracker := newUsageRegressionTracker(t, runCfg.Config)
	runCfg.BudgetTracker = tracker
	client := &usageRegressionClient{routes: usageRegressionRoutes(), streams: []*usageRegressionStream{{
		chunks: []openai.ChatCompletionStreamResponse{
			usageRegressionChunk("actual-route-model", &openai.Usage{PromptTokens: 41}, openai.ChatCompletionStreamChoiceDelta{}, ""),
			usageRegressionChunk("actual-route-model", nil, openai.ChatCompletionStreamChoiceDelta{Content: "partial answer", ReasoningContent: "partial reasoning"}, ""),
		}, terminal: context.Canceled,
	}}}
	runCfg.LLMClient = client
	broker := &usageRegressionBroker{}
	resp, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model, Stream: true, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Continue the task."}}}, runCfg, true, broker)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want receive cancellation", err)
	}
	if len(client.requests) != 1 || broker.updates != 1 {
		t.Fatalf("calls=%d token updates=%d, want one each", len(client.requests), broker.updates)
	}
	usage := tracker.GetStatus().Models["actual-route-model"]
	if usage.Calls != 1 || usage.InputTokens != 41 || usage.OutputTokens <= 0 || resp.Usage.TotalTokens != usage.InputTokens+usage.OutputTokens {
		t.Fatalf("response=%+v budget=%+v, want one prompt-only provider usage plus estimated partial output", resp.Usage, usage)
	}
	if broker.prompt != usage.InputTokens || broker.completion != usage.OutputTokens || broker.total != usage.InputTokens+usage.OutputTokens || !broker.estimated {
		t.Fatalf("token event = %+v; want exactly one matching estimated update", broker)
	}
}

func TestMinimalLoopBooksStreamReceiveCancellationOnce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	tracker := newUsageRegressionTracker(t, cfg)
	client := &usageRegressionClient{routes: minimalLoopTestRoutes(), streams: []*usageRegressionStream{{
		chunks: []openai.ChatCompletionStreamResponse{
			usageRegressionChunk("actual-route-model", &openai.Usage{PromptTokens: 37}, openai.ChatCompletionStreamChoiceDelta{}, ""),
			usageRegressionChunk("actual-route-model", nil, openai.ChatCompletionStreamChoiceDelta{Content: "incomplete text", ReasoningContent: "incomplete reasoning"}, ""),
		}, terminal: context.Canceled,
	}}}
	result, _, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "Answer safely.", "Current task", nil, &DispatchContext{Cfg: cfg, BudgetTracker: tracker}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), &MinimalLoopOptions{MaxToolRounds: 0, StreamText: true, BudgetCategory: "looper"})
	if !errors.Is(err, context.Canceled) || result.Response != "" {
		t.Fatalf("result=%+v error=%v, want canceled without applying partial text", result, err)
	}
	usage := tracker.GetStatus().Models["actual-route-model"]
	if len(client.requests) != 1 || usage.Calls != 1 || usage.InputTokens != 37 || usage.OutputTokens <= 0 || result.TotalTokens != usage.InputTokens+usage.OutputTokens {
		t.Fatalf("calls=%d result=%+v budget=%+v; want one charged partial stream", len(client.requests), result, usage)
	}
}

func TestMinimalLoopBillsSyntheticCorrectionResponseOnce(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 6000
	tracker := newUsageRegressionTracker(t, cfg)
	client := &usageRegressionClient{routes: minimalLoopTestRoutes(), responses: []openai.ChatCompletionResponse{
		{Model: "actual-route-model", Usage: openai.Usage{PromptTokens: 13, CompletionTokens: 4, TotalTokens: 17}, Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{"action":"brave_search","query":"unexecuted"}`}, FinishReason: openai.FinishReasonStop}}},
		{Model: "actual-route-model", Usage: openai.Usage{PromptTokens: 21, CompletionTokens: 3, TotalTokens: 24}, Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "done"}, FinishReason: openai.FinishReasonStop}}},
	}}
	tool := openai.Tool{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "test_tool", Parameters: map[string]any{"type": "object"}}}
	dispatch := &DispatchContext{Cfg: cfg, BudgetTracker: tracker, ToolScopeRestricted: true, AllowedTools: map[string]struct{}{"test_tool": {}}}
	result, _, err := ExecuteMinimalLoop(context.Background(), client, "primary-model", "Use native tools.", "Current task", []openai.Tool{tool}, dispatch, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), &MinimalLoopOptions{MaxToolRounds: 1, BudgetCategory: "writer"})
	if err != nil || result.Response != "done" || len(client.requests) != 2 {
		t.Fatalf("result=%+v error=%v requests=%d, want corrected final response after two streams", result, err, len(client.requests))
	}
	correctionPresent := false
	for _, message := range client.requests[1].Messages {
		correctionPresent = correctionPresent || strings.Contains(message.Content, "previous response contained tool-call syntax")
	}
	if !correctionPresent {
		t.Fatal("second request omitted the synthetic tool-syntax correction")
	}
	usage := tracker.GetStatus().Models["actual-route-model"]
	if result.PromptTokens != 34 || result.CompletionTokens != 7 || result.TotalTokens != 41 || usage.Calls != 2 || usage.InputTokens != 34 || usage.OutputTokens != 7 {
		t.Fatalf("result=%+v budget=%+v, want exactly the two provider responses (13/4 and 21/3)", result, usage)
	}
}
