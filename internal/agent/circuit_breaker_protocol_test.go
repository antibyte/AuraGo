package agent

import (
	"aurago/internal/llm"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/budget"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestAgentChargesEmptyResponseOnceBeforeRetry(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "budget-empty", "web_chat")
	defer cleanup()
	runCfg.SuppressTurnSideEffects = true
	runCfg.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
	runCfg.Config.Budget.Enabled = true
	runCfg.Config.Budget.DailyLimitUSD = 100
	runCfg.BudgetTracker = budget.NewTracker(runCfg.Config, runCfg.Logger, t.TempDir())
	defer runCfg.BudgetTracker.Flush()
	client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		{Usage: openai.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13},
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}}}},
		{Usage: openai.Usage{PromptTokens: 20, CompletionTokens: 4, TotalTokens: 24},
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Done."}}}},
	}}
	runCfg.LLMClient = client
	_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish."}}}, runCfg, false, NoopBroker{})
	if err != nil || len(client.requests) != 2 {
		t.Fatalf("agent response = %v, calls=%d", err, len(client.requests))
	}
	usage := runCfg.BudgetTracker.GetStatus().Models[runCfg.Config.LLM.Model]
	if usage.Calls != 2 || usage.InputTokens != 30 || usage.OutputTokens != 7 {
		t.Fatalf("budget usage = %+v, want two responses", usage)
	}
}

func TestAgentBooksUsageFromSyncResponseWithoutChoices(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "sync-empty-choices-usage", "web_chat")
	defer cleanup()
	runCfg.SuppressTurnSideEffects = true
	runCfg.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
	runCfg.Config.Budget.Enabled = true
	runCfg.Config.Budget.DailyLimitUSD = 100
	runCfg.BudgetTracker = budget.NewTracker(runCfg.Config, runCfg.Logger, t.TempDir())
	defer runCfg.BudgetTracker.Flush()
	client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{{
		Model: "actual-route-model",
		Usage: openai.Usage{PromptTokens: 23, CompletionTokens: 0},
	}}}
	runCfg.LLMClient = client
	resp, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish."}}}, runCfg, false, NoopBroker{})
	if err == nil || !strings.Contains(err.Error(), "no choices") || len(client.requests) != 1 {
		t.Fatalf("response=%+v error=%v calls=%d, want sync empty-choice error", resp, err, len(client.requests))
	}
	if resp.Usage.PromptTokens != 23 || resp.Usage.TotalTokens != 23 {
		t.Fatalf("returned response lost normalized provider usage: %+v", resp.Usage)
	}
	usage := runCfg.BudgetTracker.GetStatus().Models["actual-route-model"]
	if usage.Calls != 1 || usage.InputTokens != 23 || usage.OutputTokens != 0 {
		t.Fatalf("budget usage = %+v, want one recorded partial response", usage)
	}
}

func TestCoAgentTokenLimitStopsBeforeNextModelCall(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "coagent-limit", "co_agent")
	defer cleanup()
	runCfg.SuppressTurnSideEffects = true
	runCfg.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
	runCfg.IsCoAgent = true
	runCfg.CoAgentTokenLimit = 12
	client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		{Usage: openai.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13},
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}}}},
	}}
	runCfg.LLMClient = client
	_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish."}}}, runCfg, false, NoopBroker{})
	if err == nil || !strings.Contains(err.Error(), "co-agent token limit reached") || len(client.requests) != 1 {
		t.Fatalf("limit error = %v, calls=%d", err, len(client.requests))
	}
}

func TestCircuitBreakerCompletesEveryRemainingNativeToolCallWithoutDispatch(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "short-term.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(t.TempDir(), "history.json"))
	s := &agentLoopState{
		currentLogger: logger,
		pendingTCs: []ToolCall{
			{Action: "telegram", NativeCallID: "call-telegram"},
			{Action: "execute_shell", NativeCallID: "call-shell"},
		},
	}

	appendSkippedNativeResults(s, stm, history, "test-session", NoopBroker{}, notExecutedDueToCircuitBreakerResult())
	if len(s.pendingTCs) != 0 {
		t.Fatalf("pending calls = %#v", s.pendingTCs)
	}
	if len(s.req.Messages) != 2 {
		t.Fatalf("protocol results = %#v", s.req.Messages)
	}
	for i, wantID := range []string{"call-telegram", "call-shell"} {
		msg := s.req.Messages[i]
		if msg.ToolCallID != wantID || !strings.Contains(msg.Content, "not_executed_due_to_circuit_breaker") {
			t.Fatalf("result %d = %#v", i, msg)
		}
	}
}

func TestRunCompletionClosesNativeBatchWithoutAnotherDispatch(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "phase-complete", "game_maker")
	defer cleanup()
	s := &agentLoopState{runCfg: runCfg, broker: NoopBroker{}, currentLogger: runCfg.Logger,
		pendingTCs: []ToolCall{{Action: "game_maker_file", NativeCallID: "next"}},
		req: openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: "done"}, {ID: "next"}}},
			{Role: openai.ChatMessageRoleTool, ToolCallID: "done", Content: `{"status":"ok"}`},
		}},
	}
	if finishCompletedRun(s) || len(s.pendingTCs) != 1 {
		t.Fatal("nil completion hook changed normal runs")
	}
	s.runCfg.RunComplete = func() bool { return false }
	if finishCompletedRun(s) || len(s.pendingTCs) != 1 {
		t.Fatal("incomplete phase lost pending calls")
	}
	s.runCfg.RunComplete = func() bool { return true }
	for range 2 {
		if !finishCompletedRun(s) {
			t.Fatal("completed phase continued")
		}
	}
	if len(s.req.Messages) != 3 || len(s.pendingTCs) != 0 || s.toolCallCount != 0 {
		t.Fatalf("completion dispatched or duplicated a result: %+v", s.req.Messages)
	}
	last := s.req.Messages[2]
	if last.ToolCallID != "next" || !strings.Contains(last.Content, "not_executed_after_run_completion") {
		t.Fatalf("missing skipped result: %+v", last)
	}
	if _, dropped := SanitizeToolMessages(s.req.Messages); dropped != 0 {
		t.Fatalf("completion broke the native protocol: %d dropped", dropped)
	}
}

func TestRunCompletionPreservesCancellation(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "phase-cancelled", "game_maker")
	defer cleanup()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runCfg.RunComplete = func() bool { cancel(); return true }
	runCfg.SuppressTurnSideEffects = true
	client := &circuitBreakerSequenceClient{}
	runCfg.LLMClient = client
	_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Plan a game."}},
	}, runCfg, false, NoopBroker{})
	if !errors.Is(err, context.Canceled) || len(client.requests) != 0 {
		t.Fatalf("completed phase swallowed cancellation or called the LLM: %v, calls=%d", err, len(client.requests))
	}
}

func TestRunCompletionDuringProviderResponsePreservesLateCancellation(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "phase-complete-late-cancel", "game_maker")
	defer cleanup()
	runCfg.Config.LLM.UseNativeFunctions = true
	runCfg.AllowedTools = []string{"game_maker_project"}
	runCfg.NativeToolSchemas = []openai.Tool{testToolSchema("game_maker_project", "Game Maker project")}
	runCfg.SuppressTurnSideEffects = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	complete := false
	completionCheckedAfterResponse := false
	runCfg.RunComplete = func() bool {
		if complete {
			completionCheckedAfterResponse = true
			cancel()
		}
		return complete
	}
	client := &circuitBreakerSequenceClient{
		responses: []openai.ChatCompletionResponse{{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{ID: "completion-probe", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "game_maker_project", Arguments: `{"operation":"inspect"}`}}},
		}, FinishReason: openai.FinishReasonStop}}}},
		onResponse: func() { complete = true },
	}
	runCfg.LLMClient = client
	_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish this phase."}},
	}, runCfg, false, NoopBroker{})
	if !completionCheckedAfterResponse || !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation was not preserved after completion sentinel: checked=%v error=%v", completionCheckedAfterResponse, err)
	}
}

func TestRunCompletionDuringProviderResponseStopsPendingTools(t *testing.T) {
	for _, mode := range []string{"native", "xml"} {
		t.Run(mode, func(t *testing.T) {
			runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "phase-complete-during-response-"+mode, "game_maker")
			defer cleanup()
			runCfg.Config.LLM.UseNativeFunctions = true
			runCfg.AllowedTools = []string{"game_maker_project"}
			runCfg.NativeToolSchemas = []openai.Tool{testToolSchema("game_maker_project", "Game Maker project")}
			runCfg.SuppressTurnSideEffects = true
			runCfg.IsMission = true
			complete := false
			runCfg.RunComplete = func() bool { return complete }
			dispatches := 0
			runCfg.ExecutionHooks = &ExecutionHooks{BeforeTool: func(context.Context, ToolCall) error {
				dispatches++
				return nil
			}}
			var checkpoint []openai.ChatCompletionMessage
			runCfg.Checkpoint = func(messages []openai.ChatCompletionMessage) error {
				checkpoint = append([]openai.ChatCompletionMessage(nil), messages...)
				return nil
			}

			message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
			if mode == "xml" {
				message.Content = `<tool_call><function=game_maker_project><parameter=operation>inspect</parameter></function></tool_call>`
			} else {
				message.ToolCalls = []openai.ToolCall{{ID: "completion-probe", Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{Name: "game_maker_project", Arguments: `{"operation":"inspect"}`}}}
			}
			client := &circuitBreakerSequenceClient{
				responses:  []openai.ChatCompletionResponse{{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: openai.FinishReasonStop}}}},
				onResponse: func() { complete = true },
			}
			runCfg.LLMClient = client
			response, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
				Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish this phase."}},
			}, runCfg, false, NoopBroker{})
			if err != nil || len(response.Choices) != 0 || !complete || len(client.requests) != 1 || dispatches != 0 {
				t.Fatalf("completion stop response=%+v error=%v complete=%v requests=%d dispatches=%d", response, err, complete, len(client.requests), dispatches)
			}
			if mode == "native" {
				foundSkipped := false
				for _, msg := range checkpoint {
					if msg.Role == openai.ChatMessageRoleTool && msg.ToolCallID == "completion-probe" && strings.Contains(msg.Content, "tool_batch_stopped") {
						foundSkipped = true
					}
				}
				if !foundSkipped {
					t.Fatalf("native completion did not checkpoint its skipped call: %+v", checkpoint)
				}
			}
		})
	}
}

func TestRunCompletionDuringProviderResponsePreservesCheckpointFailure(t *testing.T) {
	runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "phase-complete-checkpoint-error", "game_maker")
	defer cleanup()
	runCfg.Config.LLM.UseNativeFunctions = true
	runCfg.AllowedTools = []string{"game_maker_project"}
	runCfg.NativeToolSchemas = []openai.Tool{testToolSchema("game_maker_project", "Game Maker project")}
	runCfg.SuppressTurnSideEffects = true
	complete := false
	runCfg.RunComplete = func() bool { return complete }
	checkpointErr := errors.New("checkpoint unavailable")
	checkpointSawSkippedCall := false
	runCfg.Checkpoint = func(messages []openai.ChatCompletionMessage) error {
		for _, message := range messages {
			if message.Role == openai.ChatMessageRoleTool && message.ToolCallID == "completion-probe" {
				checkpointSawSkippedCall = true
				return checkpointErr
			}
		}
		return nil
	}
	client := &circuitBreakerSequenceClient{
		responses: []openai.ChatCompletionResponse{{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant,
			ToolCalls: []openai.ToolCall{{ID: "completion-probe", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "game_maker_project", Arguments: `{"operation":"inspect"}`}}},
		}, FinishReason: openai.FinishReasonStop}}}},
		onResponse: func() { complete = true },
	}
	runCfg.LLMClient = client
	_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: runCfg.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish this phase."}},
	}, runCfg, false, NoopBroker{})
	if !checkpointSawSkippedCall {
		t.Fatal("checkpoint failure was not injected after the skipped native result")
	}
	if !errors.Is(err, checkpointErr) {
		t.Fatalf("completion hid checkpoint failure: %v", err)
	}
}

type circuitBreakerSequenceClient struct {
	responses  []openai.ChatCompletionResponse
	requests   []openai.ChatCompletionRequest
	onResponse func()
}

func (c *circuitBreakerSequenceClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	response := c.responses[len(c.requests)-1]
	if c.onResponse != nil {
		c.onResponse()
	}
	return response, nil
}

func (*circuitBreakerSequenceClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, nil
}

func TestToolLimitFinalResponseRejectsToolCallsWithoutPersistingThem(t *testing.T) {
	first := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
		ToolCalls: []openai.ToolCall{{
			ID: "initial-call", Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{Name: "list_processes", Arguments: `{}`},
		}},
	}}}}
	tests := []struct {
		name   string
		second openai.ChatCompletionResponse
		marker string
		valid  bool
	}{
		{
			name:   "valid final prose",
			second: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: "Finished safely."}}}},
			marker: "Finished safely.",
			valid:  true,
		},
		{
			name:   "text tool call",
			second: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: `{"action":"execute_shell","command":"echo forbidden"}`}}}},
			marker: "echo forbidden",
		},
		{
			name: "native tool call",
			second: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{ToolCalls: []openai.ToolCall{{
				ID: "forbidden-call", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{"command":"echo native-forbidden"}`},
			}}}}}},
			marker: "native-forbidden",
		},
		{
			name: "multiple native tool calls",
			second: openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{ToolCalls: []openai.ToolCall{
				{ID: "forbidden-one", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{"command":"echo first-forbidden"}`}},
				{ID: "forbidden-two", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "execute_shell", Arguments: `{"command":"echo second-forbidden"}`}},
			}}}}},
			marker: "first-forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runCfg, _, cleanup := newPromptPipelineTestRunConfig(t, "tool-limit-final", "web_chat")
			defer cleanup()
			runCfg.Config.LLM.UseNativeFunctions = true
			runCfg.SuppressTurnSideEffects = true
			runCfg.Config.CircuitBreaker.MaxToolCalls = 1
			client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{first, tt.second}}
			runCfg.LLMClient = client

			resp, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
				Model:    runCfg.Config.LLM.Model,
				Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Run one check."}},
			}, runCfg, false, NoopBroker{})
			if len(client.requests) != 2 {
				t.Fatalf("LLM calls = %d, want 2", len(client.requests))
			}
			if len(client.requests[1].Tools) != 0 {
				t.Fatalf("finalization request has %d tools, want 0", len(client.requests[1].Tools))
			}

			if tt.valid {
				if err != nil || len(resp.Choices) == 0 || resp.Choices[0].Message.Content != tt.marker {
					t.Fatalf("valid final response = %#v, err %v", resp, err)
				}
			} else {
				if !IsToolLimitFinalResponseInvalid(err) {
					t.Fatalf("error = %v, want tool-limit final-response error", err)
				}
				if len(resp.Choices) != 0 {
					t.Fatalf("invalid raw response leaked to caller: %#v", resp)
				}
			}

			messages, err := runCfg.ShortTermMem.GetSessionMessagesForBridge(runCfg.SessionID)
			if err != nil {
				t.Fatalf("GetSessionMessagesForBridge: %v", err)
			}
			for _, message := range messages {
				if !tt.valid && strings.Contains(message.Content, tt.marker) {
					t.Fatalf("invalid final tool call was persisted: %#v", message)
				}
			}
		})
	}
}
