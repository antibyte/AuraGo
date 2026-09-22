package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
)

const pendingMemoryWriteRetryInterval = 5 * time.Minute

// startPendingMemoryWriteRetryLoop starts the retry worker and returns a
// completion signal for lifecycle owners that must wait before closing STM or
// LTM. The signal is already closed when either dependency is unavailable.
func startPendingMemoryWriteRetryLoop(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) <-chan struct{} {
	done := make(chan struct{})
	if stm == nil || ltm == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(pendingMemoryWriteRetryInterval)
		defer ticker.Stop()
		for {
			retryPendingMemoryWrites(ctx, logger, stm, ltm)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}

func retryPendingMemoryWrites(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) (succeeded, failed int) {
	if stm == nil || ltm == nil {
		return 0, 0
	}
	writes, err := stm.GetDuePendingMemoryWrites(time.Now().UTC(), 20)
	if err != nil {
		if logger != nil {
			logger.Warn("[Memory Retry] Failed to load pending writes", "error", err)
		}
		return 0, 0
	}
	for _, write := range writes {
		if err := ctx.Err(); err != nil {
			break
		}
		stored, err := memory.StoreDocumentWithOwnership(ltm, write.Concept, write.Content)
		if err != nil {
			if rollbackErr := rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.CreatedIDs); rollbackErr != nil {
				err = errors.Join(err, rollbackErr)
			}
			failed++
			if markErr := stm.MarkPendingMemoryWriteFailed(write.ID, err, time.Now().UTC()); markErr != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to update pending write", "id", write.ID, "error", markErr)
			}
			continue
		}
		ids := append(append(append([]string(nil), stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
		if len(ids) == 0 {
			err := fmt.Errorf("memory retry returned no document IDs")
			failed++
			if markErr := stm.MarkPendingMemoryWriteFailed(write.ID, err, time.Now().UTC()); markErr != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to update pending write", "id", write.ID, "error", markErr)
			}
			continue
		}
		sourceType := strings.TrimSpace(write.Domain)
		if sourceType == "" {
			sourceType = "system"
		}
		reliability := 0.85
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(write.Concept)), "[correction:") {
			reliability = 0.90
		}
		metadataErr := error(nil)
		for _, id := range stored.CreatedIDs {
			if err := stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				VerificationStatus: "unverified",
				SourceType:         sourceType,
				SourceReliability:  reliability,
			}); err != nil {
				metadataErr = fmt.Errorf("upsert metadata for %s: %w", id, err)
				break
			}
		}
		if metadataErr == nil {
			for _, id := range stored.UnknownIDs {
				if err := stm.EnsureMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
					VerificationStatus: "unverified",
					SourceType:         sourceType,
					SourceReliability:  reliability,
				}); err != nil {
					metadataErr = fmt.Errorf("ensure metadata for unknown ownership %s: %w", id, err)
					break
				}
			}
		}
		if metadataErr != nil {
			if rollbackErr := rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.CreatedIDs); rollbackErr != nil {
				metadataErr = errors.Join(metadataErr, rollbackErr)
			}
			failed++
			if markErr := stm.MarkPendingMemoryWriteFailed(write.ID, metadataErr, time.Now().UTC()); markErr != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to update pending write metadata", "id", write.ID, "error", markErr)
			}
			continue
		}
		detectMemoryConflictsForDocIDs(logger, stm, ltm, ids, write.Concept)
		if err := stm.CompletePendingMemoryWrite(write.ID); err != nil {
			failed++
			if logger != nil {
				logger.Warn("[Memory Retry] Failed to complete pending write", "id", write.ID, "error", err)
			}
			continue
		}
		succeeded++
	}
	if logger != nil && (succeeded > 0 || failed > 0) {
		logger.Info("[Memory Retry] Processed pending writes", "succeeded", succeeded, "failed", failed)
	}
	return succeeded, failed
}

func rollbackPendingMemoryWriteCreatedIDs(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, ids []string) error {
	var resultErr error
	for _, id := range ids {
		if ltm == nil {
			continue
		}
		if err := ltm.DeleteDocument(id); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback pending memory vector %s: %w", id, err))
			if logger != nil {
				logger.Warn("[Memory Retry] Failed to roll back partially stored vector", "doc_id", id, "error", err)
			}
			continue
		}
		if stm != nil {
			resultErr = errors.Join(resultErr, stm.DeleteDocumentCleanup(id))
		}
	}
	return resultErr
}
