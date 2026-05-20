package desktop

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
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
