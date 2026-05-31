package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

const (
	AuditSourceAgentTool = "agent_tool"
	AuditSourceMission   = "mission"
	AuditSourceHeartbeat = "heartbeat"
	AuditSourceRemote    = "remote"
	AuditSourceSystem    = "system"

	AuditStatusRunning   = "running"
	AuditStatusSuccess   = "success"
	AuditStatusError     = "error"
	AuditStatusWarning   = "warning"
	AuditStatusBlocked   = "blocked"
	AuditStatusSanitized = "sanitized"

	AuditBulkDeleteConfirm = "DELETE_AUDIT_EVENTS"

	auditSummaryLimit  = 300
	auditDetailLimit   = 2000
	auditMetadataLimit = 2000
)

// AuditEvent records one agent/system action for the dashboard audit timeline.
type AuditEvent struct {
	ID            int64  `json:"id"`
	Timestamp     string `json:"timestamp"`
	Source        string `json:"source"`
	EventType     string `json:"event_type"`
	Actor         string `json:"actor"`
	SessionID     string `json:"session_id"`
	TargetID      string `json:"target_id"`
	TargetName    string `json:"target_name"`
	Status        string `json:"status"`
	Summary       string `json:"summary"`
	Detail        string `json:"detail"`
	DurationMS    int64  `json:"duration_ms"`
	CorrelationID string `json:"correlation_id"`
	MetadataJSON  string `json:"metadata_json"`
}

// AuditFilter holds search/filter parameters for audit queries and deletes.
type AuditFilter struct {
	Q        string `json:"q,omitempty"`
	Source   string `json:"source,omitempty"`
	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// AuditPage is a paginated audit query response.
type AuditPage struct {
	Entries []AuditEvent `json:"entries"`
	Total   int          `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
}

// AuditUpdate is emitted to optional subscribers after audit mutations.
type AuditUpdate struct {
	Action        string      `json:"action"`
	ID            int64       `json:"id,omitempty"`
	Deleted       int64       `json:"deleted,omitempty"`
	Source        string      `json:"source,omitempty"`
	EventType     string      `json:"event_type,omitempty"`
	Status        string      `json:"status,omitempty"`
	CorrelationID string      `json:"correlation_id,omitempty"`
	Event         *AuditEvent `json:"event,omitempty"`
}

// SetAuditNotifier registers an optional callback for audit mutations.
func (s *SQLiteMemory) SetAuditNotifier(notifier func(AuditUpdate)) {
	if s == nil {
		return
	}
	s.auditNotifierMu.Lock()
	defer s.auditNotifierMu.Unlock()
	s.auditNotifier = notifier
}

func (s *SQLiteMemory) notifyAudit(update AuditUpdate) {
	if s == nil {
		return
	}
	s.auditNotifierMu.RLock()
	notifier := s.auditNotifier
	s.auditNotifierMu.RUnlock()
	if notifier != nil {
		notifier(update)
	}
}

// InitAuditTables creates persistent audit timeline tables.
func (s *SQLiteMemory) InitAuditTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		source TEXT NOT NULL DEFAULT 'system',
		event_type TEXT NOT NULL DEFAULT '',
		actor TEXT NOT NULL DEFAULT 'agent',
		session_id TEXT NOT NULL DEFAULT '',
		target_id TEXT NOT NULL DEFAULT '',
		target_name TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'success',
		summary TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		correlation_id TEXT NOT NULL DEFAULT '',
		metadata_json TEXT NOT NULL DEFAULT '{}'
	);
	CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_events_source_status_time ON audit_events(source, status, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_events_type_time ON audit_events(event_type, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_events_target ON audit_events(target_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_audit_events_correlation ON audit_events(correlation_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("audit schema: %w", err)
	}
	return nil
}

// RecordAuditEvent inserts a new audit event.
func (s *SQLiteMemory) RecordAuditEvent(event AuditEvent) (int64, error) {
	event = normalizeAuditEvent(event)
	res, err := s.db.Exec(`
		INSERT INTO audit_events (
			timestamp, source, event_type, actor, session_id, target_id, target_name,
			status, summary, detail, duration_ms, correlation_id, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Timestamp, event.Source, event.EventType, event.Actor, event.SessionID, event.TargetID, event.TargetName,
		event.Status, event.Summary, event.Detail, event.DurationMS, event.CorrelationID, event.MetadataJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("record audit event: %w", err)
	}
	id, _ := res.LastInsertId()
	event.ID = id
	s.notifyAudit(auditUpdateFromEvent("recorded", event))
	return id, nil
}

// UpsertAuditEventByCorrelation updates an existing correlated event or inserts it.
func (s *SQLiteMemory) UpsertAuditEventByCorrelation(event AuditEvent) error {
	event = normalizeAuditEvent(event)
	if event.CorrelationID == "" {
		_, err := s.RecordAuditEvent(event)
		return err
	}

	res, err := s.db.Exec(`
		UPDATE audit_events SET
			timestamp = ?,
			source = ?,
			event_type = ?,
			actor = ?,
			session_id = ?,
			target_id = ?,
			target_name = ?,
			status = ?,
			summary = ?,
			detail = ?,
			duration_ms = ?,
			metadata_json = ?
		WHERE correlation_id = ?`,
		event.Timestamp, event.Source, event.EventType, event.Actor, event.SessionID, event.TargetID, event.TargetName,
		event.Status, event.Summary, event.Detail, event.DurationMS, event.MetadataJSON, event.CorrelationID,
	)
	if err != nil {
		return fmt.Errorf("upsert audit event: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows > 0 {
		if updated, ok := s.auditEventByCorrelation(event.CorrelationID); ok {
			s.notifyAudit(auditUpdateFromEvent("updated", updated))
		} else {
			s.notifyAudit(auditUpdateFromEvent("updated", event))
		}
		return nil
	}
	_, err = s.RecordAuditEvent(event)
	return err
}

// SearchAuditEvents returns a filtered page of audit events, newest first.
func (s *SQLiteMemory) SearchAuditEvents(filter AuditFilter) (*AuditPage, error) {
	filter = normalizeAuditFilter(filter)
	whereClause, args := auditWhereClause(filter)

	countQuery := "SELECT COUNT(*) FROM audit_events " + whereClause
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count audit events: %w", err)
	}

	queryArgs := append(append([]any{}, args...), filter.Limit, filter.Offset)
	rows, err := s.db.Query(`
		SELECT id, timestamp, source, event_type, actor, session_id, target_id, target_name,
			status, summary, detail, duration_ms, correlation_id, metadata_json
		FROM audit_events `+whereClause+`
		ORDER BY timestamp DESC, id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query audit events: %w", err)
	}
	defer rows.Close()

	entries, err := scanAuditEvents(rows)
	if err != nil {
		return nil, err
	}
	return &AuditPage{
		Entries: entries,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

// DeleteAuditEvent removes one event by ID.
func (s *SQLiteMemory) DeleteAuditEvent(id int64) error {
	if id <= 0 {
		return fmt.Errorf("audit event id is required")
	}
	event, hasEvent := s.auditEventByID(id)
	res, err := s.db.Exec(`DELETE FROM audit_events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete audit event: %w", err)
	}
	deleted, _ := res.RowsAffected()
	if deleted > 0 {
		update := AuditUpdate{Action: "deleted", ID: id, Deleted: deleted}
		if hasEvent {
			update = auditUpdateFromEvent("deleted", event)
			update.Deleted = deleted
		}
		s.notifyAudit(update)
	}
	return nil
}

// DeleteAuditEvents removes all events matching filter when the confirmation string matches.
func (s *SQLiteMemory) DeleteAuditEvents(filter AuditFilter, confirm string) (int64, error) {
	if confirm != AuditBulkDeleteConfirm {
		return 0, fmt.Errorf("confirmation %q is required", AuditBulkDeleteConfirm)
	}
	filter = normalizeAuditFilter(filter)
	whereClause, args := auditWhereClause(filter)
	res, err := s.db.Exec(`DELETE FROM audit_events `+whereClause, args...)
	if err != nil {
		return 0, fmt.Errorf("delete audit events: %w", err)
	}
	deleted, _ := res.RowsAffected()
	if deleted > 0 {
		s.notifyAudit(AuditUpdate{
			Action:    "bulk_deleted",
			Deleted:   deleted,
			Source:    filter.Source,
			EventType: filter.Type,
			Status:    filter.Status,
		})
	}
	return deleted, nil
}

func auditUpdateFromEvent(action string, event AuditEvent) AuditUpdate {
	return AuditUpdate{
		Action:        action,
		ID:            event.ID,
		Source:        event.Source,
		EventType:     event.EventType,
		Status:        event.Status,
		CorrelationID: event.CorrelationID,
		Event:         &event,
	}
}

func (s *SQLiteMemory) auditEventByID(id int64) (AuditEvent, bool) {
	if s == nil || id <= 0 {
		return AuditEvent{}, false
	}
	row := s.db.QueryRow(`
		SELECT id, timestamp, source, event_type, actor, session_id, target_id, target_name,
			status, summary, detail, duration_ms, correlation_id, metadata_json
		FROM audit_events
		WHERE id = ?
		LIMIT 1`, id)
	event, err := scanAuditEventRow(row)
	return event, err == nil
}

func (s *SQLiteMemory) auditEventByCorrelation(correlationID string) (AuditEvent, bool) {
	if s == nil || strings.TrimSpace(correlationID) == "" {
		return AuditEvent{}, false
	}
	row := s.db.QueryRow(`
		SELECT id, timestamp, source, event_type, actor, session_id, target_id, target_name,
			status, summary, detail, duration_ms, correlation_id, metadata_json
		FROM audit_events
		WHERE correlation_id = ?
		ORDER BY id DESC
		LIMIT 1`, correlationID)
	event, err := scanAuditEventRow(row)
	return event, err == nil
}

func normalizeAuditEvent(event AuditEvent) AuditEvent {
	now := time.Now().UTC()
	if strings.TrimSpace(event.Timestamp) == "" {
		event.Timestamp = now.Format(time.RFC3339)
	}
	if strings.TrimSpace(event.Source) == "" {
		event.Source = AuditSourceSystem
	}
	if strings.TrimSpace(event.Status) == "" {
		event.Status = AuditStatusSuccess
	}
	if strings.TrimSpace(event.Actor) == "" {
		event.Actor = "agent"
	}
	if strings.TrimSpace(event.MetadataJSON) == "" {
		event.MetadataJSON = "{}"
	}
	event.Source = strings.TrimSpace(event.Source)
	event.EventType = truncateAuditText(security.Scrub(event.EventType), 80)
	event.Actor = truncateAuditText(security.Scrub(event.Actor), 80)
	event.SessionID = truncateAuditText(security.Scrub(event.SessionID), 120)
	event.TargetID = truncateAuditText(security.Scrub(event.TargetID), 160)
	event.TargetName = truncateAuditText(security.Scrub(event.TargetName), 180)
	event.Status = strings.TrimSpace(event.Status)
	event.Summary = truncateAuditText(security.Scrub(event.Summary), auditSummaryLimit)
	event.Detail = truncateAuditText(security.Scrub(event.Detail), auditDetailLimit)
	event.CorrelationID = truncateAuditText(security.Scrub(event.CorrelationID), 180)
	event.MetadataJSON = normalizeAuditMetadata(event.MetadataJSON)
	return event
}

func normalizeAuditFilter(filter AuditFilter) AuditFilter {
	filter.Q = strings.TrimSpace(filter.Q)
	filter.Source = strings.TrimSpace(filter.Source)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	filter.TargetID = strings.TrimSpace(filter.TargetID)
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return filter
}

func auditWhereClause(filter AuditFilter) (string, []any) {
	conditions := make([]string, 0, 8)
	args := make([]any, 0, 8)
	if filter.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.Type != "" {
		conditions = append(conditions, "event_type = ?")
		args = append(args, filter.Type)
	}
	if filter.TargetID != "" {
		conditions = append(conditions, "target_id = ?")
		args = append(args, filter.TargetID)
	}
	if filter.From != "" {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.To)
	}
	if filter.Q != "" {
		pattern := "%" + escapeLike(filter.Q) + "%"
		conditions = append(conditions, `(summary LIKE ? ESCAPE '\' OR detail LIKE ? ESCAPE '\' OR target_name LIKE ? ESCAPE '\' OR target_id LIKE ? ESCAPE '\' OR event_type LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

func scanAuditEvents(rows *sql.Rows) ([]AuditEvent, error) {
	entries := make([]AuditEvent, 0, 16)
	for rows.Next() {
		var event AuditEvent
		if err := rows.Scan(
			&event.ID, &event.Timestamp, &event.Source, &event.EventType, &event.Actor, &event.SessionID,
			&event.TargetID, &event.TargetName, &event.Status, &event.Summary, &event.Detail,
			&event.DurationMS, &event.CorrelationID, &event.MetadataJSON,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		entries = append(entries, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return entries, nil
}

func scanAuditEventRow(row *sql.Row) (AuditEvent, error) {
	var event AuditEvent
	if err := row.Scan(
		&event.ID, &event.Timestamp, &event.Source, &event.EventType, &event.Actor, &event.SessionID,
		&event.TargetID, &event.TargetName, &event.Status, &event.Summary, &event.Detail,
		&event.DurationMS, &event.CorrelationID, &event.MetadataJSON,
	); err != nil {
		return AuditEvent{}, err
	}
	return event, nil
}

func normalizeAuditMetadata(metadata string) string {
	metadata = truncateAuditText(security.Scrub(metadata), auditMetadataLimit)
	if strings.TrimSpace(metadata) == "" {
		return "{}"
	}
	var obj any
	if err := json.Unmarshal([]byte(metadata), &obj); err != nil {
		wrapped, _ := json.Marshal(map[string]string{"text": metadata})
		return string(wrapped)
	}
	return metadata
}

func truncateAuditText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	if maxLen <= 1 {
		return string([]rune(text)[:maxLen])
	}
	return strings.TrimSpace(string([]rune(text)[:maxLen-3])) + "..."
}
