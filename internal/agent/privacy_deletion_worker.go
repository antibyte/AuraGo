package agent

import (
	"context"
	"log/slog"
	"time"
)

const (
	privacyDeletionWorkerInterval = 30 * time.Second
	privacyDeletionWorkerBatch    = 10
)

// runPrivacyDeletionWorker is owned by the maintenance controller lifecycle,
// but deliberately does not depend on maintenance being enabled. Privacy jobs
// must resume after restart and continue their persisted retry schedule.
func runPrivacyDeletionWorker(ctx context.Context, deps MaintenanceControllerDependencies) {
	if deps.ShortTermMem == nil {
		return
	}
	run := func() {
		now := time.Now().UTC()
		if _, err := deps.ShortTermMem.CleanupPersonalityObservations(now); err != nil && ctx.Err() == nil {
			privacyDeletionWorkerWarning(deps.Logger)
		}
		if err := deps.ShortTermMem.ProcessDuePrivacyDeletionJobs(ctx, deps.LongTermMem, deps.KG, now, privacyDeletionWorkerBatch); err != nil && ctx.Err() == nil {
			privacyDeletionWorkerWarning(deps.Logger)
		}
		if deps.ContactsDB != nil {
			if err := deps.ShortTermMem.ProcessDueContactPrivacyDeletionJobs(ctx, deps.ContactsDB, deps.KG, now, privacyDeletionWorkerBatch); err != nil && ctx.Err() == nil {
				privacyDeletionWorkerWarning(deps.Logger)
			}
		}
		if err := deps.ShortTermMem.CleanupExpiredPrivacyDeletionJobs(ctx, now); err != nil && ctx.Err() == nil {
			privacyDeletionWorkerWarning(deps.Logger)
		}
	}
	run()
	ticker := time.NewTicker(privacyDeletionWorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func privacyDeletionWorkerWarning(logger *slog.Logger) {
	if logger != nil {
		// Keep errors content-free; the durable job exposes a safe error code.
		logger.Warn("privacy deletion worker cycle failed", "error_code", "privacy_deletion_cycle_failed")
	}
}
