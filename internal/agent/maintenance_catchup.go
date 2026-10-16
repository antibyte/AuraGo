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
// budget after the normal maintenance phases, before the final morning briefing.
// The controller owns this call, so shutdown and maintenance disable cancel it.
func runConsolidationCatchup(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, ledger *maintenanceRunLedger) {
	if ctx == nil || ctx.Err() != nil || cfg == nil || !cfg.Consolidation.Enabled || cfg.Consolidation.CatchupMinutes <= 0 || stm == nil || ltm == nil || !ltm.IsReady() || ltm.IsDisabled() {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	backlog, err := stm.CountConsolidationCandidates(3)
	if err != nil {
		logger.Warn("[Consolidation] Failed to count catch-up backlog", "error", err)
		ledger.recordNamedError("consolidation_catchup", "consolidation_backlog", err)
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
	ledger.beginPhase("consolidation_catchup")
	result := consolidateSTMtoLTMWithContext(catchupCtx, cfg, logger, client, stm, ltm, kg)
	ledger.addProcessed("consolidation_catchup", result.MessagesConsolidated)
	ledger.phaseResults.ConsolidationFacts += result.FactsStored
	ledger.phaseResults.ConsolidationExcluded += result.MessagesExcluded
	for _, err := range result.Errors {
		ledger.recordError("consolidation_catchup", err)
	}
	remaining, countErr := stm.CountConsolidationCandidates(3)
	if countErr != nil {
		remaining = backlog
		logger.Warn("[Consolidation] Failed to count catch-up remainder", "error", countErr)
		ledger.recordError("consolidation_backlog", countErr)
	}
	// Outstanding archive work belongs to the last pass, not both phases.
	for i := range ledger.phaseResults.Phases {
		phase := &ledger.phaseResults.Phases[i]
		if phase.Name == "consolidation" {
			ledger.phaseResults.Deferred -= phase.Deferred
			phase.Deferred = 0
		}
	}
	ledger.phaseResults.ConsolidationBacklog = remaining
	ledger.addDeferred("consolidation_catchup", remaining)
	if catchupCtx.Err() != nil {
		ledger.addPhaseCode("consolidation_catchup", "phase_budget_exhausted")
	}
	ledger.finishPhase("consolidation_catchup", catchupCtx.Err() != nil)
	logger.Info("[Consolidation] Daily catch-up finished",
		"messages_processed", result.MessagesConsolidated,
		"facts_stored", result.FactsStored,
		"messages_excluded", result.MessagesExcluded,
		"remaining", remaining,
		"budget_minutes", minutes,
		"errors", len(result.Errors))
}
