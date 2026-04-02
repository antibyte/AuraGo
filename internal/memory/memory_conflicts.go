package memory

import (
	"fmt"
	"strings"
)

type MemoryConflict struct {
	ID          int64  `json:"id"`
	DocIDLeft   string `json:"doc_id_left"`
	DocIDRight  string `json:"doc_id_right"`
	ConflictKey string `json:"conflict_key"`
	LeftValue   string `json:"left_value"`
	RightValue  string `json:"right_value"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
	DetectedAt  string `json:"detected_at"`
	ResolvedAt  string `json:"resolved_at"`
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
		SELECT id, doc_id_left, doc_id_right, conflict_key, left_value, right_value, reason, status, detected_at, COALESCE(resolved_at, '')
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
		if err := rows.Scan(&item.ID, &item.DocIDLeft, &item.DocIDRight, &item.ConflictKey, &item.LeftValue, &item.RightValue, &item.Reason, &item.Status, &item.DetectedAt, &item.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan memory conflict: %w", err)
		}
		conflicts = append(conflicts, item)
	}
	return conflicts, rows.Err()
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
