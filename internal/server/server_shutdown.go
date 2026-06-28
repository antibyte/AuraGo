package server

import (
	"database/sql"
	"log/slog"

	"aurago/internal/tools"
)

// closeRuntimeResources releases server-owned runtime handles during graceful shutdown.
func (s *Server) closeRuntimeResources() {
	if s == nil {
		return
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
