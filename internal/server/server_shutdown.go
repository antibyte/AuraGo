package server

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"aurago/internal/tools"
)

// shutdownDesktopStorage closes the shared Desktop service, hub and store when
// shutdown is requested. The Video Studio worker writes through the Desktop
// service, so it is cancelled and drained first, and getDesktopService refuses
// to reopen the service afterwards.
func (s *Server) shutdownDesktopStorage() {
	s.closeVideoStudioManager()
	s.revokeDesktopRuns()
	s.DesktopMu.Lock()
	s.desktopClosed = true
	if s.DesktopHub != nil {
		s.DesktopHub.Close()
		s.DesktopHub = nil
	}
	if s.DesktopService != nil {
		_ = s.DesktopService.Close()
		s.DesktopService = nil
	}
	if s.DesktopStore != nil {
		_ = s.DesktopStore.Close()
		s.DesktopStore = nil
	}
	s.DesktopMu.Unlock()
	// Note: we intentionally do NOT call CloseToolDesktopService() here.
	// Many tests create short-lived servers; a global close would tear down
	// services belonging to other parallel tests. The real production server
	// closes its own DesktopService (which is the one registered via Set).
}

// closeRuntimeResources releases server-owned runtime handles during graceful shutdown.
func (s *Server) closeRuntimeResources() {
	if s == nil {
		return
	}
	s.closeVideoStudioManager()
	s.stopRocketChatBot()
	s.stopHomeAssistantPoller()
	s.stopFritzPoller()
	s.stopUptimeKumaPoller()
	if s.EmailWatcher != nil {
		s.EmailWatcher.Stop()
		s.EmailWatcher = nil
	}
	s.fritzWidgetMu.Lock()
	widget := s.fritzWidget
	s.fritzWidgetMu.Unlock()
	if widget != nil {
		widget.close()
	}
	if s.MissionManagerV2 != nil {
		s.MissionManagerV2.Stop()
	}
	s.missionRunTracker().close()

	if manager := currentVaultSecretPrompter(s); manager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		manager.Shutdown(ctx)
		cancel()
	}

	if s.PreparationService != nil {
		s.PreparationService.Stop()
	}
	if s.WorkspaceSearch != nil {
		tools.SetFileAccessTracker(nil)
		s.WorkspaceSearch.Stop()
		if err := s.WorkspaceSearch.Close(); err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to close workspace search service", "error", err)
		}
		s.WorkspaceSearch = nil
	}

	if s.RemoteHub != nil {
		s.RemoteHub.SetEnabled(false)
	}

	if s.SQLConnectionPool != nil {
		s.SQLConnectionPool.CloseAll()
	}

	if s.CronManager != nil {
		_ = s.CronManager.Close()
		s.CronManager = nil
	}
	if s.BackgroundTasks != nil {
		_ = s.BackgroundTasks.Close()
		s.BackgroundTasks = nil
	}
	if broker := currentAgodeskDesktopBroker(s); broker != nil {
		broker.knowledgeMu.Lock()
		broker.knowledgeClosed = true
		broker.knowledgeCtx = nil
		knowledge := broker.knowledge
		broker.knowledge = nil
		broker.knowledgeMu.Unlock()
		if knowledge != nil {
			knowledge.Close()
		}
	}

	closeSQLiteHandle(s.Logger, &s.SkillsDB, "skills")
	closeSQLiteHandle(s.Logger, &s.MissionHistoryDB, "mission_history")
	closeSQLiteHandle(s.Logger, &s.PreparedMissionsDB, "prepared_missions")

	closeGalaxaDB(s.Logger)
}

func closeSQLiteHandle(logger *slog.Logger, db **sql.DB, name string) {
	if db == nil || *db == nil {
		return
	}
	if err := (*db).Close(); err != nil && logger != nil {
		logger.Warn("Failed to close SQLite database", "db", name, "error", err)
	}
	*db = nil
}
