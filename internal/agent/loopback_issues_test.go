package agent

import (
	"context"
	"testing"
	"time"

	"aurago/internal/planner"
	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

func TestLoopbackSuccessResolvesOnlyMatchingRuntimeIssue(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, "heartbeat", "heartbeat")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.PlannerDB = newPlannerTestDB(t)
	t.Cleanup(func() { _ = run.PlannerDB.Close() })
	for _, source := range []string{"heartbeat", "mqtt"} {
		if _, err := planner.RecordOperationalIssue(run.PlannerDB, planner.OperationalIssue{
			Source: source, Context: "heartbeat", Reference: "loopback", Title: "Background agent loop failed",
		}); err != nil {
			t.Fatal(err)
		}
	}
	LoopbackContext(context.Background(), run, "routine check", NoopBroker{})
	page, err := planner.ListOperationalIssues(run.PlannerDB, planner.OperationalIssueListFilter{Status: "active"})
	if err != nil || page.Total != 1 || page.Items[0].Source != "mqtt" {
		t.Fatalf("wrong runtime issues resolved: page=%+v err=%v", page, err)
	}
}

func TestLoopbackActionRuleBlockRecordsToolIssueWithoutRuntimeFailure(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, "heartbeat", "heartbeat")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.PlannerDB = newPlannerTestDB(t)
	t.Cleanup(func() { _ = run.PlannerDB.Close() })
	run.Config.Docker.Enabled = true
	run.Config.Agent.ActionPolicy.Enabled = true
	executions := 0
	run.ExecutionHooks = &ExecutionHooks{HandleTool: func(context.Context, ToolCall) (string, bool) {
		executions++
		return `{"status":"success"}`, true
	}}
	run.LLMClient = &circuitBreakerSequenceClient{responses: []openai.ChatCompletionResponse{
		{Choices: []openai.ChatCompletionChoice{{FinishReason: openai.FinishReasonToolCalls, Message: openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant, ToolCalls: []openai.ToolCall{{ID: "blocked-remove", Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{Name: "docker", Arguments: `{"operation":"remove","container_id":"fixture-container"}`},
			}},
		}}}},
	}}
	LoopbackContext(context.Background(), run, "remove fixture-container", NoopBroker{})
	page, err := planner.ListOperationalIssues(run.PlannerDB, planner.OperationalIssueListFilter{Status: "active"})
	if err != nil || page.Total != 1 || page.Items[0].Kind != planner.OperationalIssueKindToolFailure || executions != 0 {
		t.Fatalf("policy block duplicated or executed: page=%+v executions=%d err=%v", page, executions, err)
	}
}

func TestLoopbackDeadlineStillRecordsRuntimeFailure(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "heartbeat")
	defer cleanup()
	run.SuppressTurnSideEffects = true
	run.PlannerDB = newPlannerTestDB(t)
	t.Cleanup(func() { _ = run.PlannerDB.Close() })
	client := newCtxBlockingSummaryClient()
	run.LLMClient = client
	prompts.CountTokensForModel("warmup", run.Config.LLM.Model)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	LoopbackContext(ctx, run, "routine check", NoopBroker{})
	select {
	case <-client.started:
	default:
		t.Fatal("loop never reached the provider before its deadline")
	}
	page, err := planner.ListOperationalIssues(run.PlannerDB, planner.OperationalIssueListFilter{Status: "active"})
	if err != nil || page.Total != 1 || page.Items[0].Kind != planner.OperationalIssueKindRuntimeFailure {
		t.Fatalf("deadline failure was hidden: page=%+v err=%v", page, err)
	}
}
