package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestCoAgentSlotsWaitForActualWorkerEnd(t *testing.T) {
	r := NewCoAgentRegistry(1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(r.Context())
	id, _, err := r.RegisterWorker("coagent", "first", "a", cancel, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	queued, state, err := r.RegisterWorker("coagent", "second", "b", nil, 2, true)
	if err != nil || state != CoAgentQueued {
		t.Fatalf("queue: %s %v", state, err)
	}
	if err := r.Stop(id); err != nil {
		t.Fatal(err)
	}
	<-ctx.Done()
	if r.AvailableSlots() != 0 {
		t.Fatal("cancel released a live worker's slot")
	}
	if got, _ := r.GetStatus(queued); got["state"] != string(CoAgentQueued) {
		t.Fatal(got)
	}
	if r.Cleanup(-time.Second) != 0 {
		t.Fatal("cleanup removed a live worker")
	}
	r.WorkerDone(id)
	if err := r.WaitForStart(queued, context.Background()); err != nil {
		t.Fatal(err)
	}
	r.Complete(queued, "done", 0, 0)
	if r.AvailableSlots() != 0 {
		t.Fatal("publication released a live worker's slot")
	}
	r.WorkerDone(queued)
	if r.AvailableSlots() != 1 {
		t.Fatal("worker end did not release capacity")
	}
	r.Shutdown()
}

func TestCoAgentShutdownPublishesCancellationBeforeCancellingContext(t *testing.T) {
	r := NewCoAgentRegistry(1, slog.New(slog.NewTextHandler(io.Discard, nil)))
	contextCancelledBeforeStop := false
	id, _, err := r.RegisterWorker("coagent", "fixture", "session", func() {
		contextCancelledBeforeStop = r.Context().Err() != nil
	}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		defer r.WorkerDone(id)
		<-r.Context().Done()
		r.FailIfActive(id, "fixture context cancelled")
	}()
	r.Shutdown()
	if contextCancelledBeforeStop {
		t.Fatal("registry context was cancelled before workers were marked cancelled")
	}
	status, err := r.GetStatus(id)
	if err != nil || status["state"] != string(CoAgentCancelled) {
		t.Fatalf("status=%v err=%v", status, err)
	}
}

func TestCoAgentAdmissionAndShutdownAreAtomic(t *testing.T) {
	r := NewCoAgentRegistry(1, slog.Default())
	var wg sync.WaitGroup
	accepted := make(chan string, 32)
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if id, _, err := r.RegisterWorker("coagent", "task", "session", nil, 2, false); err == nil {
				accepted <- id
			}
		}()
	}
	wg.Wait()
	close(accepted)
	var ids []string
	for id := range accepted {
		ids = append(ids, id)
	}
	if len(ids) != 1 {
		t.Fatalf("accepted %d workers for one slot", len(ids))
	}
	queued, _, err := r.RegisterWorker("coagent", "queued", "session", nil, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	r.StartCleanupLoop()
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); r.StopCleanupLoop() }()
	}
	wg.Wait()
	finished := make(chan struct{})
	go func() { r.Shutdown(); close(finished) }()
	<-r.Context().Done()
	if _, _, err := r.RegisterWorker("coagent", "late", "session", nil, 2, true); err == nil {
		t.Fatal("shutdown admitted work")
	}
	select {
	case <-finished:
		t.Fatal("shutdown did not await workers")
	default:
	}
	if err := r.WaitForStart(queued, r.Context()); err == nil {
		t.Fatal("cancelled queue started")
	}
	r.WorkerDone(ids[0])
	r.WorkerDone(queued)
	<-finished
	r.Shutdown()
}

func TestCoAgentDispatchIsSessionScoped(t *testing.T) {
	r := NewCoAgentRegistry(2, slog.Default())
	a, _, _ := r.RegisterWorker("coagent", "private-a", "a", nil, 2, true)
	b, _, _ := r.RegisterWorker("coagent", "private-b", "b", nil, 2, true)
	defer func() { r.WorkerDone(a); r.WorkerDone(b); r.Shutdown() }()
	cfg := &config.Config{}
	cfg.CoAgents.Enabled = true
	cfg.CoAgents.MaxConcurrent = 2
	dc := &DispatchContext{Cfg: cfg, Logger: slog.Default(), CoAgentRegistry: r, SessionID: "a", UserContext: "stop all co-agents"}
	for _, op := range []string{"list", "get_result", "stop"} {
		out, _ := dispatchPlatform(context.Background(), ToolCall{Action: "co_agent", Operation: op, CoAgentID: b}, dc)
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &payload); err != nil {
			t.Fatal(err, out)
		}
		if strings.Contains(out, "private-b") {
			t.Fatalf("%s leaked another session: %s", op, out)
		}
		if op != "list" && payload["status"] != "error" {
			t.Fatalf("%s accepted foreign ID: %s", op, out)
		}
	}
	if _, err := r.WaitForResultForSession(context.Background(), "a", b); err == nil {
		t.Fatal("foreign wait accepted")
	}
	if _, err := r.GetStatusForSession("", a); err == nil {
		t.Fatal("missing session authenticated")
	}
	dispatchPlatform(context.Background(), ToolCall{Action: "co_agent", Operation: "stop_all"}, dc)
	if status, _ := r.GetStatus(b); status["state"] != string(CoAgentRunning) {
		t.Fatal("foreign worker stopped", status)
	}
	if len(r.List()) != 2 {
		t.Fatal("administrative overview lost entries")
	}
}

func TestCoAgentConcurrentStartStopAndShutdown(t *testing.T) {
	r := NewCoAgentRegistry(4, slog.New(slog.NewTextHandler(io.Discard, nil)))
	release := make(chan struct{})
	first, _, err := r.RegisterWorker("coagent", "held", "session", nil, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	go func() { defer r.WorkerDone(first); <-release }()
	start := make(chan struct{})
	var callers sync.WaitGroup
	for range 32 {
		callers.Add(2)
		go func() {
			defer callers.Done()
			<-start
			if id, _, err := r.RegisterWorker("coagent", "racing", "session", nil, 2, true); err == nil {
				go func() { defer r.WorkerDone(id); <-release }()
			}
		}()
		go func() { defer callers.Done(); <-start; r.StopAllForSession("session") }()
	}
	shutdown := make(chan struct{})
	go func() { <-start; r.Shutdown(); close(shutdown) }()
	close(start)
	callers.Wait()
	<-r.Context().Done()
	select {
	case <-shutdown:
		t.Fatal("shutdown returned before held workers ended")
	default:
	}
	close(release)
	<-shutdown
	if got := r.runningCount.Load(); got != 0 {
		t.Fatalf("%d occupied slots after shutdown", got)
	}
}
