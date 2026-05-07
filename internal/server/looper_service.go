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

	stepExec := func(stepName, prompt string, system string, history []openai.ChatCompletionMessage) (agent.MinimalLoopResult, []openai.ChatCompletionMessage, error) {
		stepCtx, stepCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer stepCancel()
		r.logger.Info("[Looper] step start", "step", stepName, "iteration", r.holder.State().Iteration)
		res, h, err := agent.ExecuteMinimalLoop(stepCtx, client, model, system, prompt, tools, dispatchCtx, history, r.logger)
		if err != nil {
			r.logger.Error("[Looper] step failed", "step", stepName, "error", err)
			return res, nil, err
		}
		r.logger.Info("[Looper] step done", "step", stepName, "duration_ms", res.Duration.Milliseconds(), "tool_calls", res.ToolCalls)
		return res, h, nil
	}

	// PREPARE — runs once, result preserved as shared context for every iteration
	r.holder.SetStep("prepare")
	prepRes, _, err := stepExec("prepare", cfg.Prepare, sysPrompt, nil)
	if err != nil {
		return r.setErrorAndReturn(err)
	}
	r.holder.AppendLog(0, "prepare", cfg.Prepare, prepRes.Response, prepRes.Duration)

	// Build iteration seed: system prompt + prepare result summary so each
	// iteration starts fresh but retains the preparation context.
	iterSeed := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: cfg.Prepare},
		{Role: openai.ChatMessageRoleAssistant, Content: prepRes.Response},
	}

	ctxMode := cfg.ContextMode
	if ctxMode == "" {
		ctxMode = "every_iteration"
	}

	var fullHistory []openai.ChatCompletionMessage

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
			if i > 1 && fullHistory != nil {
				lastAssistant := ""
				for mi := len(fullHistory) - 1; mi >= 0; mi-- {
					if fullHistory[mi].Role == openai.ChatMessageRoleAssistant {
						lastAssistant = fullHistory[mi].Content
						break
					}
				}
				if lastAssistant != "" {
					history = append(history, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: "Previous iteration test result: " + lastAssistant,
					})
				}
			}
		}

		// PLAN
		r.holder.SetStep("plan")
		planRes, history, err := stepExec("plan", cfg.Plan, "", history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "plan", cfg.Plan, planRes.Response, planRes.Duration)

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: planRes.Response},
			}
		}

		// ACTION
		r.holder.SetStep("action")
		actionRes, history, err := stepExec("action", cfg.Action, "", history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "action", cfg.Action, actionRes.Response, actionRes.Duration)

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: actionRes.Response},
			}
		}

		// TEST
		r.holder.SetStep("test")
		testRes, history, err := stepExec("test", cfg.Test, "", history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "test", cfg.Test, testRes.Response, testRes.Duration)
		r.holder.SetLastResult(testRes.Response)

		if ctxMode == "every_step" {
			history = []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
				{Role: openai.ChatMessageRoleUser, Content: testRes.Response},
			}
		}

		// EXIT CONDITION
		r.holder.SetStep("exit")
		exitRes, _, err := stepExec("exit", cfg.ExitCond, "", history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "exit", cfg.ExitCond, exitRes.Response, exitRes.Duration)

		if ctxMode == "never" {
			fullHistory = history
		} else {
			fullHistory = history
		}

		if agent.ParseExitBoolean(exitRes.Response) {
			break
		}
	}

	// FINISH
	if strings.TrimSpace(cfg.Finish) != "" {
		r.holder.SetStep("finish")
		finishRes, _, err := stepExec("finish", cfg.Finish, "", iterSeed)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(0, "finish", cfg.Finish, finishRes.Response, finishRes.Duration)
		r.holder.SetLastResult(finishRes.Response)
	}

	return nil
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
