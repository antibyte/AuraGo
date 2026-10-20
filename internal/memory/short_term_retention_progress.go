package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const stmRetentionCursorKey = "maintenance.stm_retention.cursor_id"
const stmRetentionPageSize = 250

type stmRetentionRow struct {
	id      int64
	session string
	pinned  bool
}

type stmRetentionCutoff struct {
	id        int64
	timestamp sql.NullString
	found     bool
}

// EnforceSTMPRetentionContext advances a persisted global message-id cursor.
// It inspects at most maxRows IDs, and archives/deletes at most that many rows
// in bounded transactions. The cursor is committed with each archive/delete.
func (s *SQLiteMemory) EnforceSTMPRetentionContext(ctx context.Context, keepN, maxRows int) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	if keepN <= 0 {
		return 0, fmt.Errorf("keepN must be positive, got %d", keepN)
	}
	ctx = maintenanceContext(ctx)
	if maxRows <= 0 || maxRows > 5000 {
		maxRows = 1000
	}
	processedSessions := make(map[string]struct{})
	for scanned := 0; scanned < maxRows; {
		if err := ctx.Err(); err != nil {
			return len(processedSessions), err
		}
		pageSize := min(stmRetentionPageSize, maxRows-scanned)
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return len(processedSessions), fmt.Errorf("begin STM retention page: %w", err)
		}
		cursor := int64(0)
		if err := tx.QueryRowContext(ctx, `SELECT CAST(value AS INTEGER) FROM memory_maintenance_meta WHERE key=?`, stmRetentionCursorKey).Scan(&cursor); err != nil && err != sql.ErrNoRows {
			tx.Rollback()
			return len(processedSessions), fmt.Errorf("read STM retention cursor: %w", err)
		}
		var lastID int64
		rows, err := tx.QueryContext(ctx, `SELECT id,session_id,is_pinned FROM messages WHERE id>? ORDER BY id LIMIT ?`, cursor, pageSize)
		if err != nil {
			tx.Rollback()
			return len(processedSessions), fmt.Errorf("read STM retention cursor page: %w", err)
		}
		page := make([]stmRetentionRow, 0, pageSize)
		for rows.Next() {
			var row stmRetentionRow
			if err := rows.Scan(&row.id, &row.session, &row.pinned); err != nil {
				rows.Close()
				tx.Rollback()
				return len(processedSessions), fmt.Errorf("scan STM retention cursor page: %w", err)
			}
			page = append(page, row)
			lastID = row.id
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			tx.Rollback()
			return len(processedSessions), fmt.Errorf("iterate STM retention cursor page: %w", err)
		}
		if err := rows.Close(); err != nil {
			tx.Rollback()
			return len(processedSessions), fmt.Errorf("close STM retention cursor page: %w", err)
		}
		if len(page) == 0 {
			if _, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=?`, stmRetentionCursorKey); err != nil {
				tx.Rollback()
				return len(processedSessions), fmt.Errorf("reset STM retention cursor: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return len(processedSessions), fmt.Errorf("commit STM retention cursor reset: %w", err)
			}
			return len(processedSessions), nil
		}
		var pruneIDs []int64
		pageSessions := make(map[string]struct{})
		cutoffs := make(map[string]stmRetentionCutoff)
		candidates := make([]stmRetentionRow, 0, len(page))
		for _, row := range page {
			if err := ctx.Err(); err != nil {
				tx.Rollback()
				return len(processedSessions), err
			}
			pageSessions[row.session] = struct{}{}
			if row.pinned {
				continue
			}
			candidates = append(candidates, row)
			if _, cached := cutoffs[row.session]; cached {
				continue
			}
			var cutoff stmRetentionCutoff
			err := tx.QueryRowContext(ctx, `SELECT id,CAST(timestamp AS TEXT) FROM messages WHERE session_id=? ORDER BY timestamp DESC,id DESC LIMIT 1 OFFSET ?`, row.session, keepN-1).Scan(&cutoff.id, &cutoff.timestamp)
			if err == nil {
				cutoff.found = true
			} else if err != sql.ErrNoRows {
				tx.Rollback()
				return len(processedSessions), fmt.Errorf("find STM retention cutoff: %w", err)
			}
			cutoffs[row.session] = cutoff
		}
		pruneIDs, err = selectOlderSTMRetentionIDs(ctx, tx, candidates, cutoffs)
		if err != nil {
			tx.Rollback()
			return len(processedSessions), err
		}
		if err := archiveAndDeleteMessageIDs(ctx, tx, pruneIDs); err != nil {
			tx.Rollback()
			return len(processedSessions), err
		}
		if len(page) < pageSize {
			if _, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_meta WHERE key=?`, stmRetentionCursorKey); err != nil {
				tx.Rollback()
				return len(processedSessions), fmt.Errorf("reset completed STM retention cursor: %w", err)
			}
		} else if _, err := tx.ExecContext(ctx, `
			INSERT INTO memory_maintenance_meta(key,value) VALUES(?,?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, stmRetentionCursorKey, fmt.Sprint(lastID)); err != nil {
			tx.Rollback()
			return len(processedSessions), fmt.Errorf("persist STM retention cursor: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return len(processedSessions), fmt.Errorf("commit STM retention page: %w", err)
		}
		for session := range pageSessions {
			processedSessions[session] = struct{}{}
		}
		scanned += len(page)
		if len(page) < pageSize {
			return len(processedSessions), nil
		}
	}
	return len(processedSessions), nil
}

func selectOlderSTMRetentionIDs(ctx context.Context, tx *sql.Tx, candidates []stmRetentionRow, cutoffs map[string]stmRetentionCutoff) ([]int64, error) {
	eligible := 0
	for _, cutoff := range cutoffs {
		if cutoff.found {
			eligible++
		}
	}
	if len(candidates) == 0 || eligible == 0 {
		return nil, nil
	}
	var candidateValues, cutoffValues []string
	args := make([]any, 0, len(candidates)*2+eligible*3)
	for _, candidate := range candidates {
		candidateValues = append(candidateValues, "(?,?)")
		args = append(args, candidate.id, candidate.session)
	}
	for session, cutoff := range cutoffs {
		if !cutoff.found {
			continue
		}
		cutoffValues = append(cutoffValues, "(?,?,?)")
		var timestamp any
		if cutoff.timestamp.Valid {
			timestamp = cutoff.timestamp.String
		}
		args = append(args, session, cutoff.id, timestamp)
	}
	query := `WITH candidates(id,session_id) AS (VALUES ` + strings.Join(candidateValues, ",") + `),
		cutoffs(session_id,cutoff_id,cutoff_timestamp) AS (VALUES ` + strings.Join(cutoffValues, ",") + `)
		SELECT candidate.id
		FROM candidates c
		JOIN cutoffs k ON k.session_id=c.session_id
		JOIN messages candidate ON candidate.id=c.id
		WHERE candidate.timestamp < k.cutoff_timestamp
		   OR (candidate.timestamp IS k.cutoff_timestamp AND candidate.id < k.cutoff_id)
		   OR (candidate.timestamp IS NULL AND k.cutoff_timestamp IS NOT NULL)`
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("compare STM retention page cutoffs: %w", err)
	}
	defer rows.Close()
	ids := make([]int64, 0, len(candidates))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan STM retention page cutoff: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate STM retention page cutoffs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close STM retention page cutoffs: %w", err)
	}
	return ids, nil
}

// EnforceSTMPRetentionForSessionContext trims one session in a bounded page.
// Repeated calls naturally continue because successfully deleted IDs disappear.
func (s *SQLiteMemory) EnforceSTMPRetentionForSessionContext(ctx context.Context, sessionID string, keepN, limit int) error {
	if s == nil || s.db == nil || strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if keepN <= 0 {
		return fmt.Errorf("keepN must be positive, got %d", keepN)
	}
	ctx = maintenanceContext(ctx)
	if limit <= 0 || limit > 1000 {
		limit = stmRetentionPageSize
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin session STM retention: %w", err)
	}
	defer tx.Rollback()
	var cutoffID int64
	var cutoffTimestamp sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT id,CAST(timestamp AS TEXT) FROM messages WHERE session_id=? ORDER BY timestamp DESC,id DESC LIMIT 1 OFFSET ?`, sessionID, keepN-1).Scan(&cutoffID, &cutoffTimestamp)
	if err == sql.ErrNoRows {
		return tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("find session STM retention cutoff: %w", err)
	}
	var cutoffValue any
	if cutoffTimestamp.Valid {
		cutoffValue = cutoffTimestamp.String
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM messages WHERE session_id=? AND is_pinned=0 AND (
		timestamp < ? OR (timestamp IS ? AND id < ?) OR (timestamp IS NULL AND ? IS NOT NULL)
	) ORDER BY id LIMIT ?`, sessionID, cutoffValue, cutoffValue, cutoffID, cutoffValue, limit)
	if err != nil {
		return fmt.Errorf("select session STM retention candidates: %w", err)
	}
	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan session STM retention candidate: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate session STM retention candidates: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close session STM retention candidates: %w", err)
	}
	if err := archiveAndDeleteMessageIDs(ctx, tx, ids); err != nil {
		return err
	}
	return tx.Commit()
}

func archiveAndDeleteMessageIDs(ctx context.Context, tx *sql.Tx, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := strings.Join(placeholders, ",")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO archived_messages(session_id,role,content,is_internal,original_timestamp)
		SELECT session_id,role,content,is_internal,timestamp FROM messages
		WHERE id IN (`+query+`) AND `+archiveableRolesSQL, args...); err != nil {
		return fmt.Errorf("archive STM retention page: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE id IN (`+query+`) AND is_pinned=0`, args...); err != nil {
		return fmt.Errorf("delete STM retention page: %w", err)
	}
	return nil
}
