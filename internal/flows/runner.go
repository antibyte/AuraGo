package flows

import (
	"context"
	"encoding/json"
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
	// DefaultMaxPendingRuns caps the runs waiting to start, in the flows' queues and the global
	// waiting list together (E5).
	DefaultMaxPendingRuns = 200
	// DefaultMaxPendingBytes caps the encoded trigger data those runs hold in memory (E5).
	DefaultMaxPendingBytes = 64 << 20
	// DefaultMaxParkedRuns caps the runs that wait in a logic.wait without a global slot (A7).
	DefaultMaxParkedRuns = 64
	eventRetention       = 2 * time.Minute
)

var (
	ErrQueueFull    = errors.New("the flow's run queue is full")
	ErrRunnerClosed = errors.New("the flow runner is shut down")
	// ErrRunnerQueueFull is ErrQueueFull for the runner as a whole: too many runs, or too much
	// trigger data, wait already (RunnerConfig.MaxPendingRuns, MaxPendingBytes).
	ErrRunnerQueueFull = fmt.Errorf("%w: too many flow runs are waiting in total", ErrQueueFull)
)

// RunnerConfig limits concurrent runs. Values of zero or less mean the defaults.
type RunnerConfig struct {
	// MaxParallelRuns caps the runs that execute at the same time.
	MaxParallelRuns int
	// MaxQueuedPerFlow caps a flow's runs queued behind its active run (queue
	// policy), and also, counted separately, the flow's runs that wait for a global
	// slot (parallel policy and test runs). Start refuses more with ErrQueueFull.
	MaxQueuedPerFlow int
	// Gate, when set, is asked right before a run executes that has StartRequest.Gated and
	// waited in a queue (behind its flow's active run or for a global slot); a run that started
	// at once is not asked again. A non-nil error (or a panic) cancels the run instead: it ends
	// cancelled with RunsPausedCode and the error's text, without executing, and OnRunFinished
	// reports it like a run cancelled while queued. It is called on the run's goroutine,
	// outside all locks. The Service sets it to its bridge's RunGate.
	Gate func() error
	// MaxSlotsPerFlow caps the global slots that the counted (non-test) runs of one flow hold at
	// the same time; it matters for parallel flows, which otherwise only obey MaxParallelRuns.
	// Zero or less means half of MaxParallelRuns, at least 1. A run over it waits in the global
	// queue without holding up the runs of other flows behind it (S5).
	MaxSlotsPerFlow int
	// MaxPendingRuns caps the runs that wait to start (queued behind their flow's run or waiting
	// for a global slot), all flows together; MaxPendingBytes caps the encoded trigger data they
	// hold. Start refuses a run that would wait beyond either with ErrRunnerQueueFull; one run
	// may always wait when none does. Values of zero or less mean the defaults.
	MaxPendingRuns  int
	MaxPendingBytes int
	// MaxParkedRuns caps the runs that gave their global slot back while a logic.wait waits;
	// beyond it a wait keeps its slot. Zero or less means DefaultMaxParkedRuns.
	MaxParkedRuns int
}

// RunnerHooks observe runs. OnRunFinished is called for every run (also runs cancelled
// before they started); OnRunStarted only for runs that actually start executing.
type RunnerHooks struct {
	// OnRunStarted runs on the run's goroutine right before the engine starts, outside all
	// locks. doc is the document the run executes (its StartRequest's Flow), shared with the
	// engine: read-only. The Service keeps it for the run's finish report, so the report
	// does not depend on a stored version (Store.GetRunDoc).
	OnRunStarted func(rec RunRecord, doc *Flow)
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
	// Gated marks a live run that a trigger started without anyone asking at that moment
	// (Mission Control's triggers and Run, the Date/Time timers): when it has to wait, it asks
	// RunnerConfig.Gate again before it executes.
	Gated bool
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

// RunsPausedCode ends a gated run that waited in a queue while RunnerConfig.Gate refused
// automatic runs (missions read-only, flows switched off): it was cancelled, never executed.
const RunsPausedCode = "FLOW_RUNS_PAUSED"

type pendingRun struct {
	req     StartRequest
	rec     RunRecord
	counted bool
	// waited is set when the run joined a queue (its flow's or the global one); only such a
	// run asks RunnerConfig.Gate before it executes.
	waited bool
	size   int      // encoded size of the trigger data (MaxPendingBytes)
	ticket uint64   // arrival order in the global queue (dispatchLocked)
	slot   *runSlot // set at launch
}

// activeRun is a launched run: its flow and mode, for CancelFlow and CancelFlowMode, and
// how to cancel it. cancelled records that Cancel, CancelFlow, CancelFlowMode or Shutdown
// cancelled it already.
type activeRun struct {
	flowID    string
	mode      RunMode
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

	mu         sync.Mutex
	closed     bool
	slots      int
	live       map[string]int
	flowSlots  map[string]int // global slots held by counted runs, per flow (MaxSlotsPerFlow)
	flowQueue  map[string][]*pendingRun
	waiting    []*pendingRun
	resumers   []*resumeWait // parked runs waiting for a slot again (A7)
	parked     int           // runs whose global slot is given back (A7)
	nextTicket uint64        // arrival order of waiting runs and resumers
	cancels    map[string]activeRun
	baseCtx    context.Context
	baseStop   context.CancelFunc
	wg         sync.WaitGroup

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
	if cfg.MaxPendingRuns <= 0 {
		cfg.MaxPendingRuns = DefaultMaxPendingRuns
	}
	if cfg.MaxPendingBytes <= 0 {
		cfg.MaxPendingBytes = DefaultMaxPendingBytes
	}
	if cfg.MaxParkedRuns <= 0 {
		cfg.MaxParkedRuns = DefaultMaxParkedRuns
	}
	if cfg.MaxSlotsPerFlow <= 0 {
		cfg.MaxSlotsPerFlow = max(1, cfg.MaxParallelRuns/2)
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
		flowSlots: map[string]int{},
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
	// Measured before the locks: a waiting run holds its trigger data in memory (MaxPendingBytes).
	size := triggerDataSize(req.TriggerData)

	r.startMu.Lock()
	defer r.startMu.Unlock()
	r.mu.Lock()
	closed, busy := r.closed, exclusive && r.busyLocked(flowID)
	full := busy && len(r.flowQueue[flowID]) >= r.cfg.MaxQueuedPerFlow
	slotless := r.slots >= r.cfg.MaxParallelRuns || !r.fitsLocked(flowID, counted)
	if !exclusive && slotless {
		// Only Start adds runs like this one to the waiting list, so the cap holds
		// exactly; for a queue or skip flow's test run, its waiting live run counts too.
		full = r.waitingLocked(flowID) >= r.cfg.MaxQueuedPerFlow
	}
	crowded := false
	if (busy || slotless) && !(busy && policy == ConcurrencySkip) {
		n, bytes := r.pendingLocked()
		crowded = n >= r.cfg.MaxPendingRuns || (n > 0 && bytes+size > r.cfg.MaxPendingBytes)
	}
	r.mu.Unlock()
	switch {
	case closed:
		return StartResult{}, ErrRunnerClosed
	case busy && policy == ConcurrencySkip:
		return StartResult{Status: StartSkipped}, nil
	case full:
		return StartResult{}, ErrQueueFull
	case crowded:
		return StartResult{}, ErrRunnerQueueFull
	}

	p, err := r.newPending(req, counted, size)
	if err != nil {
		return StartResult{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if exclusive && r.busyLocked(flowID) {
		p.waited = true
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
func (r *Runner) newPending(req StartRequest, counted bool, size int) (*pendingRun, error) {
	rec := RunRecord{
		ID: NewRunID(), FlowID: req.Flow.ID, Revision: req.Revision, Mode: req.Mode,
		TriggerNode: req.TriggerNode, TriggerType: req.TriggerType, TriggerData: req.TriggerData,
		Status: RunQueued, StartedAt: r.now(), ParentRunID: req.ParentRunID, ParentNodeID: req.ParentNodeID,
	}
	if err := r.store.CreateRun(context.Background(), rec, req.Flow); err != nil {
		return nil, fmt.Errorf("record run: %w", err)
	}
	r.bus.Open(rec.ID)
	return &pendingRun{req: req, rec: rec, counted: counted, size: size}, nil
}

// triggerDataSize is the encoded size of a run's trigger data, which a waiting run holds in
// memory; data that cannot be encoded counts as MaxOutputBytes (the engine replaces it anyway).
func triggerDataSize(data map[string]any) int {
	if len(data) == 0 {
		return 0
	}
	enc, err := json.Marshal(data)
	if err != nil {
		return MaxOutputBytes
	}
	return len(enc)
}

// pendingLocked counts the runs that wait to start and the trigger data they hold. Starts are
// serialized and finishing runs only shrink it, so the caps hold exactly.
func (r *Runner) pendingLocked() (n, bytes int) {
	for _, q := range r.flowQueue {
		for _, p := range q {
			n, bytes = n+1, bytes+p.size
		}
	}
	for _, p := range r.waiting {
		n, bytes = n+1, bytes+p.size
	}
	return n, bytes
}

// admitLocked takes the flow slot for counted runs and puts the run at the end of the global
// queue; dispatchLocked launches it at once when a slot is free and nothing that may start waits
// before it. It reports whether the run started. A flow's next queued run therefore cannot
// overtake runs that waited for a slot before it.
func (r *Runner) admitLocked(p *pendingRun) bool {
	if p.counted {
		r.live[p.rec.FlowID]++
	}
	// A run that does not start at once waited, so it asks the gate when it executes (batch 3).
	// Decided before the launch: the run's goroutine reads the flag.
	if !(r.slots < r.cfg.MaxParallelRuns && r.fitsLocked(p.rec.FlowID, p.counted) && r.nextWaitingLocked() < 0) {
		p.waited = true
	}
	r.nextTicket++
	p.ticket = r.nextTicket
	r.waiting = append(r.waiting, p)
	r.dispatchLocked()
	_, started := r.cancels[p.rec.ID]
	return started
}

// dispatchLocked hands out free global slots in arrival (ticket) order, to waiting runs and to
// parked runs that want a slot back (A7) alike. A run whose flow already holds its
// share of slots (MaxSlotsPerFlow) is passed over and keeps its place, so one flooding parallel
// flow cannot hold up the runs of other flows queued behind its own (S5). Every run in r.waiting
// has p.waited set unless admitLocked launches it at once, so Runner.execute asks the gate.
func (r *Runner) dispatchLocked() {
	for !r.closed && r.slots < r.cfg.MaxParallelRuns {
		wi, ri := r.nextWaitingLocked(), r.nextResumerLocked()
		if wi < 0 && ri < 0 {
			return
		}
		if ri >= 0 && (wi < 0 || r.resumers[ri].ticket < r.waiting[wi].ticket) {
			w := r.resumers[ri]
			r.resumers = append(r.resumers[:ri:ri], r.resumers[ri+1:]...)
			r.grantLocked(w) // a run that already started: no gate check, no new record
			continue
		}
		p := r.waiting[wi]
		// A new slice: the old backing array must not keep the run's trigger data.
		r.waiting = append(r.waiting[:wi:wi], r.waiting[wi+1:]...)
		r.launchLocked(p)
	}
}

// nextWaitingLocked returns the index of the first waiting run that may take a slot, or -1.
func (r *Runner) nextWaitingLocked() int {
	for i, p := range r.waiting {
		if r.fitsLocked(p.rec.FlowID, p.counted) {
			return i
		}
	}
	return -1
}

// fitsLocked reports whether a run of flowID may take one more global slot: a test run always,
// a counted run while its flow holds fewer than MaxSlotsPerFlow.
func (r *Runner) fitsLocked(flowID string, counted bool) bool {
	return !counted || r.flowSlots[flowID] < r.cfg.MaxSlotsPerFlow
}

// dropFlowSlotLocked gives back one of the flow's global slots.
func (r *Runner) dropFlowSlotLocked(flowID string) {
	if r.flowSlots[flowID]--; r.flowSlots[flowID] <= 0 {
		delete(r.flowSlots, flowID)
	}
}

func (r *Runner) launchLocked(p *pendingRun) {
	r.slots++
	if p.counted {
		r.flowSlots[p.rec.FlowID]++
	}
	p.slot = &runSlot{r: r, flowID: p.rec.FlowID, counted: p.counted, held: true}
	ctx, cancel := context.WithCancel(r.baseCtx)
	r.cancels[p.rec.ID] = activeRun{flowID: p.rec.FlowID, mode: p.rec.Mode, cancel: cancel}
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
	if reason := r.gateRefusal(p); reason != nil {
		// Recorded first, then the slots go to the next waiting run (which asks the gate in
		// turn), then the hook, as for a run that executed.
		msg := truncateRunes("the run waited in a queue and was not started: "+reason.Error(), maxErrorMessageRunes)
		rec, res := r.recordUnstarted(p, RunsPausedCode, msg)
		r.logger.Info("a queued flow run was not started because automatic runs are not allowed",
			"run", runID, "flow", p.rec.FlowID, "reason", reason.Error())
		r.release(p)
		r.callHook(rec, res)
		return
	}
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

// gateRefusal asks RunnerConfig.Gate whether p, a gated live run that waited in a queue, may
// still execute. It returns nil when it may or is not asked; a panicking gate counts as a
// refusal, so the brake fails closed.
func (r *Runner) gateRefusal(p *pendingRun) (err error) {
	if r.cfg.Gate == nil || !p.req.Gated || !p.waited || p.req.Mode != ModeLive {
		return nil
	}
	defer func() {
		if v := recover(); v != nil {
			r.logger.Error("flow run gate panicked; the queued run is not started", "run", p.rec.ID, "panic", v)
			err = errors.New("the run gate failed")
		}
	}()
	return r.cfg.Gate()
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
			r.hooks.OnRunStarted(started, p.req.Flow)
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
	var slot RunSlot // never a typed nil
	if p.slot != nil {
		slot = p.slot
	}
	res = r.engine.Execute(ctx, RunRequest{
		RunID: runID, Flow: p.req.Flow, Revision: p.req.Revision, Mode: p.req.Mode,
		TriggerNode: p.req.TriggerNode, TriggerType: p.req.TriggerType, TriggerData: p.req.TriggerData,
		OnlyNode: p.req.OnlyNode, Timeout: p.req.Timeout, Slot: slot,
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
	if p.slot == nil || !r.endSlotLocked(p.slot) {
		r.slots--
		if p.counted {
			r.dropFlowSlotLocked(p.rec.FlowID)
		}
	} else {
		r.parked-- // the run ended while it waited without a slot (cancelled, shut down)
	}
	delete(r.cancels, p.rec.ID)
	if p.counted {
		r.releaseFlowLocked(p.rec.FlowID)
	}
	r.dispatchLocked()
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

// recordUnstarted persists p as cancelled with code and msg, ends its event log and returns the
// record and result for OnRunFinished. It does not touch the runner's slots.
func (r *Runner) recordUnstarted(p *pendingRun, code, msg string) (RunRecord, RunResult) {
	now := r.now()
	res := RunResult{Status: RunCancelled, ErrorCode: code, ErrorMessage: msg, StartedAt: p.rec.StartedAt, FinishedAt: now}
	if err := r.store.FinishRun(context.Background(), p.rec.ID, res); err != nil {
		r.logStoreError("cancelled flow run not saved", p.rec.ID, err)
	}
	r.endLog(p.rec.ID, 1, res)
	rec := p.rec
	rec.Status, rec.ErrorCode, rec.ErrorMessage = RunCancelled, code, msg
	rec.FinishedAt = &now
	return rec, res
}

func (r *Runner) finishUnstarted(p *pendingRun, code, msg string) {
	rec, res := r.recordUnstarted(p, code, msg)
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
