package tools

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

// CheatSheet represents a reusable workflow/instruction template for the agent.
type CheatSheet struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	Active    bool   `json:"active"`
	CreatedBy string `json:"created_by"` // "user" or "agent"
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

const cheatsheetSchemaVersion = 1

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
	);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create cheatsheets schema: %w", err)
	}

	db.Exec(fmt.Sprintf("PRAGMA user_version = %d", cheatsheetSchemaVersion))
	return db, nil
}

// CheatsheetList returns all cheat sheets, optionally filtered.
func CheatsheetList(db *sql.DB, activeOnly bool) ([]CheatSheet, error) {
	query := "SELECT id, name, content, active, created_by, created_at, updated_at FROM cheatsheets"
	if activeOnly {
		query += " WHERE active = 1"
	}
	query += " ORDER BY name ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sheets []CheatSheet
	for rows.Next() {
		var s CheatSheet
		var active int
		if err := rows.Scan(&s.ID, &s.Name, &s.Content, &active, &s.CreatedBy, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Active = active == 1
		sheets = append(sheets, s)
	}
	return sheets, nil
}

// CheatsheetGet returns a single cheat sheet by ID.
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
		return fmt.Errorf("cheat sheet not found")
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
		parts = append(parts, fmt.Sprintf("[Cheat Sheet: %q]\n%s", s.Name, s.Content))
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(parts, "\n\n")
}
