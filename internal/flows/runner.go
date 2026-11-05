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
	DurableEnabled   bool
	DurableAuthority DurableAuthority
	DurableHistory   DurableHistoryProjector
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
	Gated        bool
	blocks       *blockPlan
	missionID    string
	enabledCheck func() error
	durable      bool
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
	waited        bool
	size          int      // encoded size of the trigger data (MaxPendingBytes)
	ticket        uint64   // arrival order in the global queue (dispatchLocked)
	slot          *runSlot // set at launch
	durableCtx    context.Context
	durableCancel context.CancelFunc
	durableToken  *blockAdmission
	durableResume *durableResume
}

// activeRun is a launched run: its flow and mode, for CancelFlow and CancelFlowMode, and
// how to cancel it. cancelled records that Cancel, CancelFlow, CancelFlowMode or Shutdown
// cancelled it already.
type activeRun struct {
	flowID    string
	mode      RunMode
	durable   bool
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

	mu                   sync.Mutex
	closed               bool
	slots                int
	live                 map[string]int
	durable              map[string]durableReservation
	durableStarted       bool
	durableExpiryStarted bool
	recoveryErr          error
	flowSlots            map[string]int // global slots held by counted runs, per flow (MaxSlotsPerFlow)
	flowQueue            map[string][]*pendingRun
	waiting              []*pendingRun
	resumers             []*resumeWait // parked runs waiting for a slot again (A7)
	parked               int           // runs whose global slot is given back (A7)
	nextTicket           uint64        // arrival order of waiting runs and resumers
	cancels              map[string]activeRun
	baseCtx              context.Context
	baseStop             context.CancelFunc
	blockAdmissionCtx    context.Context
	blockAdmissionStop   context.CancelFunc
	blockAdmissions      chan struct{}
	blockAdmissionTokens blockAdmissionTracker
	durableActive        blockAdmissionTracker
	durableDiagnostics   chan struct{}
	durableSlots         chan struct{}
	wg                   sync.WaitGroup

	// testOnEvent, when set before the first Start, sees every engine event before
	// it is published. Tests use it to inject failures into the event sink.
	testOnEvent func(RunEvent)
	// testWrapDurableSet is installed only by tests before a recovery worker starts.
	testWrapDurableSet func(ExecuteFunc) ExecuteFunc
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
	blockCtx, blockCancel := context.WithCancel(context.Background())
	r := &Runner{
		engine: engine, store: store, hooks: hooks, cfg: cfg, logger: logger,
		bus:                NewEventBus(eventRetention, engine.services.Now),
		now:                engine.services.Now,
		live:               map[string]int{},
		durable:            map[string]durableReservation{},
		flowSlots:          map[string]int{},
		flowQueue:          map[string][]*pendingRun{},
		cancels:            map[string]activeRun{},
		baseCtx:            ctx,
		baseStop:           cancel,
		blockAdmissionCtx:  blockCtx,
		blockAdmissionStop: blockCancel,
		blockAdmissions:    make(chan struct{}, 4),
		durableDiagnostics: make(chan struct{}, 1),
		durableSlots:       make(chan struct{}, maxDurableCheckpoints),
	}
	reservations, err := store.durableReservations(context.Background())
	if err != nil {
		r.recoveryErr = err
	} else {
		for _, reservation := range reservations {
			r.durable[reservation.RunID] = reservation
			if reservation.Incompatible || !reservation.RunStatus.Terminal() {
				r.live[reservation.FlowID]++
			}
		}
	}
	return r
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
	return r.StartContext(context.Background(), req)
}

// StartContext bounds block-plan admission to the caller's context. Once
// admitted, a run continues under the runner's own shutdown/cancellation context.
func (r *Runner) StartContext(ctx context.Context, req StartRequest) (StartResult, error) {
	if req.Flow == nil || req.TriggerNode == "" {
		return StartResult{}, errors.New("a flow and a trigger node are required")
	}
	if req.Flow.Kind == KindBlock {
		return StartResult{}, ErrWrongFlowKind
	}
	if req.Mode == "" {
		req.Mode = ModeLive
	}
	r.mu.Lock()
	closedBeforePlan := r.closed
	r.mu.Unlock()
	if closedBeforePlan {
		return StartResult{}, ErrRunnerClosed
	}
	hasDurable := false
	for _, node := range req.Flow.Nodes {
		if node.Type == durableWaitType || node.Type == durableApprovalType {
			hasDurable = true
			break
		}
	}
	if hasDurable {
		if !r.cfg.DurableEnabled {
			return StartResult{}, ErrDurableDisabled
		}
		r.mu.Lock()
		ready := r.durableStarted
		r.mu.Unlock()
		if !ready {
			return StartResult{}, ErrDurableUnavailable
		}
		if r.cfg.DurableAuthority == nil || r.cfg.DurableHistory == nil || r.engine.services.CheckpointKeys == nil {
			return StartResult{}, ErrDurableUnavailable
		}
		if req.Mode != ModeLive || req.TriggerType != "manual" || req.OnlyNode != "" || req.Revision <= 0 || req.missionID == "" || !durableGraphAllowed(req.Flow) {
			return StartResult{}, ErrDurableInvalidGraph
		}
		trigger := req.Flow.NodeByID(req.TriggerNode)
		if trigger == nil || trigger.Type != TypeTriggerManual || trigger.Settings.Disabled {
			return StartResult{}, ErrDurableInvalidGraph
		}
		if _, err := durableLocation(req.Flow, r.engine.services.Loc()); err != nil {
			return StartResult{}, err
		}
		req.durable = true
	}
	durableSlotOwned := false
	if hasDurable {
		select {
		case r.durableSlots <- struct{}{}:
			durableSlotOwned = true
		default:
			return StartResult{}, ErrDurableCapacity
		}
		defer func() {
			if durableSlotOwned {
				<-r.durableSlots
			}
		}()
	}
	admissionCtx := context.Background()
	var cancel context.CancelFunc
	var stop func() bool
	var token *blockAdmission
	defer func() {
		if token != nil {
			r.blockAdmissionTokens.remove(token)
		}
		if stop != nil {
			stop()
		}
		if cancel != nil {
			cancel()
		}
	}()
	for _, n := range req.Flow.Nodes {
		if (!hasDurable && n.Type != blockCallType && n.Type != blockEachType) || (hasDurable && n.Type != durableWaitType && n.Type != durableApprovalType) {
			continue
		}
		if ctx == nil {
			return StartResult{}, ErrBlockInvalid
		}
		admissionCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		stop = context.AfterFunc(r.blockAdmissionCtx, cancel)
		if r.blockAdmissionCtx.Err() != nil {
			return StartResult{}, ErrRunnerClosed
		}
		if err := admissionCtx.Err(); err != nil {
			return StartResult{}, err
		}
		select {
		case r.blockAdmissions <- struct{}{}:
			defer func() { <-r.blockAdmissions }()
		default:
			return StartResult{}, ErrRunnerQueueFull
		}
		token = r.blockAdmissionTokens.add(req.Flow.ID, req.missionID, req.Mode, cancel, hasDurable)
		if hasDurable {
			if containsDurableApproval(req.Flow) {
				approvalAuthority, ok := r.cfg.DurableAuthority.(DurableApprovalAuthority)
				if !ok || approvalAuthority.DurableApprovalAllowed(admissionCtx) != nil {
					return StartResult{}, ErrDurableApprovalAuthUnavailable
				}
			}
			enabled, err := r.cfg.DurableAuthority.DurableEnabled(admissionCtx)
			if err != nil {
				return StartResult{}, ErrDurableUnavailable
			}
			if !enabled {
				return StartResult{}, ErrDurableDisabled
			}
			if err := r.cfg.DurableAuthority.DurableAllowed(admissionCtx); err != nil {
				return StartResult{}, ErrDurableUnavailable
			}
			if err := r.cfg.DurableAuthority.AuthorizeDurable(admissionCtx, req.Flow.ID, req.missionID, req.Revision); err != nil {
				return StartResult{}, ErrDurableUnavailable
			}
		}
		if req.enabledCheck != nil {
			if err := req.enabledCheck(); err != nil {
				return StartResult{}, err
			}
		}
		if err := admissionCtx.Err(); err != nil {
			return StartResult{}, err
		}
		data, err := req.Flow.Marshal()
		if err != nil {
			return StartResult{}, err
		}
		req.Flow, err = ParseFlow(data)
		if err != nil {
			return StartResult{}, err
		}
		if !hasDurable {
			req.blocks, err = prepareBlockPlan(admissionCtx, r.store, req.Flow, r.engine.services)
		}
		if err != nil {
			if admissionCtx.Err() != nil {
				return StartResult{}, admissionCtx.Err()
			}
			return StartResult{}, err
		}
		break
	}
	if req.Mode != ModeTest && r.recoveryErr != nil {
		return StartResult{}, fmt.Errorf("durable run recovery unavailable: %w", r.recoveryErr)
	}
	counted := req.Mode != ModeTest
	flowID := req.Flow.ID
	policy := req.Flow.Settings.Concurrency
	// Counted runs of queue and skip flows run one at a time.
	exclusive := counted && policy != ConcurrencyParallel
	// Measured before the locks: a waiting run holds its trigger data in memory (MaxPendingBytes).
	size := triggerDataSize(req.TriggerData)
	// The gate is external code and must never run under the runner's locks.
	if (req.blocks != nil || req.durable) && req.Gated && req.Mode == ModeLive {
		if err := r.checkGate(""); err != nil {
			return StartResult{}, err
		}
	}

	r.startMu.Lock()
	startLocked := true
	defer func() {
		if startLocked {
			r.startMu.Unlock()
		}
	}()
	r.mu.Lock()
	closed, busy := r.closed, exclusive && r.busyLocked(flowID)
	full := busy && len(r.flowQueue[flowID]) >= r.cfg.MaxQueuedPerFlow
	slotless := r.slots >= r.cfg.MaxParallelRuns || !r.fitsLocked(flowID, counted)
	if !exclusive && slotless {
		// For a queue or skip flow's test run, its waiting live run counts too. The check is
		// made before the second locked step: a resumer (A7) can take the last free slot in
		// between, so a run may then wait past MaxQueuedPerFlow by one, at most one per race.
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

	if req.blocks != nil || req.durable {
		if err := admissionCtx.Err(); err != nil {
			return StartResult{}, err
		}
	}
	p, err := r.newPending(admissionCtx, req, counted, size)
	if err != nil {
		if req.blocks != nil && admissionCtx.Err() != nil {
			return StartResult{}, admissionCtx.Err()
		}
		return StartResult{}, err
	}
	var result StartResult
	accept := func() {
		if p.rec.Durable {
			p.durableCtx, p.durableCancel = context.WithCancel(r.baseCtx)
			p.durableToken = r.durableActive.add(p.rec.FlowID, p.req.missionID, p.rec.Mode, p.durableCancel, true)
			durableSlotOwned = false // owned by p until it stops or suspends
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		result = StartResult{RunID: p.rec.ID, Status: StartQueued}
		if exclusive && r.busyLocked(flowID) {
			p.waited = true
			r.flowQueue[flowID] = append(r.flowQueue[flowID], p)
		} else if r.admitLocked(p) {
			result.Status = StartStarted
		}
	}
	if token != nil {
		if err := r.blockAdmissionTokens.transfer(token, admissionCtx, accept); err != nil {
			// The commit succeeded, but revocation won before the run was accepted.
			// Finish its recorded header outside runner locks; never retry the commit.
			r.startMu.Unlock()
			startLocked = false
			r.finishUnstarted(p, "FLOW_CANCELLED", "the run was cancelled before it started")
			return StartResult{}, err
		}
	} else {
		accept()
	}
	return result, nil
}

// newPending records the run as queued and opens its event log, so the run can be
// subscribed to as soon as Start returns its id, also while it waits.
func (r *Runner) newPending(ctx context.Context, req StartRequest, counted bool, size int) (*pendingRun, error) {
	rec := RunRecord{
		ID: NewRunID(), FlowID: req.Flow.ID, Revision: req.Revision, Mode: req.Mode,
		TriggerNode: req.TriggerNode, TriggerType: req.TriggerType, TriggerData: req.TriggerData,
		Status: RunQueued, StartedAt: r.now(), ParentRunID: req.ParentRunID, ParentNodeID: req.ParentNodeID, Durable: req.durable,
	}
	var blocks []resolvedBlock
	if req.blocks != nil {
		blocks = req.blocks.blocks
	}
	if err := r.store.createRunWithBlocks(ctx, rec, req.Flow, blocks, req.missionID); err != nil {
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
// serialized, but a resumer (A7) can take the last free slot between Start's two locked steps and
// make an admitted run wait after all, so a run may wait past MaxQueuedPerFlow, MaxPendingRuns or
// MaxPendingBytes by at most one per such race.
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
	parent := r.baseCtx
	if p.durableCtx != nil {
		parent = p.durableCtx
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancels[p.rec.ID] = activeRun{flowID: p.rec.FlowID, mode: p.rec.Mode, durable: p.rec.Durable, cancel: cancel}
	r.wg.Add(1)
	go r.execute(ctx, cancel, p)
}

func (r *Runner) execute(ctx context.Context, cancel context.CancelFunc, p *pendingRun) {
	defer r.wg.Done()
	defer cancel()
	defer r.releaseDurableToken(p)
	runID := p.rec.ID
	terminal := true
	defer func() {
		if terminal {
			r.bus.Finish(runID)
		}
	}()
	// The log was opened when the run was queued; its leak clock must count the
	// run's own time only, and a sweep may have forgotten it during a long wait.
	r.bus.Rearm(runID)
	if p.rec.Durable && ctx.Err() != nil {
		if p.durableResume != nil {
			header, err := r.store.GetRunHeader(context.Background(), runID)
			if errors.Is(err, ErrRunNotFound) || (err == nil && header.Status.Terminal()) {
				r.release(p)
				if err == nil {
					r.projectDurable(runID)
				}
			} else {
				terminal = false
				r.releaseDurableSuspended(p)
			}
			return
		}
		r.recordUnstarted(p, "FLOW_CANCELLED", "durable processing was stopped before the run started")
		r.release(p)
		r.projectDurable(runID)
		return
	}
	if reason := r.gateRefusal(p); reason != nil {
		// Recorded first, then the slots go to the next waiting run (which asks the gate in
		// turn), then the hook, as for a run that executed.
		msg := truncateRunes("the run waited in a queue and was not started: "+reason.Error(), maxErrorMessageRunes)
		rec, res := r.recordUnstarted(p, RunsPausedCode, msg)
		r.logger.Info("a queued flow run was not started because automatic runs are not allowed",
			"run", runID, "flow", p.rec.FlowID, "reason", reason.Error())
		r.release(p)
		if p.rec.Durable {
			r.projectDurable(runID)
		} else {
			r.callHook(rec, res)
		}
		return
	}
	res := r.runAndRecord(ctx, p)
	if p.rec.Durable && (res.suspended || res.durabilityLost) {
		header, err := r.store.GetRunHeader(context.Background(), runID)
		if errors.Is(err, ErrRunNotFound) || (err == nil && header.Status.Terminal()) {
			r.release(p)
			if err == nil {
				r.projectDurable(runID)
			}
			return
		}
		terminal = false
		r.releaseDurableSuspended(p)
		if res.suspended {
			r.projectDurable(runID)
		}
		return
	}
	r.bus.Finish(runID)
	rec := p.rec
	rec.Status, rec.ErrorCode, rec.ErrorMessage = res.Status, res.ErrorCode, res.ErrorMessage
	finished := res.FinishedAt
	rec.FinishedAt = &finished
	rec.DurationMS = res.DurationMS
	r.release(p)
	if p.rec.Durable {
		r.projectDurable(runID)
	} else {
		r.callHook(rec, res)
	}
}

// gateRefusal asks RunnerConfig.Gate whether p, a gated live run that waited in a queue, may
// still execute. It returns nil when it may or is not asked; a panicking gate counts as a
// refusal, so the brake fails closed.
func (r *Runner) gateRefusal(p *pendingRun) (err error) {
	if r.cfg.Gate == nil || !p.req.Gated || !p.waited || p.req.Mode != ModeLive {
		return nil
	}
	return r.checkGate(p.rec.ID)
}

func (r *Runner) checkGate(runID string) (err error) {
	if r.cfg.Gate == nil {
		return nil
	}
	defer func() {
		if v := recover(); v != nil {
			r.logger.Error("flow run gate panicked; the run is not started", "run", runID, "panic", v)
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
			if p.rec.Durable {
				res = RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "durable processing stopped unexpectedly", durabilityLost: true}
			} else {
				res = r.panicked(p, v, launched, lastSeq, ended)
			}
		}
	}()
	if p.durableResume != nil {
		state, owner, generation, err := p.durableResume.load(ctx)
		if errors.Is(err, errDurableDeadline) {
			return RunResult{Status: RunError, ErrorCode: "FLOW_RUN_TIMEOUT", ErrorMessage: "the run took longer than its time limit", StartedAt: p.rec.StartedAt, FinishedAt: r.now()}
		}
		if err != nil {
			return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
		}
		p.durableResume.state, p.durableResume.owner, p.durableResume.generation = state, owner, generation
	}
	var statusErr error
	if p.rec.Durable {
		if p.durableResume != nil {
			doc, err := p.durableResume.state.Flow.Marshal()
			if err != nil {
				return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
			}
			statusErr = r.store.startClaimedDurableRun(ctx, runID, p.durableResume.generation, p.durableResume.owner, durableClaimGuard{flowID: p.rec.FlowID, missionID: p.req.missionID, revision: p.rec.Revision, document: doc}, r.now())
		} else {
			statusErr = r.store.startDurableRun(ctx, runID)
		}
	} else {
		statusErr = r.store.SetRunStatus(bg, runID, RunRunning)
	}
	if statusErr != nil {
		r.logStoreError("flow run status not saved", runID, statusErr)
		if p.rec.Durable {
			return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
		}
	}
	if !p.rec.Durable && r.hooks.OnRunStarted != nil {
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
	var durable *durableRunControl
	if p.rec.Durable {
		owner, generation := "", int64(0)
		if p.durableResume != nil {
			owner, generation = p.durableResume.owner, p.durableResume.generation
		}
		durable = durableControl(r.store, r.engine.services.CheckpointKeys, runID, owner, generation, r.now)
		durable.testWrapSet = r.testWrapDurableSet
		seconds := clampInt(p.req.Flow.Settings.MaxRunSeconds, 1, MaxRunSecondsLimit)
		timeout := time.Duration(seconds) * time.Second
		if p.req.Timeout > 0 {
			timeout = min(p.req.Timeout, time.Duration(MaxRunSecondsLimit)*time.Second)
		}
		durable.deadline = r.now().Add(timeout)
	}
	var terminalEvent *RunEvent
	historyState := ChildHistoryComplete
	sink := func(ev RunEvent) {
		if p.rec.Durable && ((p.durableCtx != nil && p.durableCtx.Err() != nil) || (durable != nil && durable.ownerCtx != nil && durable.ownerCtx.Err() != nil)) {
			return
		}
		// An expired owner cannot prove ownership until its next checkpoint CAS.
		// Resume exposes completed steps only after their committed snapshot.
		if p.durableResume != nil && ev.Type == EventStepStarted {
			return
		}
		if p.rec.Durable && ev.Type == EventRunFinished {
			event := ev
			terminalEvent = &event
			return
		}
		if ev.Type == EventStepFinished && ev.Step != nil && ev.childTrace != nil {
			if err := r.store.saveChildStep(bg, runID, ev.Seq, *ev.Step, ev.childTrace); err != nil {
				r.logStoreError("flow child history not saved", runID, err, "node", ev.NodeID)
				historyState = ChildHistoryIncomplete
				step := *ev.Step
				step.ChildHistory = nil
				ev.Step = &step
			} else if ev.childTrace.History.State == ChildHistoryIncomplete {
				historyState = ChildHistoryIncomplete
			} else if ev.childTrace.History.Truncated && historyState == ChildHistoryComplete {
				historyState = ChildHistoryTruncated
			}
			if ev.Step.ChildHistory != nil {
				step := *ev.Step
				step.ChildHistory = &ev.childTrace.History
				ev.Step = &step
			}
		}
		if r.testOnEvent != nil {
			r.testOnEvent(ev)
		}
		r.bus.Publish(ev)
		lastSeq, ended = ev.Seq, ev.Type == EventRunFinished
		if ev.Type == EventStepFinished && ev.Step != nil && ev.childTrace == nil && (durable == nil || ev.Seq > durable.persistedSeq) {
			if err := r.store.SaveStep(bg, runID, ev.Seq, *ev.Step); err != nil {
				r.logStoreError("flow step not saved", runID, err, "node", ev.NodeID)
			}
		}
	}
	var slot RunSlot // never a typed nil
	if p.slot != nil {
		slot = p.slot
	}
	var scope *blockRunScope
	if p.req.blocks != nil {
		scope = &blockRunScope{historyScrubber: r.store.childHistoryScrubber()}
	}
	engine := r.engine
	if p.rec.Durable {
		var err error
		engine, err = newDurableEngine(r.engine.services, r.logger)
		if err != nil {
			return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
		}
	}
	if p.durableResume != nil {
		workerCtx, stopWorker := context.WithCancel(ctx)
		defer stopWorker()
		durable.ownerCtx = workerCtx
		stopHeartbeat := r.startDurableHeartbeat(workerCtx, stopWorker, p, durable)
		res = resumeDurableCoordinator(workerCtx, p.durableResume.state, r.engine.services, r.logger, durable, slot, sink, p.req.missionID)
		stopHeartbeat()
	} else {
		res = engine.Execute(ctx, RunRequest{
			RunID: runID, Flow: p.req.Flow, Revision: p.req.Revision, Mode: p.req.Mode,
			TriggerNode: p.req.TriggerNode, TriggerType: p.req.TriggerType, TriggerData: p.req.TriggerData,
			OnlyNode: p.req.OnlyNode, Timeout: p.req.Timeout, Slot: slot, missionID: p.req.missionID, blocks: p.req.blocks, scope: scope, durable: durable,
		}, sink)
	}
	if res.suspended || res.durabilityLost {
		return res
	}
	if p.rec.Durable {
		if p.durableResume == nil {
			if err := r.store.finishUncheckpointedDurable(bg, runID, res); err != nil {
				res.durabilityLost = true
				return res
			}
		}
		if terminalEvent != nil {
			r.bus.Publish(*terminalEvent)
		}
		return res
	}
	if err := r.store.finishRunWithChildHistory(bg, runID, res, historyState); err != nil {
		r.logStoreError("flow run result not saved", runID, err)
	}
	return res
}

func (r *Runner) projectDurable(runID string) {
	if r.cfg.DurableHistory == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.cfg.DurableHistory.ProjectDurableRun(ctx, runID); err != nil {
		r.logger.Warn("durable history projection unavailable", "run", runID)
	}
}

// releaseDurableSuspended releases only the physical slot; the logical flow
// reservation survives until a confirmed terminal transition or cancellation.
func (r *Runner) releaseDurableSuspended(p *pendingRun) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p.slot == nil || !r.endSlotLocked(p.slot) {
		r.slots--
		if p.counted {
			r.dropFlowSlotLocked(p.rec.FlowID)
		}
	} else {
		r.parked--
	}
	delete(r.cancels, p.rec.ID)
	r.durable[p.rec.ID] = durableReservation{RunID: p.rec.ID, FlowID: p.rec.FlowID, Revision: p.rec.Revision}
	r.dispatchLocked()
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
	if p.rec.Durable {
		delete(r.durable, p.rec.ID)
	}
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
	var err error
	if p.rec.Durable {
		err = r.store.finishQueuedDurable(context.Background(), p.rec.ID, code, msg, now)
	} else {
		err = r.store.FinishRun(context.Background(), p.rec.ID, res)
	}
	if err != nil {
		r.logStoreError("cancelled flow run not saved", p.rec.ID, err)
	}
	// Ordinary cancellation is final in memory; durable outcomes require the journal.
	if err == nil || !p.rec.Durable {
		r.endLog(p.rec.ID, 1, res)
	}
	rec := p.rec
	rec.Status, rec.ErrorCode, rec.ErrorMessage = RunCancelled, code, msg
	rec.FinishedAt = &now
	return rec, res
}

func (r *Runner) finishUnstarted(p *pendingRun, code, msg string) {
	defer r.releaseDurableToken(p)
	rec, res := r.recordUnstarted(p, code, msg)
	if p.rec.Durable {
		r.projectDurable(p.rec.ID)
	} else {
		r.callHook(rec, res)
	}
}

func (r *Runner) releaseDurableToken(p *pendingRun) {
	if p.durableToken == nil {
		return
	}
	r.durableActive.remove(p.durableToken)
	p.durableCancel()
	<-r.durableSlots
	p.durableToken = nil
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
	// Interrupt only not-yet-admitted block snapshots before waiting for startMu.
	r.blockAdmissionStop()
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
		if p.durableResume != nil {
			r.releaseDurableToken(p)
			continue
		}
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
