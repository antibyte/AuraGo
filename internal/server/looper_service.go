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

// Execute runs the loop workflow.
func (r *LooperRunner) Execute(
	ctx context.Context,
	cfg desktop.LooperRunConfig,
	auraCfg *config.Config,
	client llm.ChatClient,
	tools []openai.Tool,
	dispatchCtx *agent.DispatchContext,
	statusCh chan<- desktop.LooperRunState,
) error {
	state := r.holder.State()
	if state.Running {
		if statusCh != nil {
			close(statusCh)
		}
		return fmt.Errorf("a loop is already running")
	}
	r.holder.CancelRun() // cancel any previous run just in case

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r.holder.SetCancelFn(cancel)
	r.holder.SetRunning(cfg.MaxIter)

	broadcast := func() {
		if statusCh == nil {
			return
		}
		select {
		case statusCh <- r.holder.State():
		default:
		}
	}

	defer func() {
		r.holder.SetIdle()
		broadcast()
		if statusCh != nil {
			close(statusCh)
		}
	}()

	model := cfg.Model
	if model == "" {
		model = auraCfg.LLM.Model
	}

	sysPrompt := agent.MinimalSystemPromptBuilder(nil)
	var history []openai.ChatCompletionMessage

	// stepExec runs one minimal-loop step with a per-step timeout and logging.
	// It mutates the outer history slice directly.
	stepExec := func(stepName, prompt string, system string) (agent.MinimalLoopResult, error) {
		stepCtx, stepCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer stepCancel()
		r.logger.Info("[Looper] step start", "step", stepName, "iteration", r.holder.State().Iteration)
		res, h, err := agent.ExecuteMinimalLoop(stepCtx, client, model, system, prompt, tools, dispatchCtx, history, r.logger)
		if err != nil {
			r.logger.Error("[Looper] step failed", "step", stepName, "error", err)
			return res, err
		}
		history = h
		r.logger.Info("[Looper] step done", "step", stepName, "duration_ms", res.Duration.Milliseconds(), "tool_calls", res.ToolCalls)
		return res, nil
	}

	// PREPARE
	r.holder.SetStep("prepare")
	broadcast()
	prepRes, err := stepExec("prepare", cfg.Prepare, sysPrompt)
	if err != nil {
		return r.setErrorAndReturn(err)
	}
	r.holder.AppendLog(0, "prepare", cfg.Prepare, prepRes.Response, prepRes.Duration)
	broadcast()

	// ITERATIONS
	for i := 1; i <= cfg.MaxIter; i++ {
		select {
		case <-ctx.Done():
			return r.setErrorAndReturn(fmt.Errorf("aborted by user"))
		default:
		}

		r.holder.SetIteration(i)

		// PLAN
		r.holder.SetStep("plan")
		broadcast()
		planRes, err := stepExec("plan", cfg.Plan, "")
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "plan", cfg.Plan, planRes.Response, planRes.Duration)
		broadcast()

		// ACTION
		r.holder.SetStep("action")
		broadcast()
		actionRes, err := stepExec("action", cfg.Action, "")
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "action", cfg.Action, actionRes.Response, actionRes.Duration)
		broadcast()

		// TEST
		r.holder.SetStep("test")
		broadcast()
		testRes, err := stepExec("test", cfg.Test, "")
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "test", cfg.Test, testRes.Response, testRes.Duration)
		r.holder.SetLastResult(testRes.Response)
		broadcast()

		// EXIT CONDITION
		r.holder.SetStep("exit")
		broadcast()
		exitRes, err := stepExec("exit", cfg.ExitCond, "")
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "exit", cfg.ExitCond, exitRes.Response, exitRes.Duration)
		broadcast()

		if agent.ParseExitBoolean(exitRes.Response) {
			break
		}
	}

	// FINISH
	if strings.TrimSpace(cfg.Finish) != "" {
		r.holder.SetStep("finish")
		broadcast()
		finishRes, err := stepExec("finish", cfg.Finish, "")
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(0, "finish", cfg.Finish, finishRes.Response, finishRes.Duration)
		r.holder.SetLastResult(finishRes.Response)
		broadcast()
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
