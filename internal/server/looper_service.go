package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

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

	defer func() {
		r.holder.SetIdle()
		if statusCh != nil {
			close(statusCh)
		}
	}()

	broadcast := func() {
		if statusCh == nil {
			return
		}
		select {
		case statusCh <- r.holder.State():
		case <-ctx.Done():
		}
	}

	model := cfg.Model
	if model == "" {
		model = auraCfg.LLM.Model
	}

	sysPrompt := agent.MinimalSystemPromptBuilder(nil)
	var history []openai.ChatCompletionMessage

	// PREPARE
	r.holder.SetStep("prepare")
	broadcast()
	prepRes, history, err := agent.ExecuteMinimalLoop(ctx, client, model, sysPrompt, cfg.Prepare, tools, dispatchCtx, history, r.logger)
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
		planRes, history, err := agent.ExecuteMinimalLoop(ctx, client, model, "", cfg.Plan, tools, dispatchCtx, history, r.logger)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "plan", cfg.Plan, planRes.Response, planRes.Duration)
		broadcast()

		// ACTION
		r.holder.SetStep("action")
		broadcast()
		actionRes, history, err := agent.ExecuteMinimalLoop(ctx, client, model, "", cfg.Action, tools, dispatchCtx, history, r.logger)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "action", cfg.Action, actionRes.Response, actionRes.Duration)
		broadcast()

		// TEST
		r.holder.SetStep("test")
		broadcast()
		testRes, history, err := agent.ExecuteMinimalLoop(ctx, client, model, "", cfg.Test, tools, dispatchCtx, history, r.logger)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "test", cfg.Test, testRes.Response, testRes.Duration)
		r.holder.SetLastResult(testRes.Response)
		broadcast()

		// EXIT CONDITION
		r.holder.SetStep("exit")
		broadcast()
		exitRes, history, err := agent.ExecuteMinimalLoop(ctx, client, model, "", cfg.ExitCond, tools, dispatchCtx, history, r.logger)
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
		finishRes, _, err := agent.ExecuteMinimalLoop(ctx, client, model, "", cfg.Finish, tools, dispatchCtx, history, r.logger)
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
