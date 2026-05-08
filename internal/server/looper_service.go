package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

// LooperRunner handles the execution side of the Looper workflow.
type LooperRunner struct {
	store  *desktop.LooperPresetStore
	holder *desktop.LooperRunStateHolder
	logger *slog.Logger
}

// NewLooperRunner creates a runner backed by a preset store.
func NewLooperRunner(store *desktop.LooperPresetStore, logger *slog.Logger) *LooperRunner {
	return &LooperRunner{
		store:  store,
		holder: desktop.NewLooperRunStateHolder(),
		logger: logger,
	}
}

// State returns the current run state.
func (r *LooperRunner) State() desktop.LooperRunState {
	return r.holder.State()
}

// Stop cancels the current run.
func (r *LooperRunner) Stop() {
	r.holder.CancelRun()
}

// Shutdown cancels any running loop and resets the runner state.
func (r *LooperRunner) Shutdown() {
	r.holder.CancelRun()
	r.holder.SetIdle()
}

func (r *LooperRunner) TryStart(maxIter int, cancel context.CancelFunc) error {
	return r.holder.TryStart(maxIter, cancel)
}

func (r *LooperRunner) executeStarted(
	ctx context.Context,
	cfg desktop.LooperRunConfig,
	auraCfg *config.Config,
	client llm.ChatClient,
	tools []openai.Tool,
	dispatchCtx *agent.DispatchContext,
) error {
	defer r.holder.SetIdle()

	model := cfg.Model
	if model == "" {
		model = auraCfg.LLM.Model
	}

	sysPrompt := agent.MinimalSystemPromptBuilder(nil)

	// No-tools schema for exit step — it only needs to return true/false,
	// so sending 50+ tool schemas wastes thousands of tokens per call.
	noTools := []openai.Tool{}

	// maxHistoryChars keeps the conversation history from growing unbounded.
	// 40 000 chars ≈ 10 000 tokens, enough for rich context without explosion.
	const maxHistoryChars = 40000

	stepExec := func(stepName, prompt string, system string, stepTools []openai.Tool, opts *agent.MinimalLoopOptions, history []openai.ChatCompletionMessage) (agent.MinimalLoopResult, []openai.ChatCompletionMessage, error) {
		const maxRetries = 3
		for attempt := 1; attempt <= maxRetries; attempt++ {
			select {
			case <-ctx.Done():
				return agent.MinimalLoopResult{}, nil, fmt.Errorf("aborted by user")
			default:
			}

			timeout := 3 * time.Minute
			if len(stepTools) > 0 {
				timeout = 5 * time.Minute
			}
			stepCtx, stepCancel := context.WithTimeout(ctx, timeout)

			r.logger.Info("[Looper] step start", "step", stepName, "iteration", r.holder.State().Iteration, "tools", len(stepTools), "attempt", attempt)
			res, h, err := agent.ExecuteMinimalLoop(stepCtx, client, model, system, prompt, stepTools, dispatchCtx, history, r.logger, opts)
			stepCancel()

			if err != nil {
				r.logger.Warn("[Looper] step error", "step", stepName, "attempt", attempt, "maxRetries", maxRetries, "error", err)
				if attempt < maxRetries && ctx.Err() == nil {
					backoff := time.Duration(attempt*attempt) * time.Second
					r.logger.Info("[Looper] retrying", "step", stepName, "backoff", backoff)
					select {
					case <-time.After(backoff):
						continue
					case <-ctx.Done():
						return agent.MinimalLoopResult{}, nil, fmt.Errorf("aborted by user")
					}
				}
				r.logger.Error("[Looper] step failed after retries", "step", stepName, "error", err)
				return res, nil, err
			}
			r.logger.Info("[Looper] step done", "step", stepName, "duration_ms", res.Duration.Milliseconds(), "tool_calls", res.ToolCalls)
			return res, h, nil
		}
		return agent.MinimalLoopResult{}, nil, fmt.Errorf("unreachable")
	}

	// optsWithTools is the default options for steps that need tools.
	optsWithTools := &agent.MinimalLoopOptions{MaxToolRounds: 3}
	// optsNoTools for the exit step — no tool schemas, no tool rounds.
	optsNoTools := &agent.MinimalLoopOptions{MaxToolRounds: 0}

	// PREPARE — runs once, result preserved as shared context for every iteration
	r.holder.SetStep("prepare")
	prepRes, _, err := stepExec("prepare", cfg.Prepare, sysPrompt, tools, optsWithTools, nil)
	if err != nil {
		return r.setErrorAndReturn(err)
	}
	r.holder.AppendLog(0, "prepare", cfg.Prepare, prepRes.Response, prepRes.Duration)

	// Build iteration seed: system prompt + prepare result summary so each
	// iteration starts fresh but retains the preparation context.
	iterSeed := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: cfg.Prepare},
		{Role: openai.ChatMessageRoleAssistant, Content: truncateResponse(prepRes.Response, 2000)},
	}

	ctxMode := cfg.ContextMode
	if ctxMode == "" {
		ctxMode = "every_iteration"
	}

	var fullHistory []openai.ChatCompletionMessage
	var lastTestResult string

	// ITERATIONS
	for i := 1; i <= cfg.MaxIter; i++ {
		select {
		case <-ctx.Done():
			return r.setErrorAndReturn(fmt.Errorf("aborted by user"))
		default:
		}

		r.holder.SetIteration(i)

		var history []openai.ChatCompletionMessage

		switch ctxMode {
		case "never":
			if i == 1 {
				history = make([]openai.ChatCompletionMessage, len(iterSeed))
				copy(history, iterSeed)
			} else {
				history = fullHistory
			}

		case "every_step":
			history = make([]openai.ChatCompletionMessage, len(iterSeed))
			copy(history, iterSeed)

		default: // "every_iteration"
			history = make([]openai.ChatCompletionMessage, len(iterSeed))
			copy(history, iterSeed)
			if i > 1 && lastTestResult != "" {
				history = append(history, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: "Previous iteration test result: " + truncateResponse(lastTestResult, 2000),
				})
			}
		}

		// PLAN
		r.holder.SetStep("plan")
		planRes, history, err := stepExec("plan", cfg.Plan, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "plan", cfg.Plan, planRes.Response, planRes.Duration)

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: truncateResponse(planRes.Response, 2000)},
			}
		} else {
			history = append(history, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Plan result:\n" + truncateResponse(planRes.Response, 3000),
			})
		}

		// ACTION
		r.holder.SetStep("action")
		actionRes, history, err := stepExec("action", cfg.Action, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "action", cfg.Action, actionRes.Response, actionRes.Duration)

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: truncateResponse(actionRes.Response, 2000)},
			}
		}

		// TEST
		r.holder.SetStep("test")
		testRes, history, err := stepExec("test", cfg.Test, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "test", cfg.Test, testRes.Response, testRes.Duration)
		r.holder.SetLastResult(testRes.Response)
		lastTestResult = testRes.Response

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: truncateResponse(testRes.Response, 2000)},
			}
		}

		// EXIT CONDITION — no tools needed, just a boolean evaluation
		r.holder.SetStep("exit")
		exitRes, _, err := stepExec("exit", cfg.ExitCond, "", noTools, optsNoTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "exit", cfg.ExitCond, exitRes.Response, exitRes.Duration)

		fullHistory = history

		if agent.ParseExitBoolean(exitRes.Response) {
			break
		}

		// Trim history between iterations to prevent unbounded context growth.
		fullHistory = agent.TrimHistory(fullHistory, maxHistoryChars)
	}

	// FINISH
	if strings.TrimSpace(cfg.Finish) != "" {
		r.holder.SetStep("finish")
		finishRes, _, err := stepExec("finish", cfg.Finish, "", tools, optsWithTools, iterSeed)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(0, "finish", cfg.Finish, finishRes.Response, finishRes.Duration)
		r.holder.SetLastResult(finishRes.Response)
	}

	return nil
}

// truncateResponse shortens a response to maxLen characters for compact context passing.
func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d more chars)", len(s)-maxLen)
}

func (r *LooperRunner) setErrorAndReturn(err error) error {
	r.holder.SetError(err.Error())
	return err
}

// Server-side singleton management for looper.
var (
	looperRunnerMu sync.Mutex
	looperRunner   *LooperRunner
)

func getLooperRunner(s *Server) (*LooperRunner, error) {
	looperRunnerMu.Lock()
	defer looperRunnerMu.Unlock()
	if looperRunner != nil {
		return looperRunner, nil
	}
	svc, _, err := s.getDesktopService(context.Background())
	if err != nil {
		return nil, err
	}
	db := svc.DB()
	if db == nil {
		return nil, fmt.Errorf("desktop database not ready")
	}
	store := desktop.NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		return nil, err
	}
	looperRunner = NewLooperRunner(store, s.Logger)
	return looperRunner, nil
}

func shutdownLooper() {
	looperRunnerMu.Lock()
	defer looperRunnerMu.Unlock()
	if looperRunner != nil {
		looperRunner.Shutdown()
	}
}
