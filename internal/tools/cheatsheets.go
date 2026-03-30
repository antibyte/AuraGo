package tools

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

// CheatSheet represents a reusable workflow/instruction template for the agent.
type CheatSheet struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Content         string                 `json:"content"`
	Active          bool                   `json:"active"`
	CreatedBy       string                 `json:"created_by"` // "user" or "agent"
	CreatedAt       string                 `json:"created_at"`
	UpdatedAt       string                 `json:"updated_at"`
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

const cheatsheetSchemaVersion = 2

// MaxAttachmentChars is the total character limit across all attachments of a single cheat sheet.
const MaxAttachmentChars = 25000

// AllowedAttachmentExtensions lists the file extensions permitted for cheat sheet attachments.
var AllowedAttachmentExtensions = []string{".txt", ".md"}

// InitCheatsheetDB initializes the cheat sheets SQLite database.
func InitCheatsheetDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cheatsheet database: %w", err)
	}

	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS cheatsheets (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		content    TEXT NOT NULL DEFAULT '',
		active     INTEGER NOT NULL DEFAULT 1,
		created_by TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
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

	// Enable FK enforcement
	db.Exec("PRAGMA foreign_keys = ON")

	// Migration: add attachments table for existing v1 databases
	var version int
	db.QueryRow("PRAGMA user_version").Scan(&version)
	if version < 2 {
		db.Exec(`CREATE TABLE IF NOT EXISTS cheatsheet_attachments (
			id            TEXT PRIMARY KEY,
			cheatsheet_id TEXT NOT NULL,
			filename      TEXT NOT NULL,
			source        TEXT NOT NULL DEFAULT 'upload',
			content       TEXT NOT NULL DEFAULT '',
			char_count    INTEGER NOT NULL DEFAULT 0,
			created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (cheatsheet_id) REFERENCES cheatsheets(id) ON DELETE CASCADE
		)`)
	}

	db.Exec(fmt.Sprintf("PRAGMA user_version = %d", cheatsheetSchemaVersion))
	return db, nil
}

// CheatsheetList returns all cheat sheets, optionally filtered.
func CheatsheetList(db *sql.DB, activeOnly bool) ([]CheatSheet, error) {
	query := `SELECT c.id, c.name, c.content, c.active, c.created_by, c.created_at, c.updated_at,
		COALESCE((SELECT COUNT(*) FROM cheatsheet_attachments a WHERE a.cheatsheet_id = c.id), 0)
		FROM cheatsheets c`
	if activeOnly {
		query += " WHERE c.active = 1"
	}
	query += " ORDER BY c.name ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sheets []CheatSheet
	for rows.Next() {
		var s CheatSheet
		var active int
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt, &s.AttachmentCount); err != nil {
			return nil, err
		}
		s.Active = active == 1
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
	var active int
	err := db.QueryRow(
		"SELECT id, name, content, active, created_by, created_at, updated_at FROM cheatsheets WHERE id = ?", id,
	).Scan(&s.ID, &s.Name, &s.Content, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Active = active == 1

	attachments, _ := CheatsheetAttachmentList(db, id)
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
	var active int
	err := db.QueryRow(
		"SELECT id, name, content, active, created_by, created_at, updated_at FROM cheatsheets WHERE LOWER(name) = LOWER(?)", name,
	).Scan(&s.ID, &s.Name, &s.Content, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	s.Active = active == 1

	attachments, _ := CheatsheetAttachmentList(db, s.ID)
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
	if createdBy != "user" && createdBy != "agent" {
		createdBy = "user"
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		"INSERT INTO cheatsheets (id, name, content, active, created_by, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?, ?)",
		id, name, content, createdBy, now, now,
	)
	if err != nil {
		return nil, err
	}
	return CheatsheetGet(db, id)
}

// CheatsheetUpdate updates an existing cheat sheet.
func CheatsheetUpdate(db *sql.DB, id string, name, content *string, active *bool) (*CheatSheet, error) {
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
		existing.Content = *content
	}
	if active != nil {
		existing.Active = *active
	}

	now := time.Now().UTC().Format(time.RFC3339)
	activeInt := 0
	if existing.Active {
		activeInt = 1
	}

	_, err = db.Exec(
		"UPDATE cheatsheets SET name = ?, content = ?, active = ?, updated_at = ? WHERE id = ?",
		existing.Name, existing.Content, activeInt, now, id,
	)
	if err != nil {
		return nil, err
	}
	return CheatsheetGet(db, id)
}

// CheatsheetDelete deletes a cheat sheet by ID.
func CheatsheetDelete(db *sql.DB, id string) error {
	result, err := db.Exec("DELETE FROM cheatsheets WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CheatsheetCount returns total, active, and agent-created counts.
func CheatsheetCount(db *sql.DB) (total, active, agentCreated int) {
	db.QueryRow("SELECT COUNT(*) FROM cheatsheets").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM cheatsheets WHERE active = 1").Scan(&active)
	db.QueryRow("SELECT COUNT(*) FROM cheatsheets WHERE created_by = 'agent'").Scan(&agentCreated)
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
		part := fmt.Sprintf("[Cheat Sheet: %q]\n%s", s.Name, s.Content)

		// Append attachments if any
		attachments, _ := CheatsheetAttachmentList(db, id)
		for _, a := range attachments {
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
func cheatsheetAttachmentTotalChars(db *sql.DB, cheatsheetID string) int {
	var total int
	db.QueryRow("SELECT COALESCE(SUM(char_count), 0) FROM cheatsheet_attachments WHERE cheatsheet_id = ?", cheatsheetID).Scan(&total)
	return total
}

// cheatsheetAttachmentCount returns the number of attachments for a cheat sheet.
func cheatsheetAttachmentCount(db *sql.DB, cheatsheetID string) int {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM cheatsheet_attachments WHERE cheatsheet_id = ?", cheatsheetID).Scan(&count)
	return count
}

// CheatsheetAttachmentAdd adds an attachment to a cheat sheet.
// It validates the file extension and enforces the total character limit.
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
	currentTotal := cheatsheetAttachmentTotalChars(db, cheatsheetID)
	if currentTotal+charCount > MaxAttachmentChars {
		return nil, fmt.Errorf("attachment would exceed the %d character limit (current: %d, new: %d)", MaxAttachmentChars, currentTotal, charCount)
	}

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		"INSERT INTO cheatsheet_attachments (id, cheatsheet_id, filename, source, content, char_count, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, cheatsheetID, filename, source, content, charCount, now,
	)
	if err != nil {
		return nil, err
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

// CheatsheetAttachmentRemove removes an attachment by ID.
func CheatsheetAttachmentRemove(db *sql.DB, cheatsheetID, attachmentID string) error {
	result, err := db.Exec("DELETE FROM cheatsheet_attachments WHERE id = ? AND cheatsheet_id = ?", attachmentID, cheatsheetID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}
