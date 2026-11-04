package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/budget"

	openai "github.com/sashabaranov/go-openai"
)

func TestAgentLoopChecksDailyBudgetByRunCategory(t *testing.T) {
	for _, tc := range []struct {
		name, session, enforcement         string
		coAgent, disabled, blocked, stream bool
	}{
		{"partial main chat", "budget-main", "partial", false, false, false, false},
		{"partial co-agent flag", "budget-child", "partial", true, false, true, false},
		{"partial co-agent session", "coagent-budget", "partial", false, false, true, false},
		{"partial specialist session", "specialist-budget", "partial", false, false, true, false},
		{"partial peer session", "a2a-budget", "partial", false, false, true, false},
		{"full main chat", "budget-main", "full", false, false, true, false},
		{"full co-agent", "coagent-budget", "full", true, false, true, false},
		{"warn co-agent", "coagent-budget", "warn", true, false, false, false},
		{"disabled co-agent", "coagent-budget", "partial", true, true, false, false},
		{"partial stream co-agent", "coagent-budget", "partial", true, false, true, true},
		{"full stream chat", "budget-main", "full", false, false, true, true},
		{"partial mqtt relay", "mqtt", "partial", false, false, true, false},
		{"partial frigate relay", "frigate", "partial", false, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run, _, cleanup := newPromptPipelineTestRunConfig(t, tc.session, "web_chat")
			defer cleanup()
			run.SuppressTurnSideEffects = true
			if tc.session == "mqtt" || tc.session == "frigate" {
				run.MessageSource = tc.session
			}
			run.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
			run.IsCoAgent = tc.coAgent
			run.Config.Budget.Enabled = !tc.disabled
			run.Config.Budget.Enforcement = tc.enforcement
			run.Config.Budget.DailyLimitUSD = 1e-12
			run.Config.Budget.DefaultCost.InputPerMillion = 1
			run.Config.Budget.DefaultCost.OutputPerMillion = 1
			run.BudgetTracker = budget.NewTracker(run.Config, run.Logger, t.TempDir())
			defer run.BudgetTracker.Flush()
			run.BudgetTracker.RecordResponseUsage("coagent", "openai", "budget-seed-model", nil, 10, 3, true)
			if !tc.disabled && tc.enforcement != "warn" && !run.BudgetTracker.IsBlocked("coagent") {
				t.Fatal("fixture did not exhaust the daily budget")
			}
			client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{{
				Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{
					Role: openai.ChatMessageRoleAssistant, Content: "Done.",
				}}},
			}}}
			run.LLMClient = client
			_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
				Model: run.Config.LLM.Model,
				Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser,
					Content: "Finish."}},
			}, run, tc.stream, NoopBroker{})
			if tc.blocked {
				if err == nil || !strings.Contains(err.Error(), "budget exceeded") || len(client.requests) != 0 {
					t.Fatalf("error=%v calls=%d, want budget refusal before a model call", err, len(client.requests))
				}
			} else if err != nil || len(client.requests) != 1 {
				t.Fatalf("error=%v calls=%d, want one allowed model call", err, len(client.requests))
			}
		})
	}
}

func TestCoAgentDailyBudgetStopsBeforeRetryAndBooksOnce(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, "coagent-daily-retry", "co_agent")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.Checkpoint = func([]openai.ChatCompletionMessage) error { return nil }
	run.IsCoAgent = true
	run.Config.Budget.Enabled = true
	run.Config.Budget.Enforcement = "partial"
	run.Config.Budget.DailyLimitUSD = 1e-12
	run.Config.Budget.DefaultCost.InputPerMillion = 1
	run.Config.Budget.DefaultCost.OutputPerMillion = 1
	run.BudgetTracker = budget.NewTracker(run.Config, run.Logger, t.TempDir())
	defer run.BudgetTracker.Flush()
	client := &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		{Usage: openai.Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13},
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}}}},
		{Usage: openai.Usage{PromptTokens: 20, CompletionTokens: 4, TotalTokens: 24},
			Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Done."}}}},
	}}
	run.LLMClient = client
	_, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: run.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Finish."}}}, run, false, NoopBroker{})
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") || len(client.requests) != 1 {
		t.Fatalf("error=%v calls=%d, want budget refusal before a retry", err, len(client.requests))
	}
	usage := run.BudgetTracker.GetStatus().Models[run.Config.LLM.Model]
	if usage.Calls != 1 || usage.InputTokens != 10 || usage.OutputTokens != 3 {
		t.Fatalf("usage=%+v, want the first physical response booked once", usage)
	}
}
