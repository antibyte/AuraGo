package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

const pendingMemoryWriteRetryInterval = 5 * time.Minute

// startPendingMemoryWriteRetryLoop starts the retry worker and returns a
// completion signal for lifecycle owners that must wait before closing STM or
// LTM. The signal is already closed when either dependency is unavailable.
func startPendingMemoryWriteRetryLoop(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, configSnapshot func() *config.Config) <-chan struct{} {
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
			retryPendingMemoryWrites(ctx, logger, stm, ltm, configSnapshot())
			if ctx.Err() == nil {
				if _, err := stm.CleanupExpiredPendingMemoryWrites(ctx, time.Now().UTC(), 100); err != nil && logger != nil {
					logger.Warn("[Memory Retry] Failed to clean expired exhausted payloads", "error", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}

func retryPendingMemoryWrites(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, configs ...*config.Config) (succeeded, failed int) {
	if stm == nil || ltm == nil {
		return 0, 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return 0, 0
	}
	writes, err := stm.GetDuePendingMemoryWritesContext(ctx, time.Now().UTC(), 20)
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
		if write.MetadataError != nil {
			failed++
			if err := stm.MarkPendingMemoryWriteFailedContext(ctx, write.ID, write.MetadataError, time.Now().UTC()); err != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to record invalid metadata", "id", write.ID, "error", err)
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
		details := memory.MemoryMetaUpdate{
			VerificationStatus: "unverified", SourceType: sourceType, SourceReliability: reliability,
		}
		if write.Metadata != nil {
			details = *write.Metadata
		}
		if details.SourceSessionID == "" {
			details.SourceSessionID, _ = memory.MemoryAnalysisSession(write.Content)
		}
		if !shouldStoreExtractedMemory(write.Concept, "") {
			_ = stm.CompletePendingMemoryWriteContext(ctx, write.ID)
			continue
		}
		if len(configs) > 0 && !runtimeMemoryWriteAllowed(configs[0], "memory", details.SourceSessionID) {
			// A currently closed session gate is not a failed write. Move it out
			// of the next due page so later sessions can make progress this cycle.
			if err := stm.DeferPendingMemoryWriteContext(ctx, write.ID, time.Now().UTC()); err != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to defer session-gated write", "id", write.ID, "error", err)
			}
			continue
		}
		stored, err := memory.StoreAutomaticMemoryDocumentWithContext(ctx, stm, ltm, write.Concept, write.Content, details)
		if err != nil {
			if errors.Is(err, memory.ErrAutomaticMemorySkipped) {
				_ = stm.CompletePendingMemoryWriteContext(ctx, write.ID)
				continue
			}
			if rollbackErr := rollbackPendingMemoryWriteCreatedIDs(logger, stm, ltm, stored.Writes, ctx); rollbackErr != nil {
				err = errors.Join(err, rollbackErr)
			}
			failed++
			if ctx.Err() != nil {
				break
			}
			var markErr error
			if isTransientHelperError(err) {
				markErr = stm.DeferPendingMemoryWriteContext(ctx, write.ID, time.Now().UTC())
			} else {
				markErr = stm.MarkPendingMemoryWriteFailedContext(ctx, write.ID, err, time.Now().UTC())
			}
			if markErr != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to update pending write", "id", write.ID, "error", markErr)
			}
			continue
		}
		ids := append(append(append([]string(nil), stored.CreatedIDs...), stored.ReusedIDs...), stored.UnknownIDs...)
		if len(ids) == 0 {
			err := fmt.Errorf("memory retry returned no document IDs")
			failed++
			if markErr := stm.MarkPendingMemoryWriteFailedContext(ctx, write.ID, err, time.Now().UTC()); markErr != nil && logger != nil {
				logger.Warn("[Memory Retry] Failed to update pending write", "id", write.ID, "error", markErr)
			}
			continue
		}
		if err := detectMemoryConflictsForDocIDsWithContext(ctx, logger, stm, ltm, ids, write.Concept+"\n\n"+write.Content, conflictStoredDocument); err != nil && logger != nil {
			logger.Warn("[Memory Retry] Conflict check failed", "error", err)
		}
		if err := stm.CompletePendingMemoryWriteContext(ctx, write.ID); err != nil {
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

func rollbackPendingMemoryWriteCreatedIDs(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, writes []memory.AutomaticMemoryWrite, contexts ...context.Context) error {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	cleanupCtx, cancel := boundedMaintenanceRollbackContext(ctx)
	defer cancel()
	err := stm.RollbackAutomaticMemoryWritesContext(cleanupCtx, ltm, writes)
	if err != nil && logger != nil {
		logger.Warn("[Memory Retry] Rollback retained unproven or changed memories", "error", err)
	}
	return err
}
