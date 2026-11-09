package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func TestMissionAgentFailureDetailKeepsSafeCause(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "configured-model"
	cfg.Agent.ContextWindow = 512000
	for _, tc := range []struct {
		err  error
		code string
	}{
		{fmt.Errorf("wrapped: %w", &agent.IncompleteCompletionError{FinishReason: "length", ReasoningOnly: true}), "model_output_limit"},
		{&agent.IncompleteCompletionError{FinishReason: "content_filter"}, "model_output_filtered"},
		{&agent.IncompleteCompletionError{FinishReason: "private-provider-value"}, "model_output_incomplete"},
		{context.DeadlineExceeded, "deadline_exceeded"},
		{context.Canceled, "owner_cancelled"},
		{errors.New("private-provider-value"), "agent_execution_failed"},
	} {
		got := missionAgentFailureDetail(tc.err, "Safe localized message", 1200*time.Millisecond, cfg)
		for _, want := range []string{tc.code, `"elapsed_ms":1200`, `"configured_model":"configured-model"`, `"configured_context_window":512000`, `"retry_safe":false`} {
			if !strings.Contains(got, want) {
				t.Fatalf("missing %s: %s", want, got)
			}
		}
		if strings.Contains(got, "private-provider-value") {
			t.Fatalf("raw provider content leaked: %s", got)
		}
	}
}
