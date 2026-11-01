package agent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"aurago/internal/config"
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
		return nil, errors.Join(err, rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.Writes, ctx))
	}
	return ids, nil
}

func storeMemoryAnalysisDocumentWithExpiry(ctx context.Context, cfg *config.Config, stm *memory.SQLiteMemory, ltm memory.VectorDB, concept, content string, details memory.MemoryMetaUpdate, expiry time.Time) ([]string, error) {
	stored, err := publishModelMemory(ctx, cfg, stm, ltm, concept, content, details, &expiry)
	if err != nil {
		return nil, err
	}
	return append(append([]string(nil), stored.CreatedIDs...), stored.ReusedIDs...), nil
}
