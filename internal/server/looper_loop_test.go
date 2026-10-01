package server

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
	_ "modernc.org/sqlite"
)

// looperScriptClient plays a scripted work/review conversation. review gets
// the 1-based round and the attempt (0 first answer, 1 the JSON-only retry).
type looperScriptClient struct {
	mu     sync.Mutex
	work   int
	review func(round, attempt int) string
}

func (c *looperScriptClient) CandidateRoutes(openai.ChatCompletionRequest) []llm.ModelRoute {
	return []llm.ModelRoute{{ProviderID: "primary", ProviderType: "custom", Model: "test-model", Primary: true, ContextWindowOverride: 32000, MaxOutputTokensOverride: 2048}}
}

func (c *looperScriptClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.mu.Lock()
	last := ""
	if n := len(req.Messages); n > 0 {
		last = req.Messages[n-1].Content
	}
	var text string
	switch {
	case strings.Contains(last, "independent reviewer"):
		text = c.review(c.work, 0)
	case strings.Contains(last, "not valid JSON"):
		text = c.review(c.work, 1)
	default:
		c.work++
		text = "Wrote draft number " + string(rune('0'+c.work))
	}
	c.mu.Unlock()
	// Like a real client, give up when the caller was cancelled during the call.
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{{
			Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: text},
			FinishReason: openai.FinishReasonStop,
		}},
		Usage: openai.Usage{PromptTokens: 100, CompletionTokens: 20},
	}, nil
}

func (c *looperScriptClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, errors.New("streaming not implemented")
}

const looperGarbageReview = "I cannot put a number on this."

func looperReview(score int) string {
	raw, _ := json.Marshal(map[string]any{"score": score, "done": false, "feedback": "keep going", "summary": "ok"})
	return string(raw)
}

type looperLoopHarness struct {
	runner   *LooperRunner
	store    *desktop.LooperPresetStore
	client   *looperScriptClient
	auraCfg  *config.Config
	dispatch *agent.DispatchContext
}

func newLooperLoopHarness(t *testing.T, review func(round, attempt int) string) *looperLoopHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	store := desktop.NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init store: %v", err)
	}
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 32000
	client := &looperScriptClient{review: review}
	return &looperLoopHarness{
		runner:   NewLooperRunner(store, slog.New(slog.NewTextHandler(io.Discard, nil))),
		store:    store,
		client:   client,
		auraCfg:  cfg,
		dispatch: &agent.DispatchContext{Cfg: cfg, LLMClient: client, SessionID: "looper"},
	}
}

func (h *looperLoopHarness) run(t *testing.T, cfg desktop.LooperRunConfig) error {
	t.Helper()
	cfg.Model = "test-model"
	cfg.PresetName = "Scripted"
	cfg.Goal = "Write something"
	cfg.Work = "Improve it"
	cfg.Evaluate = "Judge it"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	desktop.NormalizeLooperRunConfig(&cfg)
	if err := h.runner.TryStart(cfg.MaxRounds, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	return h.runner.executeStarted(ctx, cfg, h.auraCfg, h.client, nil, h.dispatch, nil)
}

func failedEvaluations(logs []desktop.LooperLogEntry) int {
	n := 0
	for _, entry := range logs {
		if entry.Step == "evaluate" && entry.Failed {
			n++
		}
	}
	return n
}

func TestLooperLoopUnusableReviewIsNotAZeroScore(t *testing.T) {
	h := newLooperLoopHarness(t, func(round, attempt int) string {
		switch round {
		case 1:
			return looperGarbageReview
		case 2:
			return looperReview(70)
		}
		return looperReview(95)
	})
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 6, TargetScore: 90, StallRounds: 2}); err != nil {
		t.Fatalf("run: %v", err)
	}

	state := h.runner.State()
	if state.Status != "completed" || state.Running {
		t.Fatalf("status = %q running=%v, want completed", state.Status, state.Running)
	}
	if len(state.ScoreHistory) != 2 || state.ScoreHistory[0] != 70 || state.ScoreHistory[1] != 95 {
		t.Fatalf("score history = %v, want [70 95] without a fake zero for the broken review", state.ScoreHistory)
	}
	if state.BestScore != 95 || state.EvalFailures != 0 {
		t.Fatalf("best=%d failures=%d, want 95 and a reset streak", state.BestScore, state.EvalFailures)
	}
	if got := failedEvaluations(state.Logs); got != 1 {
		t.Fatalf("failed evaluate entries = %d, want 1", got)
	}
	if state.LogTotal != 6 || len(state.Logs) != 6 {
		t.Fatalf("log total %d len %d, want one work and one evaluate entry per round (6)", state.LogTotal, len(state.Logs))
	}
	if state.TargetScore != 90 || state.HasFinish {
		t.Fatalf("run info target=%d finish=%v", state.TargetScore, state.HasFinish)
	}
	runs, err := h.store.ListRuns(context.Background())
	if err != nil || len(runs) != 1 || runs[0].Status != "completed" || runs[0].FinalScore != 95 {
		t.Fatalf("history = %+v err=%v", runs, err)
	}
}

func TestLooperLoopEndsAfterRepeatedUnusableReviews(t *testing.T) {
	h := newLooperLoopHarness(t, func(round, attempt int) string { return looperGarbageReview })
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 8, TargetScore: 90, StallRounds: 3}); err == nil {
		t.Fatal("a run ending in repeated unusable reviews must report the failure")
	}

	state := h.runner.State()
	if state.Status != "failed" || state.Running {
		t.Fatalf("status = %q running=%v, want failed", state.Status, state.Running)
	}
	if !strings.Contains(state.Error, "no usable score") {
		t.Fatalf("error = %q, want the unusable-review explanation", state.Error)
	}
	if state.Round != desktop.LooperMaxEvalFailures {
		t.Fatalf("round = %d, want the loop to stop after %d broken reviews", state.Round, desktop.LooperMaxEvalFailures)
	}
	if len(state.ScoreHistory) != 0 || failedEvaluations(state.Logs) != desktop.LooperMaxEvalFailures {
		t.Fatalf("scores=%v failed=%d", state.ScoreHistory, failedEvaluations(state.Logs))
	}
	runs, _ := h.store.ListRuns(context.Background())
	if len(runs) != 1 || runs[0].Status != "failed" {
		t.Fatalf("history = %+v", runs)
	}
}

func TestLooperLoopNeverPausesInTheFinalRound(t *testing.T) {
	var h *looperLoopHarness
	h = newLooperLoopHarness(t, func(round, attempt int) string {
		if round == 2 {
			h.runner.Pause() // asked for during the last round
		}
		return looperReview(40 + 10*round)
	})
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 2, TargetScore: 99, StallRounds: 0}); err != nil {
		t.Fatalf("run: %v", err)
	}

	state := h.runner.State()
	if state.Status != "max_rounds" || state.Paused || state.Running || state.PauseRequested {
		t.Fatalf("state = %+v, want max_rounds without a dangling pause", state)
	}
	if _, ok := h.runner.ResumeState(); ok {
		t.Fatal("a pause in the final round must not leave a resume snapshot")
	}
	runs, _ := h.store.ListRuns(context.Background())
	if len(runs) != 1 || runs[0].Status != "max_rounds" {
		t.Fatalf("history = %+v", runs)
	}
}

func TestLooperPausedRunCanBeDiscardedIntoTheHistory(t *testing.T) {
	var h *looperLoopHarness
	h = newLooperLoopHarness(t, func(round, attempt int) string {
		if round == 1 {
			h.runner.Pause()
		}
		return looperReview(55)
	})
	if err := h.run(t, desktop.LooperRunConfig{MaxRounds: 5, TargetScore: 99, StallRounds: 0}); err != nil {
		t.Fatalf("run: %v", err)
	}

	paused := h.runner.State()
	if !paused.Paused || paused.Running || paused.PauseRequested || paused.Status != "paused" || paused.Round != 1 {
		t.Fatalf("state = %+v, want a run paused after round 1", paused)
	}
	if runs, _ := h.store.ListRuns(context.Background()); len(runs) != 0 {
		t.Fatalf("a paused run is not finished yet, history = %+v", runs)
	}

	if !h.runner.DiscardPaused() {
		t.Fatal("a paused run must be discardable")
	}
	state := h.runner.State()
	if state.Paused || state.Status != "stopped" || state.ResumeSnapshot != nil {
		t.Fatalf("after discard = %+v", state)
	}
	runs, err := h.store.ListRuns(context.Background())
	if err != nil || len(runs) != 1 {
		t.Fatalf("history = %+v err=%v", runs, err)
	}
	if runs[0].Status != "stopped" || runs[0].Rounds != 1 || runs[0].PresetName != "Scripted" || runs[0].BestScore != 55 {
		t.Fatalf("history entry = %+v", runs[0])
	}
	if h.runner.DiscardPaused() {
		t.Fatal("discarding twice must not file the run twice")
	}
	if again, _ := h.store.ListRuns(context.Background()); len(again) != 1 {
		t.Fatalf("history after second discard = %+v", again)
	}
	// The finished goroutine's own cleanup runs the same persistence; it must
	// not file the discarded run a second time.
	h.runner.persistFinishedRun(h.runner.lastCfg, h.runner.lastStart)
	if again, _ := h.store.ListRuns(context.Background()); len(again) != 1 {
		t.Fatalf("history after the runner's cleanup = %+v", again)
	}
}

type sseReader struct {
	lines chan string
}

func openLooperStream(t *testing.T, runner *LooperRunner, linger time.Duration) (*sseReader, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		streamLooperStatus(r.Context(), w, flusher, runner, linger)
	}))
	resp, err := http.Get(server.URL)
	if err != nil {
		server.Close()
		t.Fatalf("open stream: %v", err)
	}
	reader := &sseReader{lines: make(chan string, 64)}
	go func() {
		defer close(reader.lines)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1<<20), 8<<20)
		for scanner.Scan() {
			if line := scanner.Text(); strings.HasPrefix(line, "data: ") {
				reader.lines <- strings.TrimPrefix(line, "data: ")
			}
		}
	}()
	return reader, func() { resp.Body.Close(); server.Close() }
}

func (r *sseReader) next(t *testing.T, timeout time.Duration) (desktop.LooperRunState, bool) {
	t.Helper()
	select {
	case line, ok := <-r.lines:
		if !ok {
			return desktop.LooperRunState{}, false
		}
		var state desktop.LooperRunState
		if err := json.Unmarshal([]byte(line), &state); err != nil {
			t.Fatalf("bad status payload %q: %v", line, err)
		}
		return state, true
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a status message")
	}
	return desktop.LooperRunState{}, false
}

func TestLooperStatusStreamSendsOnlyNewLogEntries(t *testing.T) {
	runner := NewLooperRunner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.TryStart(5, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 1, Step: "work", Response: "first"})
	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 1, Step: "evaluate", Score: 60})

	stream, closeStream := openLooperStream(t, runner, time.Minute)
	defer closeStream()

	first, ok := stream.next(t, 2*time.Second)
	if !ok || first.RunID == 0 || first.LogsFrom != 0 || first.LogTotal != 2 || len(first.Logs) != 2 {
		t.Fatalf("first message = %+v, want the full log window", first)
	}

	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 2, Step: "work", Response: "second"})
	delta, ok := stream.next(t, 2*time.Second)
	if !ok || delta.LogsFrom != 2 || delta.LogTotal != 3 || len(delta.Logs) != 1 || delta.Logs[0].Response != "second" {
		t.Fatalf("delta = %+v, want only the new entry at absolute index 2", delta)
	}
	if delta.Rev <= first.Rev {
		t.Fatalf("revision did not advance: %d -> %d", first.Rev, delta.Rev)
	}

	// Nothing changed: the stream stays silent (no 500 ms polling payloads).
	select {
	case line := <-stream.lines:
		t.Fatalf("unexpected message without a state change: %.120s", line)
	case <-time.After(300 * time.Millisecond):
	}

	// A burst beyond the retention window: the client gets the retained tail
	// and can see from logs_from that it missed entries.
	for i := 0; i < desktop.LooperMaxLogEntries+50; i++ {
		runner.holder.AppendLog(desktop.LooperLogEntry{Round: 3, Step: "work"})
	}
	wantTotal := 3 + desktop.LooperMaxLogEntries + 50
	var last desktop.LooperRunState
	for last.LogTotal != wantTotal {
		last, ok = stream.next(t, 2*time.Second)
		if !ok {
			t.Fatal("stream closed before the burst arrived")
		}
		if last.LogsFrom+len(last.Logs) != last.LogTotal {
			t.Fatalf("inconsistent window: from %d len %d total %d", last.LogsFrom, len(last.Logs), last.LogTotal)
		}
	}
	if last.LogsFrom < wantTotal-desktop.LooperMaxLogEntries {
		t.Fatalf("logs_from %d points below the retained window start %d", last.LogsFrom, wantTotal-desktop.LooperMaxLogEntries)
	}
}

func TestLooperStatusStreamResendsLogsForANewRun(t *testing.T) {
	runner := NewLooperRunner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := runner.TryStart(3, cancel); err != nil {
		t.Fatalf("TryStart: %v", err)
	}
	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 1, Step: "work"})
	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 1, Step: "evaluate", Score: 50})

	stream, closeStream := openLooperStream(t, runner, time.Minute)
	defer closeStream()
	first, _ := stream.next(t, 2*time.Second)
	runner.holder.SetIdle()
	for { // drain until the idle state arrives
		state, ok := stream.next(t, 2*time.Second)
		if !ok {
			t.Fatal("stream closed early")
		}
		if !state.Running {
			break
		}
	}

	_, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := runner.TryStart(3, cancel2); err != nil {
		t.Fatalf("second TryStart: %v", err)
	}
	runner.holder.AppendLog(desktop.LooperLogEntry{Round: 1, Step: "work", Response: "new run"})
	for {
		state, ok := stream.next(t, 2*time.Second)
		if !ok {
			t.Fatal("stream closed before the new run arrived")
		}
		if state.RunID == first.RunID {
			continue
		}
		if state.LogsFrom != 0 {
			t.Fatalf("a new run must restart the log window, got logs_from %d", state.LogsFrom)
		}
		if len(state.Logs) == 1 && state.Logs[0].Response == "new run" {
			return
		}
	}
}

func TestLooperStatusStreamClosesAfterRestingState(t *testing.T) {
	runner := NewLooperRunner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	stream, closeStream := openLooperStream(t, runner, 80*time.Millisecond)
	defer closeStream()

	idle, ok := stream.next(t, 2*time.Second)
	if !ok || idle.Status != "idle" || idle.Logs == nil {
		t.Fatalf("idle state = %+v (logs must be a JSON array, not null)", idle)
	}
	select {
	case _, open := <-stream.lines:
		if open {
			t.Fatal("no further message expected for an idle runner")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an idle stream must close after the linger window")
	}
}
