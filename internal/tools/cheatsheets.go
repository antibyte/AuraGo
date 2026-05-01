package tools

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/memory"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// CheatSheet represents a reusable workflow/instruction template for the agent.
type CheatSheet struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Content         string                 `json:"content"`
	Abstract        string                 `json:"abstract"`
	Active          bool                   `json:"active"`
	CreatedBy       string                 `json:"created_by"` // "user" or "agent"
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
	UsageCount      int                    `json:"usage_count"`
	LastUsedAt      string                 `json:"last_used_at"`
	DeleteLocked    bool                   `json:"delete_locked"`
	ExpiresAt       string                 `json:"expires_at"`
	Attachments     []CheatSheetAttachment `json:"attachments,omitempty"`
	AttachmentCount int                    `json:"attachment_count"`
}

// CheatSheetAttachment represents a text file attached to a cheat sheet.
type CheatSheetAttachment struct {
	ID           string `json:"id"`
	CheatSheetID string `json:"cheatsheet_id"`
	Filename     string `json:"filename"`
	Source       string `json:"source"` // "upload" or "knowledge"
	Content      string `json:"content"`
	CharCount    int    `json:"char_count"`
	CreatedAt    string `json:"created_at"`
}

const cheatsheetSchemaVersion = 3

// MaxAttachmentChars is the total character limit across all attachments of a single cheat sheet.
const MaxAttachmentChars = 25000

// MaxContentChars is the maximum character count for the cheat sheet content body.
const MaxContentChars = 100000

// AllowedAttachmentExtensions lists the file extensions permitted for cheat sheet attachments.
var AllowedAttachmentExtensions = []string{".txt", ".md"}

// InitCheatsheetDB initializes the cheat sheets SQLite database.
func InitCheatsheetDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cheatsheet database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS cheatsheets (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		content    TEXT NOT NULL DEFAULT '',
		abstract   TEXT NOT NULL DEFAULT '',
		active     INTEGER NOT NULL DEFAULT 1,
		created_by TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		usage_count INTEGER NOT NULL DEFAULT 0,
		last_used_at DATETIME,
		delete_locked INTEGER NOT NULL DEFAULT 0,
		expires_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS cheatsheet_attachments (
		id            TEXT PRIMARY KEY,
		cheatsheet_id TEXT NOT NULL,
		filename      TEXT NOT NULL,
		source        TEXT NOT NULL DEFAULT 'upload',
		content       TEXT NOT NULL DEFAULT '',
		char_count    INTEGER NOT NULL DEFAULT 0,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (cheatsheet_id) REFERENCES cheatsheets(id) ON DELETE CASCADE
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create cheatsheets schema: %w", err)
	}

	// PRAGMA foreign_keys=ON is already set by dbutil.Open()

	// Migration: add attachments table for existing v1 databases
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to read schema version: %w", err)
	}
	if version < 2 {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cheatsheet_attachments (
			id            TEXT PRIMARY KEY,
			cheatsheet_id TEXT NOT NULL,
			filename      TEXT NOT NULL,
			source        TEXT NOT NULL DEFAULT 'upload',
			content       TEXT NOT NULL DEFAULT '',
			char_count    INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (cheatsheet_id) REFERENCES cheatsheets(id) ON DELETE CASCADE
		)`); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to migrate attachments table: %w", err)
		}
	}
	if version < 3 {
		migrations := []string{
			"ALTER TABLE cheatsheets ADD COLUMN abstract TEXT NOT NULL DEFAULT ''",
			"ALTER TABLE cheatsheets ADD COLUMN usage_count INTEGER NOT NULL DEFAULT 0",
			"ALTER TABLE cheatsheets ADD COLUMN last_used_at DATETIME",
			"ALTER TABLE cheatsheets ADD COLUMN delete_locked INTEGER NOT NULL DEFAULT 0",
			"ALTER TABLE cheatsheets ADD COLUMN expires_at DATETIME",
		}
		for _, stmt := range migrations {
			if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
				db.Close()
				return nil, fmt.Errorf("failed to migrate cheatsheet metadata: %w", err)
			}
		}
		if _, err := db.Exec("UPDATE cheatsheets SET expires_at = ? WHERE created_by = 'agent' AND expires_at IS NULL", time.Now().UTC().Add(7*24*time.Hour).Format(time.RFC3339)); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to backfill agent cheatsheet expirations: %w", err)
		}
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", cheatsheetSchemaVersion)); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set schema version: %w", err)
	}
	return db, nil
}

// CheatsheetList returns all cheat sheets, optionally filtered.
func CheatsheetList(db *sql.DB, activeOnly bool) ([]CheatSheet, error) {
	return CheatsheetListByCreatedBy(db, activeOnly, "")
}

// CheatsheetListByCreatedBy returns cheat sheets filtered by active state and creator.
func CheatsheetListByCreatedBy(db *sql.DB, activeOnly bool, createdBy string) ([]CheatSheet, error) {
	query := `SELECT c.id, c.name, c.content, c.abstract, c.active, c.created_by, c.created_at, c.updated_at,
		c.usage_count, c.last_used_at, c.delete_locked, c.expires_at,
		COALESCE((SELECT COUNT(*) FROM cheatsheet_attachments a WHERE a.cheatsheet_id = c.id), 0)
		FROM cheatsheets c`
	var where []string
	var args []interface{}
	if activeOnly {
		where = append(where, "c.active = 1")
	}
	if createdBy != "" {
		where = append(where, "c.created_by = ?")
		args = append(args, createdBy)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY c.name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sheets []CheatSheet
	for rows.Next() {
		var s CheatSheet
		var active, deleteLocked int
		var lastUsed, expires sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &s.Abstract, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.UsageCount, &lastUsed, &deleteLocked, &expires, &s.AttachmentCount); err != nil {
			return nil, err
		}
		s.Active = active == 1
		s.DeleteLocked = deleteLocked == 1
		if lastUsed.Valid {
			s.LastUsedAt = lastUsed.String
		}
		if expires.Valid {
			s.ExpiresAt = expires.String
		}
		sheets = append(sheets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sheets, nil
}

// CheatsheetGet returns a single cheat sheet by ID, including its attachments.
func CheatsheetGet(db *sql.DB, id string) (*CheatSheet, error) {
	var s CheatSheet
	var active, deleteLocked int
	var lastUsed, expires sql.NullString
	err := db.QueryRow(
		"SELECT id, name, content, abstract, active, created_by, created_at, updated_at, usage_count, last_used_at, delete_locked, expires_at FROM cheatsheets WHERE id = ?", id,
	).Scan(&s.ID, &s.Name, &s.Content, &s.Abstract, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.UsageCount, &lastUsed, &deleteLocked, &expires)
	if err != nil {
		return nil, err
	}
	s.Active = active == 1
	s.DeleteLocked = deleteLocked == 1
	if lastUsed.Valid {
		s.LastUsedAt = lastUsed.String
	}
	if expires.Valid {
		s.ExpiresAt = expires.String
	}

	attachments, err := CheatsheetAttachmentList(db, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load attachments: %w", err)
	}
	if attachments == nil {
		attachments = []CheatSheetAttachment{}
	}
	s.Attachments = attachments
	s.AttachmentCount = len(attachments)

	return &s, nil
}

// CheatsheetGetByName returns a single cheat sheet by name (case-insensitive).
func CheatsheetGetByName(db *sql.DB, name string) (*CheatSheet, error) {
	var s CheatSheet
	var active, deleteLocked int
	var lastUsed, expires sql.NullString
	err := db.QueryRow(
		"SELECT id, name, content, abstract, active, created_by, created_at, updated_at, usage_count, last_used_at, delete_locked, expires_at FROM cheatsheets WHERE LOWER(name) = LOWER(?)", name,
	).Scan(&s.ID, &s.Name, &s.Content, &s.Abstract, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.UsageCount, &lastUsed, &deleteLocked, &expires)
	if err != nil {
		return nil, err
	}
	s.Active = active == 1
	s.DeleteLocked = deleteLocked == 1
	if lastUsed.Valid {
		s.LastUsedAt = lastUsed.String
	}
	if expires.Valid {
		s.ExpiresAt = expires.String
	}

	attachments, err := CheatsheetAttachmentList(db, s.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load attachments: %w", err)
	}
	if attachments == nil {
		attachments = []CheatSheetAttachment{}
	}
	s.Attachments = attachments
	s.AttachmentCount = len(attachments)

	return &s, nil
}

// CheatsheetCreate creates a new cheat sheet and returns it.
func CheatsheetCreate(db *sql.DB, name, content, createdBy string) (*CheatSheet, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len([]rune(content)) > MaxContentChars {
		return nil, fmt.Errorf("content exceeds the %d character limit", MaxContentChars)
	}
	if createdBy != "user" && createdBy != "agent" {
		createdBy = "user"
	}

	id := uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	expiresAt := sql.NullString{}
	if createdBy == "agent" {
		expiresAt = sql.NullString{String: time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339), Valid: true}
	}

	_, err := db.Exec(
		"INSERT INTO cheatsheets (id, name, content, active, created_by, created_at, updated_at, expires_at) VALUES (?, ?, ?, 1, ?, ?, ?, ?)",
		id, name, content, createdBy, now, now, expiresAt,
	)
	if err != nil {
		return nil, err
	}
	return CheatsheetGet(db, id)
}

// CheatsheetUpdate updates an existing cheat sheet.
func CheatsheetUpdate(db *sql.DB, id string, name, content, abstract *string, active, deleteLocked *bool) (*CheatSheet, error) {
	existing, err := CheatsheetGet(db, id)
	if err != nil {
		return nil, fmt.Errorf("cheat sheet not found: %w", err)
	}

	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, fmt.Errorf("name cannot be empty")
		}
		existing.Name = n
	}
	if content != nil {
		if len([]rune(*content)) > MaxContentChars {
			return nil, fmt.Errorf("content exceeds the %d character limit", MaxContentChars)
		}
		existing.Content = *content
	}
	if abstract != nil {
		existing.Abstract = strings.TrimSpace(*abstract)
	}
	if active != nil {
		existing.Active = *active
	}
	if deleteLocked != nil {
		existing.DeleteLocked = *deleteLocked
	}

	now := time.Now().UTC().Format(time.RFC3339)
	activeInt := 0
	if existing.Active {
		activeInt = 1
	}
	deleteLockedInt := 0
	if existing.DeleteLocked {
		deleteLockedInt = 1
	}
	expiresAt := sql.NullString{}
	if existing.ExpiresAt != "" {
		expiresAt = sql.NullString{String: existing.ExpiresAt, Valid: true}
	}
	if content != nil && existing.CreatedBy == "agent" {
		expiresAt = sql.NullString{String: time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339), Valid: true}
	}

	_, err = db.Exec(
		"UPDATE cheatsheets SET name = ?, content = ?, abstract = ?, active = ?, delete_locked = ?, expires_at = ?, updated_at = ? WHERE id = ?",
		existing.Name, existing.Content, existing.Abstract, activeInt, deleteLockedInt, expiresAt, now, id,
	)
	if err != nil {
		return nil, err
	}
	return CheatsheetGet(db, id)
}

// CheatsheetDelete deletes a cheat sheet by ID.
func CheatsheetDelete(db *sql.DB, id string) error {
	var locked int
	if err := db.QueryRow("SELECT delete_locked FROM cheatsheets WHERE id = ?", id).Scan(&locked); err != nil {
		return err
	}
	if locked == 1 {
		return fmt.Errorf("cheat sheet is delete-locked")
	}
	result, err := db.Exec("DELETE FROM cheatsheets WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CheatsheetRecordUsage records that a cheat sheet was used by the agent or UI.
func CheatsheetRecordUsage(db *sql.DB, id string) error {
	now := time.Now().UTC()
	nowText := now.Format(time.RFC3339)
	expiresAt := now.Add(7 * 24 * time.Hour).Format(time.RFC3339)
	result, err := db.Exec(`UPDATE cheatsheets
		SET usage_count = usage_count + 1,
			last_used_at = ?,
			expires_at = CASE WHEN created_by = 'agent' THEN ? ELSE expires_at END,
			updated_at = ?
		WHERE id = ?`, nowText, expiresAt, nowText, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CheatsheetUpdateAbstract updates a cheat sheet's generated one-line abstract.
func CheatsheetUpdateAbstract(db *sql.DB, id, abstract string) (*CheatSheet, error) {
	abstract = strings.TrimSpace(abstract)
	_, err := db.Exec("UPDATE cheatsheets SET abstract = ?, updated_at = ? WHERE id = ?", abstract, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return nil, err
	}
	return CheatsheetGet(db, id)
}

// CheatsheetGetExpiredUnused returns agent-created cheat sheets due for maintenance review.
func CheatsheetGetExpiredUnused(db *sql.DB) ([]CheatSheet, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := db.Query(`SELECT id FROM cheatsheets
		WHERE created_by = 'agent'
			AND active = 1
			AND delete_locked = 0
			AND expires_at IS NOT NULL
			AND expires_at <= ?
		ORDER BY expires_at ASC`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	var sheets []CheatSheet
	for _, id := range ids {
		sheet, err := CheatsheetGet(db, id)
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, *sheet)
	}
	return sheets, nil
}

// CheatsheetMarkUnused deactivates an expired agent-created cheat sheet for maintenance review.
func CheatsheetMarkUnused(db *sql.DB, id string) error {
	result, err := db.Exec("UPDATE cheatsheets SET active = 0, updated_at = ? WHERE id = ? AND created_by = 'agent'", time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CheatsheetCount returns total, active, and agent-created counts.
func CheatsheetCount(db *sql.DB) (total, active, agentCreated int, err error) {
	if err = db.QueryRow("SELECT COUNT(*) FROM cheatsheets").Scan(&total); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to count cheatsheets: %w", err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM cheatsheets WHERE active = 1").Scan(&active); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to count active cheatsheets: %w", err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM cheatsheets WHERE created_by = 'agent'").Scan(&agentCreated); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to count agent cheatsheets: %w", err)
	}
	return
}

// CheatsheetGetMultiple returns the contents of multiple cheat sheets by IDs,
// formatted for injection into a mission prompt.
func CheatsheetGetMultiple(db *sql.DB, ids []string) string {
	if len(ids) == 0 {
		return ""
	}

	var parts []string
	for _, id := range ids {
		s, err := CheatsheetGet(db, id)
		if err != nil || !s.Active {
			continue
		}
		_ = CheatsheetRecordUsage(db, id)
		part := fmt.Sprintf("[Cheat Sheet: %q]\n%s", s.Name, s.Content)

		// Append attachments if any (already loaded by CheatsheetGet)
		for _, a := range s.Attachments {
			part += fmt.Sprintf("\n\n[Cheat Sheet Attachment: %q]\n%s", a.Filename, a.Content)
		}

		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(parts, "\n\n")
}

// ── Attachment CRUD ──────────────────────────────────────

// CheatsheetAttachmentList returns all attachments for a cheat sheet.
func CheatsheetAttachmentList(db *sql.DB, cheatsheetID string) ([]CheatSheetAttachment, error) {
	rows, err := db.Query(
		"SELECT id, cheatsheet_id, filename, source, content, char_count, created_at FROM cheatsheet_attachments WHERE cheatsheet_id = ? ORDER BY created_at ASC",
		cheatsheetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []CheatSheetAttachment
	for rows.Next() {
		var a CheatSheetAttachment
		if err := rows.Scan(&a.ID, &a.CheatSheetID, &a.Filename, &a.Source, &a.Content, &a.CharCount, &a.CreatedAt); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachments, nil
}

// cheatsheetAttachmentTotalChars returns the total character count of all attachments for a cheat sheet.
func cheatsheetAttachmentTotalChars(db *sql.DB, cheatsheetID string) (int, error) {
	var total int
	err := db.QueryRow("SELECT COALESCE(SUM(char_count), 0) FROM cheatsheet_attachments WHERE cheatsheet_id = ?", cheatsheetID).Scan(&total)
	return total, err
}

// cheatsheetAttachmentCount returns the number of attachments for a cheat sheet.
func cheatsheetAttachmentCount(db *sql.DB, cheatsheetID string) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM cheatsheet_attachments WHERE cheatsheet_id = ?", cheatsheetID).Scan(&count)
	return count, err
}

// CheatsheetAttachmentAdd adds an attachment to a cheat sheet.
// It validates the file extension and enforces the total character limit.
// The limit check and insert are wrapped in a transaction to prevent races.
func CheatsheetAttachmentAdd(db *sql.DB, cheatsheetID, filename, source, content string) (*CheatSheetAttachment, error) {
	// Validate cheat sheet exists
	_, err := CheatsheetGet(db, cheatsheetID)
	if err != nil {
		return nil, fmt.Errorf("cheat sheet not found: %w", err)
	}

	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, fmt.Errorf("filename is required")
	}

	// Validate extension
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := false
	for _, a := range AllowedAttachmentExtensions {
		if ext == a {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("only .txt and .md files are allowed as attachments")
	}

	// Validate source
	if source != "upload" && source != "knowledge" {
		source = "upload"
	}

	charCount := len([]rune(content))

	// Use a transaction to atomically check the limit and insert
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var currentTotal int
	if err := tx.QueryRow("SELECT COALESCE(SUM(char_count), 0) FROM cheatsheet_attachments WHERE cheatsheet_id = ?", cheatsheetID).Scan(&currentTotal); err != nil {
		return nil, fmt.Errorf("failed to check attachment limits: %w", err)
	}
	if currentTotal+charCount > MaxAttachmentChars {
		return nil, fmt.Errorf("attachment would exceed the %d character limit (current: %d, new: %d)", MaxAttachmentChars, currentTotal, charCount)
	}

	id := uid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.Exec(
		"INSERT INTO cheatsheet_attachments (id, cheatsheet_id, filename, source, content, char_count, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, cheatsheetID, filename, source, content, charCount, now,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit attachment: %w", err)
	}

	return &CheatSheetAttachment{
		ID:           id,
		CheatSheetID: cheatsheetID,
		Filename:     filename,
		Source:       source,
		Content:      content,
		CharCount:    charCount,
		CreatedAt:    now,
	}, nil
}

// ReindexCheatsheetInVectorDB loads the full cheatsheet (including attachments)
// and updates its vector DB index. It is a no-op if vdb is nil.
func ReindexCheatsheetInVectorDB(db *sql.DB, vdb memory.VectorDB, cheatsheetID string) error {
	if vdb == nil {
		return nil
	}
	cs, err := CheatsheetGet(db, cheatsheetID)
	if err != nil {
		return err
	}
	attachments := make([]string, len(cs.Attachments))
	for i, a := range cs.Attachments {
		attachments[i] = a.Content
	}
	content := cs.Content
	if cs.Abstract != "" {
		content = "Abstract: " + cs.Abstract + "\n\n" + content
	}
	return vdb.StoreCheatsheet(cs.ID, cs.Name, content, attachments...)
}

// CheatsheetAttachmentRemove removes an attachment by ID.
func CheatsheetAttachmentRemove(db *sql.DB, cheatsheetID, attachmentID string) error {
	result, err := db.Exec("DELETE FROM cheatsheet_attachments WHERE id = ? AND cheatsheet_id = ?", attachmentID, cheatsheetID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}
