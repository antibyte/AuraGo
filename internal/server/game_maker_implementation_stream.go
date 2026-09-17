package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

// The tool-free fallback does not pass through ExecuteAgentLoop's call deadline.
// Bound it explicitly. Stream/deadline and output-format recovery share one retry.
// Each attempt owns its stream; partial code never enters a file or the retry.
func (r *gameMakerAgentRunner) gameStarterCompletion(ctx context.Context, cfg *config.Config, client llm.ChatClient, run gamemaker.JobRun, revision, system, prompt string) (agent.MinimalLoopResult, []openai.ChatCompletionMessage, error) {
	timeout := time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 600 * time.Second // Same default as the main agent loop.
	}
	broker := &gameMakerBroker{service: r.service, projectID: run.Project.ID, jobID: run.Job.ID}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return agent.MinimalLoopResult{}, nil, err
		}
		// Reload the durable checkpoint, including interrupted private reasoning.
		// Keep the original source snapshot once, rather than duplicating it.
		history, checkpoint, err := r.gameConversation(ctx, cfg, run)
		if err != nil {
			return agent.MinimalLoopResult{}, nil, err
		}
		if len(history) > 0 {
			history = append([]openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: system}}, history...)
		}
		broker.Send("model_progress", "waiting")
		callCtx, cancel := context.WithTimeout(ctx, timeout)
		response, completion, err := agent.ExecuteMinimalLoop(callCtx, client, cfg.LLM.Model, system, prompt, nil,
			&agent.DispatchContext{Cfg: cfg, Guardian: r.server.Guardian, SessionID: "game-maker-" + run.Job.ID, MessageSource: "game_maker", ToolScopeRestricted: true, AllowedTools: map[string]struct{}{}},
			history, r.server.Logger, &agent.MinimalLoopOptions{MaxToolRounds: 0, StreamText: true, PreserveReasoning: true, Checkpoint: checkpoint})
		timedOut := errors.Is(callCtx.Err(), context.DeadlineExceeded) && errors.Is(err, context.DeadlineExceeded)
		cancel()
		broker.SendTokenUpdate(response.PromptTokens, response.CompletionTokens, response.PromptTokens+response.CompletionTokens, 0, 0, false, false, "provider_usage")
		formatError := errors.Is(err, agent.ErrUnexpectedToolCallText) && response.FinishReason == openai.FinishReasonStop
		if formatError {
			// An exact single-file envelope is source data, never a tool dispatch.
			// Mixed prose, additional operations and stale bindings stay rejected.
			code, extractErr := gameStarterWrappedSource(agent.LastAssistantPlainText(completion), run.Job.ID, revision)
			if extractErr == nil {
				response.Response, err, formatError = code, nil, false
			} else {
				err = fmt.Errorf("%w; source envelope rejected: %v", err, extractErr)
			}
		}
		if err == nil || attempt > 0 || ctx.Err() != nil || (!formatError && !timedOut && !errors.Is(err, agent.ErrIncompleteTextStream)) {
			return response, completion, err
		}
		if r.server.Logger != nil {
			r.server.Logger.Warn("game maker source generation needs correction; retrying once", "job_id", run.Job.ID, "timeout", timeout, "deadline_exceeded", timedOut, "output_format_invalid", formatError, "error", err)
		}
		broker.Send("model_progress", "retrying")
		if formatError {
			prompt = "The previous response used the old tool-call format and was rejected. None of those calls were executed and no source was saved. This phase has no tools. Use the existing plan, source snapshot and retained context; do not search, read files or repeat calls. Return only the complete TypeScript source for src/main.ts, from its imports to its final statement. No commentary, XML, JSON envelopes, patches or changes to common.ts; implement the requested game through the supplied APIs."
			continue
		}
		prompt = "The previous source-generation stream was interrupted. No source was saved or executed. Continue from the accepted plan, supplied files and retained context. Return the complete, concise src/main.ts from the beginning in one response; do not append to the discarded partial answer. Keep the requested mechanics and existing helper APIs. No tool calls or further planning."
	}
}
