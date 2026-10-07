package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"time"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/memory"
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
		Clock: flows.RealClock(), Location: time.Local}
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

// startFlows marks runs of the previous process as interrupted and arms the Date/Time
// timers. Call it after MissionManagerV2.Start (the timers ask Mission Control whether a
// flow is enabled).
//
// Then, on a goroutine of its own so that a large flows.db does not hold up the start, it
// reconciles Mission Control with the flow store once (Service.ReconcileMissions): a crash
// between a flows.db write and the Mission Control update leaves them apart.
// shutdownFlows waits for it through Service.Shutdown.
func (s *Server) startFlows(ctx context.Context) {
	if s.Flows == nil {
		return
	}
	if err := s.Flows.Start(ctx); err != nil {
		s.Logger.Error("EasyDrag flows could not start", "error", err)
		return
	}
	svc, logger := s.Flows, s.Logger
	go func() {
		err := svc.ReconcileMissions(ctx)
		if err != nil && !errors.Is(err, flows.ErrRunnerClosed) && ctx.Err() == nil {
			logger.Warn("EasyDrag flows could not be checked against Mission Control", "error", err)
		}
	}()
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
// runs use all of them. The order is Service.Shutdown (timers, runs, the retention loop,
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
