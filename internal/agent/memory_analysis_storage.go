package agent

import (
	"context"
	"errors"
	"log/slog"

	"aurago/internal/memory"
)

// storeMemoryAnalysisDocument preserves curation on reused and legacy IDs.
// Only IDs owned by this write may be removed if persistence fails.
func storeMemoryAnalysisDocument(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, concept, content string, details memory.MemoryMetaUpdate, contexts ...context.Context) ([]string, error) {
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	stored, err := memory.StoreAutomaticMemoryDocumentWithContext(ctx, stm, ltm, concept, content, details)
	ids := append(append(append([]string(nil), stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
	if err != nil {
		return nil, errors.Join(err, rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.Writes))
	}
	return ids, nil
}
