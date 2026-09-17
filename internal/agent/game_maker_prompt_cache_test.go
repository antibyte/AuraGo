package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

type gameMakerCacheClient struct {
	circuitBreakerSequenceClient
	delayFirst bool
}

func (c *gameMakerCacheClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	// Capture values, not aliases to slices that the loop may subsequently edit.
	data, err := json.Marshal(req)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	var snapshot openai.ChatCompletionRequest
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	c.requests = append(c.requests, snapshot)
	if len(c.requests) > len(c.responses) {
		return openai.ChatCompletionResponse{}, fmt.Errorf("unexpected extra model round %d", len(c.requests))
	}
	if c.delayFirst && len(c.requests) == 1 {
		time.Sleep(1100 * time.Millisecond) // Cross the RFC3339 clock's second boundary.
	}
	return c.responses[len(c.requests)-1], nil
}

func gameMakerCacheToolResponse(id, operation string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message: openai.ChatCompletionMessage{Role: "assistant", ReasoningContent: "test continuation context", ToolCalls: []openai.ToolCall{{
			ID: id, Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "list_processes", Arguments: fmt.Sprintf(`{"operation":%q}`, operation)},
		}}}, FinishReason: openai.FinishReasonToolCalls,
	}}}
}

func gameMakerCacheFinalResponse() openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant", Content: "Finished safely."}, FinishReason: openai.FinishReasonStop}}}
}

func gameMakerCacheRun(t *testing.T, client *gameMakerCacheClient) RunConfig {
	t.Helper()
	cfg, _, cleanup := newPromptPipelineTestRunConfig(t, "game-maker-cache-test", "game_maker")
	t.Cleanup(cleanup)
	cfg.LLMClient = client
	cfg.Config.LLM.UseNativeFunctions = true
	cfg.IsMission = true
	cfg.SuppressTurnSideEffects = true
	cfg.StableSystemPrompt = true
	cfg.PreserveReasoning = true
	cfg.ToolCallLimit = 40
	cfg.AllowedTools = []string{"list_processes"}
	cfg.NativeToolSchemas = []openai.Tool{{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
		Name: "list_processes", Description: "Inspect process status.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"operation": map[string]any{"type": "string"}}},
	}}}
	return cfg
}

func gameMakerCacheRequest(cfg RunConfig) openai.ChatCompletionRequest {
	return openai.ChatCompletionRequest{Model: cfg.Config.LLM.Model, Messages: []openai.ChatCompletionMessage{
		{Role: "system", Content: "Keep this separate trusted caller instruction."},
		{Role: "user", Content: "Inspect the current state and continue the game."},
	}}
}

func TestGameMakerPromptPrefixAndHistoryStayStable(t *testing.T) {
	client := &gameMakerCacheClient{delayFirst: true, circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		gameMakerCacheToolResponse("first", "status"), gameMakerCacheToolResponse("second", "details"), gameMakerCacheFinalResponse(),
	}}}
	cfg := gameMakerCacheRun(t, client)
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
	if err != nil || len(client.requests) != 3 {
		t.Fatalf("loop: %v; requests=%d", err, len(client.requests))
	}
	for i, req := range client.requests {
		if strings.Count(req.Messages[0].Content, "# NOW\n") != 1 {
			t.Fatal("test did not exercise the generated timestamp")
		}
		generated := 0
		for _, m := range req.Messages {
			if m.Role == "system" && strings.Contains(m.Content, "# CORE IDENTITY") {
				generated++
			}
		}
		if generated != 1 || !containsMessage(req.Messages, "system", "Keep this separate trusted caller instruction.") {
			t.Fatalf("request %d has %d generated prompts or lost caller instructions", i, generated)
		}
		if i == 0 {
			continue
		}
		prev := client.requests[i-1]
		if len(req.Messages) <= len(prev.Messages) || !reflect.DeepEqual(req.Messages[:len(prev.Messages)], prev.Messages) {
			t.Fatalf("request %d rewrote the previous prefix/history", i)
		}
		if !reflect.DeepEqual(req.Tools, prev.Tools) {
			t.Fatalf("request %d changed tool schemas", i)
		}
		if !containsReasoning(req.Messages, "test continuation context") {
			t.Fatalf("request %d lost continuation context", i)
		}
	}
}

func containsReasoning(messages []openai.ChatCompletionMessage, want string) bool {
	for _, m := range messages {
		if m.ReasoningContent == want {
			return true
		}
	}
	return false
}

func TestAgentToolRoundKeepsOnlyCurrentGeneratedSystemPrompt(t *testing.T) {
	client := &gameMakerCacheClient{delayFirst: true, circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		gameMakerCacheToolResponse("first", "status"), gameMakerCacheToolResponse("second", "details"), gameMakerCacheFinalResponse(),
	}}}
	cfg := gameMakerCacheRun(t, client)
	cfg.StableSystemPrompt = false
	cfg.MessageSource = "web_chat"
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
	if err != nil || len(client.requests) != 3 {
		t.Fatalf("loop: %v; requests=%d", err, len(client.requests))
	}
	if client.requests[0].Messages[0].Content == client.requests[1].Messages[0].Content {
		t.Fatal("ordinary chat must still refresh its current time")
	}
	for i, req := range client.requests {
		count := 0
		for _, m := range req.Messages {
			if m.Role == "system" && strings.Contains(m.Content, "# CORE IDENTITY") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("request %d resurrected %d generated system prompts", i, count)
		}
	}
}

func TestGameMakerDuplicateRecoveryRetainsNativeTools(t *testing.T) {
	client := &gameMakerCacheClient{}
	for cycle := range 3 {
		for _, id := range []string{"one", "two", "blocked"} {
			client.responses = append(client.responses, gameMakerCacheToolResponse(fmt.Sprintf("%d-%s", cycle, id), "status"))
		}
		client.responses = append(client.responses, gameMakerCacheToolResponse(fmt.Sprintf("different-%d", cycle), fmt.Sprintf("details-%d", cycle)))
	}
	client.responses = append(client.responses, gameMakerCacheFinalResponse())
	cfg := gameMakerCacheRun(t, client)
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
	if err != nil || len(client.requests) != 13 {
		t.Fatalf("loop: %v; requests=%d", err, len(client.requests))
	}
	for i, req := range client.requests {
		if !reflect.DeepEqual(req.Tools, client.requests[0].Tools) || req.Messages[0].Content != client.requests[0].Messages[0].Content {
			t.Fatalf("request %d switched prompt/tool mode after a blocked duplicate", i)
		}
	}
	if !containsMessage(client.requests[3].Messages, "tool", "[TOOL BLOCKED]") {
		t.Fatal("duplicate was executed instead of blocked")
	}
	if !containsMessage(client.requests[4].Messages, "tool", "Tool Output:") {
		t.Fatal("different operation could not continue")
	}
}

func TestGameMakerRepeatedBlocksStopWithoutAnotherModelRound(t *testing.T) {
	client := &gameMakerCacheClient{}
	for i := range 8 {
		client.responses = append(client.responses, gameMakerCacheToolResponse(fmt.Sprint(i), "status"))
	}
	cfg := gameMakerCacheRun(t, client)
	var saved []openai.ChatCompletionMessage
	cfg.Checkpoint = func(messages []openai.ChatCompletionMessage) error {
		saved = append([]openai.ChatCompletionMessage(nil), messages...)
		return nil
	}
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
	if err == nil || !strings.Contains(err.Error(), "blocked duplicate tool requests") || len(client.requests) != 5 {
		t.Fatalf("unbounded duplicate recovery: %v; requests=%d", err, len(client.requests))
	}
	blocked := 0
	for _, m := range saved {
		if m.Role == "tool" && strings.Contains(m.Content, "[TOOL BLOCKED]") {
			blocked++
		}
	}
	if blocked != maxGameMakerDuplicateBlocks || !containsReasoning(saved, "test continuation context") {
		t.Fatalf("checkpoint lost final blocked results or continuation: blocks=%d", blocked)
	}
}

func TestGameMakerToolLimitKeepsCatalogButRejectsMoreCalls(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprint(invalid), func(t *testing.T) {
			final := gameMakerCacheFinalResponse()
			if invalid {
				final = gameMakerCacheToolResponse("forbidden", "details")
			}
			client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{gameMakerCacheToolResponse("one", "status"), final}}}
			cfg := gameMakerCacheRun(t, client)
			cfg.ToolCallLimit = 1
			_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
			if invalid && !IsToolLimitFinalResponseInvalid(err) || !invalid && err != nil {
				t.Fatalf("limit enforcement: %v", err)
			}
			if len(client.requests) != 2 || !reflect.DeepEqual(client.requests[0].Tools, client.requests[1].Tools) || client.requests[1].ToolChoice != "none" {
				t.Fatal("finalization discarded the catalog or failed to disable calls")
			}
		})
	}
}

func TestGameMakerDuplicateInNativeBatchClosesEveryCall(t *testing.T) {
	batch := gameMakerCacheToolResponse("one", "status")
	for _, id := range []string{"two", "blocked", "skipped"} {
		call := gameMakerCacheToolResponse(id, "status").Choices[0].Message.ToolCalls[0]
		batch.Choices[0].Message.ToolCalls = append(batch.Choices[0].Message.ToolCalls, call)
	}
	client := &gameMakerCacheClient{circuitBreakerSequenceClient: circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{batch, gameMakerCacheFinalResponse()}}}
	cfg := gameMakerCacheRun(t, client)
	_, err := ExecuteAgentLoop(context.Background(), gameMakerCacheRequest(cfg), cfg, false, NoopBroker{})
	if err != nil || len(client.requests) != 2 {
		t.Fatalf("batch loop: %v; requests=%d", err, len(client.requests))
	}
	final := client.requests[1]
	if !reflect.DeepEqual(final.Tools, client.requests[0].Tools) {
		t.Fatal("batched duplicate removed the native catalog")
	}
	results := map[string]string{}
	for _, m := range final.Messages {
		if m.Role == "tool" {
			if _, exists := results[m.ToolCallID]; exists {
				t.Fatalf("duplicated result for %s", m.ToolCallID)
			}
			results[m.ToolCallID] = m.Content
		}
	}
	if len(results) != 4 || !strings.Contains(results["blocked"], "[TOOL BLOCKED]") || !strings.Contains(results["skipped"], "not_executed_due_to_circuit_breaker") {
		t.Fatalf("unclosed or dispatched blocked batch: %+v", results)
	}
}
