package desktop

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLooperRunStateHolderTryStartAllowsOneConcurrentRun(t *testing.T) {
	t.Parallel()

	holder := NewLooperRunStateHolder()
	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes int32

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, cancel := context.WithCancel(context.Background())
			if err := holder.TryStart(3, cancel); err != nil {
				cancel()
				return
			}
			atomic.AddInt32(&successes, 1)
		}()
	}

	close(start)
	wg.Wait()
	if successes != 1 {
		t.Fatalf("successful TryStart calls = %d, want 1", successes)
	}
	if state := holder.State(); !state.Running || state.MaxIterations != 3 {
		t.Fatalf("state after TryStart = %+v, want running with max iterations", state)
	}
	holder.SetIdle()
	if state := holder.State(); state.Running {
		t.Fatalf("state after SetIdle = %+v, want idle", state)
	}
}

func TestLooperRunStateHolderPauseAndResumeSnapshot(t *testing.T) {
	t.Parallel()

	holder := NewLooperRunStateHolder()

	// Fresh holder has no resume state
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("expected no resume state initially")
	}

	// Simulate a run that gets paused at iteration 7
	_, cancel := context.WithCancel(context.Background())
	if err := holder.TryStart(20, cancel); err != nil {
		cancel()
		t.Fatalf("TryStart failed: %v", err)
	}

	holder.RequestPause()
	if !holder.IsPauseRequested() {
		t.Fatal("pause request should be visible")
	}

	// Simulate the runner saving state at the checkpoint
	rs := LooperResumeState{
		Iteration:                7,
		LastTestResult:           "iteration 7 test output was excellent",
		PreviousIterationSummary: "previous work was good",
		LastIterationSummary:     "refined the chorus and bridge",
	}
	holder.SaveResumeState(rs)

	// After SaveResumeState the public state must reflect pause + snapshot
	st := holder.State()
	if !st.Paused || st.ResumeFrom != 7 || st.ResumeSnapshot == nil {
		t.Fatalf("state after pause not correct: %+v", st)
	}
	if st.Running {
		t.Fatal("running must be false while paused")
	}
	if st.CurrentStep != "paused" {
		t.Fatalf("current step should be 'paused', got %q", st.CurrentStep)
	}

	// ResumeState API returns the data
	got, ok := holder.GetResumeState()
	if !ok {
		t.Fatal("expected resume state to exist")
	}
	if got.Iteration != 7 || got.LastIterationSummary != "refined the chorus and bridge" {
		t.Fatalf("resume snapshot mismatch: %+v", got)
	}

	// SetIdle must NOT destroy the resume snapshot (critical for defer safety)
	holder.SetIdle()
	st2 := holder.State()
	if !st2.Paused || st2.ResumeFrom != 7 {
		t.Fatal("SetIdle must preserve pause/resume info")
	}

	// Clear must work
	holder.ClearResumeState()
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("resume state should be gone after ClearResumeState")
	}
	st3 := holder.State()
	if st3.Paused || st3.ResumeFrom != 0 {
		t.Fatalf("state after clear must be clean: %+v", st3)
	}
}

func TestLooperPresetStorePersistsSummarizeIterations(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS desktop_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create desktop_meta: %v", err)
	}

	store := NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Ralph Loop builtin must persist summarize_iterations=true
	var summarize int
	if err := db.QueryRow(`SELECT summarize_iterations FROM desktop_looper_presets WHERE name='Ralph Loop' AND is_builtin=1`).Scan(&summarize); err != nil {
		t.Fatalf("read summarize flag: %v", err)
	}
	if summarize != 1 {
		t.Fatalf("Ralph Loop summarize_iterations = %d, want 1", summarize)
	}

	presets, err := store.ListPresets(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var found bool
	for _, p := range presets {
		if p.Name == "Ralph Loop" {
			found = true
			if !p.SummarizeIterations {
				t.Fatal("ListPresets did not surface SummarizeIterations for Ralph Loop")
			}
			if p.PrepareTruncation != 6000 {
				t.Fatalf("PrepareTruncation = %d, want 6000", p.PrepareTruncation)
			}
		}
	}
	if !found {
		t.Fatal("Ralph Loop preset missing")
	}

	// User preset round-trip
	id, err := store.SavePreset(context.Background(), LooperPreset{
		Name:                "User Summarize",
		Prepare:             "p",
		Plan:                "pl",
		Action:              "a",
		Test:                "t",
		ExitCond:            "e",
		SummarizeIterations: true,
		PrepareTruncation:   4000,
		FinishContext:       "last_action_test",
		MaxIter:             5,
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := store.GetPreset(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.SummarizeIterations || got.PrepareTruncation != 4000 || got.FinishContext != "last_action_test" {
		t.Fatalf("user preset mismatch: %+v", got)
	}
}

func TestLooperRunStateHolderStopDistinctFromError(t *testing.T) {
	t.Parallel()
	holder := NewLooperRunStateHolder()
	_, cancel := context.WithCancel(context.Background())
	if err := holder.TryStart(5, cancel); err != nil {
		cancel()
		t.Fatalf("TryStart: %v", err)
	}
	holder.SetStopped()
	holder.SetIdle()
	st := holder.State()
	if !st.Stopped || st.Error != "" || st.CurrentStep != "stopped" {
		t.Fatalf("stopped state = %+v", st)
	}
}

func TestLooperResumeStateIncludesPrepareResponse(t *testing.T) {
	t.Parallel()
	holder := NewLooperRunStateHolder()
	_, cancel := context.WithCancel(context.Background())
	if err := holder.TryStart(10, cancel); err != nil {
		cancel()
		t.Fatalf("TryStart: %v", err)
	}
	holder.SaveResumeState(LooperResumeState{
		Iteration:       3,
		PrepareResponse: "seed from prepare",
		LastTestResult:  "score 8",
	})
	rs, ok := holder.GetResumeState()
	if !ok || rs.PrepareResponse != "seed from prepare" || rs.Iteration != 3 {
		t.Fatalf("resume state = %+v ok=%v", rs, ok)
	}
	// TryStartResume must keep snapshot until ClearResumeState
	_, cancel2 := context.WithCancel(context.Background())
	if err := holder.TryStartResume(10, 3, cancel2); err != nil {
		cancel2()
		t.Fatalf("TryStartResume: %v", err)
	}
	if _, ok := holder.GetResumeState(); !ok {
		t.Fatal("snapshot should remain after TryStartResume")
	}
	holder.ClearResumeState()
	if _, ok := holder.GetResumeState(); ok {
		t.Fatal("snapshot should be gone after ClearResumeState")
	}
}

func TestLooperPresetStoreInitRefreshesBuiltinPresets(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS desktop_meta (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create desktop_meta: %v", err)
	}

	store := NewLooperPresetStore(db)
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("initial init: %v", err)
	}
	if _, err := db.Exec(`UPDATE desktop_looper_presets SET finish='Old finish', finish_context='' WHERE name='Story Iteration' AND is_builtin=1`); err != nil {
		t.Fatalf("simulate old builtin preset: %v", err)
	}

	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("refresh init: %v", err)
	}

	var finish, finishContext string
	if err := db.QueryRow(`SELECT finish, finish_context FROM desktop_looper_presets WHERE name='Story Iteration'`).Scan(&finish, &finishContext); err != nil {
		t.Fatalf("read refreshed preset: %v", err)
	}
	if finishContext != "last_action_test" {
		t.Fatalf("finish_context = %q, want last_action_test", finishContext)
	}
	if !strings.Contains(finish, "open_in_app") {
		t.Fatalf("finish prompt was not refreshed with desktop open instruction: %q", finish)
	}
}
