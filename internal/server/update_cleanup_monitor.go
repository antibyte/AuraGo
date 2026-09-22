package server

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"aurago/internal/planner"
	"aurago/internal/upkeep"
)

const updateCleanupFingerprint = "updater|artifact_cleanup"

// StartUpdateCleanupMonitor imports updater results without emitting messages.
// The returned stop function waits for the worker before database teardown.
func StartUpdateCleanupMonitor(stop <-chan struct{}, db *sql.DB, root string, logger *slog.Logger) func() {
	if db == nil || root == "" {
		return func() {}
	}
	if filepath.Base(root) == "bin" {
		root = filepath.Dir(root)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := reconcileUpdateCleanup(db, root); err != nil && logger != nil {
				logger.Warn("Update cleanup status could not be reconciled")
			}
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() { cancel(); <-done }
}

func reconcileUpdateCleanup(db *sql.DB, root string) error {
	r, err := upkeep.ReadResult(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	const key = "update_cleanup_result_sequence"
	last, err := planner.GetPlannerMeta(db, key)
	if err != nil {
		return err
	}
	seq := strconv.FormatInt(r.Sequence, 10)
	if last == seq {
		return nil
	}
	if r.Success {
		_, err = planner.ResolveOperationalIssue(db, updateCleanupFingerprint, "Update artifact cleanup succeeded.", time.Now())
	} else if r.Failures >= 2 {
		_, err = planner.RecordOperationalIssue(db, planner.OperationalIssue{
			Source: "updater", Context: "artifact_cleanup", Fingerprint: updateCleanupFingerprint,
			Kind: planner.OperationalIssueKindRuntimeFailure, Severity: "warning",
			Title:      "Update artifact cleanup repeatedly failed",
			Detail:     "Automatic cleanup could not complete. Review the local update maintenance preview; rollback data remains protected.",
			OccurredAt: time.Now(),
		})
	}
	if err != nil {
		return err
	}
	return planner.SetPlannerMeta(db, key, seq)
}
