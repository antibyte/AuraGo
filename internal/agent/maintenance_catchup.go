package agent

import (
	"context"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
)

const maxConsolidationCatchupMinutes = 60

// runConsolidationCatchup gives old conversation turns a separate, opt-in
// budget after the normal maintenance run has persisted its morning briefing.
// The controller owns this call, so shutdown and maintenance disable cancel it.
func runConsolidationCatchup(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) {
	if ctx == nil || ctx.Err() != nil || cfg == nil || !cfg.Consolidation.Enabled || cfg.Consolidation.CatchupMinutes <= 0 || stm == nil || ltm == nil || !ltm.IsReady() || ltm.IsDisabled() {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	backlog, err := stm.CountConsolidationCandidates(3)
	if err != nil {
		logger.Warn("[Consolidation] Failed to count catch-up backlog", "error", err)
		return
	}
	if backlog == 0 {
		return
	}
	minutes := cfg.Consolidation.CatchupMinutes
	if minutes > maxConsolidationCatchupMinutes {
		minutes = maxConsolidationCatchupMinutes
	}
	catchupCtx, cancel := context.WithTimeout(ctx, time.Duration(minutes)*time.Minute)
	defer cancel()
	result := consolidateSTMtoLTMWithContext(catchupCtx, cfg, logger, client, stm, ltm, kg)
	remaining, countErr := stm.CountConsolidationCandidates(3)
	if countErr != nil {
		remaining = -1
		logger.Warn("[Consolidation] Failed to count catch-up remainder", "error", countErr)
	}
	logger.Info("[Consolidation] Daily catch-up finished",
		"messages_processed", result.MessagesConsolidated,
		"facts_stored", result.FactsStored,
		"messages_excluded", result.MessagesExcluded,
		"remaining", remaining,
		"budget_minutes", minutes,
		"errors", len(result.Errors))
}
