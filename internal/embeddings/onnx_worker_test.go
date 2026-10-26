package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeONNXProcess stands in for the worker child process: the embedder talks to
// it over in-memory pipes with the real wire protocol.
type fakeONNXProcess struct {
	conn     *onnxWorkerConn
	received chan workerRequest
	release  chan struct{} // gated mode: one token answers one request
	done     chan struct{}
	kills    atomic.Int32
	gated    bool
	reqR     *io.PipeReader
	reqW     *io.PipeWriter
	respW    *io.PipeWriter
	doneOnce sync.Once
}

func newFakeONNXProcess(t *testing.T, gated bool) *fakeONNXProcess {
	t.Helper()
	reqR, reqW := io.Pipe()
	respR, respW := io.Pipe()
	process := &fakeONNXProcess{
		received: make(chan workerRequest, 16),
		release:  make(chan struct{}, 16),
		done:     make(chan struct{}),
		gated:    gated,
		reqR:     reqR,
		reqW:     reqW,
		respW:    respW,
	}
	process.conn = &onnxWorkerConn{
		stdin:   reqW,
		decoder: json.NewDecoder(respR),
		kill: func() error {
			process.kills.Add(1)
			process.shutdown()
			return nil
		},
	}
	t.Cleanup(process.shutdown)
	go process.serve()
	return process
}

func (process *fakeONNXProcess) shutdown() {
	process.doneOnce.Do(func() {
		close(process.done)
		_ = process.reqR.Close()
		_ = process.reqW.Close()
		_ = process.respW.Close()
	})
}

// die simulates the child exiting on its own: pipes close, nobody calls kill.
func (process *fakeONNXProcess) die() {
	process.shutdown()
}

func (process *fakeONNXProcess) serve() {
	decoder := json.NewDecoder(process.reqR)
	encoder := json.NewEncoder(process.respW)
	for {
		var request workerRequest
		if decoder.Decode(&request) != nil {
			return
		}
		process.received <- request
		if process.gated {
			select {
			case <-process.release:
			case <-process.done:
				return
			}
		}
		vectors := make([][]float32, len(request.Texts))
		for i := range vectors {
			vectors[i] = unitTestVector()
		}
		if encoder.Encode(workerResponse{ID: request.ID, Type: "result", Vectors: vectors}) != nil {
			return
		}
	}
}

func (process *fakeONNXProcess) nextRequest(t *testing.T) workerRequest {
	t.Helper()
	select {
	case request := <-process.received:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not receive a request")
		return workerRequest{}
	}
}

func newTestONNXWorker(t *testing.T, start func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error), respawns *restartBudget) *onnxWorkerEmbedder {
	t.Helper()
	worker, err := newONNXWorkerEmbedderWith(context.Background(), onnxWorkerOptions{Backend: "cpu", Logger: discardEmbeddingLogger()}, start, respawns)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })
	return worker
}

type embedOutcome struct {
	vectors [][]float32
	err     error
}

func embedAsync(worker *onnxWorkerEmbedder, ctx context.Context, text string) <-chan embedOutcome {
	result := make(chan embedOutcome, 1)
	go func() {
		vectors, err := worker.Embed(ctx, []string{text})
		result <- embedOutcome{vectors: vectors, err: err}
	}()
	return result
}

func awaitOutcome(t *testing.T, result <-chan embedOutcome) embedOutcome {
	t.Helper()
	select {
	case outcome := <-result:
		return outcome
	case <-time.After(5 * time.Second):
		t.Fatal("Embed did not return")
		return embedOutcome{}
	}
}

// A caller giving up mid-request must not cost the worker process. The late
// response is drained and discarded, later callers wait their turn (and may
// give up while waiting), and the next request is answered by the same process.
func TestONNXWorkerContextCancelKeepsWorkerAlive(t *testing.T) {
	process := newFakeONNXProcess(t, true)
	var starts atomic.Int32
	worker := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		starts.Add(1)
		return process.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))

	ctx, cancel := context.WithCancel(context.Background())
	first := embedAsync(worker, ctx, "first")
	firstRequest := process.nextRequest(t)
	cancel()
	if outcome := awaitOutcome(t, first); !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("canceled Embed error = %v, want context.Canceled", outcome.err)
	}
	if process.kills.Load() != 0 || worker.Status().State != "ready" {
		t.Fatalf("cancel closed the worker: kills=%d state=%s", process.kills.Load(), worker.Status().State)
	}

	// The slot is still held by the drain: waiting for it honors the context
	// and nothing reaches the process.
	waitCtx, cancelWait := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelWait()
	if outcome := awaitOutcome(t, embedAsync(worker, waitCtx, "impatient")); !errors.Is(outcome.err, context.DeadlineExceeded) {
		t.Fatalf("slot wait error = %v, want context.DeadlineExceeded", outcome.err)
	}
	select {
	case extra := <-process.received:
		t.Fatalf("request %d reached the worker while the slot was held", extra.ID)
	default:
	}

	second := embedAsync(worker, context.Background(), "second")
	process.release <- struct{}{} // answers the abandoned first request
	secondRequest := process.nextRequest(t)
	process.release <- struct{}{}
	outcome := awaitOutcome(t, second)
	if outcome.err != nil || len(outcome.vectors) != 1 {
		t.Fatalf("request after the cancel = %v, %v; want success", outcome.vectors, outcome.err)
	}
	if secondRequest.ID == firstRequest.ID || process.kills.Load() != 0 || starts.Load() != 1 {
		t.Fatalf("worker was replaced: ids %d/%d kills=%d starts=%d", firstRequest.ID, secondRequest.ID, process.kills.Load(), starts.Load())
	}
}

// A worker that never answers an abandoned request is killed once the hard
// drain timeout passes, and the next request gets a fresh process.
func TestONNXWorkerDrainTimeoutKillsHungWorker(t *testing.T) {
	hung := newFakeONNXProcess(t, true)
	fresh := newFakeONNXProcess(t, false)
	var starts atomic.Int32
	worker := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		if starts.Add(1) == 1 {
			return hung.conn, nil
		}
		return fresh.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))
	worker.drainTimeout = 30 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	first := embedAsync(worker, ctx, "never answered")
	hung.nextRequest(t)
	cancel()
	awaitOutcome(t, first)

	outcome := awaitOutcome(t, embedAsync(worker, context.Background(), "after the timeout"))
	if outcome.err != nil || len(outcome.vectors) != 1 {
		t.Fatalf("request after the drain timeout = %v, %v; want success", outcome.vectors, outcome.err)
	}
	if hung.kills.Load() != 1 || starts.Load() != 2 {
		t.Fatalf("hung worker kills=%d starts=%d, want 1 and 2", hung.kills.Load(), starts.Load())
	}
}

// A worker that died on its own is started again by the next request, within
// the restart budget. Only an explicit Close is final.
func TestONNXWorkerRespawnsAfterCrashWithinBudgetAndCloseIsFinal(t *testing.T) {
	var processes []*fakeONNXProcess
	var starts atomic.Int32
	start := func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		starts.Add(1)
		process := newFakeONNXProcess(t, false)
		processes = append(processes, process)
		return process.conn, nil
	}
	now := time.Unix(3_000_000, 0)
	worker := newTestONNXWorker(t, start, newRestartBudgetWithClock(maxProcessRestarts, processRestartWindow, func() time.Time { return now }))

	for restart := 1; restart <= maxProcessRestarts; restart++ {
		processes[len(processes)-1].die()
		if _, err := worker.Embed(context.Background(), []string{"hits the dead process"}); err == nil {
			t.Fatalf("restart %d: Embed on a dead worker succeeded", restart)
		}
		vectors, err := worker.Embed(context.Background(), []string{"after respawn"})
		if err != nil || len(vectors) != 1 {
			t.Fatalf("restart %d: Embed after the crash = %v, %v; want success", restart, vectors, err)
		}
	}
	if starts.Load() != int32(1+maxProcessRestarts) {
		t.Fatalf("starts = %d, want %d", starts.Load(), 1+maxProcessRestarts)
	}

	// The fourth crash inside the window exceeds the budget.
	processes[len(processes)-1].die()
	_, _ = worker.Embed(context.Background(), []string{"hits the dead process"})
	if _, err := worker.Embed(context.Background(), []string{"over budget"}); err == nil || !strings.Contains(err.Error(), "restart limit") {
		t.Fatalf("over-budget error = %v, want the restart limit", err)
	}
	// No process and no restart left: the worker must not claim to be ready.
	if state := worker.Status().State; state != "unavailable" {
		t.Fatalf("state with no process and no restart budget = %q, want unavailable", state)
	}
	// ... and the window frees it again.
	now = now.Add(processRestartWindow + time.Minute)
	if state := worker.Status().State; state != "ready" {
		t.Fatalf("state once the window frees a restart = %q, want ready", state)
	}
	if _, err := worker.Embed(context.Background(), []string{"after the window"}); err != nil {
		t.Fatalf("Embed after the window = %v", err)
	}

	startsBeforeClose := starts.Load()
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Embed(context.Background(), []string{"after close"}); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("Embed after Close = %v, want a closed error", err)
	}
	if starts.Load() != startsBeforeClose {
		t.Fatal("Close was not final: the worker was started again")
	}
}

// Close must not cut a request off: a rebenchmark swaps backends while callers
// are still embedding, and those callers have to get their vectors.
func TestONNXWorkerCloseWaitsForInFlightRequest(t *testing.T) {
	process := newFakeONNXProcess(t, true)
	worker := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		return process.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))

	inFlight := embedAsync(worker, context.Background(), "in flight")
	process.nextRequest(t)

	closed := make(chan error, 1)
	go func() { closed <- worker.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned (%v) while a request was in flight", err)
	case <-time.After(50 * time.Millisecond):
	}
	if process.kills.Load() != 0 {
		t.Fatal("Close killed the process under a running request")
	}
	// A closing worker admits nothing new.
	if _, err := worker.Embed(context.Background(), []string{"late"}); !errors.Is(err, errONNXWorkerClosed) {
		t.Fatalf("Embed while closing = %v, want the closed error", err)
	}

	process.release <- struct{}{}
	if outcome := awaitOutcome(t, inFlight); outcome.err != nil || len(outcome.vectors) != 1 {
		t.Fatalf("in-flight request = %v, %v; want its vector", outcome.vectors, outcome.err)
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the request finished")
	}
	if process.kills.Load() != 1 {
		t.Fatalf("kills = %d, want 1", process.kills.Load())
	}
}

// A drain in progress holds the slot like a request does, so Close waits for it
// too. A worker that never answers is stopped once the wait is over, and a
// second Close is a no-op.
func TestONNXWorkerCloseWaitsForDrainAndIsBounded(t *testing.T) {
	process := newFakeONNXProcess(t, true)
	worker := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		return process.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))

	ctx, cancel := context.WithCancel(context.Background())
	abandoned := embedAsync(worker, ctx, "abandoned")
	process.nextRequest(t)
	cancel()
	awaitOutcome(t, abandoned) // the drain goroutine now owns the slot

	closed := make(chan error, 1)
	go func() { closed <- worker.Close() }()
	select {
	case <-closed:
		t.Fatal("Close returned while the drain still owned the request slot")
	case <-time.After(50 * time.Millisecond):
	}
	process.release <- struct{}{} // the late response ends the drain
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the drain finished")
	}
	if process.kills.Load() != 1 {
		t.Fatalf("kills = %d, want 1", process.kills.Load())
	}

	hung := newFakeONNXProcess(t, true)
	stuck := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		return hung.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))
	stuck.closeWait = 30 * time.Millisecond
	pending := embedAsync(stuck, context.Background(), "never answered")
	hung.nextRequest(t)
	if err := stuck.Close(); err != nil {
		t.Fatalf("Close of a stuck worker = %v", err)
	}
	if outcome := awaitOutcome(t, pending); outcome.err == nil {
		t.Fatal("request on a worker that was stopped after the wait succeeded")
	}
	if err := stuck.Close(); err != nil || hung.kills.Load() != 1 {
		t.Fatalf("second Close = %v, kills = %d; want nil and 1", err, hung.kills.Load())
	}
}

// Close during a respawn must neither deadlock nor leak the fresh process.
func TestONNXWorkerCloseDuringRespawnStopsFreshProcess(t *testing.T) {
	dead := newFakeONNXProcess(t, false)
	fresh := newFakeONNXProcess(t, false)
	startEntered := make(chan struct{})
	releaseStart := make(chan struct{})
	var starts atomic.Int32
	worker := newTestONNXWorker(t, func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) {
		if starts.Add(1) == 1 {
			return dead.conn, nil
		}
		close(startEntered)
		<-releaseStart
		return fresh.conn, nil
	}, newRestartBudget(maxProcessRestarts, processRestartWindow))
	worker.closeWait = 30 * time.Millisecond

	dead.die()
	if _, err := worker.Embed(context.Background(), []string{"hits the dead process"}); err == nil {
		t.Fatal("Embed on a dead worker succeeded")
	}
	respawning := embedAsync(worker, context.Background(), "respawns")
	select {
	case <-startEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("respawn did not start")
	}

	if err := worker.Close(); err != nil { // waits out closeWait, then gives up
		t.Fatalf("Close during respawn = %v", err)
	}
	close(releaseStart)
	if outcome := awaitOutcome(t, respawning); !errors.Is(outcome.err, errONNXWorkerClosed) {
		t.Fatalf("Embed that respawned during Close = %v, want the closed error", outcome.err)
	}
	if fresh.kills.Load() != 1 {
		t.Fatalf("fresh process kills = %d, want 1", fresh.kills.Load())
	}
}

// gatedRespawn is a worker whose first process is dead and whose respawn
// blocks until the test releases it, like a model load that outlasts the
// callers' deadlines.
type gatedRespawn struct {
	worker       *onnxWorkerEmbedder
	budget       *restartBudget
	fresh        *fakeONNXProcess
	startContext chan context.Context
	release      chan struct{}
	open         func() // lets the blocked respawn finish; safe to call twice
	starts       atomic.Int32
}

func newGatedRespawn(t *testing.T) *gatedRespawn {
	t.Helper()
	gate := &gatedRespawn{
		budget:       newRestartBudget(maxProcessRestarts, processRestartWindow),
		fresh:        newFakeONNXProcess(t, false),
		startContext: make(chan context.Context, 4),
		release:      make(chan struct{}),
	}
	gate.open = sync.OnceFunc(func() { close(gate.release) })
	dead := newFakeONNXProcess(t, false)
	gate.worker = newTestONNXWorker(t, func(ctx context.Context, _ onnxWorkerOptions) (*onnxWorkerConn, error) {
		if gate.starts.Add(1) == 1 {
			return dead.conn, nil
		}
		gate.startContext <- ctx
		select {
		case <-gate.release:
			return gate.fresh.conn, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, gate.budget)
	// Runs before the worker's own Close: a failed assertion must not leave
	// the respawn blocking it.
	t.Cleanup(gate.open)
	dead.die()
	if _, err := gate.worker.Embed(context.Background(), []string{"hits the dead process"}); err == nil {
		t.Fatal("Embed on a dead worker succeeded")
	}
	return gate
}

// callerLeaves runs an Embed that needs the respawn and gives up after
// 30 ms. It returns the context the respawn runs under.
func (gate *gatedRespawn) callerLeaves(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if outcome := awaitOutcome(t, embedAsync(gate.worker, ctx, "caller leaves")); !errors.Is(outcome.err, context.DeadlineExceeded) {
		t.Fatalf("caller error = %v, want its deadline error", outcome.err)
	}
	select {
	case startCtx := <-gate.startContext:
		return startCtx
	case <-time.After(5 * time.Second):
		t.Fatal("the respawn never began")
		return nil
	}
}

// A respawn that takes longer than the callers' deadlines used to be cut off
// with the caller, refunded, and begun again by the next request: the worker
// never came back. The respawn now runs on its own deadline; the caller that
// triggered it returns at its own, and the slot stays with the respawn so that
// no second start can race it.
func TestONNXWorkerRespawnOutlivesTheCallerThatStartedIt(t *testing.T) {
	gate := newGatedRespawn(t)
	startCtx := gate.callerLeaves(t)

	if startCtx.Err() != nil {
		t.Fatalf("the caller's deadline ended the respawn: %v", startCtx.Err())
	}
	if deadline, ok := startCtx.Deadline(); !ok || time.Until(deadline) < onnxStartupTimeout/2 {
		t.Fatalf("respawn deadline = %v (set %v), want about %s from now", deadline, ok, onnxStartupTimeout)
	}
	if gate.budget.remaining() != maxProcessRestarts-1 {
		t.Fatalf("budget = %d: the caller's exit gave the respawn token back", gate.budget.remaining())
	}
	// The slot belongs to the respawn until it ends.
	impatient, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if outcome := awaitOutcome(t, embedAsync(gate.worker, impatient, "impatient")); !errors.Is(outcome.err, context.DeadlineExceeded) {
		t.Fatalf("slot wait error = %v, want context.DeadlineExceeded", outcome.err)
	}

	gate.open()
	outcome := awaitOutcome(t, embedAsync(gate.worker, context.Background(), "after the respawn"))
	if outcome.err != nil || len(outcome.vectors) != 1 {
		t.Fatalf("request after the respawn = %v, %v; want the fresh process to answer", outcome.vectors, outcome.err)
	}
	gate.fresh.nextRequest(t)
	if gate.starts.Load() != 2 || gate.budget.remaining() != maxProcessRestarts-1 || gate.fresh.kills.Load() != 0 {
		t.Fatalf("starts=%d budget=%d kills=%d, want one respawn adopted and kept", gate.starts.Load(), gate.budget.remaining(), gate.fresh.kills.Load())
	}
}

// Close while an abandoned respawn is still running must neither wait for it
// forever nor leak the process it produces or the slot it holds.
func TestONNXWorkerCloseWhileAbandonedRespawnRunsStopsTheFreshProcess(t *testing.T) {
	gate := newGatedRespawn(t)
	gate.worker.closeWait = 30 * time.Millisecond
	gate.callerLeaves(t)

	if err := gate.worker.Close(); err != nil { // gives up waiting for the slot
		t.Fatalf("Close during the respawn = %v", err)
	}
	gate.open()
	deadline := time.Now().Add(5 * time.Second)
	for (gate.fresh.kills.Load() != 1 || len(gate.worker.slot) != 0) && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if gate.fresh.kills.Load() != 1 || len(gate.worker.slot) != 0 {
		t.Fatalf("fresh process kills=%d slot held=%d, want it stopped and the slot freed", gate.fresh.kills.Load(), len(gate.worker.slot))
	}
	if _, err := gate.worker.Embed(context.Background(), []string{"after close"}); !errors.Is(err, errONNXWorkerClosed) {
		t.Fatalf("Embed after Close = %v, want the closed error", err)
	}
}

// captureHandler records log records so a test can assert on them.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (handler *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (handler *captureHandler) WithAttrs([]slog.Attr) slog.Handler       { return handler }
func (handler *captureHandler) WithGroup(string) slog.Handler            { return handler }

func (handler *captureHandler) Handle(_ context.Context, record slog.Record) error {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	handler.records = append(handler.records, record.Clone())
	return nil
}

func (handler *captureHandler) snapshot() []slog.Record {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	return append([]slog.Record(nil), handler.records...)
}

// A drain that ends in a broken response must not drop the worker silently: one
// warning names the request, the reason and the worker's own log.
func TestONNXWorkerDrainFailureIsLogged(t *testing.T) {
	process := newFakeONNXProcess(t, true)
	process.conn.stderr = newLimitedBuffer(1 << 10)
	_, _ = process.conn.stderr.Write([]byte("onnxruntime: session aborted"))
	logs := &captureHandler{}
	worker, err := newONNXWorkerEmbedderWith(context.Background(),
		onnxWorkerOptions{Backend: "cpu", Logger: slog.New(logs)},
		func(context.Context, onnxWorkerOptions) (*onnxWorkerConn, error) { return process.conn, nil },
		newRestartBudget(maxProcessRestarts, processRestartWindow))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = worker.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	abandoned := embedAsync(worker, ctx, "abandoned")
	request := process.nextRequest(t)
	cancel()
	awaitOutcome(t, abandoned)
	process.die() // the worker vanishes before it answers the abandoned request

	deadline := time.Now().Add(5 * time.Second)
	for len(logs.snapshot()) == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	records := logs.snapshot()
	if len(records) != 1 || records[0].Level != slog.LevelWarn {
		t.Fatalf("drain failure produced %d records, want one warning", len(records))
	}
	attrs := map[string]any{}
	records[0].Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	if attrs["request_id"] != request.ID {
		t.Fatalf("request_id = %v, want %d", attrs["request_id"], request.ID)
	}
	if reason, _ := attrs["reason"].(string); reason == "" {
		t.Fatal("warning carries no reason")
	}
	if log, _ := attrs["log"].(string); !strings.Contains(log, "session aborted") {
		t.Fatalf("worker log = %q, want the process stderr", attrs["log"])
	}
}
