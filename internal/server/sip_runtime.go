package server

import (
	"context"
	"fmt"
	"time"

	"aurago/internal/planner"
	"aurago/internal/sipphone"
)

func (s *Server) initSIP(ctx context.Context) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("initialize SIP: server config is unavailable")
	}
	runner := s.VoiceActionRunner
	if runner == nil {
		runner = NewVoiceActionRunner(s)
	}
	reporter := func(_ context.Context, fingerprint, detail string) {
		if s.PlannerDB == nil {
			return
		}
		_, err := planner.RecordOperationalIssue(s.PlannerDB, planner.OperationalIssue{
			Source: "sip", Context: "background_service", Severity: "high",
			Title: "SIP registration failed", Detail: detail,
			Fingerprint: fingerprint, OccurredAt: time.Now().UTC(),
		})
		if err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to record SIP operational issue", "error", err)
		}
	}
	manager, err := sipphone.NewManager(s.Cfg.SIP, s.Cfg.Directories.DataDir, runner.backendFactory, reporter, s.Logger)
	if err != nil {
		return fmt.Errorf("initialize SIP endpoint: %w", err)
	}
	runner.SetEndCall(func(callID string) {
		callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = manager.Hangup(callCtx, callID)
	})
	s.SIPPhone = manager
	s.VoiceActionRunner = runner
	sipphone.SetDefaultManager(manager)
	if err := manager.Start(ctx); err != nil {
		_ = manager.Close()
		s.SIPPhone = nil
		sipphone.SetDefaultManager(nil)
		return fmt.Errorf("start SIP endpoint: %w", err)
	}
	if s.Cfg.SIP.HistoryRetentionDays > 0 {
		_ = manager.PruneHistory(ctx, time.Now().AddDate(0, 0, -s.Cfg.SIP.HistoryRetentionDays))
	}
	go s.cleanupTransientSIPSessions(ctx, manager)
	return nil
}

func (s *Server) cleanupTransientSIPSessions(ctx context.Context, manager *sipphone.Manager) {
	events, unsubscribe := manager.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if event.Call == nil || event.Call.State != sipphone.StateEnded || event.Call.SessionID == "" {
				continue
			}
			s.CfgMu.RLock()
			persist := s.Cfg.SIP.Voice.PersistTranscripts
			s.CfgMu.RUnlock()
			if !persist && s.ShortTermMem != nil {
				if err := s.ShortTermMem.PurgeChatSession(event.Call.SessionID); err != nil && s.Logger != nil {
					s.Logger.Warn("Failed to purge transient SIP conversation", "call_id", event.Call.ID, "error", err)
				}
			}
		}
	}
}
