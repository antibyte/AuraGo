package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	AgoDeskKnowledgeStatusPrepared   = "prepared"
	AgoDeskKnowledgeStatusUploading  = "uploading"
	AgoDeskKnowledgeStatusProcessing = "processing"
	AgoDeskKnowledgeStatusReady      = "ready"
	AgoDeskKnowledgeStatusFailed     = "failed"
)

type AgoDeskKnowledgeDocument struct {
	DocumentID         string
	PrepareID          string
	PrepareFingerprint string
	BatchIndex         int
	OwnerDeviceID      string
	Status             string
	Filename           string
	StoragePath        string
	Collection         string
	Title              string
	Tags               []string
	DeclaredMime       string
	DetectedMime       string
	DeclaredSizeBytes  int64
	SizeBytes          int64
	SHA256             string
	CreatedAt          time.Time
	ExpiresAt          time.Time
	UploadStartedAt    time.Time
	UploadedAt         time.Time
	CompletedAt        time.Time
	ErrorCode          string
	ErrorMessage       string
	ChunkCount         int
}

func (s *SQLiteMemory) InitAgoDeskKnowledgeTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS agodesk_knowledge_documents (
			document_id TEXT PRIMARY KEY,
			prepare_id TEXT NOT NULL,
			prepare_fingerprint TEXT NOT NULL,
			batch_index INTEGER NOT NULL,
			owner_device_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'prepared',
			filename TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			collection TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			tags_json TEXT NOT NULL DEFAULT '[]',
			declared_mime TEXT NOT NULL DEFAULT '',
			detected_mime TEXT NOT NULL DEFAULT '',
			declared_size_bytes INTEGER NOT NULL DEFAULT 0,
			size_bytes INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME NOT NULL DEFAULT '',
			upload_started_at DATETIME NOT NULL DEFAULT '',
			uploaded_at DATETIME NOT NULL DEFAULT '',
			completed_at DATETIME NOT NULL DEFAULT '',
			error_code TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			chunk_count INTEGER NOT NULL DEFAULT 0,
			UNIQUE(owner_device_id, prepare_id, batch_index)
		);
		CREATE INDEX IF NOT EXISTS idx_agodesk_knowledge_prepare
			ON agodesk_knowledge_documents(owner_device_id, prepare_id, batch_index);
		CREATE INDEX IF NOT EXISTS idx_agodesk_knowledge_status
			ON agodesk_knowledge_documents(status, completed_at);
		CREATE INDEX IF NOT EXISTS idx_agodesk_knowledge_expiry
			ON agodesk_knowledge_documents(expires_at, status);

		CREATE TABLE IF NOT EXISTS agodesk_knowledge_reservations (
			storage_path TEXT PRIMARY KEY,
			document_id TEXT NOT NULL UNIQUE
		);
		INSERT OR IGNORE INTO agodesk_knowledge_reservations(storage_path, document_id)
			SELECT storage_path, document_id
			FROM agodesk_knowledge_documents
			WHERE status IN ('prepared', 'uploading', 'processing');

		CREATE TABLE IF NOT EXISTS file_index_metadata (
			file_path TEXT NOT NULL,
			collection TEXT NOT NULL DEFAULT '',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(file_path, collection)
		);
	`)
	if err != nil {
		return fmt.Errorf("init agodesk knowledge tables: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) PrepareAgoDeskKnowledgeBatch(records []AgoDeskKnowledgeDocument) error {
	if len(records) == 0 {
		return fmt.Errorf("knowledge batch is empty")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin agodesk knowledge batch: %w", err)
	}
	defer tx.Rollback()

	for _, record := range records {
		if strings.TrimSpace(record.DocumentID) == "" ||
			strings.TrimSpace(record.PrepareID) == "" ||
			strings.TrimSpace(record.PrepareFingerprint) == "" ||
			strings.TrimSpace(record.OwnerDeviceID) == "" {
			return fmt.Errorf("knowledge document identity is incomplete")
		}
		status := strings.TrimSpace(record.Status)
		if status == "" {
			status = AgoDeskKnowledgeStatusPrepared
		}
		createdAt := record.CreatedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		tagsJSON, err := json.Marshal(record.Tags)
		if err != nil {
			return fmt.Errorf("marshal agodesk knowledge tags: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO agodesk_knowledge_reservations(storage_path, document_id)
			VALUES (?, ?)`,
			strings.TrimSpace(record.StoragePath),
			strings.TrimSpace(record.DocumentID),
		); err != nil {
			return fmt.Errorf("reserve agodesk knowledge storage path: %w", err)
		}
		if _, err := tx.Exec(`
			INSERT INTO agodesk_knowledge_documents (
				document_id, prepare_id, prepare_fingerprint, batch_index, owner_device_id,
				status, filename, storage_path, collection, title, tags_json,
				declared_mime, detected_mime, declared_size_bytes, size_bytes, sha256,
				created_at, expires_at, upload_started_at, uploaded_at, completed_at,
				error_code, error_message, chunk_count
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(record.DocumentID),
			strings.TrimSpace(record.PrepareID),
			strings.TrimSpace(record.PrepareFingerprint),
			record.BatchIndex,
			strings.TrimSpace(record.OwnerDeviceID),
			status,
			strings.TrimSpace(record.Filename),
			strings.TrimSpace(record.StoragePath),
			strings.TrimSpace(record.Collection),
			strings.TrimSpace(record.Title),
			string(tagsJSON),
			strings.TrimSpace(record.DeclaredMime),
			strings.TrimSpace(record.DetectedMime),
			record.DeclaredSizeBytes,
			record.SizeBytes,
			strings.TrimSpace(record.SHA256),
			sqliteTime(createdAt),
			sqliteTime(record.ExpiresAt),
			sqliteTime(record.UploadStartedAt),
			sqliteTime(record.UploadedAt),
			sqliteTime(record.CompletedAt),
			strings.TrimSpace(record.ErrorCode),
			strings.TrimSpace(record.ErrorMessage),
			record.ChunkCount,
		); err != nil {
			return fmt.Errorf("insert agodesk knowledge document: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit agodesk knowledge batch: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) ListAgoDeskKnowledgeByPrepare(ownerDeviceID, prepareID string) ([]AgoDeskKnowledgeDocument, error) {
	rows, err := s.db.Query(`
		SELECT `+agoDeskKnowledgeColumns+`
		FROM agodesk_knowledge_documents
		WHERE owner_device_id = ? AND prepare_id = ?
		ORDER BY batch_index`,
		strings.TrimSpace(ownerDeviceID),
		strings.TrimSpace(prepareID),
	)
	if err != nil {
		return nil, fmt.Errorf("list agodesk knowledge prepare: %w", err)
	}
	defer rows.Close()
	return scanAgoDeskKnowledgeRows(rows)
}

func (s *SQLiteMemory) GetAgoDeskKnowledgeDocument(documentID string) (*AgoDeskKnowledgeDocument, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, nil
	}
	record, err := scanAgoDeskKnowledgeDocument(s.db.QueryRow(`
		SELECT `+agoDeskKnowledgeColumns+`
		FROM agodesk_knowledge_documents
		WHERE document_id = ?`, documentID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agodesk knowledge document: %w", err)
	}
	return &record, nil
}

func (s *SQLiteMemory) AgoDeskKnowledgeStoragePathReserved(storagePath string) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*)
		FROM agodesk_knowledge_reservations
		WHERE storage_path = ?`,
		strings.TrimSpace(storagePath),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check agodesk knowledge storage reservation: %w", err)
	}
	return count > 0, nil
}

func (s *SQLiteMemory) MarkAgoDeskKnowledgeUploading(documentID string, now time.Time) (*AgoDeskKnowledgeDocument, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.transitionAgoDeskKnowledge(
		documentID,
		[]string{AgoDeskKnowledgeStatusPrepared},
		`status = ?, upload_started_at = ?, error_code = '', error_message = ''`,
		AgoDeskKnowledgeStatusUploading,
		sqliteTime(now),
	)
}

func (s *SQLiteMemory) ResetAgoDeskKnowledgeUploading(documentID string) error {
	_, err := s.db.Exec(`
		UPDATE agodesk_knowledge_documents
		SET status = ?, upload_started_at = ''
		WHERE document_id = ? AND status = ?`,
		AgoDeskKnowledgeStatusPrepared,
		strings.TrimSpace(documentID),
		AgoDeskKnowledgeStatusUploading,
	)
	if err != nil {
		return fmt.Errorf("reset agodesk knowledge upload: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) MarkAgoDeskKnowledgeProcessing(documentID, detectedMime string, sizeBytes int64, sha256Value string, now time.Time) (*AgoDeskKnowledgeDocument, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.transitionAgoDeskKnowledge(
		documentID,
		[]string{AgoDeskKnowledgeStatusUploading},
		`status = ?, detected_mime = ?, size_bytes = ?, sha256 = ?, uploaded_at = ?`,
		AgoDeskKnowledgeStatusProcessing,
		strings.TrimSpace(detectedMime),
		sizeBytes,
		strings.TrimSpace(sha256Value),
		sqliteTime(now),
	)
}

func (s *SQLiteMemory) MarkAgoDeskKnowledgeReady(documentID string, chunkCount int, now time.Time) (*AgoDeskKnowledgeDocument, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.transitionAgoDeskKnowledge(
		documentID,
		[]string{AgoDeskKnowledgeStatusProcessing},
		`status = ?, chunk_count = ?, completed_at = ?, error_code = '', error_message = ''`,
		AgoDeskKnowledgeStatusReady,
		chunkCount,
		sqliteTime(now),
	)
}

func (s *SQLiteMemory) MarkAgoDeskKnowledgeFailed(documentID, errorCode, errorMessage string, now time.Time) (*AgoDeskKnowledgeDocument, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return s.transitionAgoDeskKnowledge(
		documentID,
		[]string{
			AgoDeskKnowledgeStatusPrepared,
			AgoDeskKnowledgeStatusUploading,
			AgoDeskKnowledgeStatusProcessing,
		},
		`status = ?, error_code = ?, error_message = ?, completed_at = ?`,
		AgoDeskKnowledgeStatusFailed,
		strings.TrimSpace(errorCode),
		strings.TrimSpace(errorMessage),
		sqliteTime(now),
	)
}

func (s *SQLiteMemory) ListAgoDeskKnowledgeProcessing() ([]AgoDeskKnowledgeDocument, error) {
	rows, err := s.db.Query(`
		SELECT `+agoDeskKnowledgeColumns+`
		FROM agodesk_knowledge_documents
		WHERE status = ?
		ORDER BY uploaded_at, batch_index`,
		AgoDeskKnowledgeStatusProcessing,
	)
	if err != nil {
		return nil, fmt.Errorf("list processing agodesk knowledge documents: %w", err)
	}
	defer rows.Close()
	return scanAgoDeskKnowledgeRows(rows)
}

func (s *SQLiteMemory) ListAgoDeskKnowledgeUploading() ([]AgoDeskKnowledgeDocument, error) {
	rows, err := s.db.Query(`
		SELECT `+agoDeskKnowledgeColumns+`
		FROM agodesk_knowledge_documents
		WHERE status = ?
		ORDER BY upload_started_at, batch_index`,
		AgoDeskKnowledgeStatusUploading,
	)
	if err != nil {
		return nil, fmt.Errorf("list uploading agodesk knowledge documents: %w", err)
	}
	defer rows.Close()
	return scanAgoDeskKnowledgeRows(rows)
}

func (s *SQLiteMemory) ListAgoDeskKnowledgeReplay(ownerDeviceID string, terminalSince time.Time) ([]AgoDeskKnowledgeDocument, error) {
	rows, err := s.db.Query(`
		SELECT `+agoDeskKnowledgeColumns+`
		FROM agodesk_knowledge_documents
		WHERE owner_device_id = ?
		  AND (
			status IN (?, ?)
			OR (status IN (?, ?) AND completed_at >= ?)
		  )
		ORDER BY created_at, batch_index`,
		strings.TrimSpace(ownerDeviceID),
		AgoDeskKnowledgeStatusUploading,
		AgoDeskKnowledgeStatusProcessing,
		AgoDeskKnowledgeStatusReady,
		AgoDeskKnowledgeStatusFailed,
		sqliteTime(terminalSince),
	)
	if err != nil {
		return nil, fmt.Errorf("list agodesk knowledge replay: %w", err)
	}
	defer rows.Close()
	return scanAgoDeskKnowledgeRows(rows)
}

func (s *SQLiteMemory) ExpireAgoDeskKnowledgeDocuments(now time.Time, errorCode string) ([]AgoDeskKnowledgeDocument, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	rows, err := s.db.Query(`
		SELECT document_id
		FROM agodesk_knowledge_documents
		WHERE status = ?
		  AND expires_at != ''
		  AND expires_at < ?`,
		AgoDeskKnowledgeStatusPrepared,
		sqliteTime(now),
	)
	if err != nil {
		return nil, fmt.Errorf("list expired agodesk knowledge documents: %w", err)
	}
	var documentIDs []string
	for rows.Next() {
		var documentID string
		if err := rows.Scan(&documentID); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan expired agodesk knowledge document: %w", err)
		}
		documentIDs = append(documentIDs, documentID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var expired []AgoDeskKnowledgeDocument
	for _, documentID := range documentIDs {
		record, err := s.MarkAgoDeskKnowledgeFailed(documentID, errorCode, "Knowledge upload reservation expired.", now)
		if err != nil {
			return nil, err
		}
		expired = append(expired, *record)
	}
	return expired, nil
}

func (s *SQLiteMemory) CleanupAgoDeskKnowledgeDocuments(now time.Time, expiredCode string) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	expiredCutoff := sqliteTime(now.Add(-24 * time.Hour))
	terminalCutoff := sqliteTime(now.Add(-7 * 24 * time.Hour))
	res, err := s.db.Exec(`
		DELETE FROM agodesk_knowledge_documents
		WHERE status IN (?, ?)
		  AND completed_at != ''
		  AND (
			(error_code = ? AND completed_at < ?)
			OR completed_at < ?
		  )`,
		AgoDeskKnowledgeStatusReady,
		AgoDeskKnowledgeStatusFailed,
		strings.TrimSpace(expiredCode),
		expiredCutoff,
		terminalCutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup agodesk knowledge documents: %w", err)
	}
	if _, err := s.db.Exec(`
		DELETE FROM agodesk_knowledge_reservations
		WHERE document_id NOT IN (
			SELECT document_id
			FROM agodesk_knowledge_documents
			WHERE status IN (?, ?, ?)
		)`,
		AgoDeskKnowledgeStatusPrepared,
		AgoDeskKnowledgeStatusUploading,
		AgoDeskKnowledgeStatusProcessing,
	); err != nil {
		return 0, fmt.Errorf("cleanup agodesk knowledge reservations: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cleanup agodesk knowledge rows affected: %w", err)
	}
	return count, nil
}

func (s *SQLiteMemory) UpsertFileIndexMetadata(path, collection string, metadata map[string]string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("file path is required")
	}
	cleaned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			cleaned[key] = value
		}
	}
	raw, err := json.Marshal(cleaned)
	if err != nil {
		return fmt.Errorf("marshal file index metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO file_index_metadata(file_path, collection, metadata_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(file_path, collection) DO UPDATE SET
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at`,
		path,
		strings.TrimSpace(collection),
		string(raw),
		sqliteTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("upsert file index metadata: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) GetFileIndexMetadata(path, collection string) (map[string]string, error) {
	var raw string
	err := s.db.QueryRow(`
		SELECT metadata_json
		FROM file_index_metadata
		WHERE file_path = ? AND collection = ?`,
		strings.TrimSpace(path),
		strings.TrimSpace(collection),
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file index metadata: %w", err)
	}
	metadata := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return nil, fmt.Errorf("decode file index metadata: %w", err)
	}
	return metadata, nil
}

func (s *SQLiteMemory) DeleteFileIndexMetadata(path, collection string) error {
	_, err := s.db.Exec(`
		DELETE FROM file_index_metadata
		WHERE file_path = ? AND collection = ?`,
		strings.TrimSpace(path),
		strings.TrimSpace(collection),
	)
	if err != nil {
		return fmt.Errorf("delete file index metadata: %w", err)
	}
	return nil
}

func (s *SQLiteMemory) transitionAgoDeskKnowledge(documentID string, fromStatuses []string, setClause string, args ...interface{}) (*AgoDeskKnowledgeDocument, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, fmt.Errorf("document_id is required")
	}
	if len(fromStatuses) == 0 {
		return nil, fmt.Errorf("knowledge transition requires source statuses")
	}
	placeholders := make([]string, len(fromStatuses))
	queryArgs := make([]interface{}, 0, len(args)+1+len(fromStatuses))
	queryArgs = append(queryArgs, args...)
	queryArgs = append(queryArgs, documentID)
	for i, status := range fromStatuses {
		placeholders[i] = "?"
		queryArgs = append(queryArgs, status)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin agodesk knowledge transition: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		UPDATE agodesk_knowledge_documents
		SET `+setClause+`
		WHERE document_id = ?
		  AND status IN (`+strings.Join(placeholders, ",")+`)`,
		queryArgs...,
	)
	if err != nil {
		return nil, fmt.Errorf("transition agodesk knowledge document: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		_ = tx.Rollback()
		record, getErr := s.GetAgoDeskKnowledgeDocument(documentID)
		if getErr != nil {
			return nil, getErr
		}
		if record == nil {
			return nil, fmt.Errorf("agodesk knowledge document %q was not found", documentID)
		}
		return nil, fmt.Errorf("agodesk knowledge document %q cannot transition from %q", documentID, record.Status)
	}
	if len(args) > 0 {
		targetStatus, ok := args[0].(string)
		if ok &&
			(targetStatus == AgoDeskKnowledgeStatusReady || targetStatus == AgoDeskKnowledgeStatusFailed) {
			if _, err := tx.Exec(`
				DELETE FROM agodesk_knowledge_reservations
				WHERE document_id = ?`, documentID); err != nil {
				return nil, fmt.Errorf("release agodesk knowledge reservation: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit agodesk knowledge transition: %w", err)
	}
	return s.GetAgoDeskKnowledgeDocument(documentID)
}

const agoDeskKnowledgeColumns = `
	document_id, prepare_id, prepare_fingerprint, batch_index, owner_device_id,
	status, filename, storage_path, collection, title, tags_json,
	declared_mime, detected_mime, declared_size_bytes, size_bytes, sha256,
	created_at, expires_at, upload_started_at, uploaded_at, completed_at,
	error_code, error_message, chunk_count`

type agoDeskKnowledgeScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgoDeskKnowledgeDocument(scanner agoDeskKnowledgeScanner) (AgoDeskKnowledgeDocument, error) {
	var record AgoDeskKnowledgeDocument
	var tagsJSON string
	var createdAt, expiresAt, uploadStartedAt, uploadedAt, completedAt string
	if err := scanner.Scan(
		&record.DocumentID,
		&record.PrepareID,
		&record.PrepareFingerprint,
		&record.BatchIndex,
		&record.OwnerDeviceID,
		&record.Status,
		&record.Filename,
		&record.StoragePath,
		&record.Collection,
		&record.Title,
		&tagsJSON,
		&record.DeclaredMime,
		&record.DetectedMime,
		&record.DeclaredSizeBytes,
		&record.SizeBytes,
		&record.SHA256,
		&createdAt,
		&expiresAt,
		&uploadStartedAt,
		&uploadedAt,
		&completedAt,
		&record.ErrorCode,
		&record.ErrorMessage,
		&record.ChunkCount,
	); err != nil {
		return record, err
	}
	if err := json.Unmarshal([]byte(tagsJSON), &record.Tags); err != nil {
		return record, fmt.Errorf("decode agodesk knowledge tags: %w", err)
	}
	record.CreatedAt = parseSQLiteTimeOrZero(createdAt)
	record.ExpiresAt = parseSQLiteTimeOrZero(expiresAt)
	record.UploadStartedAt = parseSQLiteTimeOrZero(uploadStartedAt)
	record.UploadedAt = parseSQLiteTimeOrZero(uploadedAt)
	record.CompletedAt = parseSQLiteTimeOrZero(completedAt)
	return record, nil
}

func scanAgoDeskKnowledgeRows(rows *sql.Rows) ([]AgoDeskKnowledgeDocument, error) {
	var records []AgoDeskKnowledgeDocument
	for rows.Next() {
		record, err := scanAgoDeskKnowledgeDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agodesk knowledge document: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agodesk knowledge documents: %w", err)
	}
	return records, nil
}
