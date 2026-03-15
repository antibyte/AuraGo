package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Note represents a single note or to-do item.
type Note struct {
	ID        int64  `json:"id"`
	Category  string `json:"category"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Priority  int    `json:"priority"` // 1=low, 2=medium, 3=high
	Done      bool   `json:"done"`
	DueDate   string `json:"due_date,omitempty"` // RFC3339 or YYYY-MM-DD
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// InitNotesTables creates the notes table if it does not exist.
func (s *SQLiteMemory) InitNotesTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS notes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT DEFAULT 'general',
		title TEXT NOT NULL,
		content TEXT DEFAULT '',
		priority INTEGER DEFAULT 2,
		done BOOLEAN DEFAULT 0,
		due_date TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("notes schema: %w", err)
	}
	return nil
}

// AddNote inserts a new note and returns its ID.
func (s *SQLiteMemory) AddNote(category, title, content string, priority int, dueDate string) (int64, error) {
	if title == "" {
		return 0, fmt.Errorf("title is required")
	}
	if category == "" {
		category = "general"
	}
	if priority < 1 || priority > 3 {
		priority = 2
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO notes (category, title, content, priority, due_date, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		category, title, content, priority, dueDate, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert note: %w", err)
	}
	return res.LastInsertId()
}

// ListNotes returns notes filtered by optional category and/or done status.
// If category is empty, all categories are returned. doneFilter: -1=all, 0=open, 1=done.
func (s *SQLiteMemory) ListNotes(category string, doneFilter int) ([]Note, error) {
	var conditions []string
	var args []interface{}

	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	if doneFilter == 0 {
		conditions = append(conditions, "done = 0")
	} else if doneFilter == 1 {
		conditions = append(conditions, "done = 1")
	}

	query := "SELECT id, category, title, content, priority, done, due_date, created_at, updated_at FROM notes"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Content, &n.Priority, &n.Done, &n.DueDate, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return notes, nil
}

// SearchNotes returns notes whose title or content contain the search term (case-insensitive).
func (s *SQLiteMemory) SearchNotes(query string, limit int) ([]Note, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT id, category, title, content, priority, done, due_date, created_at, updated_at
		 FROM notes
		 WHERE title LIKE ? OR content LIKE ?
		 ORDER BY priority DESC, created_at DESC
		 LIMIT ?`,
		pattern, pattern, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Content, &n.Priority, &n.Done, &n.DueDate, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return notes, nil
}

// UpdateNote updates a note's title, content, priority, due_date, or category by ID.
func (s *SQLiteMemory) UpdateNote(id int64, title, content, category string, priority int, dueDate string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var sets []string
	var args []interface{}

	if title != "" {
		sets = append(sets, "title = ?")
		args = append(args, title)
	}
	if content != "" {
		sets = append(sets, "content = ?")
		args = append(args, content)
	}
	if category != "" {
		sets = append(sets, "category = ?")
		args = append(args, category)
	}
	if priority >= 1 && priority <= 3 {
		sets = append(sets, "priority = ?")
		args = append(args, priority)
	}
	if dueDate != "" {
		sets = append(sets, "due_date = ?")
		args = append(args, dueDate)
	}

	if len(sets) == 0 {
		return fmt.Errorf("nothing to update")
	}

	sets = append(sets, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	query := fmt.Sprintf("UPDATE notes SET %s WHERE id = ?", strings.Join(sets, ", "))
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("note with id %d not found", id)
	}
	return nil
}

// ToggleNoteDone flips the done status of a note by ID.
func (s *SQLiteMemory) ToggleNoteDone(id int64) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE notes SET done = NOT done, updated_at = ? WHERE id = ?`, now, id,
	)
	if err != nil {
		return false, fmt.Errorf("toggle note: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return false, fmt.Errorf("note with id %d not found", id)
	}
	// Read back the new state
	var done bool
	err = s.db.QueryRow(`SELECT done FROM notes WHERE id = ?`, id).Scan(&done)
	if err != nil {
		return false, fmt.Errorf("read toggled state: %w", err)
	}
	return done, nil
}

// DeleteNote removes a note by ID.
func (s *SQLiteMemory) DeleteNote(id int64) error {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("note with id %d not found", id)
	}
	return nil
}

// FormatNotesJSON returns the notes list as a JSON string for tool output.
func FormatNotesJSON(notes []Note) string {
	if notes == nil {
		notes = []Note{}
	}
	b, _ := json.Marshal(notes)
	return string(b)
}
