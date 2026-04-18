package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

func (s *SQLiteMemory) Close() error {
	return s.db.Close()
}

// escapeLike escapes SQLite LIKE pattern metacharacters in user-supplied input.
// The caller must include ESCAPE '\' in the LIKE clause of the SQL query.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *SQLiteMemory) InsertMessage(sessionID string, role string, content string, pinned bool, isInternal bool) (int64, error) {
	stmt := `INSERT INTO messages(session_id, role, content, is_pinned, is_internal) VALUES(?, ?, ?, ?, ?)`
	res, err := s.db.Exec(stmt, sessionID, role, content, pinned, isInternal)
	if err != nil {
		s.logger.Error("Failed to insert message into memory", "error", err)
		return -1, err // -1 indicates failure (not a valid SQLite rowid)
	}
	id, err := res.LastInsertId()
	if err != nil {
		s.logger.Warn("Failed to get last insert ID", "error", err)
		return 0, err
	}
	s.logger.Debug("Inserted message into memory", "session_id", sessionID, "role", role, "content_len", len(content), "id", id, "pinned", pinned, "internal", isInternal)
	return id, nil
}

func (s *SQLiteMemory) SetMessagePinned(id int64, pinned bool) error {
	stmt := `UPDATE messages SET is_pinned = ? WHERE id = ?`
	_, err := s.db.Exec(stmt, pinned, id)
	if err != nil {
		s.logger.Error("Failed to update message pinning", "id", id, "pinned", pinned, "error", err)
		return err
	}
	return nil
}

func (s *SQLiteMemory) GetRecentMessages(sessionID string, limit int) ([]openai.ChatCompletionMessage, error) {
	query := `
	SELECT role, content FROM (
		SELECT role, content, timestamp, id
		FROM messages
		WHERE session_id = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
	) ORDER BY timestamp ASC, id ASC;`

	return s.queryRecentMessages(query, sessionID, limit)
}

// CountInternalToolResultMessages returns how many internal tool-result messages
// were stored for the given session. Native tool calls persist results as
// role=tool, while non-native fallback calls persist them as internal role=user
// messages prefixed with Tool Output.
func (s *SQLiteMemory) CountInternalToolResultMessages(sessionID string) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE session_id = ?
		  AND is_internal = 1
		  AND (
			role = 'tool'
			OR (
				role = 'user'
				AND (
					content LIKE 'Tool Output:%'
					OR content LIKE '[Tool Output]%'
				)
			)
		  )`,
		sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count internal tool result messages: %w", err)
	}
	return count, nil
}

// GetRecentMessagesAcrossSessions returns the most recent messages across all sessions
// in chronological order (newest sessions first, then chronological within).
func (s *SQLiteMemory) GetRecentMessagesAcrossSessions(limit int) ([]openai.ChatCompletionMessage, error) {
	query := `
	SELECT role, content FROM (
		SELECT role, content, timestamp, id
		FROM messages
		ORDER BY timestamp DESC, id DESC
		LIMIT ?
	) ORDER BY timestamp ASC, id ASC;`

	return s.queryRecentMessages(query, limit)
}

// GetRecentMessagesGroupedBySession returns recent messages grouped by session,
// with the newest sessions appearing first. Within each session, messages are
// in chronological order. This provides a more intuitive cross-session recall
// compared to GetRecentMessagesAcrossSessions which uses pure chronological order.
func (s *SQLiteMemory) GetRecentMessagesGroupedBySession(limit int) ([]openai.ChatCompletionMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `
	WITH ranked_sessions AS (
		SELECT session_id, MAX(timestamp) AS latest_ts
		FROM messages
		GROUP BY session_id
		ORDER BY latest_ts DESC
		LIMIT 5
	)
	SELECT role, content FROM (
		SELECT m.role, m.content, m.session_id, m.timestamp, m.id
		FROM messages m
		INNER JOIN ranked_sessions rs ON m.session_id = rs.session_id
		ORDER BY rs.latest_ts DESC, m.timestamp ASC, m.id ASC
		LIMIT ?
	) ORDER BY timestamp ASC, id ASC;`

	return s.queryRecentMessages(query, limit)
}

func (s *SQLiteMemory) queryRecentMessages(query string, args ...interface{}) ([]openai.ChatCompletionMessage, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent messages: %w", err)
	}
	defer rows.Close()

	var messages []openai.ChatCompletionMessage
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return messages, nil
}

// GetHoursSinceLastUserMessage calculates how many hours have passed since the user last sent a message in this session.
// If no user message is found, returns 0.
func (s *SQLiteMemory) GetHoursSinceLastUserMessage(sessionID string) (float64, error) {
	query := `
	SELECT timestamp 
	FROM messages 
	WHERE session_id = ? AND role = 'user' AND is_internal = 0
	ORDER BY timestamp DESC 
	LIMIT 1;`

	var timestampStr string
	err := s.db.QueryRow(query, sessionID).Scan(&timestampStr)

	if err == sql.ErrNoRows {
		// New session or no user messages yet -> no loneliness
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to query last user message: %w", err)
	}

	// timestamp is stored as 'YYYY-MM-DD HH:MM:SS' by CURRENT_TIMESTAMP
	// In SQLite CURRENT_TIMESTAMP is UTC, so parse as UTC
	lastInteraction, err := time.Parse("2006-01-02 15:04:05", timestampStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse timestamp '%s': %w", timestampStr, err)
	}
	// Parse as UTC since that's how SQLite stores CURRENT_TIMESTAMP
	lastInteraction = lastInteraction.UTC()

	// time.Now().UTC() matches CURRENT_TIMESTAMP on the server
	hours := time.Now().UTC().Sub(lastInteraction).Hours()
	if hours < 0 {
		hours = 0
	}
	return hours, nil
}

// DeleteOldMessages archives messages to archived_messages before removing them.
// Keeps only the most recent `keepN` messages for a given session.
// Pinned messages (is_pinned = 1) are never archived or deleted.
func (s *SQLiteMemory) DeleteOldMessages(sessionID string, keepN int) error {
	if keepN <= 0 {
		return fmt.Errorf("keepN must be positive, got %d", keepN)
	}
	// First find the ID of the oldest message we want to KEEP
	query := `
	SELECT id FROM messages
	WHERE session_id = ?
	ORDER BY timestamp DESC, id DESC
	LIMIT 1 OFFSET ?;`

	var oldestKeepID int
	err := s.db.QueryRow(query, sessionID, keepN-1).Scan(&oldestKeepID)

	if err == sql.ErrNoRows {
		// Fewer than keepN messages exist, nothing to delete
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to find cutoff ID for deletion: %w", err)
	}

	// Use a transaction so archive + delete are atomic
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Archive before deleting (exclude pinned messages — they must never be archived)
	archiveQuery := `
	INSERT INTO archived_messages (session_id, role, content, original_timestamp)
	SELECT session_id, role, content, timestamp
	FROM messages
	WHERE session_id = ? AND id < ? AND role IN ('user', 'assistant') AND is_pinned = 0
	ORDER BY timestamp ASC, id ASC`
	archRes, err := tx.Exec(archiveQuery, sessionID, oldestKeepID)
	if err != nil {
		return fmt.Errorf("failed to archive old messages: %w", err)
	}
	archived, _ := archRes.RowsAffected()

	// Delete everything older than the cutoff, but never delete pinned messages
	delQuery := `DELETE FROM messages WHERE session_id = ? AND id < ? AND is_pinned = 0`
	res, err := tx.Exec(delQuery, sessionID, oldestKeepID)
	if err != nil {
		return fmt.Errorf("failed to delete old messages: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit cleanup transaction: %w", err)
	}

	rows, _ := res.RowsAffected()
	s.logger.Info("Cleaned up SQLite short-term memory", "session_id", sessionID, "deleted_rows", rows, "archived", archived)
	return nil
}

// GetUnconsolidatedMessages returns archived messages that are pending consolidation.
// Backward-compatible wrapper that includes retryable failed items.
func (s *SQLiteMemory) GetUnconsolidatedMessages(limit int) ([]ArchivedMessage, error) {
	return s.GetConsolidationCandidates(limit, 3)
}

// GetConsolidationCandidates returns archived messages that should be processed now.
// Includes pending messages and failed messages whose retry cooldown elapsed.
// Deprecated: Prefer ClaimConsolidationCandidates which atomically claims rows to
// prevent duplicate processing by concurrent consolidation runs.
func (s *SQLiteMemory) GetConsolidationCandidates(limit int, maxRetries int) ([]ArchivedMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, original_timestamp, consolidation_status, consolidation_retries
		FROM archived_messages
		WHERE consolidated = 0
		  AND (
		    consolidation_status = 'pending'
		    OR (
		      consolidation_status = 'failed'
		      AND consolidation_retries < ?
		      AND next_retry_at <= CURRENT_TIMESTAMP
		    )
		  )
		ORDER BY original_timestamp ASC
		LIMIT ?
	`, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("query unconsolidated: %w", err)
	}
	defer rows.Close()

	var msgs []ArchivedMessage
	for rows.Next() {
		var m ArchivedMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp, &m.ConsolidationStatus, &m.ConsolidationRetries); err != nil {
			return nil, fmt.Errorf("scan unconsolidated: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ClaimConsolidationCandidates atomically claims archived messages for consolidation.
// It transitions matching rows from 'pending'/'failed' to 'in_progress' within a single
// transaction, preventing concurrent consolidation runs from processing the same rows.
// On success the caller owns the returned messages and must eventually call
// MarkConsolidationSuccess or MarkConsolidationFailure to finalize them.
func (s *SQLiteMemory) ClaimConsolidationCandidates(limit int, maxRetries int) ([]ArchivedMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Find eligible row IDs
	rows, err := tx.Query(`
		SELECT id FROM archived_messages
		WHERE consolidated = 0
		  AND (
		    consolidation_status = 'pending'
		    OR (
		      consolidation_status = 'failed'
		      AND consolidation_retries < ?
		      AND next_retry_at <= CURRENT_TIMESTAMP
		    )
		  )
		ORDER BY original_timestamp ASC
		LIMIT ?
	`, maxRetries, limit)
	if err != nil {
		return nil, fmt.Errorf("query eligible ids: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan eligible id: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate eligible ids: %w", err)
	}

	if len(ids) == 0 {
		_ = tx.Commit()
		return nil, nil
	}

	// Step 2: Atomically claim these rows → in_progress
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	claimQuery := fmt.Sprintf(
		"UPDATE archived_messages SET consolidation_status = 'in_progress' WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)
	if _, err := tx.Exec(claimQuery, args...); err != nil {
		return nil, fmt.Errorf("claim rows: %w", err)
	}

	// Step 3: Fetch full row data for the claimed rows
	fetchQuery := fmt.Sprintf(`
		SELECT id, session_id, role, content, original_timestamp, consolidation_status, consolidation_retries
		FROM archived_messages
		WHERE id IN (%s)
		ORDER BY original_timestamp ASC
	`, strings.Join(placeholders, ","))
	dataRows, err := tx.Query(fetchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("fetch claimed rows: %w", err)
	}
	defer dataRows.Close()

	var msgs []ArchivedMessage
	for dataRows.Next() {
		var m ArchivedMessage
		if err := dataRows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp, &m.ConsolidationStatus, &m.ConsolidationRetries); err != nil {
			return nil, fmt.Errorf("scan claimed: %w", err)
		}
		msgs = append(msgs, m)
	}
	if err := dataRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit claim: %w", err)
	}
	return msgs, nil
}

// MarkConsolidated marks a batch of archived messages as consolidated.
// Deprecated: prefer MarkConsolidationSuccess which also sets consolidation_status.
func (s *SQLiteMemory) MarkConsolidated(ids []int64) error {
	return s.MarkConsolidationSuccess(ids)
}

// MarkConsolidationSuccess marks archived messages as successfully consolidated.
func (s *SQLiteMemory) MarkConsolidationSuccess(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`UPDATE archived_messages
		SET consolidated = 1,
		    consolidation_status = 'done',
		    consolidation_last_error = ''
		WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}

// MarkConsolidationFailure records a failed consolidation attempt and schedules retry.
func (s *SQLiteMemory) MarkConsolidationFailure(ids []int64, reason string) error {
	if len(ids) == 0 {
		return nil
	}
	if len(reason) > 300 {
		reason = reason[:300]
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, reason)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`UPDATE archived_messages
		SET consolidation_status = 'failed',
		    consolidation_retries = consolidation_retries + 1,
		    consolidation_last_error = ?,
		    next_retry_at = datetime('now', '+' || (CASE
		    	WHEN consolidation_retries < 1 THEN 5
		    	WHEN consolidation_retries < 2 THEN 30
		    	ELSE 120
		    END) || ' minutes')
		WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}

// CleanOldArchivedMessages removes archived messages older than the given number of days.
// Only successfully consolidated messages (consolidated = 1 AND consolidation_status = 'done')
// are removed. Pending, failed, or in-progress rows are always retained to prevent data loss.
func (s *SQLiteMemory) CleanOldArchivedMessages(days int) (int64, error) {
	res, err := s.db.Exec(
		"DELETE FROM archived_messages WHERE consolidated = 1 AND consolidation_status = 'done' AND archived_at < datetime('now', ?)",
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteMemory) DeleteMessagesByID(sessionID string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// Build parameterized placeholders: "?, ?, ?"
	placeholders := make([]string, len(ids))
	args := make([]interface{}, 0, len(ids)+1)
	args = append(args, sessionID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf("DELETE FROM messages WHERE session_id = ? AND id IN (%s)", strings.Join(placeholders, ","))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to delete messages by ID: %w", err)
	}
	rows, _ := res.RowsAffected()
	s.logger.Info("Deleted specific messages from SQLite", "session_id", sessionID, "deleted_rows", rows)
	return nil
}

// Clear removes all messages for a given session.
func (s *SQLiteMemory) Clear(sessionID string) error {
	delQuery := `DELETE FROM messages WHERE session_id = ?`
	res, err := s.db.Exec(delQuery, sessionID)
	if err != nil {
		return fmt.Errorf("failed to clear messages: %w", err)
	}

	rows, _ := res.RowsAffected()
	s.logger.Info("Cleared SQLite short-term memory", "session_id", sessionID, "deleted_rows", rows)
	return nil
}

// LogArchiveEvent records that a concept was saved to long-term memory.
func (s *SQLiteMemory) LogArchiveEvent(sessionID, concept string) error {
	stmt := `INSERT INTO archive_events(session_id, concept) VALUES(?, ?)`
	_, err := s.db.Exec(stmt, sessionID, concept)
	if err != nil {
		s.logger.Error("Failed to log archive event", "error", err)
		return err
	}
	return nil
}

// GetRecentArchiveEvents returns concepts archived within the last N hours.
func (s *SQLiteMemory) GetRecentArchiveEvents(hours int) ([]string, error) {
	query := `
	SELECT concept FROM archive_events 
	WHERE timestamp >= datetime('now', ?) 
	ORDER BY timestamp ASC;`

	timeMod := fmt.Sprintf("-%d hours", hours)
	rows, err := s.db.Query(query, timeMod)
	if err != nil {
		return nil, fmt.Errorf("failed to query archive events: %w", err)
	}
	defer rows.Close()

	var concepts []string
	for rows.Next() {
		var concept string
		if err := rows.Scan(&concept); err == nil {
			concepts = append(concepts, concept)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return concepts, nil
}

// AddNotification stores a new system notification (e.g. Morning Briefing).
func (s *SQLiteMemory) AddNotification(content string) error {
	stmt := `INSERT INTO system_notifications(content) VALUES(?)`
	_, err := s.db.Exec(stmt, content)
	if err != nil {
		s.logger.Error("Failed to store system notification", "error", err)
		return err
	}
	return nil
}

// GetUnreadNotifications returns all unread notifications.
func (s *SQLiteMemory) GetUnreadNotifications() ([]string, error) {
	query := `SELECT content FROM system_notifications WHERE is_read = 0 ORDER BY timestamp ASC;`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get unread notifications: %w", err)
	}
	defer rows.Close()

	var notes []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			notes = append(notes, content)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return notes, nil
}

// MarkNotificationsRead marks all system notifications as read.
func (s *SQLiteMemory) MarkNotificationsRead() error {
	stmt := `UPDATE system_notifications SET is_read = 1 WHERE is_read = 0`
	_, err := s.db.Exec(stmt)
	return err
}

// ArchivedMessage represents a message that was archived before deletion from STM.
type ArchivedMessage struct {
	ID                   int64
	SessionID            string
	Role                 string
	Content              string
	Timestamp            string
	ConsolidationStatus  string
	ConsolidationRetries int
}

// MemoryMeta models the tracking metadata for a VectorDB chunk.
type MemoryMeta struct {
	DocID                string
	AccessCount          int
	LastAccessed         string
	LastEventAt          string
	ExtractionConfidence float64
	VerificationStatus   string
	SourceType           string
	SourceReliability    float64
	UsefulCount          int
	UselessCount         int
	LastEffectivenessAt  string
	Protected            bool
	KeepForever          bool
}

// MemoryMetaUpdate allows callers to enrich a memory_meta row with quality and provenance signals.
type MemoryMetaUpdate struct {
	ExtractionConfidence float64
	VerificationStatus   string
	SourceType           string
	SourceReliability    float64
}

// MemoryUsageEntry represents one retrieval/injection usage event for a memory item.
type MemoryUsageEntry struct {
	ID               int64
	MemoryID         string
	MemoryType       string
	SessionID        string
	UsedAt           string
	ContextRelevance float64
	WasCited         bool
}

// UpsertMemoryMeta creates or resets a VectorDB chunk's metadata.
func (s *SQLiteMemory) UpsertMemoryMeta(docID string) error {
	if docID == "" {
		return nil
	}
	stmt := `
	INSERT INTO memory_meta (
		doc_id, access_count, last_accessed, last_event_at,
		extraction_confidence, verification_status, source_type, source_reliability
	)
	VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 0.75, 'unverified', 'system', 0.70)
	ON CONFLICT(doc_id) DO UPDATE SET last_accessed=CURRENT_TIMESTAMP;`

	_, err := s.db.Exec(stmt, docID)
	return err
}

// UpsertMemoryMetaWithDetails creates or refreshes VectorDB chunk metadata while preserving
// existing quality fields unless explicit overrides are provided.
func (s *SQLiteMemory) UpsertMemoryMetaWithDetails(docID string, details MemoryMetaUpdate) error {
	if docID == "" {
		return nil
	}

	extractionConfidence := details.ExtractionConfidence
	if extractionConfidence <= 0 {
		extractionConfidence = 0.75
	}
	if extractionConfidence > 1 {
		extractionConfidence = 1
	}

	verificationStatus := strings.TrimSpace(details.VerificationStatus)
	if verificationStatus == "" {
		verificationStatus = "unverified"
	}

	sourceType := strings.TrimSpace(details.SourceType)
	if sourceType == "" {
		sourceType = "system"
	}

	sourceReliability := details.SourceReliability
	if sourceReliability <= 0 {
		sourceReliability = 0.70
	}
	if sourceReliability > 1 {
		sourceReliability = 1
	}

	stmt := `
	INSERT INTO memory_meta (
		doc_id, access_count, last_accessed, last_event_at,
		extraction_confidence, verification_status, source_type, source_reliability
	)
	VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?, ?, ?, ?)
	ON CONFLICT(doc_id) DO UPDATE SET
		last_accessed = CURRENT_TIMESTAMP,
		extraction_confidence = COALESCE(NULLIF(excluded.extraction_confidence, 0), memory_meta.extraction_confidence),
		verification_status = CASE
			WHEN excluded.verification_status = '' THEN memory_meta.verification_status
			ELSE excluded.verification_status
		END,
		source_type = CASE
			WHEN excluded.source_type = '' THEN memory_meta.source_type
			ELSE excluded.source_type
		END,
		source_reliability = COALESCE(NULLIF(excluded.source_reliability, 0), memory_meta.source_reliability);`

	_, err := s.db.Exec(stmt, docID, extractionConfidence, verificationStatus, sourceType, sourceReliability)
	return err
}

// UpdateMemoryAccess increments the access count and touches the last_accessed timestamp.
func (s *SQLiteMemory) UpdateMemoryAccess(docID string) error {
	stmt := `
	UPDATE memory_meta 
	SET access_count = access_count + 1, last_accessed = CURRENT_TIMESTAMP 
	WHERE doc_id = ?;`

	_, err := s.db.Exec(stmt, docID)
	return err
}

// RecordMemoryUsage persists that a memory item was injected or preloaded for a session.
func (s *SQLiteMemory) RecordMemoryUsage(memoryID, memoryType, sessionID string, contextRelevance float64, wasCited bool) error {
	if memoryID == "" {
		return nil
	}
	if memoryType == "" {
		memoryType = "ltm"
	}
	if sessionID == "" {
		sessionID = "default"
	}
	if contextRelevance < 0 {
		contextRelevance = 0
	}
	if contextRelevance > 1 {
		contextRelevance = 1
	}

	stmt := `
	INSERT INTO memory_usage_log (memory_id, memory_type, session_id, context_relevance, was_cited)
	VALUES (?, ?, ?, ?, ?);`
	_, err := s.db.Exec(stmt, memoryID, memoryType, sessionID, contextRelevance, wasCited)
	if err != nil {
		return fmt.Errorf("record memory usage: %w", err)
	}
	return nil
}

// CleanOldMemoryUsageLog removes memory_usage_log entries older than the given number of days.
func (s *SQLiteMemory) CleanOldMemoryUsageLog(maxAgeDays int) error {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	_, err := s.db.Exec(`DELETE FROM memory_usage_log WHERE used_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", maxAgeDays))
	if err != nil {
		return fmt.Errorf("clean memory usage log: %w", err)
	}
	return nil
}

// RecordMemoryEffectiveness updates the aggregated usefulness score for an injected memory item.
func (s *SQLiteMemory) RecordMemoryEffectiveness(memoryID string, useful bool) error {
	if memoryID == "" {
		return nil
	}
	if err := s.UpsertMemoryMeta(memoryID); err != nil {
		return fmt.Errorf("ensure memory meta before effectiveness update: %w", err)
	}

	// Use CASE to avoid dynamic SQL with column names
	stmt := `UPDATE memory_meta SET 
		useful_count = useful_count + CASE WHEN ? = 'useful' THEN 1 ELSE 0 END,
		useless_count = useless_count + CASE WHEN ? = 'useless' THEN 1 ELSE 0 END,
		last_effectiveness_at = CURRENT_TIMESTAMP
	WHERE doc_id = ?`
	column := "useful"
	if !useful {
		column = "useless"
	}
	if _, err := s.db.Exec(stmt, column, column, memoryID); err != nil {
		return fmt.Errorf("record memory effectiveness: %w", err)
	}
	return nil
}

// GetRecentMemoryUsage returns the most recent memory usage events, optionally filtered by session.
func (s *SQLiteMemory) GetRecentMemoryUsage(sessionID string, limit int) ([]MemoryUsageEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	query := `
		SELECT id, memory_id, memory_type, session_id, used_at, context_relevance, was_cited
		FROM memory_usage_log`
	args := make([]interface{}, 0, 2)
	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY used_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query recent memory usage: %w", err)
	}
	defer rows.Close()

	entries := make([]MemoryUsageEntry, 0, limit)
	for rows.Next() {
		var entry MemoryUsageEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.MemoryID,
			&entry.MemoryType,
			&entry.SessionID,
			&entry.UsedAt,
			&entry.ContextRelevance,
			&entry.WasCited,
		); err != nil {
			return nil, fmt.Errorf("scan memory usage entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory usage rows: %w", err)
	}
	return entries, nil
}

// DeleteMemoryMeta removes tracking for a vector DB chunk.
func (s *SQLiteMemory) DeleteMemoryMeta(docID string) error {
	stmt := `DELETE FROM memory_meta WHERE doc_id = ?;`
	_, err := s.db.Exec(stmt, docID)
	return err
}

// DeleteDocumentCleanup removes all SQLite tracking data for a deleted vector DB document.
// This includes memory_meta, file_embedding_docs, and memory_conflicts references.
func (s *SQLiteMemory) DeleteDocumentCleanup(docID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM memory_meta WHERE doc_id = ?`, docID); err != nil {
		return fmt.Errorf("cleanup memory_meta: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE doc_id = ?`, docID); err != nil {
		return fmt.Errorf("cleanup file_embedding_docs: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM memory_conflicts WHERE doc_id_left = ? OR doc_id_right = ?`, docID, docID); err != nil {
		return fmt.Errorf("cleanup memory_conflicts: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM memory_usage_log WHERE memory_id = ?`, docID); err != nil {
		return fmt.Errorf("cleanup memory_usage_log: %w", err)
	}

	return tx.Commit()
}

// GetUniversallyUsefulMemories returns memory IDs that were cited across multiple sessions,
// indicating they are broadly useful regardless of context.
func (s *SQLiteMemory) GetUniversallyUsefulMemories(minSessions int, limit int) ([]string, error) {
	if minSessions <= 0 {
		minSessions = 2
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT memory_id
		FROM memory_usage_log
		WHERE was_cited = 1
		GROUP BY memory_id
		HAVING COUNT(DISTINCT session_id) >= ?
		ORDER BY COUNT(DISTINCT session_id) DESC, COUNT(*) DESC
		LIMIT ?
	`, minSessions, limit)
	if err != nil {
		return nil, fmt.Errorf("query universally useful memories: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// GetAllMemoryMeta retrieves the metadata for all chunks to calculate forgetting priorities.
// Uses pagination to avoid loading unbounded data into memory.
func (s *SQLiteMemory) GetAllMemoryMeta(limit int, offset int) ([]MemoryMeta, error) {
	if limit <= 0 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT doc_id, access_count, last_accessed, last_event_at, extraction_confidence, verification_status, source_type, source_reliability, useful_count, useless_count, COALESCE(last_effectiveness_at, ''), protected, keep_forever FROM memory_meta ORDER BY doc_id ASC LIMIT ? OFFSET ?;`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []MemoryMeta
	for rows.Next() {
		var m MemoryMeta
		if err := rows.Scan(
			&m.DocID,
			&m.AccessCount,
			&m.LastAccessed,
			&m.LastEventAt,
			&m.ExtractionConfidence,
			&m.VerificationStatus,
			&m.SourceType,
			&m.SourceReliability,
			&m.UsefulCount,
			&m.UselessCount,
			&m.LastEffectivenessAt,
			&m.Protected,
			&m.KeepForever,
		); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return metas, nil
}

// GetMemoryMetaAfter retrieves memory metadata using cursor-based (keyset) pagination.
// Pass the last doc_id from the previous page as the cursor, or empty string for the first page.
// This avoids the O(n) cost of OFFSET for large datasets.
func (s *SQLiteMemory) GetMemoryMetaAfter(cursor string, limit int) ([]MemoryMeta, error) {
	if limit <= 0 {
		limit = 500
	}

	var rows *sql.Rows
	var err error
	if cursor == "" {
		rows, err = s.db.Query(`
			SELECT doc_id, access_count, last_accessed, last_event_at, extraction_confidence,
			       verification_status, source_type, source_reliability, useful_count, useless_count,
			       COALESCE(last_effectiveness_at, ''), protected, keep_forever
			FROM memory_meta
			ORDER BY doc_id ASC
			LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT doc_id, access_count, last_accessed, last_event_at, extraction_confidence,
			       verification_status, source_type, source_reliability, useful_count, useless_count,
			       COALESCE(last_effectiveness_at, ''), protected, keep_forever
			FROM memory_meta
			WHERE doc_id > ?
			ORDER BY doc_id ASC
			LIMIT ?`, cursor, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []MemoryMeta
	for rows.Next() {
		var m MemoryMeta
		if err := rows.Scan(
			&m.DocID,
			&m.AccessCount,
			&m.LastAccessed,
			&m.LastEventAt,
			&m.ExtractionConfidence,
			&m.VerificationStatus,
			&m.SourceType,
			&m.SourceReliability,
			&m.UsefulCount,
			&m.UselessCount,
			&m.LastEffectivenessAt,
			&m.Protected,
			&m.KeepForever,
		); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return metas, nil
}

// GetAllMemoryMetaCount returns the total number of memory_meta rows.
func (s *SQLiteMemory) GetAllMemoryMetaCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM memory_meta").Scan(&count)
	return count, err
}

// RecordToolTransition increments the count for a transition from one tool to another.
func (s *SQLiteMemory) RecordToolTransition(from, to string) error {
	if from == "" || to == "" {
		return nil
	}
	stmt := `
	INSERT INTO tool_transitions (from_tool, to_tool, count, last_updated)
	VALUES (?, ?, 1, CURRENT_TIMESTAMP)
	ON CONFLICT(from_tool, to_tool) DO UPDATE SET
		count        = count + 1,
		last_updated = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(stmt, from, to)
	return err
}

// GetTopTransition finds the tool most likely to follow the given tool.
func (s *SQLiteMemory) GetTopTransition(from string) (string, error) {
	if from == "" {
		return "", nil
	}
	query := `SELECT to_tool FROM tool_transitions WHERE from_tool = ? ORDER BY count DESC LIMIT 1;`
	var to string
	err := s.db.QueryRow(query, from).Scan(&to)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return to, err
}

// GetToolUsageCount returns the total number of times a tool has been used.
// Uses tool_usage_adaptive which is incremented on every individual tool call,
// unlike tool_transitions which only records pairs and misses solo-tool sessions.
func (s *SQLiteMemory) GetToolUsageCount(toolName string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COALESCE(total_count, 0) FROM tool_usage_adaptive WHERE tool_name = ?`,
		toolName,
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		// Tool has never been used — not an error.
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("GetToolUsageCount: %w", err)
	}
	return count, nil
}

// Adaptive Tool Usage Tracking

// ToolUsageAdaptiveEntry represents a persistent tool usage record with decay support.
type ToolUsageAdaptiveEntry struct {
	ToolName     string
	TotalCount   int
	SuccessCount int
	LastUsed     time.Time
}

// UpsertToolUsage increments the usage counter for a tool and updates last_used.
// success indicates whether the tool call completed without error.
func (s *SQLiteMemory) UpsertToolUsage(toolName string, success bool) error {
	successDelta := 0
	if success {
		successDelta = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO tool_usage_adaptive (tool_name, total_count, success_count, last_used)
		VALUES (?, 1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tool_name)
		DO UPDATE SET
			total_count   = total_count   + 1,
			success_count = success_count + ?,
			last_used     = CURRENT_TIMESTAMP`,
		toolName, successDelta, successDelta)
	return err
}

// LoadToolUsageAdaptive returns all tracked tool usage entries.
func (s *SQLiteMemory) LoadToolUsageAdaptive() ([]ToolUsageAdaptiveEntry, error) {
	rows, err := s.db.Query(`SELECT tool_name, total_count, COALESCE(success_count, 0), last_used FROM tool_usage_adaptive`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ToolUsageAdaptiveEntry
	for rows.Next() {
		var e ToolUsageAdaptiveEntry
		if err := rows.Scan(&e.ToolName, &e.TotalCount, &e.SuccessCount, &e.LastUsed); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Temporal Memory (Phase A)

// RecordInteraction logs a topic at the current hour/weekday for pattern detection.
// Topics are kept short (<=120 chars) as coarse-grained keys, not full messages.
func (s *SQLiteMemory) RecordInteraction(topic string) error {
	if topic == "" {
		return nil
	}
	if r := []rune(topic); len(r) > 120 {
		topic = string(r[:120])
	}
	stmt := `
	INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
	VALUES (CAST(strftime('%H','now','localtime') AS INTEGER),
	        CAST(strftime('%w','now','localtime') AS INTEGER),
	        ?, 1, CURRENT_TIMESTAMP)
	ON CONFLICT(hour_of_day, day_of_week, topic)
	DO UPDATE SET count = count + 1, last_seen = CURRENT_TIMESTAMP;`
	_, err := s.db.Exec(stmt, topic)
	if err != nil {
		s.logger.Error("Failed to record interaction pattern", "topic", topic, "error", err)
	}
	return err
}

// GetTopPatterns returns the most common topics for the given hour and weekday,
// ordered by frequency. Useful for proactive memory pre-loading.
func (s *SQLiteMemory) GetTopPatterns(hour, weekday, limit int) ([]string, error) {
	query := `
	SELECT topic FROM interaction_patterns
	WHERE hour_of_day = ? AND day_of_week = ?
	ORDER BY count DESC
	LIMIT ?;`
	rows, err := s.db.Query(query, hour, weekday, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query interaction patterns: %w", err)
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			topics = append(topics, t)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return topics, nil
}

// CleanOldPatterns removes interaction patterns older than the given number of days.
func (s *SQLiteMemory) CleanOldPatterns(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM interaction_patterns WHERE last_seen < datetime('now', '-' || ? || ' days');`, olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CleanOldTransitions removes rarely-used tool transitions that haven't been
// updated in the given number of days and have a low count (pruning noise).
func (s *SQLiteMemory) CleanOldTransitions(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(
		`DELETE FROM tool_transitions WHERE last_updated < datetime('now', '-' || ? || ' days') AND count <= 2;`,
		olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CleanOldArchiveEvents removes archive events older than the given number of days.
func (s *SQLiteMemory) CleanOldArchiveEvents(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM archive_events WHERE timestamp < datetime('now', '-' || ? || ' days');`, olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CleanOldMoodLog removes mood_log entries older than the given number of days.
func (s *SQLiteMemory) CleanOldMoodLog(olderThanDays int) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM mood_log WHERE timestamp < datetime('now', '-' || ? || ' days');`, olderThanDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PredictNextQuery combines tool-transition history with temporal interaction patterns
// to predict what topics the user is likely interested in right now.
// Returns up to `limit` predicted topic strings.
func (s *SQLiteMemory) PredictNextQuery(lastTool string, hour, weekday, limit int) ([]string, error) {
	var predictions []string
	seen := make(map[string]bool)

	// 1. Temporal patterns: what does the user usually do at this time?
	topics, err := s.GetTopPatterns(hour, weekday, limit)
	if err == nil {
		for _, t := range topics {
			if !seen[t] {
				predictions = append(predictions, t)
				seen[t] = true
			}
		}
	}

	// 2. Tool transition: if we know which tool was used last, predict the next tool context
	if lastTool != "" {
		nextTool, err := s.GetTopTransition(lastTool)
		if err == nil && nextTool != "" && !seen[nextTool] {
			predictions = append(predictions, nextTool)
			seen[nextTool] = true
		}
	}

	if len(predictions) > limit {
		predictions = predictions[:limit]
	}
	return predictions, nil
}

// GetFileIndex returns the last modified time for a given file path within a collection.
// For backward compatibility, if no entry exists for path+collection, falls back to
// path-only lookup (handles pre-migration data with collection=”).
func (s *SQLiteMemory) GetFileIndex(path, collection string) (time.Time, error) {
	var lastMod time.Time
	// Try path+collection first (collection-aware)
	err := s.db.QueryRow("SELECT last_modified FROM file_indices WHERE file_path = ? AND collection = ?", path, collection).Scan(&lastMod)
	if err == nil {
		return lastMod, nil
	}
	if err != sql.ErrNoRows {
		return time.Time{}, err
	}
	// Fallback to path-only for backward compatibility with pre-migration data
	err = s.db.QueryRow("SELECT last_modified FROM file_indices WHERE file_path = ? AND collection = ''", path).Scan(&lastMod)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return lastMod, err
}

// UpdateFileIndex updates the last modified time for a given file path within a collection.
func (s *SQLiteMemory) UpdateFileIndex(path, collection string, modTime time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO file_indices (file_path, collection, last_modified)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path, collection) DO UPDATE SET
			last_modified = excluded.last_modified
	`, path, collection, modTime)
	return err
}

// UpdateFileIndexWithDocs updates the last modified time for a given file path within a collection
// and replaces the tracked VectorDB document IDs generated from that file.
func (s *SQLiteMemory) UpdateFileIndexWithDocs(path, collection string, modTime time.Time, docIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		INSERT INTO file_indices (file_path, collection, last_modified)
		VALUES (?, ?, ?)
		ON CONFLICT(file_path, collection) DO UPDATE SET
			last_modified = excluded.last_modified
	`, path, collection, modTime); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}

	for _, docID := range docIDs {
		docID = strings.TrimSpace(docID)
		if docID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO file_embedding_docs (file_path, collection, doc_id)
			VALUES (?, ?, ?)
		`, path, collection, docID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetFileEmbeddingDocIDs returns the tracked VectorDB document IDs for a file path within a collection.
// For backward compatibility, if no entry exists for path+collection, falls back to
// path-only lookup (handles pre-migration data with collection=”).
func (s *SQLiteMemory) GetFileEmbeddingDocIDs(path, collection string) ([]string, error) {
	// Try path+collection first (collection-aware)
	rows, err := s.db.Query(`
		SELECT doc_id
		FROM file_embedding_docs
		WHERE file_path = ? AND collection = ?
		ORDER BY doc_id
	`, path, collection)
	if err != nil {
		return nil, err
	}
	var docIDs []string
	for rows.Next() {
		var docID string
		if err := rows.Scan(&docID); err != nil {
			rows.Close()
			return nil, err
		}
		docIDs = append(docIDs, docID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// If we found docs, return them
	if len(docIDs) > 0 {
		return docIDs, nil
	}
	// Fallback to path-only for backward compatibility with pre-migration data
	rows2, err := s.db.Query(`
		SELECT doc_id
		FROM file_embedding_docs
		WHERE file_path = ? AND collection = ''
		ORDER BY doc_id
	`, path)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var docID string
		if err := rows2.Scan(&docID); err != nil {
			return nil, err
		}
		docIDs = append(docIDs, docID)
	}
	return docIDs, rows2.Err()
}

// ListIndexedFiles returns all tracked file paths for a given collection.
func (s *SQLiteMemory) ListIndexedFiles(collection string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT file_path
		FROM file_indices
		WHERE collection = ?
		ORDER BY file_path
	`, collection)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

// DeleteFileIndex removes file-index metadata and tracked VectorDB document IDs
// for a file path within a specific collection. This is used when a file is removed
// or before a full reindex.
func (s *SQLiteMemory) DeleteFileIndex(path, collection string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_indices WHERE file_path = ? AND collection = ?`, path, collection); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearFileIndices removes all persisted file-index timestamps so the indexer
// treats knowledge/doc files as new and rebuilds their embeddings. It also
// clears the tracked file-to-vector document mappings.
func (s *SQLiteMemory) ClearFileIndices() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM file_embedding_docs`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_indices`); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearMemoryMeta removes all long-term memory metadata. This is needed when
// the embedding database is rebuilt from scratch so stale chunk references do
// not remain in SQLite.
func (s *SQLiteMemory) ClearMemoryMeta() error {
	_, err := s.db.Exec(`DELETE FROM memory_meta`)
	return err
}

// Core Memory (SQLite)

// GetMessageCount returns the total number of chat messages.
