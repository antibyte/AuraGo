package credentials

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const schemaVersion = 1

// Record stores non-secret metadata for a service credential.
// Secret material is always stored in the vault and referenced by key.
type Record struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	Host               string    `json:"host"`
	Username           string    `json:"username"`
	Description        string    `json:"description"`
	PasswordVaultID    string    `json:"-"`
	CertificateVaultID string    `json:"-"`
	CertificateMode    string    `json:"certificate_mode"`
	HasPassword        bool      `json:"has_password"`
	HasCertificate     bool      `json:"has_certificate"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func EnsureSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	schema := `
	CREATE TABLE IF NOT EXISTS credentials (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		host TEXT NOT NULL,
		username TEXT NOT NULL,
		description TEXT,
		password_vault_id TEXT,
		certificate_vault_id TEXT,
		certificate_mode TEXT NOT NULL DEFAULT 'text',
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create credentials schema: %w", err)
	}

	var currentVer int
	_ = db.QueryRow("PRAGMA user_version").Scan(&currentVer)
	if currentVer < schemaVersion {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
			return fmt.Errorf("set credentials schema version: %w", err)
		}
	}

	return nil
}

func Create(db *sql.DB, rec Record) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is nil")
	}
	if strings.TrimSpace(rec.Name) == "" {
		return "", fmt.Errorf("name is required")
	}
	if strings.TrimSpace(rec.Host) == "" {
		return "", fmt.Errorf("host is required")
	}
	if strings.TrimSpace(rec.Username) == "" {
		return "", fmt.Errorf("username is required")
	}

	rec.ID = uuid.NewString()
	rec.Type = normalizeType(rec.Type)
	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	_, err := db.Exec(`
		INSERT INTO credentials (
			id, name, type, host, username, description,
			password_vault_id, certificate_vault_id, certificate_mode,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rec.ID, rec.Name, rec.Type, rec.Host, rec.Username, rec.Description,
		nullIfEmpty(rec.PasswordVaultID), nullIfEmpty(rec.CertificateVaultID), rec.CertificateMode,
		rec.CreatedAt, rec.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("create credential: %w", err)
	}
	return rec.ID, nil
}

func Update(db *sql.DB, rec Record) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if strings.TrimSpace(rec.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(rec.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(rec.Host) == "" {
		return fmt.Errorf("host is required")
	}
	if strings.TrimSpace(rec.Username) == "" {
		return fmt.Errorf("username is required")
	}

	rec.Type = normalizeType(rec.Type)
	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	rec.UpdatedAt = time.Now().UTC()

	res, err := db.Exec(`
		UPDATE credentials
		SET name = ?, type = ?, host = ?, username = ?, description = ?,
			password_vault_id = ?, certificate_vault_id = ?, certificate_mode = ?, updated_at = ?
		WHERE id = ?
	`,
		rec.Name, rec.Type, rec.Host, rec.Username, rec.Description,
		nullIfEmpty(rec.PasswordVaultID), nullIfEmpty(rec.CertificateVaultID), rec.CertificateMode, rec.UpdatedAt, rec.ID,
	)
	if err != nil {
		return fmt.Errorf("update credential: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credential not found: %s", rec.ID)
	}
	return nil
}

func Delete(db *sql.DB, id string) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	res, err := db.Exec(`DELETE FROM credentials WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("credential not found: %s", id)
	}
	return nil
}

func List(db *sql.DB) ([]Record, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	rows, err := db.Query(`
		SELECT id, name, type, host, username, COALESCE(description, ''),
		       COALESCE(password_vault_id, ''), COALESCE(certificate_vault_id, ''),
		       COALESCE(certificate_mode, 'text'), created_at, updated_at
		FROM credentials
		ORDER BY lower(name), created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func GetByID(db *sql.DB, id string) (Record, error) {
	if db == nil {
		return Record{}, fmt.Errorf("database is nil")
	}
	var rec Record
	err := db.QueryRow(`
		SELECT id, name, type, host, username, COALESCE(description, ''),
		       COALESCE(password_vault_id, ''), COALESCE(certificate_vault_id, ''),
		       COALESCE(certificate_mode, 'text'), created_at, updated_at
		FROM credentials
		WHERE id = ?
	`, id).Scan(
		&rec.ID, &rec.Name, &rec.Type, &rec.Host, &rec.Username, &rec.Description,
		&rec.PasswordVaultID, &rec.CertificateVaultID, &rec.CertificateMode, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Record{}, fmt.Errorf("credential not found: %s", id)
		}
		return Record{}, fmt.Errorf("get credential: %w", err)
	}
	rec.HasPassword = rec.PasswordVaultID != ""
	rec.HasCertificate = rec.CertificateVaultID != ""
	rec.Type = normalizeType(rec.Type)
	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	return rec, nil
}

func scanRows(rows *sql.Rows) ([]Record, error) {
	var result []Record
	for rows.Next() {
		var rec Record
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Type, &rec.Host, &rec.Username, &rec.Description,
			&rec.PasswordVaultID, &rec.CertificateVaultID, &rec.CertificateMode, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential row: %w", err)
		}
		rec.HasPassword = rec.PasswordVaultID != ""
		rec.HasCertificate = rec.CertificateVaultID != ""
		rec.Type = normalizeType(rec.Type)
		rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
		result = append(result, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("credentials rows: %w", err)
	}
	return result, nil
}

func normalizeType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "ssh"
	}
	return v
}

func normalizeCertificateMode(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "upload":
		return "upload"
	default:
		return "text"
	}
}

func nullIfEmpty(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}
