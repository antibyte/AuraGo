package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
)

func missionAgentFailureDetail(err error, message string, elapsed time.Duration, cfg *config.Config) string {
	code := "agent_execution_failed"
	detail := map[string]interface{}{"stage": "agent_loop", "elapsed_ms": elapsed.Milliseconds(), "retry_safe": false}
	var incomplete *agent.IncompleteCompletionError
	var timeout net.Error
	switch {
	case errors.As(err, &incomplete):
		code = "model_output_incomplete"
		if incomplete.FinishReason == "length" {
			code = "model_output_limit"
			detail["finish_reason"] = "length"
		}
		if incomplete.FinishReason == "content_filter" {
			code = "model_output_filtered"
			detail["finish_reason"] = "content_filter"
		}
		detail["incomplete_completion_applied"] = false
		detail["reasoning_only"], detail["has_tool_calls"] = incomplete.ReasoningOnly, incomplete.HasToolCalls
	case errors.Is(err, context.DeadlineExceeded):
		code = "deadline_exceeded"
	case errors.Is(err, context.Canceled):
		code = "owner_cancelled"
	case errors.As(err, &timeout) && timeout.Timeout():
		code = "request_timeout"
	case agent.IsContextBudgetExceeded(err) || llm.IsContextLimitError(err):
		code = "context_budget_exceeded"
	case agent.IsToolLimitFinalResponseInvalid(err):
		code = "tool_limit_final_response_invalid"
	}
	detail["code"] = code
	if cfg != nil {
		detail["configured_model"], detail["configured_context_window"] = cfg.LLM.Model, cfg.Agent.ContextWindow
	}
	data, _ := json.Marshal(detail)
	return message + "\n\n" + string(data)
}
