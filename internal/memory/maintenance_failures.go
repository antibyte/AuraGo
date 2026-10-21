package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const defaultMaintenanceFailureSkipThreshold = 3
const maintenanceFailureRetryDueSQL = `datetime(last_failed_at, '+' || CASE WHEN failure_count<2 THEN 5 WHEN failure_count<3 THEN 30 ELSE 120 END || ' minutes') <= CURRENT_TIMESTAMP`

func normalizeMaintenanceFailureKey(action, targetID string) (string, string) {
	return strings.TrimSpace(strings.ToLower(action)), strings.TrimSpace(targetID)
}

func (s *SQLiteMemory) RecordMemoryMaintenanceFailure(action, targetID string, cause error) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.RecordMemoryMaintenanceFailureContext(ctx, action, targetID, cause)
}

func (s *SQLiteMemory) RecordMemoryMaintenanceFailureContext(ctx context.Context, action, targetID string, cause error) error {
	if s == nil || cause == nil {
		return nil
	}
	ctx = maintenanceContext(ctx)
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_maintenance_failures (action, target_id, failure_count, last_error)
		VALUES (?, ?, 1, 'maintenance_action_failed')
		ON CONFLICT(action, target_id) DO UPDATE SET
			failure_count = failure_count + 1,
			last_error = excluded.last_error,
			last_failed_at = CURRENT_TIMESTAMP
	`, action, targetID)
	if err != nil {
		return fmt.Errorf("record memory maintenance failure: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ClearMemoryMaintenanceFailure(action, targetID string) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.ClearMemoryMaintenanceFailureContext(ctx, action, targetID)
}

func (s *SQLiteMemory) ClearMemoryMaintenanceFailureContext(ctx context.Context, action, targetID string) error {
	if s == nil {
		return nil
	}
	ctx = maintenanceContext(ctx)
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_maintenance_failures WHERE action = ? AND target_id = ?`, action, targetID); err != nil {
		return fmt.Errorf("clear memory maintenance failure: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) MemoryMaintenanceFailureCount(action, targetID string) (int, error) {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.MemoryMaintenanceFailureCountContext(ctx, action, targetID)
}

func (s *SQLiteMemory) MemoryMaintenanceFailureCountContext(ctx context.Context, action, targetID string) (int, error) {
	if s == nil {
		return 0, nil
	}
	ctx = maintenanceContext(ctx)
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return 0, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT failure_count FROM memory_maintenance_failures WHERE action = ? AND target_id = ?`, action, targetID).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read memory maintenance failure count: %w", err)
	}
	return count, nil
}

// GetMemoryMaintenanceFailureRetryPage returns a bounded keyset page of due
// failures after the persisted retry cursor, wrapping to the start when the tail is exhausted.
// The cursor is intentionally advanced separately, after the caller finishes
// the whole page, so cancellation cannot skip unfinished retries.
func (s *SQLiteMemory) GetMemoryMaintenanceFailureRetryPage(ctx context.Context, action string, limit int) ([]string, string, error) {
	if s == nil || s.db == nil {
		return nil, "", nil
	}
	ctx = maintenanceContext(ctx)
	action, _ = normalizeMaintenanceFailureKey(action, "")
	if action == "" {
		return nil, "", fmt.Errorf("maintenance failure action is required")
	}
	if limit <= 0 || limit > 25 {
		limit = 25
	}
	var cursor string
	err := s.db.QueryRowContext(ctx, `SELECT target_id FROM memory_maintenance_retry_cursors WHERE action=?`, action).Scan(&cursor)
	if err != nil && err != sql.ErrNoRows {
		return nil, "", fmt.Errorf("read memory maintenance retry cursor: %w", err)
	}
	load := func(after string) ([]string, error) {
		rows, err := s.db.QueryContext(ctx, `SELECT target_id FROM memory_maintenance_failures WHERE action=? AND target_id>? AND `+maintenanceFailureRetryDueSQL+` ORDER BY target_id LIMIT ?`, action, after, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		ids := make([]string, 0, limit)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
	ids, err := load(cursor)
	if err != nil {
		return nil, cursor, fmt.Errorf("list memory maintenance failures after cursor: %w", err)
	}
	if len(ids) == 0 && cursor != "" {
		ids, err = load("")
		if err != nil {
			return nil, cursor, fmt.Errorf("wrap memory maintenance failure cursor: %w", err)
		}
	}
	return ids, cursor, nil
}

func (s *SQLiteMemory) AdvanceMemoryMaintenanceFailureRetryCursor(ctx context.Context, action, targetID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	ctx = maintenanceContext(ctx)
	action, targetID = normalizeMaintenanceFailureKey(action, targetID)
	if action == "" || targetID == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_maintenance_retry_cursors(action,target_id)
		VALUES(?,?) ON CONFLICT(action) DO UPDATE SET target_id=excluded.target_id,updated_at=CURRENT_TIMESTAMP`, action, targetID); err != nil {
		return fmt.Errorf("advance memory maintenance retry cursor: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ShouldSkipMemoryMaintenanceAction(action, targetID string, threshold int) bool {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	skip, _ := s.ShouldSkipMemoryMaintenanceActionContext(ctx, action, targetID, threshold)
	return skip
}

func (s *SQLiteMemory) ShouldSkipMemoryMaintenanceActionContext(ctx context.Context, action, targetID string, threshold int) (bool, error) {
	if threshold <= 0 {
		threshold = defaultMaintenanceFailureSkipThreshold
	}
	count, err := s.MemoryMaintenanceFailureCountContext(ctx, action, targetID)
	if err != nil {
		return false, err
	}
	return count >= threshold, nil
}
