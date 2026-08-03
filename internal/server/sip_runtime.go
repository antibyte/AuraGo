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
	s.CfgMu.RLock()
	sipConfig := s.Cfg.SIP
	s.CfgMu.RUnlock()
	if !s.Cfg.Runtime.IsDocker {
		sipphone.ApplyProviderNetworkDefaults(ctx, &sipConfig, "")
	}
	s.CfgMu.Lock()
	s.Cfg.SIP = sipConfig
	s.CfgMu.Unlock()
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
			Title: "SIP operational issue", Detail: detail,
			Fingerprint: fingerprint, OccurredAt: time.Now().UTC(),
		})
		if err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to record SIP operational issue", "error", err)
		}
	}
	finishedHook := func(call sipphone.CallRecord, persist bool) {
		if persist || call.SessionID == "" || s.ShortTermMem == nil {
			return
		}
		if err := s.ShortTermMem.PurgeChatSession(call.SessionID); err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Failed to purge transient SIP conversation", "call_id", call.ID, "error_type", fmt.Sprintf("%T", err))
			}
			reporter(context.Background(), "sip_transient_session_purge_failed", "A transient SIP session could not be purged")
		}
	}
	manager, err := sipphone.NewManager(sipConfig, s.Cfg.Directories.DataDir, runner.backendFactory, reporter, s.Logger,
		sipphone.WithDockerRuntime(s.Cfg.Runtime.IsDocker), sipphone.WithCallFinishedHook(finishedHook))
	if err != nil {
		return fmt.Errorf("initialize SIP endpoint: %w", err)
	}
	browserMedia, err := sipphone.NewBrowserMediaService(sipConfig, manager.BrowserMediaFailed)
	if err != nil {
		// Browser media is optional. Keep registration and the agent call path
		// available while app/state reports the restart-required blocker.
		if s.Logger != nil {
			s.Logger.Warn("SIP browser media is unavailable", "error", err)
		}
		browserMedia = nil
	}
	runner.SetEndCall(func(callID string) {
		callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = manager.Hangup(callCtx, callID)
	})
	runner.SetEndCallInternal(manager.EndCallInternal)
	s.SIPPhone = manager
	s.SIPBrowserMedia = browserMedia
	s.VoiceActionRunner = runner
	sipphone.SetDefaultManager(manager)
	if s.ShortTermMem != nil {
		sessionIDs, listErr := manager.NonPersistentSessionIDs(ctx)
		if listErr != nil {
			reporter(ctx, "sip_transient_session_purge_failed", "Transient SIP sessions could not be enumerated")
		} else {
			for _, sessionID := range sessionIDs {
				if purgeErr := s.ShortTermMem.PurgeChatSession(sessionID); purgeErr != nil {
					reporter(ctx, "sip_transient_session_purge_failed", "An orphaned transient SIP session could not be purged")
				}
			}
		}
	}
	if err := manager.Start(ctx); err != nil {
		if browserMedia != nil {
			_ = browserMedia.Close()
		}
		_ = manager.Close()
		s.SIPPhone = nil
		s.SIPBrowserMedia = nil
		sipphone.SetDefaultManager(nil)
		return fmt.Errorf("start SIP endpoint: %w", err)
	}
	if s.Cfg.SIP.HistoryRetentionDays > 0 {
		_ = manager.PruneHistory(ctx, time.Now().AddDate(0, 0, -s.Cfg.SIP.HistoryRetentionDays))
	}
	return nil
}

func (s *Server) cleanupTransientSIPSessions(ctx context.Context, manager *sipphone.Manager) {
	events, unsubscribe := manager.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Call == nil || event.Call.State != sipphone.StateEnded || event.Call.SessionID == "" {
				continue
			}
			persist := false
			if data, ok := event.Data.(map[string]any); ok {
				persist, _ = data["persist_transcripts"].(bool)
			}
			if !persist && s.ShortTermMem != nil {
				if err := s.ShortTermMem.PurgeChatSession(event.Call.SessionID); err != nil && s.Logger != nil {
					s.Logger.Warn("Failed to purge transient SIP conversation", "call_id", event.Call.ID, "error", err)
				}
			}
		}
	}
}
