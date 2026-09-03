package agent

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

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

	appendCircuitBreakerSkippedNativeResults(s, stm, history, "test-session", NoopBroker{})
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

type circuitBreakerSequenceClient struct {
	responses []openai.ChatCompletionResponse
	requests  []openai.ChatCompletionRequest
}

func (c *circuitBreakerSequenceClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	return c.responses[len(c.requests)-1], nil
}

func (*circuitBreakerSequenceClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
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
