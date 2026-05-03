// Package launchpad provides URL collection / virtual-desktop tile storage
// for the AuraGo Launchpad feature.
package launchpad

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// LaunchpadLink represents a single bookmark/tile in the launchpad.
type LaunchpadLink struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Description string   `json:"description,omitempty"`
	IconPath    string   `json:"icon_path,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SortOrder   int      `json:"sort_order"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// InitDB initializes the launchpad SQLite database.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open launchpad database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS launchpad_links (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		url TEXT NOT NULL,
		description TEXT,
		icon_path TEXT,
		category TEXT,
		tags TEXT,
		sort_order INTEGER DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_launchpad_category ON launchpad_links(category);
	CREATE INDEX IF NOT EXISTS idx_launchpad_sort ON launchpad_links(sort_order);

	CREATE TABLE IF NOT EXISTS launchpad_icon_cache (
		name TEXT PRIMARY KEY,
		has_svg INTEGER DEFAULT 0,
		has_png INTEGER DEFAULT 0,
		has_webp INTEGER DEFAULT 0,
		has_light INTEGER DEFAULT 0,
		has_dark INTEGER DEFAULT 0,
		cached_at TEXT NOT NULL
	);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create launchpad schema: %w", err)
	}

	if err := dbutil.SetUserVersion(db, 1); err != nil {
		return nil, fmt.Errorf("set launchpad schema version: %w", err)
	}

	return db, nil
}

// validateURL ensures only http/https URLs are stored.
func validateURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("URL is required")
	}
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	return nil
}

// Create adds a new launchpad link and returns its ID.
func Create(db *sql.DB, link LaunchpadLink) (string, error) {
	if strings.TrimSpace(link.Title) == "" {
		return "", fmt.Errorf("title is required")
	}
	if err := validateURL(link.URL); err != nil {
		return "", err
	}

	link.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	link.CreatedAt = now
	link.UpdatedAt = now

	tagsJSON, _ := json.Marshal(link.Tags)

	_, err := db.Exec(
		`INSERT INTO launchpad_links (id, title, url, description, icon_path, category, tags, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		link.ID, link.Title, link.URL, link.Description, link.IconPath, link.Category, string(tagsJSON), link.SortOrder, link.CreatedAt, link.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create launchpad link: %w", err)
	}
	return link.ID, nil
}

// GetByID retrieves a single launchpad link by ID.
func GetByID(db *sql.DB, id string) (*LaunchpadLink, error) {
	row := db.QueryRow(
		`SELECT id, title, url, description, icon_path, category, tags, sort_order, created_at, updated_at
		 FROM launchpad_links WHERE id = ?`, id,
	)
	return scanLink(row)
}

// Update modifies an existing launchpad link.
func Update(db *sql.DB, link LaunchpadLink) error {
	if strings.TrimSpace(link.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if err := validateURL(link.URL); err != nil {
		return err
	}

	link.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	tagsJSON, _ := json.Marshal(link.Tags)

	res, err := db.Exec(
		`UPDATE launchpad_links
		 SET title = ?, url = ?, description = ?, icon_path = ?, category = ?, tags = ?, sort_order = ?, updated_at = ?
		 WHERE id = ?`,
		link.Title, link.URL, link.Description, link.IconPath, link.Category, string(tagsJSON), link.SortOrder, link.UpdatedAt, link.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update launchpad link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("link not found")
	}
	return nil
}

// Delete removes a launchpad link by ID and returns the old icon path (for cleanup).
func Delete(db *sql.DB, id string) (iconPath string, err error) {
	link, err := GetByID(db, id)
	if err != nil {
		return "", err
	}
	_, err = db.Exec(`DELETE FROM launchpad_links WHERE id = ?`, id)
	if err != nil {
		return "", fmt.Errorf("failed to delete launchpad link: %w", err)
	}
	return link.IconPath, nil
}

// List returns all launchpad links, optionally filtered by category.
func List(db *sql.DB, category string) ([]LaunchpadLink, error) {
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = db.Query(
			`SELECT id, title, url, description, icon_path, category, tags, sort_order, created_at, updated_at
			 FROM launchpad_links WHERE category = ? ORDER BY sort_order ASC, title ASC`, category,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, title, url, description, icon_path, category, tags, sort_order, created_at, updated_at
			 FROM launchpad_links ORDER BY sort_order ASC, title ASC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list launchpad links: %w", err)
	}
	defer rows.Close()

	var links []LaunchpadLink
	for rows.Next() {
		link, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		links = append(links, *link)
	}
	return links, rows.Err()
}

// ListCategories returns all distinct categories ordered alphabetically.
func ListCategories(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT category FROM launchpad_links WHERE category IS NOT NULL AND category != '' ORDER BY category ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	return cats, rows.Err()
}

// scanLink scans a single row into a LaunchpadLink.
func scanLink(scanner interface {
	Scan(dest ...interface{}) error
}) (*LaunchpadLink, error) {
	var link LaunchpadLink
	var tagsJSON string
	var descNull, iconNull, catNull sql.NullString

	err := scanner.Scan(
		&link.ID, &link.Title, &link.URL, &descNull, &iconNull, &catNull, &tagsJSON, &link.SortOrder, &link.CreatedAt, &link.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if descNull.Valid {
		link.Description = descNull.String
	}
	if iconNull.Valid {
		link.IconPath = iconNull.String
	}
	if catNull.Valid {
		link.Category = catNull.String
	}
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &link.Tags)
	}
	return &link, nil
}
