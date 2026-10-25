package embeddings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// llamaHangThreshold is how many requests in a row may end at the caller's
	// deadline before the server is probed. Callers give up after 15-60 s and
	// the HTTP clients after 2 min, so a wedged server (process alive, no
	// answer) would otherwise never surface as a failure.
	llamaHangThreshold = 3
	// llamaProbeTimeout bounds the liveness probe: a healthy server answers one
	// short text well inside it, even right after a cold start.
	llamaProbeTimeout = 15 * time.Second
	// llamaRestartTimeout bounds one server start. It sits above the 2-minute
	// health wait of both embedders, which leaves room for the process spawn
	// and for the Docker restart call (5 s stop grace) in front of it.
	llamaRestartTimeout = 3 * time.Minute
	// llamaProbeText is the one short text a liveness probe embeds.
	llamaProbeText = "ping"
)

// llamaStateRestarting is the Status state of an embedder whose server is being
// started. It reuses "setting_up", which the config UI already translates and
// polls on, instead of adding a state the UI and API consumers do not know.
const llamaStateRestarting = "setting_up"

var errLlamaClosing = errors.New("llama.cpp embedder is closing")

// llamaLifecycle is the part of an embedder's state that Close and Status must
// reach while a start holds the embedder lock, which can be minutes. It has a
// mutex of its own and never waits for the embedder lock.
type llamaLifecycle struct {
	mu       sync.Mutex
	closing  bool
	starting bool
	cancel   context.CancelFunc // ends the running start
	last     Status             // the last status computed under the embedder lock
	haveLast bool
}

// begin registers a start that runs under a context derived from parent, which
// shutdown cancels. It refuses once the embedder is closing. A nil lifecycle
// (tests with fake servers) cancels nothing.
func (lifecycle *llamaLifecycle) begin(parent context.Context) (context.Context, func(), error) {
	if lifecycle == nil {
		return parent, func() {}, nil
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if lifecycle.closing {
		return nil, nil, errLlamaClosing
	}
	ctx, cancel := context.WithCancel(parent)
	lifecycle.cancel = cancel
	lifecycle.starting = true
	return ctx, func() {
		lifecycle.mu.Lock()
		lifecycle.cancel = nil
		lifecycle.starting = false
		lifecycle.mu.Unlock()
		cancel()
	}, nil
}

// shutdown marks the embedder as closing and cancels the start in flight, so
// that Close does not wait for a health check that can run for two minutes.
// Call it before taking the embedder lock.
func (lifecycle *llamaLifecycle) shutdown() {
	lifecycle.mu.Lock()
	lifecycle.closing = true
	cancel := lifecycle.cancel
	lifecycle.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (lifecycle *llamaLifecycle) isClosing() bool {
	if lifecycle == nil {
		return false
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	return lifecycle.closing
}

// remember keeps the status Status computed under the embedder lock.
func (lifecycle *llamaLifecycle) remember(status Status) {
	lifecycle.mu.Lock()
	lifecycle.last, lifecycle.haveLast = status, true
	lifecycle.mu.Unlock()
}

// busy answers Status while the embedder lock is taken: the last status seen
// (or base when there is none), reported as restarting while a start runs and
// as closed once Close has begun.
func (lifecycle *llamaLifecycle) busy(base Status) Status {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	status := base
	if lifecycle.haveLast {
		status = lifecycle.last
	}
	status.UpdatedAt = time.Now().UTC()
	switch {
	case lifecycle.closing:
		status.State = "closed"
	case lifecycle.starting:
		status.State = llamaStateRestarting
	}
	return status
}

// llamaHangState counts the requests of one embedder that ended at the
// caller's deadline with no success in between. It is guarded by the lock the
// embedder's Embed callers hold.
type llamaHangState struct {
	timeouts int
	// probeTimeout replaces llamaProbeTimeout when set; tests shorten it.
	probeTimeout time.Duration
}

// llamaServerOps is the part of a llama.cpp embedder that the shared restart
// policy needs. The native process and the Docker sidecar implement it with
// their own start and stop mechanics.
type llamaServerOps struct {
	// name is used in error messages, for example "llama-server".
	name string
	// running reports whether the server is believed to be up.
	running func() bool
	// start brings a server that is not running up again.
	start func(ctx context.Context) error
	embed func(ctx context.Context, texts []string) ([][]float32, error)
	// stop takes a server that just failed out of service so that the next
	// attempt restarts it.
	stop func()
	// hangs is the embedder's deadline bookkeeping; nil turns the liveness
	// probe off.
	hangs *llamaHangState
	// hold is called when the caller leaves while a start still runs. The
	// embedder must keep its lock until finished is closed, so that no second
	// start can race the one in flight.
	hold func(finished <-chan struct{})
	// logger receives the warnings no caller is left to see; nil means
	// slog.Default().
	logger *slog.Logger
	// lifecycle lets Close cancel a start in flight and keeps a closing
	// embedder from beginning one; nil (fake servers) does neither.
	lifecycle *llamaLifecycle
}

// embedWithRestarts runs one embedding request with at most one restart of the
// server in between. Restarts are paid from budget, a sliding window.
//
// An attempt that failed because the caller's context ended says nothing about
// the server: it is returned at once and the server is neither stopped nor
// restarted. Only a run of llamaHangThreshold deadline errors is followed up
// (see afterCallerError). A restart is never begun for a context that is
// already done.
//
// A restart runs detached from the caller (see startDetached), so its budget
// token is spent for good once it has begun, whoever stops waiting for it.
func embedWithRestarts(ctx context.Context, budget *restartBudget, ops llamaServerOps, texts []string) ([][]float32, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !ops.running() {
			if ops.lifecycle.isClosing() {
				return nil, errLlamaClosing
			}
			if !budget.take() {
				return nil, fmt.Errorf("%s restart limit reached (%d per %s): %w",
					ops.name, maxProcessRestarts, processRestartWindow, restartCause(lastErr))
			}
			left, err := startDetached(ctx, ops)
			if left {
				return nil, ctx.Err()
			}
			if err != nil {
				lastErr = err
				continue
			}
		}
		vectors, err := ops.embed(ctx, texts)
		if err == nil {
			ops.hangs.reset()
			return vectors, nil
		}
		if isCallerError(ctx, err) {
			return nil, ops.afterCallerError(ctx, err)
		}
		lastErr = err
		ops.stop()
		ops.hangs.reset()
	}
	return nil, fmt.Errorf("%s embedding failed after restart: %w", ops.name, restartCause(lastErr))
}

// startDetached runs ops.start on its own deadline and without the caller's
// cancellation. A cold start can outlast the callers' deadlines; tied to the
// caller it was killed and begun again by every request and never finished.
//
// left reports that ctx ended before the start did. The start then keeps
// running, ops.hold keeps the embedder lock until it is over so that no second
// start can race it, and a failure is logged because no caller is left to
// receive it. A server that did come up is kept for the next call.
//
// Close ends the start through ops.lifecycle instead of waiting for it, and a
// closing embedder begins none.
func startDetached(ctx context.Context, ops llamaServerOps) (left bool, err error) {
	timeoutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), llamaRestartTimeout)
	startCtx, endStart, err := ops.lifecycle.begin(timeoutCtx)
	if err != nil {
		cancel()
		return false, err
	}
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer cancel()
		defer endStart()
		result <- ops.start(startCtx)
	}()
	select {
	case err := <-result:
		return false, err
	case <-ctx.Done():
	}
	if ops.hold != nil {
		ops.hold(finished)
	}
	go func() {
		<-finished
		// A start that Close ended is no failure worth a warning.
		if err := <-result; err != nil && !ops.lifecycle.isClosing() {
			ops.log().Warn("Detached llama.cpp server start failed after its caller left",
				"server", ops.name,
				"error", err)
		}
	}()
	return true, nil
}

// afterCallerError decides what a caller whose attempt ended early gets back.
// Normally err itself: the caller's own doing says nothing about the server.
// But a server that keeps running into its callers' deadlines may be wedged,
// and callers would only ever see their own deadline errors, so the server is
// never stopped and never counted as failing. After llamaHangThreshold
// deadline errors in a row it is probed with one short text. A server that
// does not answer is stopped (the next call restarts it within the budget) and
// the call fails with errBackendUnresponsive, a backend failure that
// Manager.Embed counts and can fall back on.
//
// Plain cancellations neither count nor reset the run; a successful request
// does. The caller holds the embedder lock.
func (ops llamaServerOps) afterCallerError(ctx context.Context, err error) error {
	hangs := ops.hangs
	if hangs == nil || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return err
	}
	hangs.timeouts++
	if hangs.timeouts < llamaHangThreshold {
		return err
	}
	hangs.timeouts = 0
	probeErr := ops.probe(ctx, hangs.probeTimeout)
	if probeErr == nil {
		return err
	}
	ops.stop()
	ops.log().Warn("llama.cpp server did not answer a probe after repeated caller timeouts; stopping it for a restart",
		"server", ops.name,
		"timeouts", llamaHangThreshold,
		"error", probeErr)
	return fmt.Errorf("%w: %s did not answer a probe after %d requests in a row ended at the caller's deadline; stopped it for a restart (probe: %v): %w",
		errBackendUnresponsive, ops.name, llamaHangThreshold, probeErr, err)
}

// probe sends one short text under its own deadline: the caller's has passed.
func (ops llamaServerOps) probe(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = llamaProbeTimeout
	}
	probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	_, err := ops.embed(probeCtx, []string{llamaProbeText})
	return err
}

func (ops llamaServerOps) log() *slog.Logger {
	if ops.logger != nil {
		return ops.logger
	}
	return slog.Default()
}

// reset forgets the run of deadline errors; the server answered or was replaced.
func (hangs *llamaHangState) reset() {
	if hangs != nil {
		hangs.timeouts = 0
	}
}

// lockContext takes mu, or gives up when ctx ends first: a restart can hold the
// embedder lock for minutes, and callers queued behind it must still leave at
// their own deadline. A caller that gives up leaves a goroutine that takes the
// lock when its turn comes and frees it at once.
func lockContext(ctx context.Context, mu *sync.Mutex) error {
	if mu.TryLock() {
		return nil
	}
	acquired := make(chan struct{})
	go func() {
		mu.Lock()
		close(acquired)
	}()
	select {
	case <-acquired:
		return nil
	case <-ctx.Done():
		go func() {
			<-acquired
			mu.Unlock()
		}()
		return ctx.Err()
	}
}

// restartCause keeps the %w verb valid when no attempt has failed yet (the
// budget was already empty when the call arrived).
func restartCause(err error) error {
	if err == nil {
		return fmt.Errorf("no restart left from earlier failures")
	}
	return err
}
