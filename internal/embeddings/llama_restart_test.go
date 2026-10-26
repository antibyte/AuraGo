package embeddings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// fakeLlamaServer drives embedWithRestarts without a real llama-server. The
// counters are plain ints: the tests read them only after the call, or after
// the detached start they wait for through held, has finished.
type fakeLlamaServer struct {
	up       bool
	starts   int
	stops    int
	embeds   int
	failures int // remaining embed attempts that fail with a backend error
	onEmbed  func() error
	onStart  func() error
	// embedCtx and startCtx see the context the call runs under and win over
	// the context-less hooks above; a nil result from startCtx brings the
	// server up.
	embedCtx func(ctx context.Context, texts []string) error
	startCtx func(ctx context.Context) error
	hangs    *llamaHangState
	logger   *slog.Logger
	// held is the channel embedWithRestarts handed over when its caller left
	// while a start still ran.
	held <-chan struct{}
}

func (server *fakeLlamaServer) ops() llamaServerOps {
	return llamaServerOps{
		name:    "fake-server",
		running: func() bool { return server.up },
		start: func(ctx context.Context) error {
			server.starts++
			if server.startCtx != nil {
				if err := server.startCtx(ctx); err != nil {
					return err
				}
				server.up = true
				return nil
			}
			if server.onStart != nil {
				return server.onStart()
			}
			server.up = true
			return nil
		},
		embed: func(ctx context.Context, texts []string) ([][]float32, error) {
			server.embeds++
			if server.embedCtx != nil {
				if err := server.embedCtx(ctx, texts); err != nil {
					return nil, err
				}
				return [][]float32{unitTestVector()}, nil
			}
			if server.onEmbed != nil {
				return nil, server.onEmbed()
			}
			if server.failures > 0 {
				server.failures--
				return nil, errors.New("connection refused")
			}
			return [][]float32{unitTestVector()}, nil
		},
		stop: func() {
			server.stops++
			server.up = false
		},
		hangs:  server.hangs,
		hold:   func(finished <-chan struct{}) { server.held = finished },
		logger: server.logger,
	}
}

// hangingEmbed is a server that took the request and never answers: it returns
// only when the request's context ends, the way the HTTP call does.
func hangingEmbed(ctx context.Context, _ []string) error {
	<-ctx.Done()
	return fmt.Errorf("call fake-server: %w", ctx.Err())
}

func awaitClosed(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(5 * time.Second):
		t.Fatal("the detached start did not finish")
	}
}

// The old lifetime counter made the third restart of a process permanent. With
// a sliding window the embedder recovers once the window has passed.
func TestLlamaRestartBudgetIsATimeWindowNotALifetimeCap(t *testing.T) {
	now := time.Unix(2_000_000, 0)
	budget := newRestartBudgetWithClock(maxProcessRestarts, processRestartWindow, func() time.Time { return now })
	server := &fakeLlamaServer{failures: 3}
	texts := []string{"text"}

	// Call 1 burns two restarts, call 2 the third and then hits the limit.
	if _, err := embedWithRestarts(context.Background(), budget, server.ops(), texts); err == nil {
		t.Fatal("call 1 unexpectedly succeeded")
	}
	_, err := embedWithRestarts(context.Background(), budget, server.ops(), texts)
	if err == nil || !strings.Contains(err.Error(), "restart limit reached") {
		t.Fatalf("call 2 error = %v, want the restart limit inside the window", err)
	}

	now = now.Add(processRestartWindow + time.Minute)
	vectors, err := embedWithRestarts(context.Background(), budget, server.ops(), texts)
	if err != nil || len(vectors) != 1 {
		t.Fatalf("call after the window = %v, %v; want success", vectors, err)
	}
}

// A caller that gives up is not a server failure: no kill, no restart, and no
// budget spent for an attempt that never began a restart.
func TestLlamaCallerContextEndLeavesServerAndBudgetAlone(t *testing.T) {
	newBudget := func() *restartBudget {
		return newRestartBudget(maxProcessRestarts, processRestartWindow)
	}

	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		budget := newBudget()
		server := &fakeLlamaServer{} // down: a restart would be due
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.starts+server.stops+server.embeds != 0 || budget.remaining() != maxProcessRestarts {
			t.Fatalf("canceled call touched the server: %+v, budget %d", server, budget.remaining())
		}
	})

	t.Run("canceled during embed", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		budget := newBudget()
		server := &fakeLlamaServer{up: true}
		server.onEmbed = func() error {
			cancel()
			return errors.Join(errors.New("call llama-server"), ctx.Err())
		}
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.stops != 0 || server.starts != 0 || !server.up || budget.remaining() != maxProcessRestarts {
			t.Fatalf("cancel killed or restarted the server: %+v, budget %d", server, budget.remaining())
		}
	})

	t.Run("canceled during restart", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		budget := newBudget()
		server := &fakeLlamaServer{}
		server.onStart = func() error {
			cancel()
			return ctx.Err()
		}
		_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if server.held != nil {
			awaitClosed(t, server.held)
		}
		// The restart had begun and runs on whoever stops waiting for it, so its
		// token stays spent (TestLlamaRestartOutlivesTheCallerThatStartedIt).
		if server.embeds != 0 || budget.remaining() != maxProcessRestarts-1 {
			t.Fatalf("restart bookkeeping after the cancel: embeds=%d budget=%d, want 0 and %d", server.embeds, budget.remaining(), maxProcessRestarts-1)
		}
	})

	t.Run("embedders do not restart for a canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		native := &llamaEmbedder{serverPath: "does-not-exist"}
		if _, err := native.Embed(ctx, []string{"text"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("native embedder error = %v, want context.Canceled", err)
		}
		if native.restarts.remaining() != maxProcessRestarts {
			t.Fatalf("native embedder spent budget: %d", native.restarts.remaining())
		}
		docker := &dockerLlamaEmbedder{needsRestart: true}
		if _, err := docker.Embed(ctx, []string{"text"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("docker embedder error = %v, want context.Canceled", err)
		}
		if docker.restarts.remaining() != maxProcessRestarts {
			t.Fatalf("docker embedder spent budget: %d", docker.restarts.remaining())
		}
	})
}

// embedUnder runs one embedWithRestarts call whose caller gives up after
// within, and returns the context so the caller-error class can be checked.
func embedUnder(within time.Duration, budget *restartBudget, server *fakeLlamaServer) (context.Context, error) {
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()
	_, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"})
	return ctx, err
}

// A wedged llama-server (process alive, no answer) only ever produces caller
// deadline errors, which are no backend failure by themselves. After
// llamaHangThreshold of them in a row the server is probed; a probe that does
// not answer stops it and the error becomes a backend failure.
func TestLlamaServerThatKeepsMissingDeadlinesIsProbedAndStopped(t *testing.T) {
	const callerDeadline = 20 * time.Millisecond
	newHangs := func() *llamaHangState { return &llamaHangState{probeTimeout: 20 * time.Millisecond} }

	t.Run("a probe that hangs stops the server and fails the call as a backend error", func(t *testing.T) {
		logs := &captureHandler{}
		budget := newRestartBudget(maxProcessRestarts, processRestartWindow)
		server := &fakeLlamaServer{up: true, embedCtx: hangingEmbed, hangs: newHangs(), logger: slog.New(logs)}

		for call := 1; call < llamaHangThreshold; call++ {
			ctx, err := embedUnder(callerDeadline, budget, server)
			if !errors.Is(err, context.DeadlineExceeded) || !isCallerError(ctx, err) {
				t.Fatalf("call %d error = %v, want the caller's deadline error", call, err)
			}
		}
		if server.stops != 0 || server.embeds != llamaHangThreshold-1 {
			t.Fatalf("probed too early: stops=%d embeds=%d", server.stops, server.embeds)
		}

		ctx, err := embedUnder(callerDeadline, budget, server)
		if !errors.Is(err, errBackendUnresponsive) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("call %d error = %v, want errBackendUnresponsive wrapping the deadline error", llamaHangThreshold, err)
		}
		if isCallerError(ctx, err) {
			t.Fatal("an unresponsive server was reported as a caller error")
		}
		if server.stops != 1 || server.embeds != llamaHangThreshold+1 {
			t.Fatalf("stops=%d embeds=%d, want 1 stop and %d embeds (the requests and one probe)", server.stops, server.embeds, llamaHangThreshold+1)
		}
		if records := logs.snapshot(); len(records) != 1 || records[0].Level != slog.LevelWarn {
			t.Fatalf("probe failure produced %d records, want one warning", len(records))
		}
		if budget.remaining() != maxProcessRestarts {
			t.Fatal("stopping the server spent a restart before one was needed")
		}

		// The next call restarts the stopped server within the budget and the
		// count starts over.
		server.embedCtx = nil
		if _, err := embedWithRestarts(context.Background(), budget, server.ops(), []string{"text"}); err != nil {
			t.Fatalf("call after the stop = %v, want the restarted server to answer", err)
		}
		if server.starts != 1 || budget.remaining() != maxProcessRestarts-1 || server.hangs.timeouts != 0 {
			t.Fatalf("restart bookkeeping: starts=%d budget=%d timeouts=%d", server.starts, budget.remaining(), server.hangs.timeouts)
		}
	})

	t.Run("a probe that answers leaves the server alone and the count starts over", func(t *testing.T) {
		budget := newRestartBudget(maxProcessRestarts, processRestartWindow)
		server := &fakeLlamaServer{up: true, hangs: newHangs()}
		server.embedCtx = func(ctx context.Context, texts []string) error {
			if server.embeds > llamaHangThreshold { // the probe
				return nil
			}
			return hangingEmbed(ctx, texts)
		}

		var ctx context.Context
		var err error
		for call := 1; call <= llamaHangThreshold; call++ {
			ctx, err = embedUnder(callerDeadline, budget, server)
		}
		if !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errBackendUnresponsive) || !isCallerError(ctx, err) {
			t.Fatalf("error after an answered probe = %v, want the plain caller error", err)
		}
		if server.stops != 0 || server.embeds != llamaHangThreshold+1 || server.hangs.timeouts != 0 {
			t.Fatalf("stops=%d embeds=%d timeouts=%d, want the server kept and the count reset", server.stops, server.embeds, server.hangs.timeouts)
		}
		server.embedCtx = hangingEmbed
		_, _ = embedUnder(callerDeadline, budget, server)
		if server.embeds != llamaHangThreshold+2 {
			t.Fatalf("embeds = %d: the next timeout was probed again at once", server.embeds)
		}
	})

	t.Run("only deadlines count and only a success resets", func(t *testing.T) {
		budget := newRestartBudget(maxProcessRestarts, processRestartWindow)
		server := &fakeLlamaServer{up: true, embedCtx: hangingEmbed, hangs: newHangs()}
		canceled := func() {
			ctx, cancel := context.WithCancel(context.Background())
			time.AfterFunc(callerDeadline, cancel)
			if _, err := embedWithRestarts(ctx, budget, server.ops(), []string{"text"}); !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled call error = %v", err)
			}
		}
		deadline := func() { _, _ = embedUnder(callerDeadline, budget, server) }

		// Cancellations are neither counted nor do they reset the count.
		deadline()
		canceled()
		deadline()
		canceled()
		if server.embeds != 4 || server.stops != 0 {
			t.Fatalf("cancellations were counted: embeds=%d stops=%d", server.embeds, server.stops)
		}
		deadline() // the third deadline: probed, and the probe hangs
		if server.stops != 1 || server.embeds != 6 {
			t.Fatalf("third deadline: stops=%d embeds=%d, want a probe and a stop", server.stops, server.embeds)
		}

		// A success in between restarts the count.
		server.up, server.stops, server.embeds = true, 0, 0
		deadlineThenSuccess := func() {
			server.embedCtx = hangingEmbed
			deadline()
			deadline()
			server.embedCtx = nil
			if _, err := embedWithRestarts(context.Background(), budget, server.ops(), []string{"text"}); err != nil {
				t.Fatalf("successful call = %v", err)
			}
		}
		deadlineThenSuccess()
		deadlineThenSuccess()
		if server.stops != 0 {
			t.Fatalf("deadlines separated by successes were probed (stops=%d)", server.stops)
		}
	})
}

// A cold start can take longer than the callers' deadlines. The restart must
// not run under (and die with) the caller that began it, or every request
// would start the process, abandon and kill it, and get the budget token back:
// the backend would never come up and the budget would never limit it.
func TestLlamaRestartOutlivesTheCallerThatStartedIt(t *testing.T) {
	newServer := func() (*fakeLlamaServer, chan context.Context, chan error) {
		startCtxs := make(chan context.Context, 4)
		finish := make(chan error)
		server := &fakeLlamaServer{logger: discardEmbeddingLogger()}
		server.startCtx = func(ctx context.Context) error {
			startCtxs <- ctx
			select {
			case err := <-finish:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return server, startCtxs, finish
	}
	leave := func(t *testing.T, budget *restartBudget, server *fakeLlamaServer, startCtxs chan context.Context) {
		t.Helper()
		begin := time.Now()
		_, err := embedUnder(30*time.Millisecond, budget, server)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("caller error = %v, want its deadline error", err)
		}
		if elapsed := time.Since(begin); elapsed > 2*time.Second {
			t.Fatalf("the caller waited %s for the restart", elapsed)
		}
		if server.held == nil {
			t.Fatal("the caller left without handing the embedder lock to the running start")
		}
		var startCtx context.Context
		select {
		case startCtx = <-startCtxs:
		case <-time.After(5 * time.Second):
			t.Fatal("the restart never began")
		}
		if startCtx.Err() != nil {
			t.Fatalf("the caller's deadline ended the restart: %v", startCtx.Err())
		}
		if deadline, ok := startCtx.Deadline(); !ok || time.Until(deadline) < llamaRestartTimeout/2 {
			t.Fatalf("restart deadline = %v (set %v), want about %s from now", deadline, ok, llamaRestartTimeout)
		}
		select {
		case <-server.held:
			t.Fatal("the lock was released before the start ended")
		default:
		}
	}

	t.Run("a start that finishes after the caller left is kept for the next call", func(t *testing.T) {
		budget := newRestartBudget(maxProcessRestarts, processRestartWindow)
		server, startCtxs, finish := newServer()
		leave(t, budget, server, startCtxs)
		if budget.remaining() != maxProcessRestarts-1 {
			t.Fatalf("budget = %d: the caller's exit gave the restart token back", budget.remaining())
		}

		finish <- nil
		awaitClosed(t, server.held)
		if _, err := embedWithRestarts(context.Background(), budget, server.ops(), []string{"text"}); err != nil {
			t.Fatalf("call after the start finished = %v", err)
		}
		if server.starts != 1 || server.embeds != 1 || budget.remaining() != maxProcessRestarts-1 {
			t.Fatalf("starts=%d embeds=%d budget=%d, want the started server reused: 1, 1, %d",
				server.starts, server.embeds, budget.remaining(), maxProcessRestarts-1)
		}
	})

	t.Run("a start that fails on its own stays spent and is logged", func(t *testing.T) {
		logs := &captureHandler{}
		budget := newRestartBudget(maxProcessRestarts, processRestartWindow)
		server, startCtxs, finish := newServer()
		server.logger = slog.New(logs)
		leave(t, budget, server, startCtxs)

		finish <- errors.New("wait for llama-server: health check timed out")
		awaitClosed(t, server.held)
		deadline := time.Now().Add(5 * time.Second)
		for len(logs.snapshot()) == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if records := logs.snapshot(); len(records) != 1 || records[0].Level != slog.LevelWarn {
			t.Fatalf("failed detached start produced %d records, want one warning", len(records))
		}
		if budget.remaining() != maxProcessRestarts-1 || server.up {
			t.Fatalf("after the failed start: budget=%d up=%v, want the token spent and no server", budget.remaining(), server.up)
		}
	})
}

// The native embedder shares the lifecycle with the Docker one. A start that
// holds the lock (here: a stand-in that ends when its context does, like the
// health wait) must neither delay Status nor survive Close, and nothing may
// start once Close has begun.
func TestLlamaEmbedderCloseCancelsARunningStartAndStatusDoesNotWaitForIt(t *testing.T) {
	embedder := &llamaEmbedder{backend: "cpu"}
	embedder.mu.Lock() // what a detached start holds
	startCtx, endStart, err := embedder.lifecycle.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	startEnded := make(chan struct{})
	go func() {
		defer close(startEnded)
		<-startCtx.Done() // the health wait fails fast, the server is stopped
		endStart()
		embedder.mu.Unlock()
	}()

	var status Status
	returnsWithin(t, "Status during the start", 2*time.Second, func() { status = embedder.Status() })
	if status.State != llamaStateRestarting || status.Backend != "cpu" || status.Runtime != "llama.cpp" {
		t.Fatalf("Status during the start = %q/%q/%q, want %q on the cpu llama.cpp backend", status.State, status.Backend, status.Runtime, llamaStateRestarting)
	}

	var closeErr error
	returnsWithin(t, "Close during the start", 2*time.Second, func() { closeErr = embedder.Close() })
	awaitClosed(t, startEnded)
	if closeErr != nil {
		t.Fatalf("Close = %v", closeErr)
	}
	if state := embedder.Status().State; state != "closed" {
		t.Fatalf("Status after Close = %q, want closed", state)
	}
	if _, _, err := embedder.lifecycle.begin(context.Background()); !errors.Is(err, errLlamaClosing) {
		t.Fatalf("start after Close = %v, want errLlamaClosing", err)
	}
}
