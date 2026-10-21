package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// archiveableRolesSQL restricts which STM rows are copied into archived_messages
// for nightly STM→LTM consolidation.
const archiveableRolesSQL = `role IN ('user', 'assistant', 'tool')`

// archiveMessagesInTx copies matching messages into archived_messages within an open transaction.
// extraWhere must start with "AND" when non-empty. sessionID is always bound as the first arg.
func (s *SQLiteMemory) archiveMessagesInTx(tx *sql.Tx, sessionID, extraWhere string, extraArgs ...interface{}) (int64, error) {
	query := `
	INSERT INTO archived_messages (session_id, role, content, is_internal, original_timestamp)
	SELECT session_id, role, content, is_internal, timestamp
	FROM messages
	WHERE session_id = ? AND ` + archiveableRolesSQL
	if extraWhere != "" {
		query += " " + extraWhere
	}
	query += " ORDER BY timestamp ASC, id ASC"

	args := make([]interface{}, 0, 1+len(extraArgs))
	args = append(args, sessionID)
	args = append(args, extraArgs...)

	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to archive messages: %w", err)
	}
	archived, _ := res.RowsAffected()
	return archived, nil
}

// deleteMessagesInTx removes messages for a session matching extraWhere (must start with "AND" when set).
func (s *SQLiteMemory) deleteMessagesInTx(tx *sql.Tx, sessionID, extraWhere string, extraArgs ...interface{}) (int64, error) {
	query := `DELETE FROM messages WHERE session_id = ?`
	if extraWhere != "" {
		query += " " + extraWhere
	}
	args := make([]interface{}, 0, 1+len(extraArgs))
	args = append(args, sessionID)
	args = append(args, extraArgs...)
	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to delete messages: %w", err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// EnforceSTMPRetentionForSession archives and deletes up to 1000 eligible STM messages older than the newest keepN rows.
func (s *SQLiteMemory) EnforceSTMPRetentionForSession(sessionID string, keepN int) error {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.EnforceSTMPRetentionForSessionContext(ctx, sessionID, keepN, 1000)
}

// EnforceSTMPRetention advances the bounded global retention cursor by at most 5000 STM rows.
// It returns the number of distinct sessions processed in committed pages.
func (s *SQLiteMemory) EnforceSTMPRetention(keepN int) (int, error) {
	ctx, cancel := maintenanceCompatibilityContext()
	defer cancel()
	return s.EnforceSTMPRetentionContext(ctx, keepN, 5000)
}

// CleanOldArchivedMessagesContext deletes one bounded page of terminal archive
// rows. The archive-delete trigger removes any source-bound checkpoints in the
// same transaction; interrupted cleanup is resumed by the next maintenance run.
func (s *SQLiteMemory) CleanOldArchivedMessagesContext(ctx context.Context, days, limit int) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("memory archive store is unavailable")
	}
	if days <= 0 {
		return 0, fmt.Errorf("archive retention days must be positive")
	}
	ctx = maintenanceContext(ctx)
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin archived memory cleanup: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM archived_messages
		WHERE consolidated=1 AND consolidation_status IN ('done','excluded')
		  AND archived_at < datetime('now', ?)
		ORDER BY id LIMIT ?`, fmt.Sprintf("-%d days", days), limit)
	if err != nil {
		return 0, fmt.Errorf("select terminal archived messages for cleanup: %w", err)
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan terminal archived message for cleanup: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate terminal archived messages for cleanup: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close terminal archived messages for cleanup: %w", err)
	}
	if len(ids) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit empty archived memory cleanup: %w", err)
		}
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+2)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, days)
	query := `DELETE FROM archived_messages WHERE id IN (` + strings.Join(placeholders, ",") + `)
		AND consolidated=1 AND consolidation_status IN ('done','excluded')
		AND archived_at < datetime('now', '-' || ? || ' days')`
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete terminal archived messages: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted archived messages: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit terminal archived message cleanup: %w", err)
	}
	return deleted, nil
}
