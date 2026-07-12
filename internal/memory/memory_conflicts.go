package memory

import (
	"database/sql"
	"fmt"
	"strings"
)

type MemoryConflict struct {
	ID              int64  `json:"id"`
	DocIDLeft       string `json:"doc_id_left"`
	DocIDRight      string `json:"doc_id_right"`
	ConflictKey     string `json:"conflict_key"`
	LeftValue       string `json:"left_value"`
	RightValue      string `json:"right_value"`
	Reason          string `json:"reason"`
	Status          string `json:"status"`
	WinningDocID    string `json:"winning_doc_id,omitempty"`
	SupersededDocID string `json:"superseded_doc_id,omitempty"`
	DetectedAt      string `json:"detected_at"`
	ResolvedAt      string `json:"resolved_at"`
}

func canonicalConflictPair(leftDocID, rightDocID, leftValue, rightValue string) (string, string, string, string) {
	if strings.TrimSpace(leftDocID) <= strings.TrimSpace(rightDocID) {
		return leftDocID, rightDocID, leftValue, rightValue
	}
	return rightDocID, leftDocID, rightValue, leftValue
}

func (s *SQLiteMemory) RegisterMemoryConflict(leftDocID, rightDocID, conflictKey, leftValue, rightValue, reason string) error {
	if s == nil || strings.TrimSpace(leftDocID) == "" || strings.TrimSpace(rightDocID) == "" || strings.TrimSpace(conflictKey) == "" {
		return nil
	}
	leftDocID, rightDocID, leftValue, rightValue = canonicalConflictPair(leftDocID, rightDocID, leftValue, rightValue)
	if leftDocID == rightDocID {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("register memory conflict begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(`
		INSERT INTO memory_conflicts (doc_id_left, doc_id_right, conflict_key, left_value, right_value, reason, status, detected_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, 'open', CURRENT_TIMESTAMP, '')
		ON CONFLICT(doc_id_left, doc_id_right, conflict_key) DO UPDATE SET
			left_value = excluded.left_value,
			right_value = excluded.right_value,
			reason = excluded.reason,
			status = 'open',
			winning_doc_id = '',
			superseded_doc_id = '',
			detected_at = CURRENT_TIMESTAMP,
			resolved_at = ''
	`, leftDocID, rightDocID, conflictKey, leftValue, rightValue, reason)
	if err != nil {
		return fmt.Errorf("register memory conflict: %w", err)
	}
	if _, err = tx.Exec(`UPDATE memory_meta SET verification_status = 'contradicted', last_event_at = CURRENT_TIMESTAMP WHERE doc_id IN (?, ?)`, leftDocID, rightDocID); err != nil {
		return fmt.Errorf("mark contradicted memory meta: %w", err)
	}
	return tx.Commit()
}

func (s *SQLiteMemory) GetOpenMemoryConflicts(limit int) ([]MemoryConflict, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, doc_id_left, doc_id_right, conflict_key, left_value, right_value, reason, status,
		       COALESCE(winning_doc_id, ''), COALESCE(superseded_doc_id, ''),
		       detected_at, COALESCE(resolved_at, '')
		FROM memory_conflicts
		WHERE status = 'open'
		ORDER BY detected_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query open memory conflicts: %w", err)
	}
	defer rows.Close()

	conflicts := make([]MemoryConflict, 0, limit)
	for rows.Next() {
		var item MemoryConflict
		if err := rows.Scan(
			&item.ID, &item.DocIDLeft, &item.DocIDRight, &item.ConflictKey,
			&item.LeftValue, &item.RightValue, &item.Reason, &item.Status,
			&item.WinningDocID, &item.SupersededDocID, &item.DetectedAt, &item.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memory conflict: %w", err)
		}
		conflicts = append(conflicts, item)
	}
	return conflicts, rows.Err()
}

func (s *SQLiteMemory) GetMemoryConflictByID(id int64) (MemoryConflict, error) {
	if s == nil || id <= 0 {
		return MemoryConflict{}, fmt.Errorf("memory conflict id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return MemoryConflict{}, fmt.Errorf("begin memory conflict lookup: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	conflict, err := getMemoryConflictByIDTx(tx, id)
	if err != nil {
		return MemoryConflict{}, err
	}
	return conflict, tx.Commit()
}

func (s *SQLiteMemory) ResolveMemoryConflict(conflictID int64, winningDocID, reason string) error {
	if s == nil {
		return fmt.Errorf("memory store is nil")
	}
	winningDocID = strings.TrimSpace(winningDocID)
	reason = strings.TrimSpace(reason)
	if conflictID <= 0 || winningDocID == "" {
		return fmt.Errorf("conflict id and winning doc id are required")
	}
	if reason == "" {
		reason = "memory conflict resolved"
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("resolve memory conflict begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	conflict, err := getMemoryConflictByIDTx(tx, conflictID)
	if err != nil {
		return fmt.Errorf("load memory conflict: %w", err)
	}
	if conflict.Status != "open" {
		return fmt.Errorf("memory conflict %d is not open", conflictID)
	}
	var losingDocID string
	switch winningDocID {
	case conflict.DocIDLeft:
		losingDocID = conflict.DocIDRight
	case conflict.DocIDRight:
		losingDocID = conflict.DocIDLeft
	default:
		return fmt.Errorf("winning doc %s does not belong to conflict %d", winningDocID, conflictID)
	}
	survivorDocIDs, err := memoryConflictSurvivorDocIDsForLoserTx(tx, losingDocID, conflictID)
	if err != nil {
		return err
	}
	survivorDocIDs = append(survivorDocIDs, winningDocID)

	if _, err := tx.Exec(`
		UPDATE memory_conflicts
		SET status = 'resolved',
		    winning_doc_id = ?,
		    superseded_doc_id = ?,
		    reason = ?,
		    resolved_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, winningDocID, losingDocID, reason, conflictID); err != nil {
		return fmt.Errorf("mark memory conflict resolved: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE memory_conflicts
		SET status = 'resolved',
		    winning_doc_id = CASE WHEN doc_id_left = ? THEN doc_id_right ELSE doc_id_left END,
		    superseded_doc_id = ?,
		    reason = ?,
		    resolved_at = CURRENT_TIMESTAMP
		WHERE status = 'open'
		  AND id != ?
		  AND (doc_id_left = ? OR doc_id_right = ?)
	`, losingDocID, losingDocID, "closed because superseded memory was archived: "+reason, conflictID, losingDocID, losingDocID); err != nil {
		return fmt.Errorf("close loser memory conflicts: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE memory_meta
		SET verification_status = ?,
		    archived_at = CURRENT_TIMESTAMP,
		    archived_reason = ?,
		    last_reviewed_at = CURRENT_TIMESTAMP,
		    review_note = ?,
		    last_event_at = CURRENT_TIMESTAMP
		WHERE doc_id = ?
	`, MemoryVerificationArchived, reason, reason, losingDocID); err != nil {
		return fmt.Errorf("archive losing memory meta: %w", err)
	}

	for _, survivorDocID := range uniqueNonEmptyStrings(survivorDocIDs) {
		if err := confirmMemoryDocIfNoOpenConflictsTx(tx, survivorDocID, reason); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func memoryConflictSurvivorDocIDsForLoserTx(tx *sql.Tx, losingDocID string, excludeConflictID int64) ([]string, error) {
	rows, err := tx.Query(`
		SELECT CASE WHEN doc_id_left = ? THEN doc_id_right ELSE doc_id_left END
		FROM memory_conflicts
		WHERE status = 'open'
		  AND id != ?
		  AND (doc_id_left = ? OR doc_id_right = ?)
	`, losingDocID, excludeConflictID, losingDocID, losingDocID)
	if err != nil {
		return nil, fmt.Errorf("load loser conflict survivor docs: %w", err)
	}
	defer rows.Close()

	var docIDs []string
	for rows.Next() {
		var docID string
		if err := rows.Scan(&docID); err != nil {
			return nil, fmt.Errorf("scan loser conflict survivor doc: %w", err)
		}
		docIDs = append(docIDs, docID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate loser conflict survivor docs: %w", err)
	}
	return docIDs, nil
}

func confirmMemoryDocIfNoOpenConflictsTx(tx *sql.Tx, docID, reason string) error {
	var remainingOpen int
	if err := tx.QueryRow(`
		SELECT COUNT(*)
		FROM memory_conflicts
		WHERE status = 'open' AND (doc_id_left = ? OR doc_id_right = ?)
	`, docID, docID).Scan(&remainingOpen); err != nil {
		return fmt.Errorf("count remaining memory conflicts for %s: %w", docID, err)
	}
	if remainingOpen != 0 {
		return nil
	}
	if _, err := tx.Exec(`
		UPDATE memory_meta
		SET verification_status = ?,
		    archived_at = NULL,
		    archived_reason = '',
		    last_reviewed_at = CURRENT_TIMESTAMP,
		    review_note = ?,
		    last_event_at = CURRENT_TIMESTAMP
		WHERE doc_id = ?
	`, MemoryVerificationConfirmed, reason, docID); err != nil {
		return fmt.Errorf("confirm memory meta %s: %w", docID, err)
	}
	return nil
}

func getMemoryConflictByIDTx(tx *sql.Tx, id int64) (MemoryConflict, error) {
	var item MemoryConflict
	err := tx.QueryRow(`
		SELECT id, doc_id_left, doc_id_right, conflict_key, left_value, right_value, reason, status,
		       COALESCE(winning_doc_id, ''), COALESCE(superseded_doc_id, ''),
		       detected_at, COALESCE(resolved_at, '')
		FROM memory_conflicts
		WHERE id = ?
	`, id).Scan(
		&item.ID, &item.DocIDLeft, &item.DocIDRight, &item.ConflictKey,
		&item.LeftValue, &item.RightValue, &item.Reason, &item.Status,
		&item.WinningDocID, &item.SupersededDocID, &item.DetectedAt, &item.ResolvedAt,
	)
	return item, err
}

func (s *SQLiteMemory) SetMemoryMetaProtection(docID string, protected bool, keepForever bool) error {
	if strings.TrimSpace(docID) == "" {
		return nil
	}
	_, err := s.db.Exec(`
		UPDATE memory_meta
		SET protected = ?, keep_forever = ?, last_event_at = CURRENT_TIMESTAMP
		WHERE doc_id = ?
	`, protected, keepForever, docID)
	if err != nil {
		return fmt.Errorf("set memory meta protection: %w", err)
	}
	return nil
}
