package contacts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("contact not found")

type contactLockEntry struct {
	gate chan struct{}
	refs int
}

var contactLocks = struct {
	sync.Mutex
	entries map[string]*contactLockEntry
}{entries: make(map[string]*contactLockEntry)}

// LockID serializes updates, deletion, and graph synchronization for one
// contact. Callers must re-read the contact after acquiring the lock.
func LockID(id string) func() {
	unlock, _ := LockIDContext(context.Background(), id)
	return unlock
}

// LockIDContext makes queued contact operations cancellable during shutdown.
func LockIDContext(ctx context.Context, id string) (func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	contactLocks.Lock()
	entry := contactLocks.entries[id]
	if entry == nil {
		entry = &contactLockEntry{gate: make(chan struct{}, 1)}
		contactLocks.entries[id] = entry
	}
	entry.refs++
	contactLocks.Unlock()
	dropReference := func() {
		contactLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(contactLocks.entries, id)
		}
		contactLocks.Unlock()
	}
	select {
	case entry.gate <- struct{}{}:
	case <-ctx.Done():
		dropReference()
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-entry.gate; dropReference() }) }, nil
}

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
	Birthday     string `json:"birthday,omitempty"`
	Reminder     string `json:"reminder,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// InitDB initializes the contacts SQLite database.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open contacts database: %w", err)
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

	for _, migration := range []string{
		"ALTER TABLE contacts ADD COLUMN birthday TEXT",
		"ALTER TABLE contacts ADD COLUMN reminder TEXT",
	} {
		if _, err := db.Exec(migration); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return nil, fmt.Errorf("migration %q: %w", migration, err)
		}
	}

	// Set schema version
	if err := dbutil.SetUserVersion(db, 2); err != nil {
		return nil, fmt.Errorf("set contacts schema version: %w", err)
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
		`INSERT INTO contacts (id, name, email, phone, mobile, address, relationship, notes, birthday, reminder, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Email, c.Phone, c.Mobile, c.Address, c.Relationship, c.Notes, c.Birthday, c.Reminder, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert contact: %w", err)
	}
	return c.ID, nil
}

// Update modifies an existing contact.
func Update(db *sql.DB, c Contact) error {
	return UpdateContext(context.Background(), db, c)
}

func UpdateContext(ctx context.Context, db *sql.DB, c Contact) error {
	unlock, err := LockIDContext(ctx, c.ID)
	if err != nil {
		return err
	}
	defer unlock()
	return updateLockedContext(ctx, db, c)
}

// UpdateLocked modifies an existing contact while the caller holds LockID.
func UpdateLocked(db *sql.DB, c Contact) error {
	return updateLockedContext(context.Background(), db, c)
}

func updateLockedContext(ctx context.Context, db *sql.DB, c Contact) error {
	if c.ID == "" {
		return fmt.Errorf("id is required")
	}
	c.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	res, err := db.ExecContext(ctx,
		`UPDATE contacts SET name=?, email=?, phone=?, mobile=?, address=?, relationship=?, notes=?, birthday=?, reminder=?, updated_at=?
		 WHERE id=?`,
		c.Name, c.Email, c.Phone, c.Mobile, c.Address, c.Relationship, c.Notes, c.Birthday, c.Reminder, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update contact: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect updated contact: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, c.ID)
	}
	return nil
}

// Delete removes a contact by ID.
func Delete(db *sql.DB, id string) error {
	unlock := LockID(id)
	defer unlock()
	return DeleteLocked(db, id)
}

// DeleteLocked removes a contact while the caller holds LockID.
func DeleteLocked(db *sql.DB, id string) error {
	return DeleteLockedContext(context.Background(), db, id)
}

func DeleteLockedContext(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, "DELETE FROM contacts WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to inspect deleted contact: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return nil
}

// GetByID returns a single contact.
func GetByID(db *sql.DB, id string) (*Contact, error) {
	return GetByIDContext(context.Background(), db, id)
}

func GetByIDContext(ctx context.Context, db *sql.DB, id string) (*Contact, error) {
	row := db.QueryRowContext(ctx, "SELECT id, name, email, phone, mobile, address, relationship, notes, COALESCE(birthday,''), COALESCE(reminder,''), created_at, updated_at FROM contacts WHERE id=?", id)
	c := &Contact{}
	if err := row.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Mobile, &c.Address, &c.Relationship, &c.Notes, &c.Birthday, &c.Reminder, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
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
		escapedQuery := dbutil.EscapeLike(strings.ToLower(query))
		like := "%" + escapedQuery + "%"
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, COALESCE(birthday,''), COALESCE(reminder,''), created_at, updated_at
			 FROM contacts
			 WHERE lower(name) LIKE ? ESCAPE '\' OR lower(email) LIKE ? ESCAPE '\' OR lower(phone) LIKE ? ESCAPE '\' OR lower(mobile) LIKE ? ESCAPE '\' OR lower(relationship) LIKE ? ESCAPE '\'
			 ORDER BY name ASC`,
			like, like, like, like, like,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, COALESCE(birthday,''), COALESCE(reminder,''), created_at, updated_at
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
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Mobile, &c.Address, &c.Relationship, &c.Notes, &c.Birthday, &c.Reminder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
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

// Close closes the database connection.
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// UpcomingBirthdays returns contacts with birthdays within the next N days.
func UpcomingBirthdays(db *sql.DB, days int) ([]Contact, error) {
	if days <= 0 {
		days = 30
	}
	today := time.Now()
	todayMD := today.Format("01-02")
	endMD := today.AddDate(0, 0, days).Format("01-02")

	var rows *sql.Rows
	var err error
	if todayMD <= endMD {
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, COALESCE(birthday,''), COALESCE(reminder,''), created_at, updated_at
			 FROM contacts
			 WHERE birthday != '' AND substr(birthday,6) BETWEEN ? AND ?
			 ORDER BY substr(birthday,6) ASC`,
			todayMD, endMD,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, name, email, phone, mobile, address, relationship, notes, COALESCE(birthday,''), COALESCE(reminder,''), created_at, updated_at
			 FROM contacts
			 WHERE birthday != '' AND (substr(birthday,6) >= ? OR substr(birthday,6) <= ?)
			 ORDER BY substr(birthday,6) ASC`,
			todayMD, endMD,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query upcoming birthdays: %w", err)
	}
	defer rows.Close()

	var result []Contact
	for rows.Next() {
		var c Contact
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Mobile, &c.Address, &c.Relationship, &c.Notes, &c.Birthday, &c.Reminder, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan upcoming birthday: %w", err)
		}
		result = append(result, c)
	}
	if result == nil {
		result = []Contact{}
	}
	return result, nil
}
