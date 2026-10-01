package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/desktop"
)

// restart builds a second runner on the same database, as a new server process
// would after the first one went away.
func (h *looperLoopHarness) restart(review func(round, attempt int) string) *looperLoopHarness {
	client := &looperScriptClient{review: review}
	next := &looperLoopHarness{
		runner:   NewLooperRunner(h.store, slog.New(slog.NewTextHandler(io.Discard, nil))),
		store:    h.store,
		client:   client,
		auraCfg:  h.auraCfg,
		dispatch: &agent.DispatchContext{Cfg: h.auraCfg, LLMClient: client, SessionID: "looper"},
	}
	next.runner.restoreActive(context.Background())
	return next
}

// resume continues the paused run with the stored settings, as the resume
// endpoint does for an empty request.
func (h *looperLoopHarness) resume(t *testing.T) error {
	t.Helper()
	cfg, ok := h.runner.StoredConfig()
	if !ok {
		t.Fatal("no stored settings to resume with")
	}
	rs, ok := h.runner.ResumeState()
	if !ok {
		t.Fatal("no resume snapshot")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := h.runner.TryStartResume(cfg.MaxRounds, rs.Round, cancel); err != nil {
		t.Fatalf("TryStartResume: %v", err)
	}
	return h.runner.Resume(ctx, cfg, h.auraCfg, h.client, nil, h.dispatch)
}

func TestLooperRunKeepsItsSettingsInTheHistory(t *testing.T) {
	h := newLooperLoopHarness(t, func(round, attempt int) string { return looperReview(95) })
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 4, TargetScore: 90, StallRounds: 2, ProviderID: "main", Finish: ""}); err != nil {
		t.Fatalf("run: %v", err)
	}
	runs, err := h.store.ListRuns(context.Background())
	if err != nil || len(runs) != 1 {
		t.Fatalf("history = %+v err=%v", runs, err)
	}
	stored, err := h.store.GetRun(context.Background(), runs[0].ID)
	if err != nil || stored.Config == nil {
		t.Fatalf("a finished run must keep what it was started with: %+v err=%v", stored.Config, err)
	}
	if stored.Config.Goal != "Write something" || stored.Config.Work != "Improve it" || stored.Config.Evaluate != "Judge it" ||
		stored.Config.MaxRounds != 4 || stored.Config.TargetScore != 90 || stored.Config.ProviderID != "main" || stored.Config.Model != "test-model" {
		t.Fatalf("stored settings = %+v", *stored.Config)
	}
	if active, ok, _ := h.store.LoadActiveRun(context.Background()); ok {
		t.Fatalf("a filed run leaves no checkpoint behind: %+v", active.State.Status)
	}
}

func TestLooperRunSurvivesAServerRestart(t *testing.T) {
	var first *looperLoopHarness
	first = newLooperLoopHarness(t, func(round, attempt int) string {
		if round == 2 {
			first.runner.Shutdown() // the server stops while round 2 is being reviewed
		}
		return looperReview(40 + 10*round)
	})
	err := first.run(t, desktop.LooperRunConfig{MaxRounds: 4, TargetScore: 95, StallRounds: 0})
	if err == nil {
		t.Fatal("a run cut off by shutdown reports the abort")
	}
	if runs, _ := first.store.ListRuns(context.Background()); len(runs) != 0 {
		t.Fatalf("a shutdown is not a user stop and must not be filed as one: %+v", runs)
	}
	checkpoint, ok, loadErr := first.store.LoadActiveRun(context.Background())
	if loadErr != nil || !ok {
		t.Fatalf("the run must keep its checkpoint: ok=%v err=%v", ok, loadErr)
	}
	if checkpoint.Resume.Round != 1 || checkpoint.Config.Goal != "Write something" {
		t.Fatalf("checkpoint = round %d goal %q, want the last finished round and the settings", checkpoint.Resume.Round, checkpoint.Config.Goal)
	}

	second := first.restart(func(round, attempt int) string { return looperReview(96) })
	state := second.runner.State()
	if !state.Paused || state.Running || state.PauseReason != "interrupted" || state.ResumeFrom != 1 || state.Round != 1 {
		t.Fatalf("restored state = %+v, want an interrupted run waiting after round 1", state)
	}
	if state.PresetName != "Scripted" || state.TargetScore != 95 || len(state.Logs) != 2 {
		t.Fatalf("restored run lost its description or logs: preset %q target %d logs %d", state.PresetName, state.TargetScore, len(state.Logs))
	}
	cfg, ok := second.runner.StoredConfig()
	if !ok || cfg.Goal != "Write something" || cfg.TargetScore != 95 {
		t.Fatalf("stored settings = %+v ok=%v", cfg, ok)
	}

	if err := second.resume(t); err != nil {
		t.Fatalf("resume: %v", err)
	}
	done := second.runner.State()
	if done.Status != "completed" || done.Running || done.Paused || done.PauseReason != "" {
		t.Fatalf("resumed run = %+v, want completed", done)
	}
	if len(done.ScoreHistory) != 2 || done.ScoreHistory[0] != 50 || done.ScoreHistory[1] != 96 {
		t.Fatalf("scores = %v, want the pre-restart round followed by the repeated round", done.ScoreHistory)
	}
	runs, _ := second.store.ListRuns(context.Background())
	if len(runs) != 1 || runs[0].Status != "completed" || runs[0].Rounds != 2 {
		t.Fatalf("history after the resume = %+v", runs)
	}
	if _, ok, _ := second.store.LoadActiveRun(context.Background()); ok {
		t.Fatal("a finished run must clear its checkpoint")
	}
}

func TestLooperDeliberatePauseSurvivesARestart(t *testing.T) {
	var first *looperLoopHarness
	first = newLooperLoopHarness(t, func(round, attempt int) string {
		if round == 1 {
			first.runner.Pause()
		}
		return looperReview(55)
	})
	if err := first.run(t, desktop.LooperRunConfig{MaxRounds: 5, TargetScore: 99, StallRounds: 0}); err != nil {
		t.Fatalf("run: %v", err)
	}
	second := first.restart(func(round, attempt int) string { return looperReview(55) })
	state := second.runner.State()
	if !state.Paused || state.PauseReason != "" {
		t.Fatalf("a pause the user asked for stays a plain pause after a restart: %+v", state)
	}
	if !second.runner.DiscardPaused() {
		t.Fatal("a restored run can be ended")
	}
	runs, _ := second.store.ListRuns(context.Background())
	if len(runs) != 1 || runs[0].Status != "stopped" || runs[0].Rounds != 1 {
		t.Fatalf("history = %+v", runs)
	}
	if _, ok, _ := second.store.LoadActiveRun(context.Background()); ok {
		t.Fatal("ending the run clears the checkpoint")
	}
}

func TestLooperRunPausesWhenTheDailyBudgetIsUsedUp(t *testing.T) {
	h := newLooperLoopHarness(t, func(round, attempt int) string { return looperReview(60) })
	cfg := h.auraCfg
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 0.003
	cfg.Budget.Enforcement = "full"
	cfg.Budget.WarningThreshold = 0.9
	cfg.Budget.Models = []config.ModelCost{{Name: "test-model", InputPerMillion: 10, OutputPerMillion: 30}}
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	if tracker == nil {
		t.Fatal("budget tracker expected")
	}
	h.dispatch.BudgetTracker = tracker

	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 5, TargetScore: 99, StallRounds: 0}); err != nil {
		t.Fatalf("run: %v", err)
	}
	state := h.runner.State()
	if !state.Paused || state.PauseReason != "budget" || state.Round != 1 || state.ResumeFrom != 1 {
		t.Fatalf("state = %+v, want a budget pause after round 1", state)
	}
	// Two calls at 100 in / 20 out on 10/30 USD per million: 2 x 0.0016.
	if state.EstimatedCostUSD < 0.0031 || state.EstimatedCostUSD > 0.0033 || state.CostApproximate {
		t.Fatalf("cost %.5f approximate=%v, want the model's own price", state.EstimatedCostUSD, state.CostApproximate)
	}
	if runs, _ := h.store.ListRuns(context.Background()); len(runs) != 0 {
		t.Fatalf("a budget pause is not a finished run: %+v", runs)
	}
	if _, ok, _ := h.store.LoadActiveRun(context.Background()); !ok {
		t.Fatal("a budget pause keeps its checkpoint so it can be resumed later")
	}
	if !looperBudgetBlocked(h.dispatch) {
		t.Fatal("the helper used by the HTTP handlers must report the same block")
	}
}

func TestLooperCostFallsBackToAFlagGuessWithoutAModelPrice(t *testing.T) {
	h := newLooperLoopHarness(t, func(round, attempt int) string { return looperReview(95) })
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 2, TargetScore: 90}); err != nil {
		t.Fatalf("run: %v", err)
	}
	state := h.runner.State()
	if state.EstimatedCostUSD <= 0 || !state.CostApproximate {
		t.Fatalf("without a tracker the cost is a flagged estimate: %.6f approximate=%v", state.EstimatedCostUSD, state.CostApproximate)
	}
}

func TestLooperErrorBodiesCarryAMachineReadableCode(t *testing.T) {
	rec := httptest.NewRecorder()
	looperError(rec, http.StatusPaymentRequired, "budget_exceeded", looperBudgetMessage)
	body := rec.Body.String()
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(body, `"code":"budget_exceeded"`) || !strings.Contains(body, `"error":"The daily budget is used up."`) {
		t.Fatalf("status %d body %s", rec.Code, body)
	}
}
