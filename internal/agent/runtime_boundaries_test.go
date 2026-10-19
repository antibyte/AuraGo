package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func TestNativeFallbackRequiresWholeJSONAnswer(t *testing.T) {
	for _, content := range []string{`{"action":"list_files","path":"."}`, `[{"action":"list_files"},{"action":"read_file"}]`, `{"tool_calls":[{"name":"list_files","arguments":{}}]}`} {
		if !exactJSONToolAnswer(content) {
			t.Fatalf("whole call rejected: %q", content)
		}
	}
	for _, content := range []string{"Example: {\"action\":\"delete_file\"}", "```json\n{\"action\":\"delete_file\"}\n```", `<think>{"action":"delete_file"}</think>`, `<tool_call>{"action":"delete_file"}</tool_call>`, `{"text":"an example with action"}`, `{"action":"list_files"} trailing`, `null`, `[]`} {
		if exactJSONToolAnswer(content) {
			t.Fatalf("example or wrapper accepted: %q", content)
		}
	}
}

func TestSyncCompletionRejectsTruncationAndCancellation(t *testing.T) {
	for _, reason := range []openai.FinishReason{"", openai.FinishReasonLength, openai.FinishReasonContentFilter, openai.FinishReasonStop, openai.FinishReasonToolCalls} {
		response := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: reason}}}
		err := syncCompletionError(context.Background(), response)
		valid := reason == openai.FinishReasonStop || reason == openai.FinishReasonToolCalls
		if (err == nil) != valid {
			t.Fatalf("finish=%q error=%v", reason, err)
		}
		if !valid {
			var incomplete *IncompleteCompletionError
			if !errors.As(err, &incomplete) || incomplete.FinishReason != reason || incomplete.ReasoningOnly || incomplete.HasToolCalls {
				t.Fatalf("invalid completion diagnostics: %#v, error=%v", incomplete, err)
			}
		}
	}
	for _, tools := range []bool{false, true} {
		message := openai.ChatCompletionMessage{ReasoningContent: "private reasoning"}
		if tools {
			message.FunctionCall = &openai.FunctionCall{Name: "must-not-run", Arguments: `{"partial":`}
		}
		err := syncCompletionError(context.Background(), openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonLength, Message: message}}})
		var incomplete *IncompleteCompletionError
		if !errors.As(err, &incomplete) || incomplete.ReasoningOnly != !tools || incomplete.HasToolCalls != tools {
			t.Fatalf("reasoning-only diagnostic admitted a partial call: %#v, error=%v", incomplete, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := syncCompletionError(ctx, openai.ChatCompletionResponse{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRuntimePanicBecomesUnknownOutcomeWithoutSecret(t *testing.T) {
	result := func() (result ToolDispatchResult) {
		defer recoverToolDispatch(nil, &result)
		panic("secret request data")
	}()
	if result.Status != ToolResultUnknown || !result.IsError || result.Output == "" {
		t.Fatalf("panic outcome = %+v", result)
	}
	var runtimeErr error
	func() { defer recoverAgentRuntime(context.Background(), nil, &runtimeErr); panic("private") }()
	if !errors.Is(runtimeErr, ErrRuntimePanic) {
		t.Fatal(runtimeErr)
	}
}

func TestAsyncTaskPanicDrainsAndKeepsGroupUsable(t *testing.T) {
	group := NewAsyncTaskGroup(context.Background())
	if !group.Go(func(context.Context) { panic("private") }) {
		t.Fatal("task rejected")
	}
	if !group.Go(func(context.Context) {}) {
		t.Fatal("panic prevented another task from starting")
	}
	if err := group.Shutdown(time.Second); err != nil {
		t.Fatal(err)
	}
}

type boundaryStreamClient struct{ stream llm.CompletionStream }

func (c boundaryStreamClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return openai.ChatCompletionResponse{}, errors.New("unexpected synchronous call")
}
func (c boundaryStreamClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return c.stream, nil
}

type boundaryStream struct {
	ctx        context.Context
	step       int
	heartbeat  bool
	firstDelay time.Duration
}

func (s *boundaryStream) Close() error { return nil }
func (s *boundaryStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	s.step++
	if s.heartbeat && s.step == 1 {
		return openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: "started"}}}}, nil
	}
	if s.heartbeat {
		select {
		case <-s.ctx.Done():
			return openai.ChatCompletionStreamResponse{}, s.ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
		return openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Role: "assistant"}}}}, nil
	}
	if s.step > 1 {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	delay := s.firstDelay
	if delay == 0 {
		delay = 80 * time.Millisecond
	}
	select {
	case <-s.ctx.Done():
		return openai.ChatCompletionStreamResponse{}, s.ctx.Err()
	case <-time.After(delay):
	}
	return openai.ChatCompletionStreamResponse{Choices: []openai.ChatCompletionStreamChoice{{Delta: openai.ChatCompletionStreamChoiceDelta{Content: "done"}, FinishReason: openai.FinishReasonStop}}}, nil
}

func TestStreamFirstTokenAndIdleHaveSeparateBudgets(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		stream := &boundaryStream{ctx: ctx, heartbeat: heartbeat}
		retries := 0
		result := handleStreamingResponse(ctx, openai.ChatCompletionRequest{Model: "test"}, boundaryStreamClient{stream}, false,
			defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), NoopBroker{}, AgentTelemetryScope{}, cancel, 25*time.Millisecond, &retries)
		cancel()
		if heartbeat && !errors.Is(result.err, errStreamIdleTimeout) {
			t.Fatalf("empty keepalives bypassed idle limit: %v", result.err)
		}
		if !heartbeat && result.err != nil {
			t.Fatalf("first token incorrectly used idle budget: %v", result.err)
		}
	}
}

func TestStreamParentDeadlineBoundsFirstTokenWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	retries := 0
	result := handleStreamingResponse(ctx, openai.ChatCompletionRequest{Model: "test"}, boundaryStreamClient{&boundaryStream{ctx: ctx, firstDelay: time.Second}}, false,
		defaultRecoveryPolicy(), slog.New(slog.NewTextHandler(io.Discard, nil)), NoopBroker{}, AgentTelemetryScope{}, cancel, time.Second, &retries)
	if !errors.Is(result.err, context.DeadlineExceeded) || !errors.Is(result.err, errStreamFirstTokenTimeout) {
		t.Fatalf("parent deadline lost first-token classification: %v", result.err)
	}
}

func TestExplicitUnknownAndApprovalStopNativeBatch(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		wantError    bool
	}{
		{"panic", `{"status":"unknown","code":"tool_panic"}`, true},
		{"uncertain mutation", `{"status":"unknown","message":"request outcome could not be verified"}`, true},
		{"approval", `{"status":"deferred","code":"approval_required"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := newToolOutcomeTestState(t, func(context.Context, ToolCall) (string, bool) { calls++; return tc.output, true })
			primary := ToolCall{Action: "core_memory", Operation: "add", Content: "test fact", NativeCallID: "first"}
			s.pendingTCs = []ToolCall{{Action: "core_memory", Operation: "list", NativeCallID: "second"}}
			response, err, next := executeAgentToolTurn(s, context.Background(), primary, openai.ChatCompletionResponse{}, "", true,
				nativeAssistantForCalls(primary, s.pendingTCs[0]), "test", "", false)
			if calls != 1 || next || (err != nil) != tc.wantError {
				t.Fatalf("calls=%d next=%v error=%v", calls, next, err)
			}
			if tc.wantError && !errors.Is(err, ErrToolOutcomeUnknown) {
				t.Fatal(err)
			}
			if !tc.wantError && len(response.Choices) != 1 {
				t.Fatal("approval did not return a bounded response")
			}
			toolResultForCall(t, s.req.Messages, "first")
			toolResultForCall(t, s.req.Messages, "second")
		})
	}
}

func TestLegacyUnclassifiedTextDoesNotStopRun(t *testing.T) {
	s := &agentLoopState{}
	s.noteToolRunBoundary(ToolCall{Action: "core_memory", Operation: "add"}, nil, "legacy output without status")
	s.noteToolRunBoundary(ToolCall{Action: "core_memory", Operation: "list"}, nil, `{"status":"unknown"}`)
	if s.toolBoundaryErr != nil || s.toolBoundaryNotice != "" {
		t.Fatal("legacy output or unknown read incorrectly stopped the run")
	}
}

func TestModelCircuitBreakerOverrideCannotRaiseLimit(t *testing.T) {
	cfg := &config.Config{}
	cfg.CircuitBreaker.MaxToolCalls = 3
	got := calculateEffectiveMaxCalls(RunConfig{Config: cfg}, ToolCall{Tool: "core_memory", CircuitBreakerOverride: 99}, false, false, nil, nil)
	if got != 3 {
		t.Fatalf("model override widened configured limit to %d", got)
	}
}

func TestQueuedToolGuardStopsCancelledCallBeforeDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &agentLoopState{pendingTCs: []ToolCall{{Action: "core_memory"}}}
	if !processPendingToolCalls(s, ctx, "test") || len(s.pendingTCs) != 0 || !errors.Is(s.toolBoundaryErr, context.Canceled) {
		t.Fatal("cancelled queued call was not stopped")
	}
}
