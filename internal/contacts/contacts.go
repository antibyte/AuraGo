package contacts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// Contact represents a single address book entry.
type Contact struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Mobile       string `json:"mobile,omitempty"`
	Address      string `json:"address,omitempty"`
	Relationship string `json:"relationship,omitempty"`
	Notes        string `json:"notes,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// InitDB initializes the contacts SQLite database.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open contacts database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS contacts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT,
		phone TEXT,
		mobile TEXT,
		address TEXT,
		relationship TEXT,
		notes TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_contacts_name ON contacts(name);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create contacts schema: %w", err)
	}

	return db, nil
}

// Create adds a new contact and returns its ID.
func Create(db *sql.DB, c Contact) (string, error) {
	if c.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	c.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := db.Exec(
		`INSERT INTO contacts (id, name, email, phone, mobile, address, relationship, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Email, c.Phone, c.Mobile, c.Address, c.Relationship, c.Notes, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert contact: %w", err)
	}
	return c.ID, nil
}

// Update modifies an existing contact.
func Update(db *sql.DB, c Contact) error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	res, err := db.Exec(
		`UPDATE contacts SET name=?, email=?, phone=?, mobile=?, address=?, relationship=?, notes=?, updated_at=?
		 WHERE id=?`,
		c.Name, c.Email, c.Phone, c.Mobile, c.Address, c.Relationship, c.Notes, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update contact: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("contact not found: %s", c.ID)
	}
	return nil
}

// Delete removes a contact by ID.
func Delete(db *sql.DB, id string) error {
	res, err := db.Exec("DELETE FROM contacts WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("contact not found: %s", id)
	}
	return nil
}

// GetByID returns a single contact.
func GetByID(db *sql.DB, id string) (*Contact, error) {
	row := db.QueryRow("SELECT id, name, email, phone, mobile, address, relationship, notes, created_at, updated_at FROM contacts WHERE id=?", id)
	c := &Contact{}
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Mobile, &c.Address, &c.Relationship, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("contact not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	return c, nil
}

// List returns all contacts, optionally filtered by a search query.
func List(db *sql.DB, query string) ([]Contact, error) {
	var rows *sql.Rows
	var err error

	if query != "" {
		like := "%" + strings.ToLower(query) + "%"
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, created_at, updated_at
			 FROM contacts
			 WHERE lower(name) LIKE ? OR lower(email) LIKE ? OR lower(phone) LIKE ? OR lower(mobile) LIKE ? OR lower(relationship) LIKE ?
			 ORDER BY name ASC`,
			like, like, like, like, like,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, created_at, updated_at
			 FROM contacts ORDER BY name ASC`,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Mobile, &c.Address, &c.Relationship, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	if contacts == nil {
		contacts = []Contact{}
	}
	return contacts, nil
}

// ToJSON marshals a contact to JSON string (for tool output).
func ToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "%v"}`, err)
	}
	return string(b)
}
