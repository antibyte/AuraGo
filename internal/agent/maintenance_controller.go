package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/tiendc/go-deepcopy"
)

// MaintenanceControllerDependencies contains the long-lived services used by
// one maintenance run. The controller keeps this set stable while each run
// receives its own immutable configuration snapshot.
type MaintenanceControllerDependencies struct {
	Logger           *slog.Logger
	LLMClient        llm.ChatClient
	Vault            *security.Vault
	Registry         *tools.ProcessRegistry
	Manifest         *tools.Manifest
	CronManager      *tools.CronManager
	LongTermMem      memory.VectorDB
	ShortTermMem     *memory.SQLiteMemory
	HistoryManager   *memory.HistoryManager
	KG               *memory.KnowledgeGraph
	InventoryDB      *sql.DB
	ContactsDB       *sql.DB
	PlannerDB        *sql.DB
	CheatsheetDB     *sql.DB
	MissionManagerV2 *tools.MissionManagerV2
	Guardian         *security.LLMGuardian
	DaemonSupervisor *tools.DaemonSupervisor
}

// MaintenanceControllerStatus is the scheduler state exposed to the server
// dashboard. A zero NextRun means that maintenance is disabled or stopped.
type MaintenanceControllerStatus struct {
	Enabled bool
	Running bool
	NextRun time.Time
}

const maintenanceSchedulerIssueFingerprint = "maintenance|phase|scheduler"

type maintenanceControllerTimer interface {
	Chan() <-chan time.Time
	Stop() bool
}

type maintenanceControllerTimerFactory func(time.Duration) maintenanceControllerTimer

type maintenanceControllerOptions struct {
	now          func() time.Time
	timerFactory maintenanceControllerTimerFactory
	run          func(context.Context, *config.Config)
	claimDay     func(time.Time) (bool, error)
	dayClaimed   func(time.Time) (bool, error)
}

type maintenanceRunStartedAtContextKey struct{}

func withMaintenanceRunStartedAt(ctx context.Context, startedAt time.Time) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, maintenanceRunStartedAtContextKey{}, startedAt)
}

func maintenanceRunStartedAt(ctx context.Context) time.Time {
	if ctx == nil {
		return time.Time{}
	}
	startedAt, _ := ctx.Value(maintenanceRunStartedAtContextKey{}).(time.Time)
	return startedAt
}

type maintenanceController struct {
	ctx    context.Context
	cancel context.CancelFunc
	deps   MaintenanceControllerDependencies
	logger *slog.Logger

	mu          sync.RWMutex
	cfg         *config.Config
	nextRun     time.Time
	skipDay     string
	stopped     bool
	running     bool
	runCancel   context.CancelFunc
	runDone     chan struct{}
	pendingDone <-chan struct{}
	stopOnce    sync.Once
	loopWG      sync.WaitGroup
	wake        chan struct{}
	now         func() time.Time
	timerMaker  maintenanceControllerTimerFactory
	run         func(context.Context, *config.Config)
	claimDay    func(time.Time) (bool, error)
	dayClaimed  func(time.Time) (bool, error)
}

type maintenanceControllerRealTimer struct {
	timer *time.Timer
}

func (t maintenanceControllerRealTimer) Chan() <-chan time.Time { return t.timer.C }
func (t maintenanceControllerRealTimer) Stop() bool             { return t.timer.Stop() }

func newMaintenanceControllerRealTimer(duration time.Duration) maintenanceControllerTimer {
	if duration < 0 {
		duration = 0
	}
	return maintenanceControllerRealTimer{timer: time.NewTimer(duration)}
}

// StartMaintenanceController starts a controller even when maintenance is
// disabled. The server can then hot-enable it through UpdateConfig without
// creating a second background loop.
func StartMaintenanceController(ctx context.Context, cfg *config.Config, deps MaintenanceControllerDependencies) *MaintenanceController {
	return newMaintenanceController(ctx, cfg, deps, maintenanceControllerOptions{})
}

// MaintenanceController owns the maintenance timer and the active run.
// Configuration updates are non-blocking; Stop waits for all work to exit.
type MaintenanceController struct {
	inner *maintenanceController
}

func newMaintenanceController(ctx context.Context, cfg *config.Config, deps MaintenanceControllerDependencies, opts maintenanceControllerOptions) *MaintenanceController {
	if ctx == nil {
		ctx = context.Background()
	}
	controllerCtx, cancel := context.WithCancel(ctx)
	if opts.now == nil {
		opts.now = time.Now
	}
	if opts.timerFactory == nil {
		opts.timerFactory = newMaintenanceControllerRealTimer
	}
	if opts.run == nil {
		opts.run = func(runCtx context.Context, runCfg *config.Config) {
			if runCfg == nil {
				return
			}
			runMaintenanceTask(runCtx, runCfg, deps.Logger, deps.LLMClient, deps.Vault, deps.Registry, deps.Manifest, deps.CronManager, deps.LongTermMem, deps.ShortTermMem, deps.HistoryManager, deps.KG, deps.InventoryDB, deps.ContactsDB, deps.PlannerDB, deps.CheatsheetDB, deps.MissionManagerV2, deps.Guardian, deps.DaemonSupervisor)
		}
	}
	if opts.claimDay == nil {
		opts.claimDay = func(startedAt time.Time) (bool, error) {
			if deps.ShortTermMem == nil {
				return false, fmt.Errorf("short-term memory is required for the maintenance day claim")
			}
			return deps.ShortTermMem.ClaimMaintenanceDay(startedAt)
		}
	}
	if opts.dayClaimed == nil {
		opts.dayClaimed = func(startedAt time.Time) (bool, error) {
			if deps.ShortTermMem == nil {
				return false, nil
			}
			return deps.ShortTermMem.IsMaintenanceDayClaimed(startedAt)
		}
	}
	controllerLogger := deps.Logger
	if controllerLogger == nil {
		controllerLogger = slog.Default()
	}
	snapshot, snapshotErr := cloneMaintenanceConfig(cfg)
	if snapshotErr != nil {
		controllerLogger.Error("failed to clone maintenance configuration; scheduler disabled", "error", snapshotErr)
	}
	inner := &maintenanceController{
		ctx:        controllerCtx,
		cancel:     cancel,
		deps:       deps,
		logger:     controllerLogger,
		cfg:        snapshot,
		wake:       make(chan struct{}, 1),
		now:        opts.now,
		timerMaker: opts.timerFactory,
		run:        opts.run,
		claimDay:   opts.claimDay,
		dayClaimed: opts.dayClaimed,
	}
	inner.pendingDone = startPendingMemoryWriteRetryLoop(controllerCtx, deps.Logger, deps.ShortTermMem, deps.LongTermMem)
	inner.loopWG.Add(1)
	go inner.loop()
	return &MaintenanceController{inner: inner}
}

// UpdateConfig publishes a new immutable configuration pointer. It wakes the
// timer immediately and cancels an active run when maintenance is disabled.
func (c *MaintenanceController) UpdateConfig(cfg *config.Config) {
	if c == nil || c.inner == nil || cfg == nil {
		return
	}
	inner := c.inner
	snapshot, snapshotErr := cloneMaintenanceConfig(cfg)
	if snapshotErr != nil {
		inner.logger.Error("failed to clone maintenance configuration; update ignored", "error", snapshotErr)
		return
	}
	nextRun := time.Time{}
	if snapshot.Maintenance.Enabled {
		nextRun = inner.nextRunFor(snapshot, inner.now())
	}
	inner.mu.Lock()
	if inner.stopped {
		inner.mu.Unlock()
		return
	}
	inner.cfg = snapshot
	if !snapshot.Maintenance.Enabled {
		inner.nextRun = time.Time{}
		if inner.runCancel != nil {
			inner.runCancel()
		}
	} else {
		inner.nextRun = nextRun
	}
	inner.mu.Unlock()
	inner.signalWake()
}

// cloneMaintenanceConfig gives every controller publication its own complete
// config object. Runtime capability fields are intentionally copied as-is,
// including their zero value; YAML round-tripping would lose runtime-only
// state and resolved credentials. The callback field is a function and is
// retained by the deepcopy package so authorization snapshots keep their
// publication semantics.
func cloneMaintenanceConfig(cfg *config.Config) (*config.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	var clone config.Config
	if err := deepcopy.Copy(&clone, cfg, deepcopy.IgnoreNonCopyableTypes(true)); err != nil {
		return nil, fmt.Errorf("deep-copy maintenance config: %w", err)
	}
	return &clone, nil
}

// Status returns a consistent scheduler snapshot for API/dashboard callers.
func (c *MaintenanceController) Status() MaintenanceControllerStatus {
	if c == nil || c.inner == nil {
		return MaintenanceControllerStatus{}
	}
	inner := c.inner
	inner.mu.RLock()
	defer inner.mu.RUnlock()
	return MaintenanceControllerStatus{Enabled: !inner.stopped && inner.cfg != nil && inner.cfg.Maintenance.Enabled, Running: inner.running, NextRun: inner.nextRun}
}

// Stop cancels the scheduler and active maintenance run, then waits for both
// to exit before returning. The context bounds shutdown waiting.
func (c *MaintenanceController) Stop(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return nil
	}
	inner := c.inner
	inner.stopOnce.Do(func() {
		inner.cancel()
		inner.mu.Lock()
		inner.stopped = true
		inner.nextRun = time.Time{}
		if inner.runCancel != nil {
			inner.runCancel()
		}
		inner.mu.Unlock()
		inner.signalWake()
	})
	if ctx == nil {
		inner.loopWG.Wait()
		if inner.pendingDone != nil {
			<-inner.pendingDone
		}
		return nil
	}
	done := make(chan struct{})
	go func() {
		inner.loopWG.Wait()
		if inner.pendingDone != nil {
			<-inner.pendingDone
		}
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *maintenanceController) signalWake() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *maintenanceController) loop() {
	defer c.loopWG.Done()
	for {
		if c.ctx.Err() != nil {
			return
		}
		cfg, nextRun, disabled := c.schedule(c.now())
		if disabled {
			c.setNextRun(cfg, time.Time{})
			select {
			case <-c.ctx.Done():
				return
			case <-c.wake:
				continue
			}
		}
		if !c.setNextRun(cfg, nextRun) {
			continue
		}
		now := c.now()
		duration := nextRun.Sub(now)
		if duration < 0 {
			duration = 0
		}
		timer := c.timerMaker(duration)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-c.wake:
			timer.Stop()
			continue
		case <-timer.Chan():
		}
		if !c.startRun(cfg) {
			continue
		}
		c.refreshNextRun()
		c.waitRun()
	}
}

func (c *maintenanceController) schedule(now time.Time) (*config.Config, time.Time, bool) {
	c.mu.RLock()
	cfg := c.cfg
	c.mu.RUnlock()
	if cfg == nil || !cfg.Maintenance.Enabled {
		return cfg, time.Time{}, true
	}
	return cfg, c.nextRunFor(cfg, now), false
}

func (c *maintenanceController) nextRunFor(cfg *config.Config, now time.Time) time.Time {
	c.mu.RLock()
	skipDay := c.skipDay
	c.mu.RUnlock()
	if skipDay == now.Format("2006-01-02") {
		return nextMaintenanceRunAfterDay(cfg, now)
	}
	if claimed, err := c.dayClaimed(now); err == nil && claimed {
		c.resolveSchedulerIssue(now)
		return nextMaintenanceRunAfterDay(cfg, now)
	} else if err != nil {
		c.recordSchedulerIssue(err, now)
		c.logger.Error("failed to inspect maintenance day claim; scheduler will verify before running", "error", err)
	}
	return ComputeNextMaintenanceRun(cfg, now)
}

func (c *maintenanceController) setNextRun(cfg *config.Config, nextRun time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped || c.cfg == nil || !c.cfg.Maintenance.Enabled {
		c.nextRun = time.Time{}
		return false
	}
	if cfg != c.cfg {
		return false
	}
	c.nextRun = nextRun
	return true
}

func (c *maintenanceController) refreshNextRun() {
	now := c.now()
	c.mu.RLock()
	cfg := c.cfg
	c.mu.RUnlock()
	if cfg == nil || !cfg.Maintenance.Enabled {
		c.setNextRun(cfg, time.Time{})
		return
	}
	c.setNextRun(cfg, c.nextRunFor(cfg, now))
}

func (c *maintenanceController) startRun(cfg *config.Config) bool {
	c.mu.Lock()
	// Re-read the published snapshot after the timer fires. A disable/update
	// racing with the timer must never launch the stale scheduled snapshot.
	if c.stopped || c.ctx.Err() != nil || cfg == nil || cfg != c.cfg || !cfg.Maintenance.Enabled {
		c.mu.Unlock()
		return false
	}
	if c.running {
		c.mu.Unlock()
		return false
	}
	startedAt := c.now()
	claimed, err := c.claimDay(startedAt)
	if err != nil {
		c.recordSchedulerIssue(err, startedAt)
		c.logger.Error("maintenance day claim failed; run skipped", "started_at", startedAt, "error", err)
		c.mu.Unlock()
		return false
	}
	if !claimed {
		c.resolveSchedulerIssue(startedAt)
		c.skipDay = startedAt.Format("2006-01-02")
		c.mu.Unlock()
		return false
	}
	if c.stopped || c.ctx.Err() != nil {
		c.mu.Unlock()
		return false
	}
	c.resolveSchedulerIssue(startedAt)
	runCtx, cancel := context.WithCancel(c.ctx)
	runCtx = withMaintenanceRunStartedAt(runCtx, startedAt)
	c.running = true
	c.runCancel = cancel
	c.runDone = make(chan struct{})
	runDone := c.runDone
	c.mu.Unlock()
	go func() {
		defer close(runDone)
		defer cancel()
		c.run(runCtx, cfg)
		c.mu.Lock()
		c.running = false
		c.runCancel = nil
		c.mu.Unlock()
	}()
	return true
}

func (c *maintenanceController) waitRun() {
	c.mu.RLock()
	runDone := c.runDone
	c.mu.RUnlock()
	if runDone == nil {
		return
	}
	select {
	case <-runDone:
	case <-c.ctx.Done():
		<-runDone
	}
}

func (c *maintenanceController) recordSchedulerIssue(err error, occurredAt time.Time) {
	if c.deps.PlannerDB == nil {
		return
	}
	detail := "automatic maintenance day claim failed"
	if err != nil {
		detail = fmt.Sprintf("%s: %v", detail, err)
	}
	if _, recordErr := planner.RecordOperationalIssue(c.deps.PlannerDB, planner.OperationalIssue{
		Source:      "maintenance",
		Context:     "scheduler",
		Title:       "Maintenance scheduler day claim failed",
		Detail:      detail,
		Severity:    "error",
		Kind:        planner.OperationalIssueKindRuntimeFailure,
		Reference:   "scheduler",
		Fingerprint: maintenanceSchedulerIssueFingerprint,
		OccurredAt:  occurredAt,
	}); recordErr != nil {
		c.logger.Warn("failed to record maintenance scheduler issue", "error", recordErr)
	}
}

func (c *maintenanceController) resolveSchedulerIssue(resolvedAt time.Time) {
	if c.deps.PlannerDB == nil {
		return
	}
	if _, err := planner.ResolveOperationalIssue(c.deps.PlannerDB, maintenanceSchedulerIssueFingerprint, "Maintenance scheduler day claim persisted successfully.", resolvedAt); err != nil {
		c.logger.Warn("failed to resolve maintenance scheduler issue", "error", err)
	}
}
