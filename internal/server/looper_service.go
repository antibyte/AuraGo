package server

import (
	"context"
	"encoding/json"
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

// Pause requests a graceful pause at the next iteration boundary.
// Safe to call from the HTTP handler while the loop goroutine is running.
func (r *LooperRunner) Pause() {
	r.holder.RequestPause()
}

// ResumeState returns any saved resume snapshot from a previously paused run.
func (r *LooperRunner) ResumeState() (desktop.LooperResumeState, bool) {
	return r.holder.GetResumeState()
}

// Resume continues a paused loop from the saved snapshot (clean E8 completion).
// It re-runs Prepare to re-establish context and then continues the iteration
// loop from the saved point using the seeded carry-over state.
func (r *LooperRunner) Resume(
	ctx context.Context,
	cfg desktop.LooperRunConfig,
	auraCfg *config.Config,
	client llm.ChatClient,
	tools []openai.Tool,
	dispatchCtx *agent.DispatchContext,
) error {
	rs, ok := r.holder.GetResumeState()
	if !ok {
		return fmt.Errorf("no paused run to resume")
	}
	r.holder.ClearResumeState()
	return r.executeStarted(ctx, cfg, auraCfg, client, tools, dispatchCtx, &rs)
}

func (r *LooperRunner) executeStarted(
	ctx context.Context,
	cfg desktop.LooperRunConfig,
	auraCfg *config.Config,
	client llm.ChatClient,
	tools []openai.Tool,
	dispatchCtx *agent.DispatchContext,
	resumeSeed *desktop.LooperResumeState, // non-nil when continuing a paused run (E8)
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
	optsWithTools := &agent.MinimalLoopOptions{MaxToolRounds: 10}
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
	truncLen := cfg.PrepareTruncation
	if truncLen <= 0 {
		truncLen = 2000 // conservative default
	}

	iterSeed := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: cfg.Prepare},
		{Role: openai.ChatMessageRoleAssistant, Content: truncateResponse(prepRes.Response, truncLen)},
	}

	ctxMode := cfg.ContextMode
	if ctxMode == "" {
		ctxMode = "every_iteration"
	}

	var lastActionResult string
	var lastTestResult string
	var previousIterationSummary string // used primarily by "never" mode
	var lastIterationSummary string     // result of the optional "summarize" step

	startIter := 1
	if resumeSeed != nil {
		startIter = resumeSeed.Iteration + 1
		lastTestResult = resumeSeed.LastTestResult
		previousIterationSummary = resumeSeed.PreviousIterationSummary
		lastIterationSummary = resumeSeed.LastIterationSummary
		r.logger.Info("[Looper] resuming from saved state", "from_iteration", resumeSeed.Iteration, "starting_at", startIter)
	}

	// ─────────────────────────────────────────────────────────────────────
	// ITERATION LOOP + CONTEXT MODE SEMANTICS
	//
	// We support three different strategies for how much history is carried
	// between iterations. The goal is to give the LLM the right amount of
	// context for different kinds of tasks.
	//
	// 1. "every_iteration" (DEFAULT / recommended for creative work like Ralph Loop)
	//    - Every iteration starts with the original Prepare result (iterSeed).
	//    - The Test result of the *previous* iteration is injected as additional context.
	//    - This gives good continuity while still keeping the original task visible.
	//
	// 2. "every_step"
	//    - After every single step (Plan, Action, Test) the history is reset to
	//      only System + the result of the just completed step.
	//    - Very low token usage, strong isolation between steps.
	//    - **Warning**: The original Prepare context disappears quickly.
	//      Only use for very isolated, stateless micro-tasks.
	//
	// 3. "never"  (fresh start per iteration, but with progress memory)
	//    - Every iteration starts from the original task (iterSeed).
	//    - For iterations > 1 we append a compact summary of what was achieved
	//      in the previous iteration.
	//    - This is the intended "relatively fresh but still progressing" mode.
	//    - IMPORTANT: The original Prepare must NEVER be lost in this mode.
	//
	// The implementation below (especially buildBaseHistoryForIteration + the
	// special handling after it) enforces these semantics.
	// ─────────────────────────────────────────────────────────────────────

	// ITERATIONS
	for i := startIter; i <= cfg.MaxIter; i++ {
		select {
		case <-ctx.Done():
			return r.setErrorAndReturn(fmt.Errorf("aborted by user"))
		default:
		}

		r.holder.SetIteration(i)

		// Early pause check at iteration start (catches pause requests that arrived
		// exactly after the previous checkpoint but before we began the new round).
		if r.holder.IsPauseRequested() {
			rs := desktop.LooperResumeState{
				Iteration:                i,
				LastTestResult:           lastTestResult,
				PreviousIterationSummary: previousIterationSummary,
				LastIterationSummary:     lastIterationSummary,
			}
			r.holder.SaveResumeState(rs)
			r.logger.Info("[Looper] run paused before starting iteration", "iteration", i)
			return nil
		}

		history := buildBaseHistoryForIteration(iterSeed, ctxMode, i, lastTestResult, previousIterationSummary)

		// Special handling for "never" mode:
		// We always want the original task (iterSeed) to be present.
		// On top of that we add a compact summary of the previous iteration's progress.
		if ctxMode == "never" && i > 1 && previousIterationSummary != "" {
			history = append(history, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Summary of previous iteration:\n" + truncateResponse(previousIterationSummary, 2800),
			})
		}

		// Inject the explicit iteration summary (from the "summarize" step) if available.
		// This is especially powerful in "every_iteration" and "never" modes.
		if i > 1 && lastIterationSummary != "" {
			history = append(history, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Reflection / Summary of the previous iteration:\n" + truncateResponse(lastIterationSummary, 2500),
			})
		}

		// PLAN
		r.holder.SetStep("plan")
		planRes, history, err := stepExec("plan", cfg.Plan, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "plan", cfg.Plan, planRes.Response, planRes.Duration)

		history = appendStepResult(history, "plan", planRes.Response, ctxMode, sysPrompt)

		// ACTION
		r.holder.SetStep("action")
		actionRes, history, err := stepExec("action", cfg.Action, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "action", cfg.Action, actionRes.Response, actionRes.Duration)
		lastActionResult = buildActionFinishResult(actionRes.Response, history, cfg.Action)

		history = appendStepResult(history, "action", actionRes.Response, ctxMode, sysPrompt)

		// TEST
		r.holder.SetStep("test")
		testRes, history, err := stepExec("test", cfg.Test, "", tools, optsWithTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(i, "test", cfg.Test, testRes.Response, testRes.Duration)
		r.holder.SetLastResult(testRes.Response)
		lastTestResult = testRes.Response

		// Optional explicit summarization step (greatly helps long creative loops)
		if cfg.SummarizeIterations {
			r.holder.SetStep("summarize")
			summaryPrompt := "Provide a concise but insightful summary (max ~600 words) of the key decisions, changes, and outcome of this iteration. Focus on what improved, what the main insights were, and what still needs attention for the next round."
			summaryRes, _, err := stepExec("summarize", summaryPrompt, sysPrompt, tools, optsWithTools, history)
			if err == nil {
				r.holder.AppendLog(i, "summarize", summaryPrompt, summaryRes.Response, summaryRes.Duration)
				lastIterationSummary = summaryRes.Response // will be injected in next iteration
			} else {
				r.logger.Warn("[Looper] iteration summarization failed", "iteration", i, "err", err)
			}
		}

		history = appendStepResult(history, "test", testRes.Response, ctxMode, sysPrompt)

		// EXIT CONDITION — no tools needed, just a boolean evaluation.
		// We now prefer structured output from the model:
		//   {"decision": true/false, "reason": "...", "confidence": 0.0-1.0}
		// Falls back to the previous boolean parser + clarification if no valid JSON is returned.
		r.holder.SetStep("exit")
		exitRes, _, err := stepExec("exit", cfg.ExitCond, "", noTools, optsNoTools, history)
		if err != nil {
			return r.setErrorAndReturn(err)
		}

		decision, reason, confidence, usedStructured := parseStructuredExit(exitRes.Response)

		if usedStructured {
			r.holder.AppendLogWithReason(i, "exit", cfg.ExitCond, exitRes.Response, exitRes.Duration, reason)
			r.logger.Info("[Looper] exit structured", "iteration", i, "decision", decision, "confidence", confidence, "reason", reason)
		} else {
			r.holder.AppendLog(i, "exit", cfg.ExitCond, exitRes.Response, exitRes.Duration)

			shouldExit, decisive := agent.ParseExitBooleanWithConfidence(exitRes.Response)
			if !decisive {
				// One-shot clarification (very cheap, no tools)
				clarityPrompt := "The previous answer was ambiguous. Reply with ONLY the single lowercase word \"true\" or \"false\". No explanation."
				clarityRes, _, cerr := stepExec("exit_clarify", clarityPrompt, sysPrompt, noTools, optsNoTools, history)
				if cerr == nil {
					r.holder.AppendLog(i, "exit_clarify", clarityPrompt, clarityRes.Response, clarityRes.Duration)
					shouldExit = agent.ParseExitBoolean(clarityRes.Response)
				} else {
					r.logger.Warn("[Looper] exit clarification call failed", "err", cerr)
				}
			}
			decision = shouldExit
		}

		if decision {
			break
		}

		// Build a compact summary for the next iteration (mainly used by "never" mode).
		previousIterationSummary = ""
		if lastTestResult != "" {
			previousIterationSummary = "Test result: " + truncateResponse(lastTestResult, 1500)
		}

		// E8 – Pause checkpoint (safe iteration boundary).
		// We only pause between iterations, never in the middle of Plan/Action/Test/Exit.
		// This guarantees that lastTestResult + summaries are consistent for resume.
		if r.holder.IsPauseRequested() {
			rs := desktop.LooperResumeState{
				Iteration:                i,
				LastTestResult:           lastTestResult,
				PreviousIterationSummary: previousIterationSummary,
				LastIterationSummary:     lastIterationSummary,
			}
			r.holder.SaveResumeState(rs)
			r.logger.Info("[Looper] run paused by user request", "iteration", i, "resume_from", i)
			return nil // clean return – holder is now in paused + resumable state
		}
	}

	// FINISH
	if strings.TrimSpace(cfg.Finish) != "" {
		r.holder.SetStep("finish")
		finishHistory := buildFinishHistory(iterSeed, cfg.FinishContext, lastActionResult, lastTestResult, sysPrompt)

		finishRes, _, err := stepExec("finish", cfg.Finish, "", tools, optsWithTools, finishHistory)
		if err != nil {
			return r.setErrorAndReturn(err)
		}
		r.holder.AppendLog(0, "finish", cfg.Finish, finishRes.Response, finishRes.Duration)
		r.holder.SetLastResult(finishRes.Response)
	}

	return nil
}

func buildFinishHistory(iterSeed []openai.ChatCompletionMessage, finishCtxMode, lastActionResult, lastTestResult, sysPrompt string) []openai.ChatCompletionMessage {
	finishHistory := make([]openai.ChatCompletionMessage, len(iterSeed))
	copy(finishHistory, iterSeed)

	basePrompt := sysPrompt
	if basePrompt == "" && len(finishHistory) > 0 && finishHistory[0].Role == openai.ChatMessageRoleSystem {
		basePrompt = finishHistory[0].Content
	}
	finishSystem := buildLooperFinishSystemPrompt(basePrompt)
	if finishSystem != "" {
		if len(finishHistory) > 0 && finishHistory[0].Role == openai.ChatMessageRoleSystem {
			finishHistory[0].Content = finishSystem
		} else {
			finishHistory = append([]openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: finishSystem},
			}, finishHistory...)
		}
	}

	if finalPart := buildFinishContextMessage(finishCtxMode, lastActionResult, lastTestResult); finalPart != "" {
		finishHistory = append(finishHistory, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: finalPart,
		})
	}

	return finishHistory
}

func buildActionFinishResult(actionResult string, actionHistory []openai.ChatCompletionMessage, actionPrompt string) string {
	actionResult = strings.TrimSpace(actionResult)
	toolOutputs := recentToolOutputsAfterLastUserPrompt(actionHistory, actionPrompt, 2500)
	if toolOutputs == "" {
		return actionResult
	}
	if actionResult == "" {
		return "Action tool output:\n" + toolOutputs
	}
	return actionResult + "\n\nAction tool output:\n" + toolOutputs
}

func recentToolOutputsAfterLastUserPrompt(history []openai.ChatCompletionMessage, prompt string, maxLen int) string {
	start := -1
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == openai.ChatMessageRoleUser && history[i].Content == prompt {
			start = i + 1
			break
		}
	}
	if start == -1 {
		return ""
	}

	var chunks []string
	used := 0
	for _, msg := range history[start:] {
		if msg.Role != openai.ChatMessageRoleTool || strings.TrimSpace(msg.Content) == "" {
			continue
		}
		name := strings.TrimSpace(msg.Name)
		if name == "" {
			name = "tool"
		}
		remaining := maxLen - used
		if remaining <= 0 {
			break
		}
		chunk := name + " output:\n" + truncateResponse(strings.TrimSpace(msg.Content), remaining)
		chunks = append(chunks, chunk)
		used += len(chunk)
	}
	return strings.Join(chunks, "\n\n")
}

func buildLooperFinishSystemPrompt(basePrompt string) string {
	basePrompt = strings.TrimSpace(basePrompt)
	const finishRules = "Looper finish step rules:\n" +
		"- If the finish instruction asks to open, show, present, export, or hand off a result in the desktop, perform the concrete desktop tool action before final text.\n" +
		"- Use virtual_desktop write_document/write_file/open_in_app/open_app as appropriate. Do not merely say that you will open something.\n" +
		"- For Writer documents, create or update a workspace document, then call virtual_desktop open_in_app with app_id \"writer\" and the document path.\n" +
		"- For view-only files, call virtual_desktop open_in_app with app_id \"viewer\" and the file path. For code files, use app_id \"code-studio\"."
	if basePrompt == "" {
		return finishRules
	}
	return basePrompt + "\n\n" + finishRules
}

func buildFinishContextMessage(finishCtxMode, lastActionResult, lastTestResult string) string {
	switch strings.TrimSpace(finishCtxMode) {
	case "none":
		return ""
	case "last_action_test", "full":
		parts := make([]string, 0, 2)
		if strings.TrimSpace(lastActionResult) != "" {
			parts = append(parts, "Final action result:\n"+truncateResponse(lastActionResult, 4000))
		}
		if strings.TrimSpace(lastTestResult) != "" {
			parts = append(parts, "Final test result:\n"+truncateResponse(lastTestResult, 3000))
		}
		if len(parts) == 0 {
			return ""
		}
		return "Result of the last iteration:\n\n" + strings.Join(parts, "\n\n")
	default:
		if strings.TrimSpace(lastTestResult) == "" {
			return ""
		}
		return "Final test result of the loop:\n" + truncateResponse(lastTestResult, 3000)
	}
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

// parseStructuredExit tries to interpret the model's response as structured exit output.
// Expected format (best effort):
//
//	{"decision": true/false, "reason": "short explanation", "confidence": 0.75}
//
// Returns (decision, reason, confidence, usedStructured).
func parseStructuredExit(raw string) (bool, string, float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false, "", 0, false
	}

	// Try to find a JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return false, "", 0, false
	}

	jsonPart := s[start : end+1]

	var data struct {
		Decision   *bool    `json:"decision"`
		Reason     string   `json:"reason"`
		Confidence *float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonPart), &data); err != nil {
		return false, "", 0, false
	}

	if data.Decision == nil {
		return false, "", 0, false
	}

	conf := 0.0
	if data.Confidence != nil {
		conf = *data.Confidence
	}

	return *data.Decision, strings.TrimSpace(data.Reason), conf, true
}

// buildBaseHistoryForIteration returns a fresh history slice for the given iteration
// according to the chosen context mode. It never mutates iterSeed.
//
// For "never" mode the caller is responsible for appending a previousIterationSummary
// after calling this function (see the loop in executeStarted).
func buildBaseHistoryForIteration(
	iterSeed []openai.ChatCompletionMessage,
	ctxMode string,
	i int,
	lastTestResult string,
	previousIterationSummary string, // currently only used for documentation / future use in "never"
) []openai.ChatCompletionMessage {

	switch ctxMode {
	case "never":
		// "never" = fresh start from the original task every time.
		// The progress summary is added by the caller after this call.
		h := make([]openai.ChatCompletionMessage, len(iterSeed))
		copy(h, iterSeed)
		return h

	case "every_step":
		h := make([]openai.ChatCompletionMessage, len(iterSeed))
		copy(h, iterSeed)
		return h

	default: // "every_iteration"
		h := make([]openai.ChatCompletionMessage, len(iterSeed))
		copy(h, iterSeed)
		if i > 1 && lastTestResult != "" {
			h = append(h, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "Previous iteration test result: " + truncateResponse(lastTestResult, 2000),
			})
		}
		return h
	}
}

// resetHistoryAfterStep is used by "every_step" mode to give the *next* step
// a minimal, clean context containing only the system prompt and the just-completed step result.
func resetHistoryAfterStep(sysPrompt, stepResult string) []openai.ChatCompletionMessage {
	return []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: truncateResponse(stepResult, 2000)},
	}
}

// appendStepResult encapsulates the logic of how to extend the conversation
// history after receiving the result of a step (Plan, Action, Test, Summarize...),
// depending on the chosen context mode.
//
// This makes the main loop much easier to read and reason about.
func appendStepResult(
	history []openai.ChatCompletionMessage,
	stepName string,
	result string,
	ctxMode string,
	sysPrompt string,
) []openai.ChatCompletionMessage {

	if ctxMode == "every_step" {
		return resetHistoryAfterStep(sysPrompt, result)
	}

	// For "every_iteration" and "never" we keep accumulating
	label := ""
	switch stepName {
	case "plan":
		label = "Plan result:"
	case "action":
		label = "Action result:"
	case "test":
		label = "Test result:"
	case "summarize":
		label = "Iteration reflection:"
	default:
		label = stepName + " result:"
	}

	return append(history, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: label + "\n" + truncateResponse(result, 3000),
	})
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
