package invasion

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/uid"
)

const (
	ArtifactStatusPending   = "pending"
	ArtifactStatusUploading = "uploading"
	ArtifactStatusCompleted = "completed"
	ArtifactStatusFailed    = "failed"
)

// ArtifactRecord describes a file produced by an egg and stored on the host.
type ArtifactRecord struct {
	ID           string `json:"id"`
	NestID       string `json:"nest_id"`
	EggID        string `json:"egg_id"`
	MissionID    string `json:"mission_id,omitempty"`
	TaskID       string `json:"task_id,omitempty"`
	Filename     string `json:"filename"`
	MIMEType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
	StoragePath  string `json:"storage_path"`
	WebPath      string `json:"web_path"`
	MetadataJSON string `json:"metadata_json,omitempty"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	CompletedAt  string `json:"completed_at,omitempty"`
}

// ArtifactUploadRequest reserves a host-side upload slot for an egg artifact.
type ArtifactUploadRequest struct {
	NestID         string
	EggID          string
	MissionID      string
	TaskID         string
	Filename       string
	MIMEType       string
	ExpectedSize   int64
	ExpectedSHA256 string
	MetadataJSON   string
	TTL            time.Duration
}

// ArtifactUploadSlot is a pending upload token joined with its artifact metadata.
type ArtifactUploadSlot struct {
	TokenHash      string
	Artifact       ArtifactRecord
	ExpectedSize   int64
	ExpectedSHA256 string
	ExpiresAt      time.Time
	Status         string
}

// ArtifactFilter restricts artifact list queries.
type ArtifactFilter struct {
	NestID    string
	EggID     string
	MissionID string
	TaskID    string
	Status    string
	Limit     int
}

type ArtifactCleanupResult struct {
	ExpiredUploads        int64
	StalePendingArtifacts int64
}

func CreateArtifactUpload(db *sql.DB, req ArtifactUploadRequest) (string, ArtifactRecord, error) {
	if db == nil {
		return "", ArtifactRecord{}, fmt.Errorf("invasion database is unavailable")
	}
	req.NestID = strings.TrimSpace(req.NestID)
	req.EggID = strings.TrimSpace(req.EggID)
	if req.NestID == "" {
		return "", ArtifactRecord{}, fmt.Errorf("nest_id is required")
	}
	if req.Filename == "" {
		return "", ArtifactRecord{}, fmt.Errorf("filename is required")
	}
	if req.ExpectedSize < 0 {
		return "", ArtifactRecord{}, fmt.Errorf("expected_size must not be negative")
	}
	if req.ExpectedSize <= 0 {
		return "", ArtifactRecord{}, fmt.Errorf("expected_size is required")
	}
	if req.ExpectedSize > MaxArtifactSizeBytes {
		return "", ArtifactRecord{}, fmt.Errorf("expected_size exceeds maximum %d", MaxArtifactSizeBytes)
	}
	expectedSHA := normalizeSHA256(req.ExpectedSHA256)
	if len(expectedSHA) != 64 {
		return "", ArtifactRecord{}, fmt.Errorf("expected_sha256 must be a 64-character hex string")
	}
	if strings.TrimSpace(req.MetadataJSON) != "" && !json.Valid([]byte(req.MetadataJSON)) {
		return "", ArtifactRecord{}, fmt.Errorf("metadata_json must be valid JSON")
	}

	ttl := req.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if ttl > time.Hour {
		ttl = time.Hour
	}

	artifact := ArtifactRecord{
		ID:           uid.New(),
		NestID:       req.NestID,
		EggID:        req.EggID,
		MissionID:    strings.TrimSpace(req.MissionID),
		TaskID:       strings.TrimSpace(req.TaskID),
		Filename:     SanitizeArtifactFilename(req.Filename),
		MIMEType:     strings.TrimSpace(req.MIMEType),
		SizeBytes:    req.ExpectedSize,
		SHA256:       expectedSHA,
		WebPath:      artifactWebPath(""),
		MetadataJSON: strings.TrimSpace(req.MetadataJSON),
		Status:       ArtifactStatusPending,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	artifact.WebPath = artifactWebPath(artifact.ID)
	token, tokenHash, err := newUploadToken()
	if err != nil {
		return "", ArtifactRecord{}, err
	}
	now := time.Now().UTC()
	expiresAt := now.Add(ttl).Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return "", ArtifactRecord{}, err
	}
	defer tx.Rollback()
	if err := insertArtifactTx(tx, artifact); err != nil {
		return "", ArtifactRecord{}, err
	}
	_, err = tx.Exec(`INSERT INTO invasion_artifact_uploads
		(token_hash, artifact_id, expected_size, expected_sha256, expires_at, created_at, status)
		VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
		tokenHash, artifact.ID, req.ExpectedSize, expectedSHA, expiresAt, now.Format(time.RFC3339))
	if err != nil {
		return "", ArtifactRecord{}, fmt.Errorf("failed to create artifact upload slot: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", ArtifactRecord{}, err
	}
	return token, artifact, nil
}

func GetArtifactUploadByToken(db *sql.DB, token string, now time.Time) (ArtifactUploadSlot, error) {
	if db == nil {
		return ArtifactUploadSlot{}, fmt.Errorf("invasion database is unavailable")
	}
	return getArtifactUploadByTokenHash(db, uploadTokenHash(token), now, ArtifactStatusPending)
}

func ClaimArtifactUploadByToken(db *sql.DB, token string, now time.Time) (ArtifactUploadSlot, error) {
	if db == nil {
		return ArtifactUploadSlot{}, fmt.Errorf("invasion database is unavailable")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tokenHash := uploadTokenHash(token)
	res, err := db.Exec(`UPDATE invasion_artifact_uploads SET status=?
		WHERE token_hash=? AND status=? AND expires_at >= ?`,
		ArtifactStatusUploading, tokenHash, ArtifactStatusPending, now.UTC().Format(time.RFC3339))
	if err != nil {
		return ArtifactUploadSlot{}, fmt.Errorf("failed to claim artifact upload: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return ArtifactUploadSlot{}, fmt.Errorf("failed to check artifact upload claim: %w", err)
	}
	if rows != 1 {
		return ArtifactUploadSlot{}, fmt.Errorf("upload token not found, expired, or already used")
	}
	return getArtifactUploadByTokenHash(db, tokenHash, now, ArtifactStatusUploading)
}

func getArtifactUploadByTokenHash(db *sql.DB, tokenHash string, now time.Time, allowedStatuses ...string) (ArtifactUploadSlot, error) {
	row := db.QueryRow(`SELECT u.token_hash, u.expected_size, u.expected_sha256, u.expires_at, u.status,
		a.id, a.nest_id, a.egg_id, a.mission_id, a.task_id, a.filename, a.mime_type, a.size_bytes,
		a.sha256, a.storage_path, a.web_path, a.metadata_json, a.status, a.created_at, a.completed_at
		FROM invasion_artifact_uploads u
		JOIN invasion_artifacts a ON a.id = u.artifact_id
		WHERE u.token_hash = ?`, tokenHash)
	var slot ArtifactUploadSlot
	var expiresRaw string
	var completedAt sql.NullString
	err := row.Scan(&slot.TokenHash, &slot.ExpectedSize, &slot.ExpectedSHA256, &expiresRaw, &slot.Status,
		&slot.Artifact.ID, &slot.Artifact.NestID, &slot.Artifact.EggID, &slot.Artifact.MissionID, &slot.Artifact.TaskID,
		&slot.Artifact.Filename, &slot.Artifact.MIMEType, &slot.Artifact.SizeBytes, &slot.Artifact.SHA256,
		&slot.Artifact.StoragePath, &slot.Artifact.WebPath, &slot.Artifact.MetadataJSON, &slot.Artifact.Status,
		&slot.Artifact.CreatedAt, &completedAt)
	if err != nil {
		return ArtifactUploadSlot{}, fmt.Errorf("upload token not found: %w", err)
	}
	slot.Artifact.CompletedAt = nullStr(completedAt)
	expires, err := time.Parse(time.RFC3339, expiresRaw)
	if err != nil {
		return ArtifactUploadSlot{}, fmt.Errorf("invalid upload token expiry: %w", err)
	}
	slot.ExpiresAt = expires
	if now.IsZero() {
		now = time.Now()
	}
	if len(allowedStatuses) == 0 {
		allowedStatuses = []string{ArtifactStatusPending}
	}
	allowed := false
	for _, status := range allowedStatuses {
		if slot.Status == status {
			allowed = true
			break
		}
	}
	if !allowed {
		return ArtifactUploadSlot{}, fmt.Errorf("upload token is not available")
	}
	if now.After(expires) {
		return ArtifactUploadSlot{}, fmt.Errorf("upload token expired")
	}
	return slot, nil
}

func CompleteArtifactUpload(db *sql.DB, token, storagePath string, sizeBytes int64, sha string, completedAt time.Time) error {
	if db == nil {
		return fmt.Errorf("invasion database is unavailable")
	}
	slot, err := getArtifactUploadByTokenHash(db, uploadTokenHash(token), completedAt, ArtifactStatusPending, ArtifactStatusUploading)
	if err != nil {
		return err
	}
	return completeArtifact(db, slot.Artifact.ID, uploadTokenHash(token), storagePath, sizeBytes, sha, completedAt)
}

func RegisterCompletedArtifact(db *sql.DB, artifact ArtifactRecord) (string, error) {
	if db == nil {
		return "", fmt.Errorf("invasion database is unavailable")
	}
	artifact.ID = strings.TrimSpace(artifact.ID)
	if artifact.ID == "" {
		artifact.ID = uid.New()
	}
	artifact.NestID = strings.TrimSpace(artifact.NestID)
	if artifact.NestID == "" {
		return "", fmt.Errorf("nest_id is required")
	}
	artifact.Filename = SanitizeArtifactFilename(artifact.Filename)
	if artifact.Filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	if artifact.Status == "" {
		artifact.Status = ArtifactStatusCompleted
	}
	if artifact.CreatedAt == "" {
		artifact.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if artifact.CompletedAt == "" && artifact.Status == ArtifactStatusCompleted {
		artifact.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if artifact.WebPath == "" {
		artifact.WebPath = artifactWebPath(artifact.ID)
	}
	artifact.SHA256 = normalizeSHA256(artifact.SHA256)
	if artifact.Status == ArtifactStatusCompleted {
		if err := validateCompletedArtifactFile(&artifact); err != nil {
			return "", err
		}
	}
	if err := insertArtifactTx(db, artifact); err != nil {
		return "", err
	}
	return artifact.ID, nil
}

func validateCompletedArtifactFile(artifact *ArtifactRecord) error {
	if artifact == nil {
		return fmt.Errorf("artifact is required")
	}
	artifact.StoragePath = strings.TrimSpace(artifact.StoragePath)
	if artifact.StoragePath == "" {
		return fmt.Errorf("storage_path is required for completed artifact")
	}
	info, err := os.Stat(artifact.StoragePath)
	if err != nil {
		return fmt.Errorf("completed artifact file is not accessible: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("completed artifact storage_path points to a directory")
	}
	if artifact.SizeBytes > 0 && artifact.SizeBytes != info.Size() {
		return fmt.Errorf("artifact size mismatch: got %d bytes on disk, expected %d", info.Size(), artifact.SizeBytes)
	}
	artifact.SizeBytes = info.Size()
	actualSHA, err := fileSHA256(artifact.StoragePath)
	if err != nil {
		return err
	}
	if artifact.SHA256 != "" && artifact.SHA256 != actualSHA {
		return fmt.Errorf("artifact sha256 mismatch: got %s, expected %s", actualSHA, artifact.SHA256)
	}
	artifact.SHA256 = actualSHA
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact file: %w", err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("hash artifact file: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GetArtifact(db *sql.DB, id string) (ArtifactRecord, error) {
	row := db.QueryRow(`SELECT id, nest_id, egg_id, mission_id, task_id, filename, mime_type, size_bytes,
		sha256, storage_path, web_path, metadata_json, status, created_at, completed_at
		FROM invasion_artifacts WHERE id = ?`, strings.TrimSpace(id))
	return scanArtifact(row)
}

func ListArtifacts(db *sql.DB, filter ArtifactFilter) ([]ArtifactRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("invasion database is unavailable")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	clauses := []string{"1=1"}
	args := make([]interface{}, 0, 6)
	add := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		clauses = append(clauses, field+" = ?")
		args = append(args, strings.TrimSpace(value))
	}
	add("nest_id", filter.NestID)
	add("egg_id", filter.EggID)
	add("mission_id", filter.MissionID)
	add("task_id", filter.TaskID)
	add("status", filter.Status)
	args = append(args, limit)
	rows, err := db.Query(`SELECT id, nest_id, egg_id, mission_id, task_id, filename, mime_type, size_bytes,
		sha256, storage_path, web_path, metadata_json, status, created_at, completed_at
		FROM invasion_artifacts WHERE `+strings.Join(clauses, " AND ")+` ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}
	defer rows.Close()
	var artifacts []ArtifactRecord
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

func SanitizeArtifactFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(strings.ReplaceAll(name, "\x00", ""))
	if name == "" || name == "." || name == "/" {
		return "artifact.bin"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_', r == ' ':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	clean := strings.Trim(b.String(), ". ")
	if clean == "" {
		return "artifact.bin"
	}
	return clean
}

func insertArtifactTx(exec interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}, a ArtifactRecord) error {
	_, err := exec.Exec(`INSERT INTO invasion_artifacts
		(id, nest_id, egg_id, mission_id, task_id, filename, mime_type, size_bytes, sha256, storage_path,
		 web_path, metadata_json, status, created_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.NestID, a.EggID, a.MissionID, a.TaskID, a.Filename, a.MIMEType, a.SizeBytes, a.SHA256,
		a.StoragePath, a.WebPath, a.MetadataJSON, a.Status, a.CreatedAt, a.CompletedAt)
	if err != nil {
		return fmt.Errorf("failed to insert artifact: %w", err)
	}
	return nil
}

func completeArtifact(db *sql.DB, artifactID, tokenHash, storagePath string, sizeBytes int64, sha string, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now().UTC()
	}
	sha = normalizeSHA256(sha)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE invasion_artifacts SET status=?, storage_path=?, size_bytes=?, sha256=?, completed_at=?
		WHERE id=?`, ArtifactStatusCompleted, storagePath, sizeBytes, sha, completedAt.UTC().Format(time.RFC3339), artifactID)
	if err != nil {
		return fmt.Errorf("failed to complete artifact: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check artifact completion: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("artifact not found: %s", artifactID)
	}
	res, err = tx.Exec(`UPDATE invasion_artifact_uploads SET status=?, completed_at=?
		WHERE token_hash=? AND status IN (?, ?)`,
		ArtifactStatusCompleted, completedAt.UTC().Format(time.RFC3339), tokenHash, ArtifactStatusPending, ArtifactStatusUploading)
	if err != nil {
		return fmt.Errorf("failed to complete artifact upload: %w", err)
	}
	rows, err = res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check artifact upload completion: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("artifact upload token was already used")
	}
	return tx.Commit()
}

func CleanupExpiredArtifactUploads(db *sql.DB, pendingMaxAge time.Duration) (ArtifactCleanupResult, error) {
	if db == nil {
		return ArtifactCleanupResult{}, fmt.Errorf("invasion database is unavailable")
	}
	if pendingMaxAge <= 0 {
		pendingMaxAge = 24 * time.Hour
	}
	now := time.Now().UTC().Format(time.RFC3339)
	pendingCutoff := time.Now().UTC().Add(-pendingMaxAge).Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return ArtifactCleanupResult{}, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`DELETE FROM invasion_artifact_uploads
		WHERE status IN (?, ?) AND expires_at < ?`,
		ArtifactStatusPending, ArtifactStatusUploading, now)
	if err != nil {
		return ArtifactCleanupResult{}, fmt.Errorf("failed to cleanup expired artifact uploads: %w", err)
	}
	expiredUploads, err := res.RowsAffected()
	if err != nil {
		return ArtifactCleanupResult{}, fmt.Errorf("failed to check expired artifact uploads cleanup: %w", err)
	}

	res, err = tx.Exec(`DELETE FROM invasion_artifacts
		WHERE status=? AND created_at < ?`,
		ArtifactStatusPending, pendingCutoff)
	if err != nil {
		return ArtifactCleanupResult{}, fmt.Errorf("failed to cleanup stale pending artifacts: %w", err)
	}
	staleArtifacts, err := res.RowsAffected()
	if err != nil {
		return ArtifactCleanupResult{}, fmt.Errorf("failed to check stale artifact cleanup: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ArtifactCleanupResult{}, err
	}
	return ArtifactCleanupResult{ExpiredUploads: expiredUploads, StalePendingArtifacts: staleArtifacts}, nil
}

type artifactScanner interface {
	Scan(dest ...interface{}) error
}

func scanArtifact(row artifactScanner) (ArtifactRecord, error) {
	var a ArtifactRecord
	var completed sql.NullString
	err := row.Scan(&a.ID, &a.NestID, &a.EggID, &a.MissionID, &a.TaskID, &a.Filename, &a.MIMEType, &a.SizeBytes,
		&a.SHA256, &a.StoragePath, &a.WebPath, &a.MetadataJSON, &a.Status, &a.CreatedAt, &completed)
	if err != nil {
		return ArtifactRecord{}, fmt.Errorf("artifact not found: %w", err)
	}
	a.CompletedAt = nullStr(completed)
	return a, nil
}

func newUploadToken() (token, tokenHash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate upload token: %w", err)
	}
	token = hex.EncodeToString(b)
	return token, uploadTokenHash(token), nil
}

func uploadTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func normalizeSHA256(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func artifactWebPath(id string) string {
	if id == "" {
		return ""
	}
	return "/api/invasion/artifacts/" + id + "/download"
}

func decodeArtifactIDsJSON(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, fmt.Errorf("invalid artifact_ids_json: %w", err)
	}
	return ids, nil
}
