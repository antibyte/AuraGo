package memory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	AgoDeskAttachmentStatusPrepared = "prepared"
	AgoDeskAttachmentStatusUploaded = "uploaded"
	AgoDeskAttachmentStatusAccepted = "accepted"
)

type AgoDeskAttachmentRecord struct {
	AttachmentID       string
	TransportSessionID string
	ConversationID     string
	Status             string
	Filename           string
	MimeType           string
	Kind               string
	DeclaredSizeBytes  int64
	SizeBytes          int64
	ExpectedSHA256     string
	SHA256             string
	RelativePath       string
	CreatedAt          time.Time
	UploadedAt         time.Time
	ExpiresAt          time.Time
	MessageID          int64
}

func (s *SQLiteMemory) InitAgoDeskAttachmentsTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS agodesk_chat_attachments (
			attachment_id TEXT PRIMARY KEY,
			transport_session_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'prepared',
			filename TEXT NOT NULL DEFAULT '',
			mime_type TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT '',
			declared_size_bytes INTEGER DEFAULT 0,
			size_bytes INTEGER DEFAULT 0,
			expected_sha256 TEXT DEFAULT '',
			sha256 TEXT DEFAULT '',
			relative_path TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			uploaded_at DATETIME DEFAULT '',
			expires_at DATETIME DEFAULT '',
			message_id INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_agodesk_chat_attachments_conversation ON agodesk_chat_attachments(conversation_id, status);
		CREATE INDEX IF NOT EXISTS idx_agodesk_chat_attachments_message ON agodesk_chat_attachments(message_id);
		CREATE INDEX IF NOT EXISTS idx_agodesk_chat_attachments_expiry ON agodesk_chat_attachments(expires_at);
	`)
	if err != nil {
		return fmt.Errorf("init agodesk attachments table: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) PrepareAgoDeskAttachment(record AgoDeskAttachmentRecord) error {
	record.AttachmentID = strings.TrimSpace(record.AttachmentID)
	if record.AttachmentID == "" {
		return fmt.Errorf("attachment_id is required")
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		status = AgoDeskAttachmentStatusPrepared
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT INTO agodesk_chat_attachments (
			attachment_id, transport_session_id, conversation_id, status, filename, mime_type, kind,
			declared_size_bytes, size_bytes, expected_sha256, sha256, relative_path,
			created_at, uploaded_at, expires_at, message_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.AttachmentID,
		strings.TrimSpace(record.TransportSessionID),
		strings.TrimSpace(record.ConversationID),
		status,
		strings.TrimSpace(record.Filename),
		strings.TrimSpace(record.MimeType),
		strings.TrimSpace(record.Kind),
		record.DeclaredSizeBytes,
		record.SizeBytes,
		strings.TrimSpace(record.ExpectedSHA256),
		strings.TrimSpace(record.SHA256),
		strings.TrimSpace(record.RelativePath),
		sqliteTime(createdAt),
		sqliteTime(record.UploadedAt),
		sqliteTime(record.ExpiresAt),
		record.MessageID,
	)
	if err != nil {
		return fmt.Errorf("prepare agodesk attachment: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) GetAgoDeskAttachment(attachmentID string) (*AgoDeskAttachmentRecord, error) {
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return nil, nil
	}
	row := s.db.QueryRow(`
		SELECT attachment_id, transport_session_id, conversation_id, status, filename, mime_type, kind,
		       declared_size_bytes, size_bytes, expected_sha256, sha256, relative_path,
		       created_at, uploaded_at, expires_at, message_id
		FROM agodesk_chat_attachments
		WHERE attachment_id = ?`, attachmentID)
	record, err := scanAgoDeskAttachment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agodesk attachment: %w", err)
	}
	return &record, nil
}

func (s *SQLiteMemory) MarkAgoDeskAttachmentUploaded(attachmentID string, sizeBytes int64, sha256Value, relativePath, kind, mimeType string) (*AgoDeskAttachmentRecord, error) {
	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return nil, fmt.Errorf("attachment_id is required")
	}
	res, err := s.db.Exec(`
		UPDATE agodesk_chat_attachments
		SET status = ?, size_bytes = ?, sha256 = ?, relative_path = ?, kind = ?, mime_type = ?, uploaded_at = ?
		WHERE attachment_id = ? AND status = ?`,
		AgoDeskAttachmentStatusUploaded,
		sizeBytes,
		strings.TrimSpace(sha256Value),
		strings.TrimSpace(relativePath),
		strings.TrimSpace(kind),
		strings.TrimSpace(mimeType),
		sqliteTime(time.Now().UTC()),
		attachmentID,
		AgoDeskAttachmentStatusPrepared,
	)
	if err != nil {
		return nil, fmt.Errorf("mark agodesk attachment uploaded: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, fmt.Errorf("agodesk attachment %q is not prepared", attachmentID)
	}
	return s.GetAgoDeskAttachment(attachmentID)
}

func (s *SQLiteMemory) BindAgoDeskAttachmentsToMessage(conversationID string, messageID int64, attachmentIDs []string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	if messageID <= 0 {
		return fmt.Errorf("message_id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin agodesk attachment bind: %w", err)
	}
	defer tx.Rollback()
	for _, attachmentID := range attachmentIDs {
		attachmentID = strings.TrimSpace(attachmentID)
		if attachmentID == "" {
			continue
		}
		res, err := tx.Exec(`
			UPDATE agodesk_chat_attachments
			SET status = ?, message_id = ?
			WHERE attachment_id = ? AND conversation_id = ? AND status = ?`,
			AgoDeskAttachmentStatusAccepted,
			messageID,
			attachmentID,
			conversationID,
			AgoDeskAttachmentStatusUploaded,
		)
		if err != nil {
			return fmt.Errorf("bind agodesk attachment %s: %w", attachmentID, err)
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			return fmt.Errorf("agodesk attachment %q is not ready for conversation %q", attachmentID, conversationID)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agodesk attachment bind: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ListAgoDeskAttachmentsForMessages(messageIDs []int64) (map[int64][]AgoDeskAttachmentRecord, error) {
	out := make(map[int64][]AgoDeskAttachmentRecord)
	if len(messageIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, 0, len(messageIDs))
	args := make([]interface{}, 0, len(messageIDs))
	for _, messageID := range messageIDs {
		if messageID <= 0 {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, messageID)
	}
	if len(args) == 0 {
		return out, nil
	}
	query := fmt.Sprintf(`
		SELECT attachment_id, transport_session_id, conversation_id, status, filename, mime_type, kind,
		       declared_size_bytes, size_bytes, expected_sha256, sha256, relative_path,
		       created_at, uploaded_at, expires_at, message_id
		FROM agodesk_chat_attachments
		WHERE message_id IN (%s)
		ORDER BY rowid ASC`, strings.Join(placeholders, ","))
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agodesk attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		record, err := scanAgoDeskAttachment(rows)
		if err != nil {
			return nil, err
		}
		out[record.MessageID] = append(out[record.MessageID], record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agodesk attachments: %w", err)
	}
	return out, nil
}

func (s *SQLiteMemory) CleanupExpiredAgoDeskAttachments(now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res, err := s.db.Exec(`
		DELETE FROM agodesk_chat_attachments
		WHERE message_id = 0
		  AND expires_at != ''
		  AND expires_at < ?
		  AND status IN (?, ?)`,
		sqliteTime(now),
		AgoDeskAttachmentStatusPrepared,
		AgoDeskAttachmentStatusUploaded,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired agodesk attachments: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cleanup expired agodesk attachments rows affected: %w", err)
	}
	return rows, nil
}

type agodeskAttachmentScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgoDeskAttachment(scanner agodeskAttachmentScanner) (AgoDeskAttachmentRecord, error) {
	var record AgoDeskAttachmentRecord
	var createdAt, uploadedAt, expiresAt string
	if err := scanner.Scan(
		&record.AttachmentID,
		&record.TransportSessionID,
		&record.ConversationID,
		&record.Status,
		&record.Filename,
		&record.MimeType,
		&record.Kind,
		&record.DeclaredSizeBytes,
		&record.SizeBytes,
		&record.ExpectedSHA256,
		&record.SHA256,
		&record.RelativePath,
		&createdAt,
		&uploadedAt,
		&expiresAt,
		&record.MessageID,
	); err != nil {
		return record, err
	}
	record.CreatedAt = parseSQLiteTimeOrZero(createdAt)
	record.UploadedAt = parseSQLiteTimeOrZero(uploadedAt)
	record.ExpiresAt = parseSQLiteTimeOrZero(expiresAt)
	return record, nil
}

func sqliteTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func parseSQLiteTimeOrZero(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := parseSQLiteTimestamp(value); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}
