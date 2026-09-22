package agent

import (
	"fmt"
	"log/slog"
	"strings"

	"aurago/internal/memory"
)

// Each half is acknowledged only after persistence. A successful half is never
// repeated when the other half needs its separate fallback.
type maintenanceSummaryKGResult struct {
	SummaryStored bool
	KGStored      bool
	SummaryErr    error
	KGErr         error
}

func persistMaintenanceSummaryAndKG(stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, logger *slog.Logger, date string, entries []memory.JournalEntry, result helperMaintenanceBatchResult, contentLength int) maintenanceSummaryKGResult {
	var outcome maintenanceSummaryKGResult
	if strings.TrimSpace(result.DailySummary) == "" {
		outcome.SummaryErr = fmt.Errorf("maintenance batch summary missing")
	} else {
		outcome.SummaryStored, outcome.SummaryErr = storeDailySummaryText(stm, logger, date, entries, result.DailySummary)
	}
	if !result.KGValid || kg == nil {
		outcome.KGErr = fmt.Errorf("maintenance batch KG extraction missing or invalid")
	} else {
		outcome.KGErr = storeKGExtraction(logger, kg, result.KGExtraction.Nodes, result.KGExtraction.Edges, contentLength)
		outcome.KGStored = outcome.KGErr == nil
	}
	return outcome
}
