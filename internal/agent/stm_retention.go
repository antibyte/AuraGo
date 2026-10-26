package agent

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"context"
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
	if err := stm.EnforceSTMPRetentionForSessionContext(context.Background(), sessionID, keepN, 250); err != nil && logger != nil {
		logger.Warn("STM retention enforcement failed", "session_id", sessionID, "keep_n", keepN, "error", err)
	}
}

// RunSTMPRetentionMaintenance enforces STM retention for every session with stored messages.
func RunSTMPRetentionMaintenance(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory) error {
	return RunSTMPRetentionMaintenanceContext(context.Background(), cfg, logger, stm)
}

func RunSTMPRetentionMaintenanceContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory) error {
	if cfg == nil || stm == nil {
		return nil
	}
	keepN := cfg.Consolidation.StmRetentionMessages
	if keepN <= 0 {
		return nil
	}
	sessions, err := stm.EnforceSTMPRetentionContext(ctx, keepN, 1000)
	if err != nil {
		if logger != nil {
			logger.Warn("[Maintenance] STM retention enforcement had failures", "sessions_ok", sessions, "keep_n", keepN, "error", err)
		}
		return err
	}
	if sessions > 0 && logger != nil {
		logger.Info("[Maintenance] STM retention enforced", "sessions", sessions, "keep_n", keepN)
	}
	return nil
}
