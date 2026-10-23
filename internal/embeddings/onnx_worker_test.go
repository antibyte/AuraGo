package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	worker, err := newONNXWorkerEmbedderWith(context.Background(), onnxWorkerOptions{Backend: "cpu"}, start, respawns)
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
	// ... and the window frees it again.
	now = now.Add(processRestartWindow + time.Minute)
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
