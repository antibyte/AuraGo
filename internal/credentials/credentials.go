package credentials

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const schemaVersion = 2

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
	TokenVaultID       string    `json:"-"`
	CertificateMode    string    `json:"certificate_mode"`
	AllowPython        bool      `json:"allow_python"`
	HasPassword        bool      `json:"has_password"`
	HasCertificate     bool      `json:"has_certificate"`
	HasToken           bool      `json:"has_token"`
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
		host TEXT NOT NULL DEFAULT '',
		username TEXT NOT NULL,
		description TEXT,
		password_vault_id TEXT,
		certificate_vault_id TEXT,
		token_vault_id TEXT,
		certificate_mode TEXT NOT NULL DEFAULT 'text',
		allow_python INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create credentials schema: %w", err)
	}

	// Migration: add columns that may be missing on older databases.
	var hasTokenCol bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('credentials') WHERE name='token_vault_id'").Scan(&hasTokenCol)
	if !hasTokenCol {
		if _, err := db.Exec("ALTER TABLE credentials ADD COLUMN token_vault_id TEXT"); err != nil {
			return fmt.Errorf("add token_vault_id column: %w", err)
		}
	}
	var hasAllowPython bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('credentials') WHERE name='allow_python'").Scan(&hasAllowPython)
	if !hasAllowPython {
		if _, err := db.Exec("ALTER TABLE credentials ADD COLUMN allow_python INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add allow_python column: %w", err)
		}
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
	rec.Type = normalizeType(rec.Type)
	if rec.Type == "ssh" && strings.TrimSpace(rec.Host) == "" {
		return "", fmt.Errorf("host is required for SSH credentials")
	}
	if strings.TrimSpace(rec.Username) == "" {
		return "", fmt.Errorf("username is required")
	}

	rec.ID = uuid.NewString()
	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now

	allowPython := 0
	if rec.AllowPython {
		allowPython = 1
	}

	_, err := db.Exec(`
		INSERT INTO credentials (
			id, name, type, host, username, description,
			password_vault_id, certificate_vault_id, token_vault_id,
			certificate_mode, allow_python,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rec.ID, rec.Name, rec.Type, rec.Host, rec.Username, rec.Description,
		nullIfEmpty(rec.PasswordVaultID), nullIfEmpty(rec.CertificateVaultID), nullIfEmpty(rec.TokenVaultID),
		rec.CertificateMode, allowPython,
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
	rec.Type = normalizeType(rec.Type)
	if rec.Type == "ssh" && strings.TrimSpace(rec.Host) == "" {
		return fmt.Errorf("host is required for SSH credentials")
	}
	if strings.TrimSpace(rec.Username) == "" {
		return fmt.Errorf("username is required")
	}

	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	rec.UpdatedAt = time.Now().UTC()

	allowPython := 0
	if rec.AllowPython {
		allowPython = 1
	}

	res, err := db.Exec(`
		UPDATE credentials
		SET name = ?, type = ?, host = ?, username = ?, description = ?,
			password_vault_id = ?, certificate_vault_id = ?, token_vault_id = ?,
			certificate_mode = ?, allow_python = ?, updated_at = ?
		WHERE id = ?
	`,
		rec.Name, rec.Type, rec.Host, rec.Username, rec.Description,
		nullIfEmpty(rec.PasswordVaultID), nullIfEmpty(rec.CertificateVaultID), nullIfEmpty(rec.TokenVaultID),
		rec.CertificateMode, allowPython, rec.UpdatedAt, rec.ID,
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
		SELECT id, name, type, COALESCE(host, ''), username, COALESCE(description, ''),
		       COALESCE(password_vault_id, ''), COALESCE(certificate_vault_id, ''),
		       COALESCE(token_vault_id, ''), COALESCE(certificate_mode, 'text'),
		       COALESCE(allow_python, 0), created_at, updated_at
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
	var allowPython int
	err := db.QueryRow(`
		SELECT id, name, type, COALESCE(host, ''), username, COALESCE(description, ''),
		       COALESCE(password_vault_id, ''), COALESCE(certificate_vault_id, ''),
		       COALESCE(token_vault_id, ''), COALESCE(certificate_mode, 'text'),
		       COALESCE(allow_python, 0), created_at, updated_at
		FROM credentials
		WHERE id = ?
	`, id).Scan(
		&rec.ID, &rec.Name, &rec.Type, &rec.Host, &rec.Username, &rec.Description,
		&rec.PasswordVaultID, &rec.CertificateVaultID, &rec.TokenVaultID, &rec.CertificateMode,
		&allowPython, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Record{}, fmt.Errorf("credential not found: %s", id)
		}
		return Record{}, fmt.Errorf("get credential: %w", err)
	}
	rec.HasPassword = rec.PasswordVaultID != ""
	rec.HasCertificate = rec.CertificateVaultID != ""
	rec.HasToken = rec.TokenVaultID != ""
	rec.AllowPython = allowPython != 0
	rec.Type = normalizeType(rec.Type)
	rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
	return rec, nil
}

func scanRows(rows *sql.Rows) ([]Record, error) {
	var result []Record
	for rows.Next() {
		var rec Record
		var allowPython int
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Type, &rec.Host, &rec.Username, &rec.Description,
			&rec.PasswordVaultID, &rec.CertificateVaultID, &rec.TokenVaultID, &rec.CertificateMode,
			&allowPython, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential row: %w", err)
		}
		rec.HasPassword = rec.PasswordVaultID != ""
		rec.HasCertificate = rec.CertificateVaultID != ""
		rec.HasToken = rec.TokenVaultID != ""
		rec.AllowPython = allowPython != 0
		rec.Type = normalizeType(rec.Type)
		rec.CertificateMode = normalizeCertificateMode(rec.CertificateMode)
		result = append(result, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("credentials rows: %w", err)
	}
	return result, nil
}

// ValidCredentialTypes lists the accepted credential type values.
var ValidCredentialTypes = map[string]bool{
	"ssh":   true,
	"login": true,
	"token": true,
}

func normalizeType(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return "ssh"
	}
	return v
}

// ListPythonAccessible returns only credentials that have allow_python enabled.
func ListPythonAccessible(db *sql.DB) ([]Record, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	rows, err := db.Query(`
		SELECT id, name, type, COALESCE(host, ''), username, COALESCE(description, ''),
		       COALESCE(password_vault_id, ''), COALESCE(certificate_vault_id, ''),
		       COALESCE(token_vault_id, ''), COALESCE(certificate_mode, 'text'),
		       COALESCE(allow_python, 0), created_at, updated_at
		FROM credentials
		WHERE allow_python = 1
		ORDER BY lower(name), created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list python-accessible credentials: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
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
