package memory

import (
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
	INSERT INTO archived_messages (session_id, role, content, original_timestamp)
	SELECT session_id, role, content, timestamp
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

// EnforceSTMPRetentionForSession archives and deletes STM messages older than the newest keepN rows.
func (s *SQLiteMemory) EnforceSTMPRetentionForSession(sessionID string, keepN int) error {
	return s.DeleteOldMessages(sessionID, keepN)
}

// EnforceSTMPRetention applies per-session retention across every session that has STM rows.
// Returns the number of sessions for which retention ran without error (including no-op runs).
func (s *SQLiteMemory) EnforceSTMPRetention(keepN int) (int, error) {
	if keepN <= 0 {
		return 0, fmt.Errorf("keepN must be positive, got %d", keepN)
	}
	rows, err := s.db.Query(`SELECT DISTINCT session_id FROM messages`)
	if err != nil {
		return 0, fmt.Errorf("list stm sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return 0, fmt.Errorf("scan stm session id: %w", err)
		}
		sessionIDs = append(sessionIDs, sessionID)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	pruned := 0
	var errs []string
	for _, sessionID := range sessionIDs {
		if err := s.DeleteOldMessages(sessionID, keepN); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", sessionID, err))
			continue
		}
		pruned++
	}
	if len(errs) > 0 {
		return pruned, fmt.Errorf("stm retention failures: %s", strings.Join(errs, "; "))
	}
	return pruned, nil
}