package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

const defaultMaintenanceFailureSkipThreshold = 3

func normalizeMaintenanceFailureKey(action, targetID string) (string, string) {
	return strings.TrimSpace(strings.ToLower(action)), strings.TrimSpace(targetID)
}

func (s *SQLiteMemory) RecordMemoryMaintenanceFailure(action, targetID string, cause error) error {
	if s == nil || cause == nil {
		return nil
	}
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO memory_maintenance_failures (action, target_id, failure_count, last_error)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(action, target_id) DO UPDATE SET
			failure_count = failure_count + 1,
			last_error = excluded.last_error,
			last_failed_at = CURRENT_TIMESTAMP
	`, action, targetID, cause.Error())
	if err != nil {
		return fmt.Errorf("record memory maintenance failure: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ClearMemoryMaintenanceFailure(action, targetID string) error {
	if s == nil {
		return nil
	}
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return nil
	}
	if _, err := s.db.Exec(`DELETE FROM memory_maintenance_failures WHERE action = ? AND target_id = ?`, action, targetID); err != nil {
		return fmt.Errorf("clear memory maintenance failure: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) MemoryMaintenanceFailureCount(action, targetID string) (int, error) {
	if s == nil {
		return 0, nil
	}
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return 0, nil
	}
	var count int
	err := s.db.QueryRow(`SELECT failure_count FROM memory_maintenance_failures WHERE action = ? AND target_id = ?`, action, targetID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read memory maintenance failure count: %w", err)
	}
	return count, nil
}

func (s *SQLiteMemory) ShouldSkipMemoryMaintenanceAction(action, targetID string, threshold int) bool {
	if threshold <= 0 {
		threshold = defaultMaintenanceFailureSkipThreshold
	}
	count, err := s.MemoryMaintenanceFailureCount(action, targetID)
	return err == nil && count >= threshold
}
