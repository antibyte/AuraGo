package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/desktop"
)

// freshLooperRunner gives a test its own process-wide runner on the test
// server's database, and removes it again afterwards.
func freshLooperRunner(t *testing.T, s *Server) *LooperRunner {
	t.Helper()
	looperRunnerMu.Lock()
	looperRunner = nil
	looperRunnerMu.Unlock()
	t.Cleanup(func() {
		looperRunnerMu.Lock()
		looperRunner = nil
		looperRunnerMu.Unlock()
	})
	runner, err := getLooperRunner(s)
	if err != nil {
		t.Fatalf("getLooperRunner: %v", err)
	}
	return runner
}

func restoreInterruptedRun(runner *LooperRunner) desktop.LooperRunConfig {
	cfg := desktop.LooperRunConfig{
		Goal: "Stored goal", Work: "Stored work", Evaluate: "Stored review",
		MaxRounds: 3, TargetScore: 95, StallRounds: 0, PresetName: "Stored loop",
	}
	runner.holder.Restore(desktop.LooperRunState{
		Round: 1, MaxRounds: 3, TargetScore: 95, PresetName: "Stored loop", BestScore: 50, ScoreHistory: []int{50},
		Logs: []desktop.LooperLogEntry{{Round: 1, Step: "work"}, {Round: 1, Step: "evaluate", Score: 50}},
	}, desktop.LooperResumeState{Round: 1, BestScore: 50, ScoreHistory: []int{50}, LastFeedback: "keep going"}, "interrupted")
	runner.rememberRun(cfg, time.Now().UTC())
	return cfg
}

func looperRequest(method, path, body string) *http.Request {
	return httptest.NewRequest(method, path, strings.NewReader(body))
}

func TestLooperActiveEndpointReportsTheStoredSettings(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	runner := freshLooperRunner(t, s)

	rec := httptest.NewRecorder()
	handleLooperActive(s).ServeHTTP(rec, looperRequest(http.MethodGet, "/api/desktop/looper/active", ""))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"no_active_run"`) {
		t.Fatalf("idle: status %d body %s", rec.Code, rec.Body.String())
	}

	want := restoreInterruptedRun(runner)
	rec = httptest.NewRecorder()
	handleLooperActive(s).ServeHTTP(rec, looperRequest(http.MethodGet, "/api/desktop/looper/active", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("restored run: status %d body %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Config desktop.LooperRunConfig `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Config != want {
		t.Fatalf("config = %+v err=%v, want %+v", body.Config, err, want)
	}

	rec = httptest.NewRecorder()
	handleLooperActive(s).ServeHTTP(rec, looperRequest(http.MethodPost, "/api/desktop/looper/active", ""))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("only GET is allowed, got %d", rec.Code)
	}
}

func TestLooperResumeEndpointContinuesWithTheStoredSettings(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.LLM.Model = "test-model"
	s.Cfg.Agent.ContextWindow = 32000
	s.LLMClient = &looperScriptClient{review: func(round, attempt int) string { return looperReview(96) }}
	runner := freshLooperRunner(t, s)
	restoreInterruptedRun(runner)

	rec := httptest.NewRecorder()
	handleLooperResume(s).ServeHTTP(rec, looperRequest(http.MethodPost, "/api/desktop/looper/resume", `{}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "resuming") {
		t.Fatalf("resume with an empty editor: status %d body %s", rec.Code, rec.Body.String())
	}
	deadline := time.Now().Add(15 * time.Second)
	for runner.State().Running {
		if time.Now().After(deadline) {
			t.Fatal("the resumed run did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
	state := runner.State()
	if state.Status != "completed" || state.PauseReason != "" || state.Round != 2 {
		t.Fatalf("state = %+v, want a run completed in the round after the checkpoint", state)
	}
	if len(state.ScoreHistory) != 2 || state.ScoreHistory[0] != 50 || state.ScoreHistory[1] != 96 {
		t.Fatalf("scores = %v, want the restored score followed by the new one", state.ScoreHistory)
	}
	runs, err := runner.store.ListRuns(context.Background())
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" || runs[0].PresetName != "Stored loop" {
		t.Fatalf("history = %+v err=%v", runs, err)
	}
}

func TestLooperEndpointsRefuseWhenTheBudgetIsUsedUp(t *testing.T) {
	s := newDesktopFilesystemTestServer(t)
	s.Cfg.LLM.Model = "test-model"
	s.LLMClient = &looperScriptClient{review: func(round, attempt int) string { return looperReview(96) }}
	bcfg := &config.Config{}
	bcfg.Budget.Enabled = true
	bcfg.Budget.DailyLimitUSD = 1
	bcfg.Budget.Enforcement = "full"
	bcfg.Budget.WarningThreshold = 0.8
	tracker := budget.NewTracker(bcfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	tracker.RecordCost(5)
	s.BudgetTracker = tracker
	runner := freshLooperRunner(t, s)

	rec := httptest.NewRecorder()
	handleLooperRun(s).ServeHTTP(rec, looperRequest(http.MethodPost, "/api/desktop/looper/run",
		`{"goal":"g","work":"w","evaluate":"e","max_rounds":2,"target_score":80}`))
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), `"code":"budget_exceeded"`) {
		t.Fatalf("start: status %d body %s", rec.Code, rec.Body.String())
	}
	if runner.State().Running {
		t.Fatal("a refused start must not leave a run behind")
	}

	restoreInterruptedRun(runner)
	rec = httptest.NewRecorder()
	handleLooperResume(s).ServeHTTP(rec, looperRequest(http.MethodPost, "/api/desktop/looper/resume", `{}`))
	if rec.Code != http.StatusPaymentRequired || !strings.Contains(rec.Body.String(), `"code":"budget_exceeded"`) {
		t.Fatalf("resume: status %d body %s", rec.Code, rec.Body.String())
	}
	if state := runner.State(); !state.Paused || state.Running {
		t.Fatalf("a refused resume leaves the run paused: %+v", state)
	}
}
