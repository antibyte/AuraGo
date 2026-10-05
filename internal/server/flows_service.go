package server

import (
	"context"
	"path/filepath"
	"time"

	"aurago/internal/flows"
	"aurago/internal/memory"
)

// flowsDBPath places flows.db next to the other desktop databases.
func (s *Server) flowsDBPath() string {
	if s.Cfg != nil && s.Cfg.SQLite.GameMakerPath != "" {
		return filepath.Join(filepath.Dir(s.Cfg.SQLite.GameMakerPath), "flows.db")
	}
	return "data/flows.db"
}

// initFlows creates the flow service and connects it to Mission Control. It must run
// before MissionManagerV2.Start so that flow triggers and the startup trigger find the
// hooks; startFlows runs after it.
func (s *Server) initFlows() {
	if s.Cfg == nil || !s.Cfg.Flows.Enabled || s.MissionManagerV2 == nil {
		return
	}
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
	services := &flows.Services{Tools: newFlowToolInvoker(s, env), LLM: newFlowLLM(s), Secrets: flowSecrets{s: s},
		Clock: flows.RealClock(), Location: time.Local}
	limits := s.Cfg.Flows
	s.Flows = flows.NewService(store, reg, services, flowMissionBridge{s: s}, flows.ServiceConfig{
		MaxParallelRuns: limits.EffectiveMaxParallelRuns(), MaxParallelNodes: limits.EffectiveMaxParallelNodes(),
		RunRetentionDays: limits.EffectiveRunRetentionDays(), MaxRunsPerFlow: limits.EffectiveMaxRunsPerFlow(),
	}, s.Logger)
	s.flowsCatalog = env
	s.MissionManagerV2.SetFlowHooks(flowMissionHooks{s: s})
}

// startFlows marks runs of the previous process as interrupted and arms the Date/Time
// timers. Call it after MissionManagerV2.Start (the timers ask Mission Control whether a
// flow is enabled).
func (s *Server) startFlows(ctx context.Context) {
	if s.Flows == nil {
		return
	}
	if err := s.Flows.Start(ctx); err != nil {
		s.Logger.Error("EasyDrag flows could not start", "error", err)
	}
}

// shutdownFlows stops timers and runs and closes the store before the databases close.
func (s *Server) shutdownFlows(ctx context.Context) {
	if s.Flows == nil {
		return
	}
	if err := s.Flows.Shutdown(ctx); err != nil {
		s.Logger.Warn("EasyDrag flows did not stop in time", "error", err)
	}
	if err := s.Flows.Store().Close(); err != nil {
		s.Logger.Warn("EasyDrag flow store could not be closed", "error", err)
	}
}

// flowsAvailable backs the desktop capability "flows" and the API gate.
func (s *Server) flowsAvailable() bool {
	cfg := s.ConfigSnapshot()
	return s.Flows != nil && cfg != nil && cfg.Flows.Enabled && cfg.Tools.Missions.Enabled
}

// recordFlowAudit writes a user action on a flow to the audit timeline.
func (s *Server) recordFlowAudit(eventType, flowID, name, summary string) {
	if s.ShortTermMem == nil {
		return
	}
	if _, err := s.ShortTermMem.RecordAuditEvent(memory.AuditEvent{Source: memory.AuditSourceMission, EventType: eventType,
		Actor: "user", SessionID: "easydrag", TargetID: flowID, TargetName: name, Status: memory.AuditStatusSuccess,
		Summary: summary}); err != nil {
		s.Logger.Warn("Flow audit event could not be recorded", "event", eventType, "error", err)
	}
}
