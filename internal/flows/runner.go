package flows

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// Runner defaults.
const (
	DefaultMaxParallelRuns  = 8
	DefaultMaxQueuedPerFlow = 20
	eventRetention          = 2 * time.Minute
)

var (
	ErrQueueFull    = errors.New("the flow's run queue is full")
	ErrRunnerClosed = errors.New("the flow runner is shut down")
)

// RunnerConfig limits concurrent runs. Values of zero or less mean the defaults.
type RunnerConfig struct {
	// MaxParallelRuns caps the runs that execute at the same time.
	MaxParallelRuns int
	// MaxQueuedPerFlow caps a flow's runs queued behind its active run (queue
	// policy), and also, counted separately, the flow's runs that wait for a global
	// slot (parallel policy and test runs). Start refuses more with ErrQueueFull.
	MaxQueuedPerFlow int
}

// RunnerHooks observe runs. OnRunFinished is called for every run (also runs cancelled
// before they started); OnRunStarted only for runs that actually start executing.
type RunnerHooks struct {
	// OnRunStarted runs on the run's goroutine right before the engine starts, outside all locks.
	OnRunStarted func(rec RunRecord)
	// OnRunFinished runs after the run is persisted and its slot is released, outside all locks.
	OnRunFinished func(rec RunRecord, res RunResult)
}

// StartRequest asks the runner for a run. Start refuses it with ErrQueueFull when
// the run would wait and the flow already has RunnerConfig.MaxQueuedPerFlow runs
// waiting the same way: queued behind its active run (queue policy) or waiting for
// a global slot (parallel policy and test runs).
type StartRequest struct {
	Flow         *Flow
	Revision     int
	Mode         RunMode
	TriggerNode  string
	TriggerType  string
	TriggerData  map[string]any
	OnlyNode     string
	Timeout      time.Duration
	ParentRunID  string
	ParentNodeID string
}

// StartStatus tells what Start did.
type StartStatus string

const (
	StartStarted StartStatus = "started"
	StartQueued  StartStatus = "queued"
	StartSkipped StartStatus = "skipped"
)

// StartResult is returned by Start. RunID is empty for skipped triggers.
type StartResult struct {
	RunID  string      `json:"run_id,omitempty"`
	Status StartStatus `json:"status"`
}

type pendingRun struct {
	req     StartRequest
	rec     RunRecord
	counted bool
}

// activeRun is a launched run: its flow, for CancelFlow, and how to cancel it.
// cancelled records that Cancel, CancelFlow or Shutdown cancelled it already.
type activeRun struct {
	flowID    string
	cancel    context.CancelFunc
	cancelled bool
}

// Runner admits, executes and persists runs. Test runs bypass the per-flow policy
// but count against the global limit. Runs waiting for a global slot start in
// arrival order.
//
// Every run that has an id has an event log on the bus from the moment it is
// recorded, also while it waits, and the log ends with a run_finished event however
// the run ends: finished, cancelled while waiting, shut down or failed by a panic in
// the runner. A queued run that waits longer than the bus's leak horizon has its log
// recreated at launch, and subscribers attached during the wait see their channel
// close (see EventBus.Rearm).
type Runner struct {
	engine *Engine
	store  *Store
	bus    *EventBus
	hooks  RunnerHooks
	cfg    RunnerConfig
	logger *slog.Logger
	now    func() time.Time

	// startMu serializes Start, and Shutdown's closing, so Start can write the run
	// record while holding it instead of mu (see Start). It is taken before mu.
	startMu sync.Mutex

	mu        sync.Mutex
	closed    bool
	slots     int
	live      map[string]int
	flowQueue map[string][]*pendingRun
	waiting   []*pendingRun
	cancels   map[string]activeRun
	baseCtx   context.Context
	baseStop  context.CancelFunc
	wg        sync.WaitGroup

	// testOnEvent, when set before the first Start, sees every engine event before
	// it is published. Tests use it to inject failures into the event sink.
	testOnEvent func(RunEvent)
}

// NewRunner creates a runner. Call Shutdown when done.
func NewRunner(engine *Engine, store *Store, hooks RunnerHooks, cfg RunnerConfig, logger *slog.Logger) *Runner {
	if cfg.MaxParallelRuns <= 0 {
		cfg.MaxParallelRuns = DefaultMaxParallelRuns
	}
	if cfg.MaxQueuedPerFlow <= 0 {
		cfg.MaxQueuedPerFlow = DefaultMaxQueuedPerFlow
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{
		engine: engine, store: store, hooks: hooks, cfg: cfg, logger: logger,
		bus:       NewEventBus(eventRetention, engine.services.Now),
		now:       engine.services.Now,
		live:      map[string]int{},
		flowQueue: map[string][]*pendingRun{},
		cancels:   map[string]activeRun{},
		baseCtx:   ctx,
		baseStop:  cancel,
	}
}

// Bus returns the run event bus.
func (r *Runner) Bus() *EventBus { return r.bus }

// Start admits a run according to the flow's concurrency policy.
//
// The run record is written without holding the runner's lock, so Cancel, IsBusy
// and finishing runs never wait for the database; Starts are serialized instead.
// The policy is applied under the lock in two steps: before the record is written
// Start refuses the run (shut down, skip policy, full queue or too many runs of the
// flow waiting for a global slot, see RunnerConfig), after it the run is
// queued or admitted. In between only finishing and cancelled runs change the state
// (Shutdown waits for a Start in progress), and they only make the flow less busy:
// a run that passed the first step still fits, and one that found its flow busy
// starts right away if the flow became idle meanwhile.
func (r *Runner) Start(req StartRequest) (StartResult, error) {
	if req.Flow == nil || req.TriggerNode == "" {
		return StartResult{}, errors.New("a flow and a trigger node are required")
	}
	if req.Mode == "" {
		req.Mode = ModeLive
	}
	counted := req.Mode != ModeTest
	flowID := req.Flow.ID
	policy := req.Flow.Settings.Concurrency
	// Counted runs of queue and skip flows run one at a time.
	exclusive := counted && policy != ConcurrencyParallel

	r.startMu.Lock()
	defer r.startMu.Unlock()
	r.mu.Lock()
	closed, busy := r.closed, exclusive && r.busyLocked(flowID)
	full := busy && len(r.flowQueue[flowID]) >= r.cfg.MaxQueuedPerFlow
	if !exclusive && r.slots >= r.cfg.MaxParallelRuns {
		// Only Start adds runs like this one to the waiting list, so the cap holds
		// exactly; for a queue or skip flow's test run, its waiting live run counts too.
		full = r.waitingLocked(flowID) >= r.cfg.MaxQueuedPerFlow
	}
	r.mu.Unlock()
	switch {
	case closed:
		return StartResult{}, ErrRunnerClosed
	case busy && policy == ConcurrencySkip:
		return StartResult{Status: StartSkipped}, nil
	case full:
		return StartResult{}, ErrQueueFull
	}

	p, err := r.newPending(req, counted)
	if err != nil {
		return StartResult{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if exclusive && r.busyLocked(flowID) {
		r.flowQueue[flowID] = append(r.flowQueue[flowID], p)
		return StartResult{RunID: p.rec.ID, Status: StartQueued}, nil
	}
	if r.admitLocked(p) {
		return StartResult{RunID: p.rec.ID, Status: StartStarted}, nil
	}
	return StartResult{RunID: p.rec.ID, Status: StartQueued}, nil
}

// newPending records the run as queued and opens its event log, so the run can be
// subscribed to as soon as Start returns its id, also while it waits.
func (r *Runner) newPending(req StartRequest, counted bool) (*pendingRun, error) {
	rec := RunRecord{
		ID: NewRunID(), FlowID: req.Flow.ID, Revision: req.Revision, Mode: req.Mode,
		TriggerNode: req.TriggerNode, TriggerType: req.TriggerType, TriggerData: req.TriggerData,
		Status: RunQueued, StartedAt: r.now(), ParentRunID: req.ParentRunID, ParentNodeID: req.ParentNodeID,
	}
	if err := r.store.CreateRun(context.Background(), rec, req.Flow); err != nil {
		return nil, fmt.Errorf("record run: %w", err)
	}
	r.bus.Open(rec.ID)
	return &pendingRun{req: req, rec: rec, counted: counted}, nil
}

// admitLocked takes the flow slot for counted runs and launches the run when a
// global slot is free and no earlier run waits for one. Otherwise the run waits at
// the end of the global queue, so a flow's next queued run cannot overtake runs
// that waited for a slot before it.
func (r *Runner) admitLocked(p *pendingRun) bool {
	if p.counted {
		r.live[p.rec.FlowID]++
	}
	if r.slots < r.cfg.MaxParallelRuns && len(r.waiting) == 0 {
		r.launchLocked(p)
		return true
	}
	r.waiting = append(r.waiting, p)
	return false
}

func (r *Runner) launchLocked(p *pendingRun) {
	r.slots++
	ctx, cancel := context.WithCancel(r.baseCtx)
	r.cancels[p.rec.ID] = activeRun{flowID: p.rec.FlowID, cancel: cancel}
	r.wg.Add(1)
	go r.execute(ctx, cancel, p)
}

func (r *Runner) execute(ctx context.Context, cancel context.CancelFunc, p *pendingRun) {
	defer r.wg.Done()
	defer cancel()
	runID := p.rec.ID
	defer r.bus.Finish(runID) // idempotent; no way out leaves the log open
	// The log was opened when the run was queued; its leak clock must count the
	// run's own time only, and a sweep may have forgotten it during a long wait.
	r.bus.Rearm(runID)
	res := r.runAndRecord(ctx, p)
	r.bus.Finish(runID)
	rec := p.rec
	rec.Status, rec.ErrorCode, rec.ErrorMessage = res.Status, res.ErrorCode, res.ErrorMessage
	finished := res.FinishedAt
	rec.FinishedAt = &finished
	rec.DurationMS = res.DurationMS
	r.release(p)
	r.callHook(rec, res)
}

// runAndRecord executes the run and persists it. The engine recovers panics in
// nodes itself; a panic in the runner's own code on the way (store calls, the
// event sink) becomes a FLOW_RUNNER_PANIC result here, so execute always gets a
// result and frees the run's slots.
func (r *Runner) runAndRecord(ctx context.Context, p *pendingRun) (res RunResult) {
	bg := context.Background()
	runID := p.rec.ID
	// The sink runs on this goroutine (the engine's coordinator), so the deferred
	// recover may read what it recorded.
	lastSeq, ended := 0, false
	launched := r.now()
	defer func() {
		if v := recover(); v != nil {
			res = r.panicked(p, v, launched, lastSeq, ended)
		}
	}()
	if err := r.store.SetRunStatus(bg, runID, RunRunning); err != nil {
		r.logStoreError("flow run status not saved", runID, err)
	}
	if r.hooks.OnRunStarted != nil {
		started := p.rec
		started.Status = RunRunning
		func() {
			defer func() {
				if v := recover(); v != nil {
					r.logger.Error("flow run start hook panicked", "run", runID, "panic", v)
				}
			}()
			r.hooks.OnRunStarted(started)
		}()
	}
	sink := func(ev RunEvent) {
		if r.testOnEvent != nil {
			r.testOnEvent(ev)
		}
		r.bus.Publish(ev)
		lastSeq, ended = ev.Seq, ev.Type == EventRunFinished
		if ev.Type == EventStepFinished && ev.Step != nil {
			if err := r.store.SaveStep(bg, runID, ev.Seq, *ev.Step); err != nil {
				r.logStoreError("flow step not saved", runID, err, "node", ev.NodeID)
			}
		}
	}
	res = r.engine.Execute(ctx, RunRequest{
		RunID: runID, Flow: p.req.Flow, Revision: p.req.Revision, Mode: p.req.Mode,
		TriggerNode: p.req.TriggerNode, TriggerType: p.req.TriggerType, TriggerData: p.req.TriggerData,
		OnlyNode: p.req.OnlyNode, Timeout: p.req.Timeout,
	}, sink)
	if err := r.store.FinishRun(bg, runID, res); err != nil {
		r.logStoreError("flow run result not saved", runID, err)
	}
	return res
}

// panicked turns a panic v in the runner's code during a run into the run's result,
// a FLOW_RUNNER_PANIC error. launched is when the run left its queue, so the
// duration leaves out the wait. lastSeq and ended describe the events published so
// far. Recording the result is best effort: store errors, and panics, are only logged.
func (r *Runner) panicked(p *pendingRun, v any, launched time.Time, lastSeq int, ended bool) RunResult {
	runID := p.rec.ID
	r.logger.Error("flow runner panicked during a run", "run", runID, "panic", v, "stack", string(debug.Stack()))
	now := r.now()
	res := RunResult{Status: RunError, ErrorCode: "FLOW_RUNNER_PANIC",
		ErrorMessage: "the flow runner failed unexpectedly; the AuraGo log has the details",
		StartedAt:    launched, FinishedAt: now, DurationMS: now.Sub(launched).Milliseconds()}
	func() {
		defer func() {
			if v := recover(); v != nil {
				r.logger.Error("flow run result not saved after a runner panic", "run", runID, "panic", v)
			}
		}()
		if err := r.store.FinishRun(context.Background(), runID, res); err != nil {
			r.logStoreError("flow run result not saved", runID, err)
		}
		if !ended {
			r.endLog(runID, lastSeq+1, res)
		}
	}()
	return res
}

// endLog closes the event log of a run that ended without the engine's run_finished
// event (cancelled while waiting, shut down, runner panic). It publishes one with
// sequence number seq first, so every log ends with run_finished and subscribers can
// tell the end of the run from a dropped stream.
func (r *Runner) endLog(runID string, seq int, res RunResult) {
	r.bus.Publish(RunEvent{Seq: seq, RunID: runID, Type: EventRunFinished, Time: res.FinishedAt,
		Run: &RunSummary{Status: res.Status, ErrorCode: res.ErrorCode, ErrorMessage: res.ErrorMessage, DurationMS: res.DurationMS}})
	r.bus.Finish(runID)
}

// logStoreError logs a failed write of a run's state. ErrRunNotFound means the
// run's row is gone, normally because its flow was deleted while the run was
// queued or active (DeleteFlow cascades to the runs); that is expected and logged
// at Debug only. Other errors are logged at Warn.
func (r *Runner) logStoreError(msg, runID string, err error, attrs ...any) {
	level := slog.LevelWarn
	if errors.Is(err, ErrRunNotFound) {
		level = slog.LevelDebug
	}
	r.logger.Log(context.Background(), level, msg, append([]any{"run", runID, "error", err}, attrs...)...)
}

func (r *Runner) release(p *pendingRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.slots--
	delete(r.cancels, p.rec.ID)
	if p.counted {
		r.releaseFlowLocked(p.rec.FlowID)
	}
	for !r.closed && r.slots < r.cfg.MaxParallelRuns && len(r.waiting) > 0 {
		next := r.waiting[0]
		r.waiting[0] = nil // the backing array must not keep the run's trigger data
		r.waiting = r.waiting[1:]
		r.launchLocked(next)
	}
}

// releaseFlowLocked frees the flow slot and admits the next queued run of the flow.
func (r *Runner) releaseFlowLocked(flowID string) {
	r.live[flowID]--
	if r.live[flowID] > 0 {
		return
	}
	delete(r.live, flowID)
	if r.closed {
		return
	}
	if q := r.flowQueue[flowID]; len(q) > 0 {
		next := q[0]
		q[0] = nil // the backing array must not keep the run's trigger data
		if len(q) == 1 {
			delete(r.flowQueue, flowID)
		} else {
			r.flowQueue[flowID] = q[1:]
		}
		r.admitLocked(next)
	}
}

func (r *Runner) callHook(rec RunRecord, res RunResult) {
	if r.hooks.OnRunFinished == nil {
		return
	}
	defer func() {
		if v := recover(); v != nil {
			r.logger.Error("flow run hook panicked", "run", rec.ID, "panic", v)
		}
	}()
	r.hooks.OnRunFinished(rec, res)
}

// Cancel stops a running run or removes a queued one. It returns false for unknown runs.
// A run whose Start has not returned yet is not known to Cancel.
func (r *Runner) Cancel(runID string) bool {
	r.mu.Lock()
	if run, ok := r.cancels[runID]; ok {
		run.cancelled = true
		r.cancels[runID] = run
		r.mu.Unlock()
		run.cancel()
		return true
	}
	p := r.removeQueuedLocked(runID)
	r.mu.Unlock()
	if p == nil {
		return false
	}
	r.finishUnstarted(p, "FLOW_CANCELLED", "the run was cancelled before it started")
	return true
}

func (r *Runner) removeQueuedLocked(runID string) *pendingRun {
	for flowID, q := range r.flowQueue {
		for i, p := range q {
			if p.rec.ID != runID {
				continue
			}
			rest := append(q[:i:i], q[i+1:]...)
			if len(rest) == 0 {
				delete(r.flowQueue, flowID)
			} else {
				r.flowQueue[flowID] = rest
			}
			return p
		}
	}
	for i, p := range r.waiting {
		if p.rec.ID != runID {
			continue
		}
		r.waiting = append(r.waiting[:i:i], r.waiting[i+1:]...)
		if p.counted {
			r.releaseFlowLocked(p.rec.FlowID)
		}
		return p
	}
	return nil
}

// CancelFlow cancels every run of the flow that the runner knows, test runs included,
// and returns how many runs this call cancelled; a running run that Cancel, CancelFlow
// or Shutdown cancelled before is not counted again while it winds down. Running runs
// are cancelled through their context, like Cancel does, and end in the background.
// Runs queued behind the flow's active run or waiting for a global slot end at once
// with FLOW_CANCELLED; OnRunFinished is called for them before CancelFlow returns,
// outside all locks.
//
// CancelFlow first waits for a Start that is writing its run record, like Shutdown, so
// every run whose Start returned before CancelFlow was called is cancelled. A Start that
// begins later is not affected. A caller that deletes the flow therefore calls CancelFlow
// again once the flow row is gone: from then on Start cannot record a run of the flow.
func (r *Runner) CancelFlow(flowID string) int {
	r.startMu.Lock()
	r.mu.Lock()
	pending := append([]*pendingRun(nil), r.flowQueue[flowID]...)
	delete(r.flowQueue, flowID)
	released := 0
	kept := r.waiting[:0]
	for _, p := range r.waiting {
		if p.rec.FlowID != flowID {
			kept = append(kept, p)
			continue
		}
		pending = append(pending, p)
		if p.counted {
			released++
		}
	}
	clear(r.waiting[len(kept):]) // the backing array must not keep the runs' trigger data
	r.waiting = kept
	// The flow's queue is gone, so freeing the flow slots of its waiting runs admits nothing.
	for ; released > 0; released-- {
		r.releaseFlowLocked(flowID)
	}
	var cancels []context.CancelFunc
	for id, run := range r.cancels {
		if run.flowID == flowID && !run.cancelled {
			run.cancelled = true
			r.cancels[id] = run
			cancels = append(cancels, run.cancel)
		}
	}
	r.mu.Unlock()
	r.startMu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	for _, p := range pending {
		r.finishUnstarted(p, "FLOW_CANCELLED", "the run was cancelled before it started")
	}
	return len(cancels) + len(pending)
}

func (r *Runner) finishUnstarted(p *pendingRun, code, msg string) {
	now := r.now()
	res := RunResult{Status: RunCancelled, ErrorCode: code, ErrorMessage: msg, StartedAt: p.rec.StartedAt, FinishedAt: now}
	if err := r.store.FinishRun(context.Background(), p.rec.ID, res); err != nil {
		r.logStoreError("cancelled flow run not saved", p.rec.ID, err)
	}
	r.endLog(p.rec.ID, 1, res)
	rec := p.rec
	rec.Status, rec.ErrorCode, rec.ErrorMessage = RunCancelled, code, msg
	rec.FinishedAt = &now
	r.callHook(rec, res)
}

// IsBusy reports whether the flow has an admitted or queued live run.
func (r *Runner) IsBusy(flowID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.busyLocked(flowID)
}

func (r *Runner) busyLocked(flowID string) bool {
	return r.live[flowID] > 0 || len(r.flowQueue[flowID]) > 0
}

// waitingLocked counts the flow's runs that wait for a global slot.
func (r *Runner) waitingLocked(flowID string) int {
	n := 0
	for _, p := range r.waiting {
		if p.rec.FlowID == flowID {
			n++
		}
	}
	return n
}

// Subscribe forwards to the event bus.
func (r *Runner) Subscribe(runID string, afterSeq int) ([]RunEvent, <-chan RunEvent, func(), bool) {
	return r.bus.Subscribe(runID, afterSeq)
}

// Shutdown cancels queued runs (FLOW_SHUTDOWN), stops running ones and waits for them.
//
// It first waits for a Start that is writing its run record, which takes at most
// the database's busy timeout and does not observe ctx. Running runs are cancelled
// through their context; the engine gives nodes that do not return a grace of 30 s
// before it abandons them, so the runs end within about 30 s plus the time to
// persist their results. If ctx ends first, Shutdown returns ctx.Err() and the runs
// finish in the background. Called from a hook on a run's goroutine (OnRunStarted,
// or OnRunFinished of a run that executed), it waits until ctx ends, because that
// goroutine is one of those it waits for.
func (r *Runner) Shutdown(ctx context.Context) error {
	r.startMu.Lock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		r.startMu.Unlock()
		return nil
	}
	r.closed = true
	var pending []*pendingRun
	for _, q := range r.flowQueue {
		pending = append(pending, q...)
	}
	pending = append(pending, r.waiting...)
	r.flowQueue = map[string][]*pendingRun{}
	r.waiting = nil
	for id, run := range r.cancels { // baseStop below cancels them all
		run.cancelled = true
		r.cancels[id] = run
	}
	r.mu.Unlock()
	r.startMu.Unlock()

	r.baseStop()
	for _, p := range pending {
		r.finishUnstarted(p, "FLOW_SHUTDOWN", "AuraGo shut down before the run started")
	}
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
