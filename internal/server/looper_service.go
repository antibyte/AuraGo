package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
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

	// lastRun remembers the active (or last paused) run's settings so a paused
	// run that gets discarded can still be written to the history.
	lastRunMu      sync.Mutex
	lastCfg        desktop.LooperRunConfig
	lastStart      time.Time
	hasLast        bool
	persistedRunID int64

	// shuttingDown is set when the server stops: a run cut off by that is not
	// a user stop, so it keeps its checkpoint and is offered for resuming after
	// the restart instead of being filed as "stopped".
	shuttingDown atomic.Bool
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

// StateSince returns the run state with only the log entries at or after the
// given absolute index, for incremental status streaming.
func (r *LooperRunner) StateSince(since int) desktop.LooperRunState {
	return r.holder.StateSince(since)
}

// Changed returns a channel closed on the next state change.
func (r *LooperRunner) Changed() <-chan struct{} {
	return r.holder.Changed()
}

// Stop cancels the current run.
func (r *LooperRunner) Stop() {
	r.holder.CancelRun()
}

func (r *LooperRunner) rememberRun(cfg desktop.LooperRunConfig, startedAt time.Time) {
	r.lastRunMu.Lock()
	defer r.lastRunMu.Unlock()
	r.lastCfg = cfg
	r.lastStart = startedAt
	r.hasLast = true
}

// DiscardPaused ends a paused run for good: it is stored in the history as
// "stopped" and the resume snapshot is dropped. It reports false when there
// was nothing paused.
func (r *LooperRunner) DiscardPaused() bool {
	if !r.holder.DiscardPaused() {
		return false
	}
	r.lastRunMu.Lock()
	cfg, startedAt, ok := r.lastCfg, r.lastStart, r.hasLast
	r.lastRunMu.Unlock()
	if ok {
		r.persistFinishedRun(cfg, startedAt)
	}
	return true
}

// Shutdown cancels any running loop and resets the runner state. The last
// round-boundary checkpoint stays in the database for the next start.
func (r *LooperRunner) Shutdown() {
	r.shuttingDown.Store(true)
	r.holder.CancelRun()
	r.holder.SetIdle()
}

// StoredConfig returns the settings of the run that is executing or paused, so
// a window opened after a restart can show and resume it without the form.
func (r *LooperRunner) StoredConfig() (desktop.LooperRunConfig, bool) {
	st := r.holder.State()
	if !st.Running && !st.Paused {
		return desktop.LooperRunConfig{}, false
	}
	r.lastRunMu.Lock()
	defer r.lastRunMu.Unlock()
	return r.lastCfg, r.hasLast
}

// checkpoint writes the active run to the database so a crash or restart can
// resume from the last finished round instead of losing the run.
func (r *LooperRunner) checkpoint(cfg desktop.LooperRunConfig, startedAt time.Time, resume desktop.LooperResumeState) {
	if r.store == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.store.SaveActiveRun(ctx, desktop.LooperActiveRun{
		Config:    cfg,
		StartedAt: startedAt,
		State:     r.holder.State(),
		Resume:    resume,
	}); err != nil && r.logger != nil {
		r.logger.Warn("[Looper] checkpoint failed", "error", err)
	}
}

// restoreActive turns a checkpoint left behind by a previous process into a
// paused run. A run the user had paused stays paused for the same reason; one
// that was executing is reported as interrupted.
func (r *LooperRunner) restoreActive(ctx context.Context) {
	if r.store == nil {
		return
	}
	rec, ok, err := r.store.LoadActiveRun(ctx)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("[Looper] dropping unreadable checkpoint", "error", err)
		}
		_ = r.store.ClearActiveRun(ctx)
		return
	}
	if !ok {
		return
	}
	reason := rec.State.PauseReason
	if !rec.State.Paused {
		reason = "interrupted"
	}
	r.holder.Restore(rec.State, rec.Resume, reason)
	r.rememberRun(rec.Config, rec.StartedAt)
	if r.logger != nil {
		r.logger.Info("[Looper] restored a run from the last checkpoint", "round", rec.Resume.Round, "reason", reason)
	}
}

func looperBudgetBlocked(d *agent.DispatchContext) bool {
	return d != nil && d.BudgetTracker != nil && d.BudgetTracker.IsBlocked("looper")
}

func (r *LooperRunner) TryStart(maxRounds int, cancel context.CancelFunc) error {
	return r.holder.TryStart(maxRounds, cancel)
}

// Pause requests a graceful pause at the next round boundary.
func (r *LooperRunner) Pause() {
	r.holder.RequestPause()
}

// ResumeState returns any saved resume snapshot from a previously paused run.
func (r *LooperRunner) ResumeState() (desktop.LooperResumeState, bool) {
	return r.holder.GetResumeState()
}

// Resume continues a paused loop from the saved snapshot.
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
	return r.executeStarted(ctx, cfg, auraCfg, client, tools, dispatchCtx, &rs)
}

// TryStartResume exposes the holder's resume-friendly start for HTTP handlers.
func (r *LooperRunner) TryStartResume(maxRounds, resumeFrom int, cancel context.CancelFunc) error {
	return r.holder.TryStartResume(maxRounds, resumeFrom, cancel)
}

// looperEvalFailureHint replaces missing reviewer feedback so the next work
// round still knows it has to judge the result itself.
const looperEvalFailureHint = "The previous review returned no usable score. Judge the result against the goal yourself and keep improving it."

type looperEvaluation struct {
	Score    int
	Done     bool
	Feedback string
	Summary  string
}

func (r *LooperRunner) executeStarted(
	ctx context.Context,
	cfg desktop.LooperRunConfig,
	auraCfg *config.Config,
	client llm.ChatClient,
	tools []openai.Tool,
	dispatchCtx *agent.DispatchContext,
	resumeSeed *desktop.LooperResumeState,
) error {
	desktop.NormalizeLooperRunConfig(&cfg)
	startedAt := time.Now().UTC()
	if st := r.holder.State(); !st.StartedAt.IsZero() {
		startedAt = st.StartedAt
	}
	r.rememberRun(cfg, startedAt)
	r.holder.SetRunInfo(cfg.TargetScore, strings.TrimSpace(cfg.Finish) != "", cfg.PresetName, cfg.Goal)
	defer func() {
		r.persistFinishedRun(cfg, startedAt)
		r.holder.SetIdle()
	}()

	model := cfg.Model
	if model == "" && auraCfg != nil {
		model = auraCfg.LLM.Model
	}

	sysPrompt := looperSystemPrompt(auraCfg)
	noTools := []openai.Tool{}
	optsWithTools := &agent.MinimalLoopOptions{MaxToolRounds: 10}
	optsNoTools := &agent.MinimalLoopOptions{MaxToolRounds: 0}

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

			r.logger.Info("[Looper] step start", "step", stepName, "round", r.holder.State().Round, "tools", len(stepTools), "attempt", attempt)
			res, h, err := agent.ExecuteMinimalLoop(stepCtx, client, model, system, prompt, stepTools, dispatchCtx, history, r.logger, opts)
			stepCancel()

			if err != nil {
				r.logger.Warn("[Looper] step error", "step", stepName, "attempt", attempt, "maxRetries", maxRetries, "error", err)
				if looperShouldKeepMinimalLoopResult(stepName, err, res, h, prompt) {
					r.recordLooperUsage(res, model, dispatchCtx)
					r.logger.Warn("[Looper] recovered format error", "step", stepName, "tool_calls", res.ToolCalls)
					return res, h, err
				}
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
				return res, h, err
			}
			r.recordLooperUsage(res, model, dispatchCtx)
			r.logger.Info("[Looper] step done", "step", stepName, "duration_ms", res.Duration.Milliseconds(), "tool_calls", res.ToolCalls)
			return res, h, nil
		}
		return agent.MinimalLoopResult{}, nil, fmt.Errorf("unreachable")
	}

	var lastWorkResult string
	var lastWorkSummary string
	var lastFeedback string
	scoreHistory := make([]int, 0, cfg.MaxRounds)
	bestScore := 0
	startRound := 1
	if resumeSeed != nil {
		startRound = resumeSeed.Round + 1
		lastFeedback = resumeSeed.LastFeedback
		lastWorkSummary = resumeSeed.LastWorkSummary
		bestScore = resumeSeed.BestScore
		if len(resumeSeed.ScoreHistory) > 0 {
			scoreHistory = append(scoreHistory, resumeSeed.ScoreHistory...)
		}
		r.logger.Info("[Looper] resuming from saved state", "from_round", resumeSeed.Round, "starting_at", startRound)
		r.holder.ClearResumeState()
	}

	// snapshot is the state a resume continues from: everything up to and
	// including the given finished round.
	snapshot := func(round int) desktop.LooperResumeState {
		return desktop.LooperResumeState{
			Round:           round,
			BestScore:       bestScore,
			ScoreHistory:    append([]int(nil), scoreHistory...),
			LastFeedback:    lastFeedback,
			LastWorkSummary: lastWorkSummary,
		}
	}
	// pauseAfter parks the run until Resume and keeps it across restarts.
	pauseAfter := func(round int, reason string) {
		rs := snapshot(round)
		r.holder.SaveResumeState(rs)
		if reason != "" {
			r.holder.SetPauseReason(reason)
		}
		r.checkpoint(cfg, startedAt, rs)
	}
	r.checkpoint(cfg, startedAt, snapshot(startRound-1))

	terminal := ""
	evalFailures := 0
	for i := startRound; i <= cfg.MaxRounds; i++ {
		select {
		case <-ctx.Done():
			return r.setErrorAndReturn(fmt.Errorf("aborted by user"))
		default:
		}

		// An exhausted daily budget parks the run at the round boundary; it can
		// be resumed once the budget resets or the limit is raised.
		if looperBudgetBlocked(dispatchCtx) {
			pauseAfter(i-1, "budget")
			r.logger.Warn("[Looper] daily budget used up; run paused before starting round", "round", i)
			return nil
		}

		if r.holder.IsPauseRequested() {
			pauseAfter(i-1, "")
			r.logger.Info("[Looper] run paused before starting round", "round", i)
			return nil
		}

		r.holder.SetRound(i)

		workPrompt := buildLooperWorkPrompt(cfg, i, lastFeedback, lastWorkSummary, scoreHistory)
		r.holder.SetStep("work")
		workRes, workHistory, err := stepExec("work", workPrompt, sysPrompt, tools, optsWithTools, nil)
		lastWorkResult = buildActionFinishResult(workRes.Response, workHistory, workPrompt)
		if err != nil {
			if recovered, ok := looperRecoveredWorkReport(err, workHistory, workPrompt); ok {
				lastWorkResult = recovered
				r.logger.Warn("[Looper] work recovered from format error; continuing to evaluate", "round", i, "error", err)
			} else {
				return r.setErrorAndReturn(err)
			}
		}
		lastWorkSummary = truncateResponse(lastWorkResult, 2500)
		workResponse := strings.TrimSpace(workRes.Response)
		if workResponse == "" {
			workResponse = lastWorkResult
		}
		r.holder.AppendLog(desktop.LooperLogEntry{
			Round:    i,
			Step:     "work",
			Prompt:   workPrompt,
			Response: workResponse,
			Duration: workRes.Duration.Milliseconds(),
		})

		evalPrompt := buildLooperEvaluatePrompt(cfg, lastWorkResult)
		r.holder.SetStep("evaluate")
		evalRes, evalHistory, err := stepExec("evaluate", evalPrompt, sysPrompt, noTools, optsNoTools, nil)
		ev := looperEvaluation{}
		evalOK := false
		if evalRes.Response == "" {
			evalRes.Response = agent.LastAssistantPlainText(evalHistory)
		}
		if err != nil {
			r.logger.Warn("[Looper] evaluate failed; this round gets no score", "round", i, "error", err)
		} else {
			ev, evalOK = parseEvaluation(evalRes.Response)
			if !evalOK {
				clarityPrompt := "Your previous answer was not valid JSON. Reply with ONLY this object and no extra text: {\"score\":0-100,\"done\":true/false,\"feedback\":\"...\",\"summary\":\"...\"}"
				clarityRes, _, cerr := stepExec("evaluate_clarify", clarityPrompt, sysPrompt, noTools, optsNoTools, []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
					{Role: openai.ChatMessageRoleUser, Content: evalPrompt},
					{Role: openai.ChatMessageRoleAssistant, Content: evalRes.Response},
				})
				if cerr == nil {
					// The retry answers the same review; it is logged once, as the
					// round's evaluate entry, instead of as a second score-less row.
					ev, evalOK = parseEvaluation(clarityRes.Response)
					evalRes = clarityRes
				}
			}
		}

		// A stop or shutdown during the review is an abort, not a review that
		// failed: nothing is scored, logged or checkpointed for this round.
		if ctx.Err() != nil {
			return r.setErrorAndReturn(fmt.Errorf("aborted by user"))
		}

		evalResponse := strings.TrimSpace(evalRes.Response)
		if evalResponse == "" && err != nil {
			evalResponse = err.Error()
		}
		if !evalOK {
			// No usable review: the round neither scores 0 nor feeds the stall
			// detector. Repeated failures end the run instead of burning budget.
			evalFailures++
			fallback := looperEvaluationOrFallback(evalRes.Response, err)
			r.holder.RecordEvaluationFailure(evalFailures)
			r.holder.AppendLog(desktop.LooperLogEntry{
				Round:    i,
				Step:     "evaluate",
				Prompt:   evalPrompt,
				Response: evalResponse,
				Duration: evalRes.Duration.Milliseconds(),
				Feedback: fallback.Feedback,
				Failed:   true,
			})
			r.logger.Warn("[Looper] review returned no usable score", "round", i, "consecutive", evalFailures)
			if evalFailures >= desktop.LooperMaxEvalFailures {
				return r.setErrorAndReturn(fmt.Errorf("the reviewer returned no usable score in %d rounds in a row", evalFailures))
			}
			if lastFeedback == "" {
				lastFeedback = looperEvalFailureHint
			}
		} else {
			evalFailures = 0
			scoreHistory = append(scoreHistory, ev.Score)
			if ev.Score > bestScore {
				bestScore = ev.Score
			}
			lastFeedback = ev.Feedback
			r.holder.RecordEvaluation(ev.Score, ev.Feedback, ev.Summary)
			r.holder.AppendLog(desktop.LooperLogEntry{
				Round:    i,
				Step:     "evaluate",
				Prompt:   evalPrompt,
				Response: evalResponse,
				Duration: evalRes.Duration.Milliseconds(),
				Score:    ev.Score,
				Done:     ev.Done,
				Feedback: ev.Feedback,
			})

			if looperReachedTarget(ev, cfg.TargetScore) {
				terminal = "completed"
				r.holder.SetStatus(terminal)
				break
			}
			if desktop.StallWithoutImprovement(scoreHistory, cfg.StallRounds) {
				terminal = "stalled"
				r.holder.SetStatus(terminal)
				r.logger.Warn("[Looper] stalled", "round", i, "stall_rounds", cfg.StallRounds)
				break
			}
		}

		// A pause asked for during the final round has nothing left to pause.
		if r.holder.IsPauseRequested() && i < cfg.MaxRounds {
			pauseAfter(i, "")
			r.logger.Info("[Looper] run paused by user request", "round", i)
			return nil
		}
		r.checkpoint(cfg, startedAt, snapshot(i))
	}

	if terminal == "" {
		terminal = "max_rounds"
		r.holder.SetStatus(terminal)
	}

	if strings.TrimSpace(cfg.Finish) != "" {
		r.holder.SetStep("finish")
		finishHistory := buildLooperFinishHistory(sysPrompt, cfg.Goal, lastWorkResult, lastFeedback, r.holder.State().LastSummary)
		finishRes, finishOut, err := stepExec("finish", cfg.Finish, "", tools, optsWithTools, finishHistory)
		finishText := strings.TrimSpace(finishRes.Response)
		if finishText == "" {
			finishText = strings.TrimSpace(buildActionFinishResult("", finishOut, cfg.Finish))
		}
		if err != nil {
			r.logger.Warn("[Looper] finish failed; keeping completed loop status", "error", err)
			if finishText == "" {
				finishText = err.Error()
			}
		}
		r.holder.AppendLog(desktop.LooperLogEntry{
			Round:    0,
			Step:     "finish",
			Prompt:   cfg.Finish,
			Response: finishText,
			Duration: finishRes.Duration.Milliseconds(),
		})
		if strings.TrimSpace(finishRes.Response) != "" || err == nil {
			r.holder.SetLastResult(finishRes.Response)
		} else if recovered := strings.TrimSpace(buildActionFinishResult("", finishOut, cfg.Finish)); recovered != "" {
			r.holder.SetLastResult(recovered)
		}
	}

	return nil
}

func (r *LooperRunner) persistFinishedRun(cfg desktop.LooperRunConfig, startedAt time.Time) {
	if r.store == nil {
		return
	}
	if r.shuttingDown.Load() {
		// Cut off by a server stop: the checkpoint resumes this run next start.
		return
	}
	st := r.holder.State()
	if st.Paused || st.Status == "paused" {
		return
	}
	// A run is filed once. Discarding a paused run races with the finished
	// goroutine's own cleanup, and both paths end up here.
	r.lastRunMu.Lock()
	already := st.RunID != 0 && st.RunID == r.persistedRunID
	r.persistedRunID = st.RunID
	r.lastRunMu.Unlock()
	if already {
		return
	}
	status := st.Status
	if status == "" || status == "running" || status == "idle" {
		if st.Stopped {
			status = "stopped"
		} else if st.Error != "" {
			status = "failed"
		} else {
			status = "completed"
		}
	}
	finalScore := 0
	if n := len(st.ScoreHistory); n > 0 {
		finalScore = st.ScoreHistory[n-1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := r.store.SaveRun(ctx, desktop.LooperRunRecord{
		PresetName:   cfg.PresetName,
		GoalExcerpt:  cfg.Goal,
		Status:       status,
		Rounds:       st.Round,
		MaxRounds:    st.MaxRounds,
		BestScore:    st.BestScore,
		FinalScore:   finalScore,
		TargetScore:  cfg.TargetScore,
		InputTokens:  st.InputTokens,
		OutputTokens: st.OutputTokens,
		CostUSD:      st.EstimatedCostUSD,
		Error:        st.Error,
		StartedAt:    startedAt,
		FinishedAt:   time.Now().UTC(),
		Logs:         st.Logs,
		Config:       &cfg,
	}); err != nil && r.logger != nil {
		r.logger.Warn("[Looper] persist run failed", "error", err)
	}
	if err := r.store.ClearActiveRun(ctx); err != nil && r.logger != nil {
		r.logger.Warn("[Looper] clear checkpoint failed", "error", err)
	}
}

func looperSystemPrompt(cfg *config.Config) string {
	base := agent.MinimalSystemPromptBuilder(nil)
	rules := "Looper rules:\n" +
		"- Persist the target artifact in the workspace. Do not only describe it.\n" +
		"- Follow the current step instruction exactly.\n" +
		"- During work and evaluate, do not open files in desktop apps. That happens only in the finish step.\n" +
		"- Never write XML or JSON tool calls as the final answer. Use only native function calling, then summarize in plain prose.\n" +
		"- Be concise and direct."
	if cfg != nil {
		if lang := strings.TrimSpace(cfg.Agent.SystemLanguage); lang != "" {
			rules += "\n- Write user-visible text in " + lang + " unless the goal names another language."
		}
	}
	return base + "\n\n" + rules
}

func buildLooperWorkPrompt(cfg desktop.LooperRunConfig, round int, lastFeedback, lastWorkSummary string, scores []int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Round %d of %d.\n\nGoal:\n%s\n", round, cfg.MaxRounds, strings.TrimSpace(cfg.Goal))
	if round == 1 {
		b.WriteString("\nIf the target artifact does not exist yet, create the first complete version now.\n")
	}
	if lastFeedback != "" {
		fmt.Fprintf(&b, "\nPrevious evaluation")
		if n := len(scores); n > 0 {
			fmt.Fprintf(&b, " (score %d)", scores[n-1])
		}
		fmt.Fprintf(&b, ":\n%s\n", strings.TrimSpace(lastFeedback))
	}
	if len(scores) > 0 {
		parts := make([]string, len(scores))
		for i, score := range scores {
			parts[i] = fmt.Sprintf("%d", score)
		}
		fmt.Fprintf(&b, "\nScore history: %s\n", strings.Join(parts, ", "))
	}
	if lastWorkSummary != "" && round > 1 {
		fmt.Fprintf(&b, "\nPrevious work summary:\n%s\n", truncateResponse(lastWorkSummary, 2500))
	}
	fmt.Fprintf(&b, "\nWork instructions:\n%s\n", strings.TrimSpace(cfg.Work))
	b.WriteString("\nDo not open the artifact in a desktop app in this step.\n")
	return b.String()
}

func looperReachedTarget(ev looperEvaluation, targetScore int) bool {
	return ev.Score >= targetScore
}

func buildLooperEvaluatePrompt(cfg desktop.LooperRunConfig, workResult string) string {
	target := cfg.TargetScore
	if target <= 0 {
		target = desktop.LooperDefaultTargetScore
	}
	return "You are an independent reviewer. Score the artifact from the goal, the criteria, and the work report below (it includes tool output from this round). Do not call tools and do not open desktop apps.\n\n" +
		"Goal:\n" + strings.TrimSpace(cfg.Goal) + "\n\n" +
		"Evaluation criteria:\n" + strings.TrimSpace(cfg.Evaluate) + "\n\n" +
		"Work report from this round:\n" + truncateResponse(workResult, 4000) + "\n\n" +
		fmt.Sprintf("The loop continues until the score is at least %d, the round limit, or a stall. A first complete draft is not automatically done. Set done true only when the score meets that target; done true below the target does not stop the loop.\n\n", target) +
		"Reply with valid JSON only:\n" +
		`{"score":0-100,"done":true/false,"feedback":"concrete next improvements","summary":"one-sentence outcome"}`
}

func looperEvaluationOrFallback(raw string, execErr error) looperEvaluation {
	if ev, ok := parseEvaluation(raw); ok {
		return ev
	}
	if execErr != nil {
		return looperEvaluation{
			Score:    0,
			Feedback: truncateResponse(execErr.Error(), 800),
			Summary:  "Evaluation failed; continuing.",
		}
	}
	return looperEvaluation{
		Score:    0,
		Feedback: truncateResponse(raw, 800),
		Summary:  "Evaluation was not valid JSON; continuing.",
	}
}

func isRecoverableMinimalLoopFormatError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unexpected tool-call text in llm response") ||
		strings.Contains(msg, "unexpected tool calls in a tool-free summary") ||
		strings.Contains(msg, "unexpected tool calls in a tool-free request")
}

func looperRecoveredWorkReport(err error, history []openai.ChatCompletionMessage, prompt string) (string, bool) {
	if !isRecoverableMinimalLoopFormatError(err) {
		return "", false
	}
	report := strings.TrimSpace(buildActionFinishResult("", history, prompt))
	if report == "" {
		return "", false
	}
	return report, true
}

func looperShouldKeepMinimalLoopResult(stepName string, err error, res agent.MinimalLoopResult, history []openai.ChatCompletionMessage, prompt string) bool {
	if !isRecoverableMinimalLoopFormatError(err) {
		return false
	}
	switch stepName {
	case "work", "finish":
		_, ok := looperRecoveredWorkReport(err, history, prompt)
		return ok
	case "evaluate", "evaluate_clarify":
		raw := strings.TrimSpace(res.Response)
		if raw == "" {
			raw = agent.LastAssistantPlainText(history)
		}
		_, ok := parseEvaluation(raw)
		return ok
	default:
		return false
	}
}

func (r *LooperRunner) recordLooperUsage(res agent.MinimalLoopResult, model string, dispatchCtx *agent.DispatchContext) {
	if r == nil || (res.PromptTokens == 0 && res.CompletionTokens == 0) {
		return
	}
	cost, priced := looperUsageCost(dispatchCtx, model, res.PromptTokens, res.CompletionTokens)
	r.holder.AddUsage(res.PromptTokens, res.CompletionTokens, cost)
	if !priced {
		r.holder.MarkCostApproximate()
	}
	if dispatchCtx != nil && dispatchCtx.BudgetTracker != nil {
		dispatchCtx.BudgetTracker.RecordForCategory("looper", model, res.PromptTokens, res.CompletionTokens)
	}
}

func buildLooperFinishHistory(sysPrompt, goal, lastWork, lastFeedback, lastSummary string) []openai.ChatCompletionMessage {
	finishSystem := buildLooperFinishSystemPrompt(sysPrompt)
	history := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: finishSystem},
		{Role: openai.ChatMessageRoleUser, Content: "Goal:\n" + strings.TrimSpace(goal)},
	}
	var parts []string
	if strings.TrimSpace(lastWork) != "" {
		parts = append(parts, "Final work result:\n"+truncateResponse(lastWork, 4000))
	}
	if strings.TrimSpace(lastFeedback) != "" {
		parts = append(parts, "Final evaluation:\n"+truncateResponse(lastFeedback, 2000))
	}
	if strings.TrimSpace(lastSummary) != "" {
		parts = append(parts, "Final summary:\n"+truncateResponse(lastSummary, 1500))
	}
	if len(parts) > 0 {
		history = append(history, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: "Result of the last round:\n\n" + strings.Join(parts, "\n\n"),
		})
	}
	return history
}

func parseEvaluation(raw string) (looperEvaluation, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return looperEvaluation{}, false
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end <= start {
		return looperEvaluation{}, false
	}
	var data struct {
		Score    *float64 `json:"score"`
		Done     *bool    `json:"done"`
		Feedback string   `json:"feedback"`
		Summary  string   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &data); err != nil {
		return looperEvaluation{}, false
	}
	if data.Score == nil {
		return looperEvaluation{}, false
	}
	score := int(*data.Score + 0.5)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	done := false
	if data.Done != nil {
		done = *data.Done
	}
	return looperEvaluation{
		Score:    score,
		Done:     done,
		Feedback: strings.TrimSpace(data.Feedback),
		Summary:  strings.TrimSpace(data.Summary),
	}, true
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

func truncateResponse(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d more chars)", len(s)-maxLen)
}

func (r *LooperRunner) setErrorAndReturn(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "aborted by user") || (strings.Contains(msg, "context canceled") && !strings.Contains(msg, "deadline")) {
		r.holder.SetStopped()
		return err
	}
	r.holder.SetError(msg)
	return err
}

// looperUsageCost prices a call with the budget tracker's model rates, the same
// ones the daily budget is charged with. priced is false when the model has no
// configured price and a fallback rate was used instead.
func looperUsageCost(d *agent.DispatchContext, model string, promptTokens, completionTokens int) (cost float64, priced bool) {
	if d != nil && d.BudgetTracker != nil {
		c, known := d.BudgetTracker.EstimateCost(model, promptTokens, completionTokens)
		if known || c > 0 {
			return c, known
		}
	}
	return estimateLooperCostUSD(promptTokens, completionTokens), false
}

func estimateLooperCostUSD(promptTokens, completionTokens int) float64 {
	const inPerM = 0.50
	const outPerM = 1.50
	return (float64(promptTokens)/1_000_000.0)*inPerM + (float64(completionTokens)/1_000_000.0)*outPerM
}

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
	store.SetDBPath(svc.DBPath())
	if err := store.Init(context.Background()); err != nil {
		return nil, err
	}
	looperRunner = NewLooperRunner(store, s.Logger)
	looperRunner.restoreActive(context.Background())
	return looperRunner, nil
}

func shutdownLooper() {
	looperRunnerMu.Lock()
	defer looperRunnerMu.Unlock()
	if looperRunner != nil {
		looperRunner.Shutdown()
	}
}
