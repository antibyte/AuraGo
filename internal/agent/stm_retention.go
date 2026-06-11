package agent

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"log/slog"
)

// EnforceSTMPRetentionIfConfigured trims STM rows for one session when consolidation.stm_retention_messages is set.
func EnforceSTMPRetentionIfConfigured(cfg *config.Config, stm *memory.SQLiteMemory, sessionID string, logger *slog.Logger) {
	if cfg == nil || stm == nil || sessionID == "" {
		return
	}
	keepN := cfg.Consolidation.StmRetentionMessages
	if keepN <= 0 {
		return
	}
	if err := stm.EnforceSTMPRetentionForSession(sessionID, keepN); err != nil && logger != nil {
		logger.Warn("STM retention enforcement failed", "session_id", sessionID, "keep_n", keepN, "error", err)
	}
}

// RunSTMPRetentionMaintenance enforces STM retention for every session with stored messages.
func RunSTMPRetentionMaintenance(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory) {
	if cfg == nil || stm == nil {
		return
	}
	keepN := cfg.Consolidation.StmRetentionMessages
	if keepN <= 0 {
		return
	}
	sessions, err := stm.EnforceSTMPRetention(keepN)
	if err != nil {
		if logger != nil {
			logger.Warn("[Maintenance] STM retention enforcement had failures", "sessions_ok", sessions, "keep_n", keepN, "error", err)
		}
		return
	}
	if sessions > 0 && logger != nil {
		logger.Info("[Maintenance] STM retention enforced", "sessions", sessions, "keep_n", keepN)
	}
}