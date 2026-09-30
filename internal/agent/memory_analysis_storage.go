package agent

import (
	"errors"
	"fmt"
	"log/slog"

	"aurago/internal/memory"
)

// storeMemoryAnalysisDocument preserves curation on reused and legacy IDs.
// Only IDs owned by this write may be removed if persistence fails.
func storeMemoryAnalysisDocument(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, concept, content string, details memory.MemoryMetaUpdate) ([]string, error) {
	stored, err := memory.StoreDocumentWithOwnership(ltm, concept, content)
	if err == nil && stm != nil {
		for _, id := range stored.CreatedIDs {
			if err = stm.UpsertMemoryMetaWithDetails(id, details); err != nil {
				break
			}
		}
		if err == nil {
			for _, id := range stored.UnknownIDs {
				if err = stm.EnsureMemoryMetaWithDetails(id, details); err != nil {
					break
				}
			}
		}
	}
	ids := append(append(append([]string(nil), stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
	if err == nil && len(ids) == 0 {
		err = fmt.Errorf("memory analysis returned no document IDs")
	}
	if err != nil {
		return nil, errors.Join(err, rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.CreatedIDs))
	}
	return ids, nil
}
