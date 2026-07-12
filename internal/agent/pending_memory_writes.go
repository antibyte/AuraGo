package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
)

const pendingMemoryWriteRetryInterval = 5 * time.Minute

func startPendingMemoryWriteRetryLoop(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) {
	if stm == nil || ltm == nil {
		return
	}
	go func() {
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
		ids, err := ltm.StoreDocument(write.Concept, write.Content)
		if err != nil {
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
		for _, id := range ids {
			if err := stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				VerificationStatus: "unverified",
				SourceType:         sourceType,
				SourceReliability:  reliability,
			}); err != nil {
				metadataErr = fmt.Errorf("upsert metadata for %s: %w", id, err)
				break
			}
		}
		if metadataErr != nil {
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
