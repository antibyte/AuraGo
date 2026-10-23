package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

// flowsDBPath places flows.db next to the other desktop databases (config.FlowsDBPath).
// It has no sqlite setting of its own, so backup and export tooling must include it by
// name: config.SQLiteDatabasePaths lists it (the backup and the agent's protected files
// use that list).
func (s *Server) flowsDBPath() string {
	if path := config.FlowsDBPath(s.Cfg); path != "" {
		return path
	}
	return filepath.Join("data", config.FlowsDBFilename)
}

// flowsInitTestHook, when a test sets it, runs inside initFlows once the service exists,
// just before the hooks are installed, so a test can inject a panic there.
var flowsInitTestHook func()

// initFlows creates the flow service and connects it to Mission Control. It must run
// before MissionManagerV2.Start so that flow triggers and the startup trigger find the
// hooks; startFlows runs after it.
//
// Flows are on by default, so a panic while they are set up must not take the server
// down: it is recovered and logged at Error, the store is closed again and s.Flows stays
// nil (the capability and the API are off, flow missions report that flows are not
// available), and the rest of the server starts as usual.
func (s *Server) initFlows() {
	if s.Cfg == nil || !s.Cfg.Flows.Enabled || s.MissionManagerV2 == nil {
		return
	}
	var store *flows.Store
	defer func() {
		if r := recover(); r != nil {
			s.Flows, s.flowsCatalog = nil, nil
			if store != nil {
				_ = store.Close()
			}
			s.Logger.Error("EasyDrag flows could not be set up; they stay off until the next start",
				"panic", flowBoundRunes(fmt.Sprint(r), flowErrorRunes), "stack", string(debug.Stack()))
		}
	}()
	store, err := flows.OpenStore(s.flowsDBPath(), s.Logger)
	if err != nil {
		s.Logger.Error("EasyDrag flow store could not be opened", "error", err)
		return
	}
	env := newFlowCatalogEnv(s)
	reg := flows.NewRegistry()
	if err := flows.RegisterCatalog(reg, env); err != nil {
		s.Logger.Error("EasyDrag node catalog could not be registered", "error", err)
		_ = store.Close()
		return
	}
	env.refreshRegistry(reg, s.ConfigSnapshot())
	// time.Local: the cron manager runs flow schedules in time.Local, so Date/Time triggers
	// and yearly timers must use the same zone.
	services := &flows.Services{Tools: newFlowToolInvoker(s, env), LLM: newFlowLLM(s), Secrets: flowSecrets{s: s},
		Clock: flows.RealClock(), Location: time.Local, Decisions: flowDecisions{s: s}}
	limits := s.Cfg.Flows
	s.Flows = flows.NewService(store, reg, services, flowMissionBridge{s: s}, flows.ServiceConfig{
		MaxParallelRuns: limits.EffectiveMaxParallelRuns(), MaxParallelNodes: limits.EffectiveMaxParallelNodes(),
		RunRetentionDays: limits.EffectiveRunRetentionDays(), MaxRunsPerFlow: limits.EffectiveMaxRunsPerFlow(),
	}, s.Logger)
	s.flowsCatalog = env
	if flowsInitTestHook != nil {
		flowsInitTestHook()
	}
	s.MissionManagerV2.SetFlowHooks(flowMissionHooks{s: s})
}

// The delays between the start attempts of the flow service (flowsStartRetryDelay).
const (
	flowsStartRetryInitial = 5 * time.Second
	flowsStartRetryMax     = 5 * time.Minute
)

// flowsStart starts the flow service; a variable so a test can make it fail.
var flowsStart = func(ctx context.Context, svc *flows.Service) error { return svc.Start(ctx) }

// flowsStartRetryDelay returns how long to wait after the failed start attempt n (1 for the
// first): flowsStartRetryInitial, doubling, at most flowsStartRetryMax. A variable so a test
// can shorten it. (The server package shadows the builtin min and max.)
var flowsStartRetryDelay = func(attempt int) time.Duration {
	d := flowsStartRetryInitial
	for i := 1; i < attempt && d < flowsStartRetryMax; i++ {
		d *= 2
	}
	if d > flowsStartRetryMax {
		d = flowsStartRetryMax
	}
	return d
}

// flowsStartRetry is the background retry of a failed Service.Start: cancel ends it, done
// closes when its goroutine has returned.
type flowsStartRetry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// flowsStartRetries holds the running start retry of each flow service (a *flowsStartRetry),
// so shutdownFlows can end and join it.
var flowsStartRetries sync.Map

// startFlows marks runs of the previous process as interrupted and arms the Date/Time
// timers. Call it after MissionManagerV2.Start (the timers ask Mission Control whether a
// flow is enabled).
//
// Then, on a goroutine of its own so that a large flows.db does not hold up the start, it
// reconciles Mission Control with the flow store once (Service.ReconcileMissions): a crash
// between a flows.db write and the Mission Control update leaves them apart.
// shutdownFlows waits for it through Service.Shutdown.
//
// A Start that fails (a locked or unreadable flows.db) would leave the Date/Time timers, the
// retention loop and the marking of interrupted runs off until the next restart, so it is
// retried on a goroutine of its own (retryFlowsStart: flowsStartRetryDelay between the
// attempts, every failure logged at Warn with the attempt and the next delay, the recovery
// at Info), followed by the same reconciliation. The retry ends when Start worked, when ctx
// ends, when the service was shut down (ErrRunnerClosed) and in shutdownFlows, which joins it.
func (s *Server) startFlows(ctx context.Context) {
	if s.Flows == nil {
		return
	}
	svc, logger := s.Flows, s.Logger
	if err := flowsStart(ctx, svc); err != nil {
		if errors.Is(err, flows.ErrRunnerClosed) || ctx.Err() != nil {
			logger.Warn("EasyDrag flows could not start", "error", err)
			return
		}
		delay := flowsStartRetryDelay(1)
		logger.Warn("EasyDrag flows could not start; trying again in the background", "error", err, "attempt", 1, "next_attempt_in", delay.String())
		retryCtx, cancel := context.WithCancel(ctx)
		retry := &flowsStartRetry{cancel: cancel, done: make(chan struct{})}
		flowsStartRetries.Store(svc, retry)
		go func() {
			defer close(retry.done)
			defer flowsStartRetries.CompareAndDelete(svc, retry)
			defer cancel()
			if retryFlowsStart(retryCtx, svc, logger, delay) {
				reconcileFlows(retryCtx, svc, logger)
			}
		}()
		return
	}
	go reconcileFlows(ctx, svc, logger)
}

// reconcileFlows runs Service.ReconcileMissions and logs what is worth a line.
func reconcileFlows(ctx context.Context, svc *flows.Service, logger *slog.Logger) {
	err := svc.ReconcileMissions(ctx)
	if err != nil && !errors.Is(err, flows.ErrRunnerClosed) && ctx.Err() == nil {
		logger.Warn("EasyDrag flows could not be checked against Mission Control", "error", err)
	}
}

// retryFlowsStart calls Service.Start again after delay, and after each further failure
// after flowsStartRetryDelay of the attempt, until it works (true), ctx ends or the service
// was shut down (false). Attempt 1 was the one that failed in startFlows.
func retryFlowsStart(ctx context.Context, svc *flows.Service, logger *slog.Logger, delay time.Duration) bool {
	for attempt := 2; ; attempt++ {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		err := flowsStart(ctx, svc)
		if err == nil {
			logger.Info("EasyDrag flows started after earlier failures", "attempt", attempt)
			return true
		}
		if errors.Is(err, flows.ErrRunnerClosed) || ctx.Err() != nil {
			return false
		}
		delay = flowsStartRetryDelay(attempt)
		logger.Warn("EasyDrag flows could not start; trying again in the background", "error", err, "attempt", attempt, "next_attempt_in", delay.String())
	}
}

// The shutdown budget of the flow service: what is left of the server's shutdown context,
// within [flowsShutdownMinimum, flowsShutdownTimeout] (see flowsShutdownBudget).
const (
	flowsShutdownTimeout = 15 * time.Second
	flowsShutdownMinimum = 5 * time.Second
)

// flowsShutdownBudget returns how long Service.Shutdown may take: the time left before
// ctx's deadline, at most flowsShutdownTimeout and at least flowsShutdownMinimum. The
// server's context (45 s) is shared with the HTTP drain; taking only what is left keeps
// the drain and the flows within 50 s together, under systemd's TimeoutStopSec=60s
// (install.sh), while the minimum still lets cancelled runs record their end after a
// drain that used up its time. Without a deadline the budget is flowsShutdownTimeout.
func flowsShutdownBudget(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return flowsShutdownTimeout
	}
	// The server package shadows the builtins with int max (and its tests with int min).
	budget := time.Until(deadline)
	if budget > flowsShutdownTimeout {
		budget = flowsShutdownTimeout
	}
	if budget < flowsShutdownMinimum {
		budget = flowsShutdownMinimum
	}
	return budget
}

// shutdownFlows stops timers and runs and closes the store before the databases close.
//
// The server calls it once the HTTP API is drained, so no flow API request is in flight,
// and before MQTT, mail, MCP, the sandbox, the mission history and the planner stop: flow
// runs use all of them. The order is the background start retry, if one is still going
// (ended and joined: see startFlows), Service.Shutdown (timers, runs, the retention loop,
// a startup reconciliation still in progress), then the store. Shutdown gets
// flowsShutdownBudget(ctx) on a context of its own (ctx's cancellation is ignored); runs
// that do not stop in time end in the background, and their last writes then fail on the
// closed store and are logged.
//
// Service.Shutdown does not wait for flow operations that Mission Control starts through
// the hooks: until the process ends a cron job, an MQTT or mail trigger or a mission
// deleted or switched by the agent can still call them. After Shutdown a run is refused
// (flows.ErrRunnerClosed); after the store is closed every hook gets the store's "database
// is closed" error. Both are logged, by the trigger (StartFlowRun's error) or by the hook
// (FlowMissionDeleted, FlowEnabledChanged at Warn), and NextFlowRun answers "no timer";
// nothing panics (TestC16HooksAfterShutdownOnlyLog).
func (s *Server) shutdownFlows(ctx context.Context) {
	if s.Flows == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), flowsShutdownBudget(ctx))
	defer cancel()
	// A start retry that is still going would call Start (and reconcile) against the
	// service that is stopping: end it and wait for it first, within the same budget.
	if v, ok := flowsStartRetries.LoadAndDelete(s.Flows); ok {
		retry := v.(*flowsStartRetry)
		retry.cancel()
		select {
		case <-retry.done:
		case <-ctx.Done():
		}
	}
	if err := s.Flows.Shutdown(ctx); err != nil {
		s.Logger.Warn("EasyDrag flows did not stop in time", "error", err)
	}
	if err := s.Flows.Store().Close(); err != nil {
		s.Logger.Warn("EasyDrag flow store could not be closed", "error", err)
	}
}

// flowsRuntimeConfigChanged reports whether a flows setting changed that initFlows reads
// only at boot: the switch and the four limits. The limits are compared as effective
// values, so writing out a default or a value beyond the clamp needs no restart.
// ai_provider and agent.* are read live.
func flowsRuntimeConfigChanged(oldCfg, newCfg config.FlowsConfig) bool {
	return oldCfg.Enabled != newCfg.Enabled ||
		oldCfg.EffectiveMaxParallelRuns() != newCfg.EffectiveMaxParallelRuns() ||
		oldCfg.EffectiveMaxParallelNodes() != newCfg.EffectiveMaxParallelNodes() ||
		oldCfg.EffectiveRunRetentionDays() != newCfg.EffectiveRunRetentionDays() ||
		oldCfg.EffectiveMaxRunsPerFlow() != newCfg.EffectiveMaxRunsPerFlow()
}

// flowsAvailable backs the desktop capability "flows" and the API gate.
func (s *Server) flowsAvailable() bool {
	cfg := s.ConfigSnapshot()
	return s.Flows != nil && cfg != nil && cfg.Flows.Enabled && cfg.Tools.Missions.Enabled
}

// automaticFlowRunsAllowed tells whether a flow run may start on a trigger instead of a
// user's request: the flows gate (flowsAvailable: the service exists, flows.enabled and
// tools.missions.enabled) and tools.missions.readonly, both read from the current config
// snapshot, so a switch counts at once. flows.enabled is otherwise read at boot only, and
// published flows used to keep running after it was switched off. Cron and event triggers
// ask through flowMissionHooks.StartFlowRun, the Date/Time timers through the bridge's
// flows.RunGate. It returns tools.ErrFlowsUnavailable or tools.ErrFlowRunsPaused, which
// Mission Control notes on the flow mission instead of logging a failed start.
func (s *Server) automaticFlowRunsAllowed() error {
	if !s.flowsAvailable() {
		return tools.ErrFlowsUnavailable
	}
	if cfg := s.ConfigSnapshot(); cfg.Tools.Missions.ReadOnly {
		return tools.ErrFlowRunsPaused
	}
	return nil
}

// recordFlowAudit writes a user action on a flow to the audit timeline.
//
// The event types (flow_create, flow_import, flow_publish, flow_enable, flow_disable,
// flow_delete, flow_secret_set, flow_secret_delete, flow_run_cancel) are listed in the
// dashboard's audit type filter (ui/dashboard.html) and labelled in
// ui/lang/dashboard/*.json (dashboard.audit_type_<type>); a new type needs both.
func (s *Server) recordFlowAudit(eventType, flowID, name, summary string) {
	s.recordFlowAuditStatus(eventType, flowID, name, memory.AuditStatusSuccess, summary)
}

// recordFlowAuditStatus is recordFlowAudit with an audit status (memory.AuditStatus*).
func (s *Server) recordFlowAuditStatus(eventType, flowID, name, status, summary string) {
	if s.ShortTermMem == nil {
		return
	}
	if _, err := s.ShortTermMem.RecordAuditEvent(memory.AuditEvent{Source: memory.AuditSourceMission, EventType: eventType,
		Actor: "user", SessionID: "easydrag", TargetID: flowID, TargetName: name, Status: status,
		Summary: summary}); err != nil {
		s.Logger.Warn("Flow audit event could not be recorded", "event", eventType, "error", err)
	}
}
