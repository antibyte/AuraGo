package embeddings

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// countingEmbedder records how often the manager reached the backend.
type countingEmbedder struct {
	status  Status
	vectors [][]float32
	err     error
	calls   atomic.Int32
}

func (embedder *countingEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	embedder.calls.Add(1)
	return embedder.vectors, embedder.err
}

func (embedder *countingEmbedder) Dimensions() int     { return GraniteDimensions }
func (embedder *countingEmbedder) ModelID() string     { return GraniteModelID }
func (embedder *countingEmbedder) Fingerprint() string { return embedder.status.Fingerprint }
func (embedder *countingEmbedder) Status() Status      { return embedder.status }
func (embedder *countingEmbedder) Close() error        { return nil }
func unitTestVector() []float32 {
	vector := make([]float32, GraniteDimensions)
	vector[0] = 1
	return vector
}

// A caller that gives up, or sends a bad request, must not look like a backend
// outage: no failure count, no fallback, no re-benchmark, no degraded state.
func TestManagerEmbedTreatsCallerErrorsAsNotBackendFailures(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	expiredCtx, expire := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer expire()

	cases := []struct {
		name        string
		ctx         context.Context
		backendErr  error
		wantErr     error
		wantFailure bool
	}{
		{name: "canceled caller", ctx: canceledCtx, backendErr: context.Canceled, wantErr: context.Canceled},
		{name: "expired caller", ctx: expiredCtx, backendErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
		{name: "wrapped context error", ctx: canceledCtx, backendErr: errors.Join(errors.New("call llama-server"), context.Canceled), wantErr: context.Canceled},
		{name: "invalid input", ctx: context.Background(), backendErr: ErrInvalidInput, wantErr: ErrInvalidInput},
		// The same context error with a healthy caller context is a backend
		// problem (for example an internal client timeout) and must still count.
		{name: "backend timeout with live caller", ctx: context.Background(), backendErr: context.DeadlineExceeded, wantFailure: true},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			ready := make(chan struct{})
			close(ready)
			active := &countingEmbedder{
				status: Status{Runtime: "onnxruntime", Backend: "cuda", Fingerprint: onnxFingerprint()},
				err:    current.backendErr,
			}
			fallback := &countingEmbedder{
				status:  Status{Runtime: "onnxruntime", Backend: "cpu", Fingerprint: onnxFingerprint()},
				vectors: [][]float32{unitTestVector()},
			}
			var benchmarkCalls atomic.Int32
			rootCtx, cancelRoot := context.WithCancel(context.Background())
			manager := &Manager{
				active:   active,
				fallback: fallback,
				status:   Status{State: "ready"},
				ready:    ready,
				logger:   discardEmbeddingLogger(),
				rootCtx:  rootCtx,
				cancel:   cancelRoot,
				setupOverride: func(context.Context, bool) error {
					benchmarkCalls.Add(1)
					return nil
				},
			}
			defer manager.Close()

			// Three calls: past the "two failures trigger a re-benchmark" limit.
			for i := 0; i < 3; i++ {
				_, err := manager.Embed(current.ctx, []string{"text"})
				if current.wantErr == nil {
					// A genuine backend failure is served by the fallback.
					if err != nil {
						t.Fatalf("call %d error = %v, want the fallback to serve it", i, err)
					}
				} else if !errors.Is(err, current.wantErr) {
					t.Fatalf("call %d error = %v, want %v", i, err, current.wantErr)
				}
			}
			manager.initWG.Wait()

			manager.mu.RLock()
			failures, state := manager.failures, manager.status.State
			manager.mu.RUnlock()
			if current.wantFailure {
				if failures == 0 || state != "degraded" || fallback.calls.Load() == 0 {
					t.Fatalf("backend failure was not recorded: failures=%d state=%s fallbackCalls=%d", failures, state, fallback.calls.Load())
				}
				return
			}
			if failures != 0 || state != "ready" {
				t.Fatalf("caller error changed manager health: failures=%d state=%s", failures, state)
			}
			if fallback.calls.Load() != 0 {
				t.Fatalf("fallback was tried %d times for a caller error", fallback.calls.Load())
			}
			if benchmarkCalls.Load() != 0 {
				t.Fatalf("caller error triggered %d benchmarks", benchmarkCalls.Load())
			}
		})
	}
}

func TestManagerEmbedRejectsEmptyInputWithoutTouchingBackend(t *testing.T) {
	ready := make(chan struct{})
	close(ready)
	active := &countingEmbedder{
		status: Status{Runtime: "onnxruntime", Backend: "cpu", Fingerprint: onnxFingerprint()},
		err:    errors.New("must not be reached"),
	}
	manager := &Manager{active: active, fallback: active, status: Status{State: "ready"}, ready: ready, logger: discardEmbeddingLogger()}
	if _, err := manager.Embed(context.Background(), nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty input error = %v, want ErrInvalidInput", err)
	}
	if active.calls.Load() != 0 || manager.failures != 0 {
		t.Fatalf("empty input reached the backend: calls=%d failures=%d", active.calls.Load(), manager.failures)
	}
}

// While a scheduled reindex waits for a restart, a benchmark could activate a
// backend of another vector space and lift the restart lock too early.
func TestRebenchmarkRefusedWhileRestartPending(t *testing.T) {
	rootCtx, cancel := context.WithCancel(context.Background())
	var setupCalls atomic.Int32
	manager := &Manager{
		logger:  discardEmbeddingLogger(),
		rootCtx: rootCtx,
		cancel:  cancel,
		ready:   make(chan struct{}),
		status:  Status{State: "restart_required", RestartRequired: true, Fingerprint: ggufFingerprint()},
		setupOverride: func(context.Context, bool) error {
			setupCalls.Add(1)
			return nil
		},
	}
	defer manager.Close()

	if err := manager.Rebenchmark(context.Background()); !errors.Is(err, ErrRestartRequired) {
		t.Fatalf("Rebenchmark error = %v, want ErrRestartRequired", err)
	}
	manager.triggerBackgroundBenchmark()
	manager.initWG.Wait()
	if setupCalls.Load() != 0 {
		t.Fatalf("a benchmark was launched while a restart is pending (%d calls)", setupCalls.Load())
	}
}

func TestActivateCandidateKeepsPendingRestartLock(t *testing.T) {
	active := &countingEmbedder{status: Status{Runtime: "onnxruntime", Backend: "cpu", State: "ready", Fingerprint: onnxFingerprint()}}
	manager := &Manager{
		logger: discardEmbeddingLogger(),
		ready:  make(chan struct{}),
		status: Status{
			State:           "restart_required",
			RestartRequired: true,
			FallbackReason:  "reindex scheduled",
			Fingerprint:     ggufFingerprint(),
		},
		startOverride: func(context.Context, candidate) (Embedder, error) { return active, nil },
	}
	selected := candidate{ID: "onnx-cpu", Runtime: "onnxruntime", Backend: "cpu", HasAsset: true}
	if err := manager.activateCandidate(context.Background(), selected, nil, false); err != nil {
		t.Fatal(err)
	}
	status := manager.Status()
	if !status.RestartRequired || status.State != "restart_required" {
		t.Fatalf("activation lifted the restart lock: state=%s restartRequired=%v", status.State, status.RestartRequired)
	}
	if manager.status.Fingerprint != ggufFingerprint() {
		t.Fatalf("activation overwrote the pending vector space fingerprint: %q", manager.status.Fingerprint)
	}

	// Without a pending restart the same activation still publishes "ready".
	clean := &Manager{
		logger:        discardEmbeddingLogger(),
		ready:         make(chan struct{}),
		status:        Status{State: "benchmarking"},
		startOverride: func(context.Context, candidate) (Embedder, error) { return active, nil },
	}
	if err := clean.activateCandidate(context.Background(), selected, nil, false); err != nil {
		t.Fatal(err)
	}
	if status := clean.Status(); status.RestartRequired || status.State != "ready" {
		t.Fatalf("regular activation status = %s / restartRequired=%v", status.State, status.RestartRequired)
	}
}

func TestRestartBudgetSlidesOverTheWindow(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	budget := newRestartBudgetWithClock(3, 10*time.Minute, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		if !budget.take() {
			t.Fatalf("restart %d was refused inside the budget", i+1)
		}
		now = now.Add(time.Minute)
	}
	if budget.take() || budget.remaining() != 0 {
		t.Fatal("a fourth restart inside the window was allowed")
	}
	// The first restart was at t0; it leaves the window at t0+10m.
	now = now.Add(7*time.Minute + time.Second)
	if budget.remaining() != 1 || !budget.take() {
		t.Fatalf("window did not slide: remaining=%d", budget.remaining())
	}
	budget.release()
	if budget.remaining() != 1 {
		t.Fatalf("release did not return the restart: remaining=%d", budget.remaining())
	}
}
