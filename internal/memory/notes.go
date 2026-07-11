package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Note represents a single note or to-do item.
type Note struct {
	ID             int64  `json:"id"`
	Category       string `json:"category"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	Priority       int    `json:"priority"` // 1=low, 2=medium, 3=high
	Done           bool   `json:"done"`
	DueDate        string `json:"due_date,omitempty"` // RFC3339 or YYYY-MM-DD
	Protected      bool   `json:"protected"`
	KeepForever    bool   `json:"keep_forever"`
	Archived       bool   `json:"archived"`
	ArchivedAt     string `json:"archived_at,omitempty"`
	ArchivedReason string `json:"archived_reason,omitempty"`
	LastReviewedAt string `json:"last_reviewed_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// NotesListOptions controls note listing without changing the legacy ListNotes signature.
type NotesListOptions struct {
	Category        string
	DoneFilter      int // -1=all, 0=open, 1=done
	IncludeArchived bool
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
		protected BOOLEAN DEFAULT 0,
		keep_forever BOOLEAN DEFAULT 0,
		archived BOOLEAN DEFAULT 0,
		archived_at DATETIME DEFAULT '',
		archived_reason TEXT DEFAULT '',
		last_reviewed_at DATETIME DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("notes schema: %w", err)
	}
	for _, column := range []struct {
		Name    string
		TypeDef string
	}{
		{Name: "archived", TypeDef: "BOOLEAN DEFAULT 0"},
		{Name: "archived_at", TypeDef: "DATETIME DEFAULT ''"},
		{Name: "archived_reason", TypeDef: "TEXT DEFAULT ''"},
		{Name: "last_reviewed_at", TypeDef: "DATETIME DEFAULT ''"},
		{Name: "protected", TypeDef: "BOOLEAN DEFAULT 0"},
		{Name: "keep_forever", TypeDef: "BOOLEAN DEFAULT 0"},
	} {
		if err := migrateAddColumn(s.db, s.logger, "notes", column.Name, column.TypeDef); err != nil {
			return fmt.Errorf("notes migration %s: %w", column.Name, err)
		}
	}
	if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_archived ON notes(archived, done, priority, updated_at)`); err != nil {
		return fmt.Errorf("notes archived index: %w", err)
	}
	return s.migrateNotesFTS5()
}

// migrateNotesFTS5 creates and populates the FTS5 virtual table for notes.
func (s *SQLiteMemory) migrateNotesFTS5() error {
	if s == nil {
		return nil
	}
	schema := `
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
	title,
	content,
	content='notes',
	content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS notes_fts_insert AFTER INSERT ON notes BEGIN
	INSERT INTO notes_fts(rowid, title, content)
	VALUES (new.rowid, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_fts_delete AFTER DELETE ON notes BEGIN
	INSERT INTO notes_fts(notes_fts, rowid, title, content)
	VALUES ('delete', old.rowid, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_fts_update AFTER UPDATE ON notes BEGIN
	INSERT INTO notes_fts(notes_fts, rowid, title, content)
	VALUES ('delete', old.rowid, old.title, old.content);
	INSERT INTO notes_fts(rowid, title, content)
	VALUES (new.rowid, new.title, new.content);
END;
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("notes fts5 schema: %w", err)
	}
	return s.rebuildFTS5IfNeeded("fts.notes", "notes_fts", "notes")
}

// maxNoteContentLen is the maximum rune count for note content.
const maxNoteContentLen = 100_000

// maxNoteTitleLen is the maximum rune count for a note title.
const maxNoteTitleLen = 500

// AddNote inserts a new note and returns its ID.
func (s *SQLiteMemory) AddNote(category, title, content string, priority int, dueDate string) (int64, error) {
	if title == "" {
		return 0, fmt.Errorf("title is required")
	}
	if utf8.RuneCountInString(title) > maxNoteTitleLen {
		title = string([]rune(title)[:maxNoteTitleLen])
	}
	if utf8.RuneCountInString(content) > maxNoteContentLen {
		content = string([]rune(content)[:maxNoteContentLen])
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
	return s.ListNotesWithOptions(NotesListOptions{
		Category:   category,
		DoneFilter: doneFilter,
	})
}

// ListNotesWithOptions returns notes with explicit archive handling.
func (s *SQLiteMemory) ListNotesWithOptions(opts NotesListOptions) ([]Note, error) {
	var conditions []string
	var args []interface{}

	category := strings.TrimSpace(opts.Category)
	if category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, category)
	}
	if opts.DoneFilter == 0 {
		conditions = append(conditions, "done = 0")
	} else if opts.DoneFilter == 1 {
		conditions = append(conditions, "done = 1")
	}
	if !opts.IncludeArchived {
		conditions = append(conditions, "archived = 0")
	}

	query := "SELECT id, category, title, content, priority, done, due_date, COALESCE(protected, 0), COALESCE(keep_forever, 0), archived, COALESCE(archived_at, ''), COALESCE(archived_reason, ''), COALESCE(last_reviewed_at, ''), created_at, updated_at FROM notes"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY archived ASC, priority DESC, created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Content, &n.Priority, &n.Done, &n.DueDate, &n.Protected, &n.KeepForever, &n.Archived, &n.ArchivedAt, &n.ArchivedReason, &n.LastReviewedAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
// Uses the FTS5 virtual table for fast full-text search.
func (s *SQLiteMemory) SearchNotes(query string, limit int) ([]Note, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	rows, err := s.db.Query(
		`SELECT n.id, n.category, n.title, n.content, n.priority, n.done, n.due_date, COALESCE(n.protected, 0), COALESCE(n.keep_forever, 0), n.archived, COALESCE(n.archived_at, ''), COALESCE(n.archived_reason, ''), COALESCE(n.last_reviewed_at, ''), n.created_at, n.updated_at
		 FROM notes_fts fts
		 JOIN notes n ON n.id = fts.rowid
		 WHERE n.archived = 0 AND notes_fts MATCH ?
		 ORDER BY n.priority DESC, n.created_at DESC
		 LIMIT ?`,
		escapeFTS5(query), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Content, &n.Priority, &n.Done, &n.DueDate, &n.Protected, &n.KeepForever, &n.Archived, &n.ArchivedAt, &n.ArchivedReason, &n.LastReviewedAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update note rows affected: %w", err)
	}
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
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("toggle note rows affected: %w", err)
	}
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
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete note rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("note with id %d not found", id)
	}
	return nil
}

// GetHighPriorityOpenNotes returns up to `limit` open notes with priority=3 (high), ordered by due date then creation.
func (s *SQLiteMemory) GetHighPriorityOpenNotes(limit int) ([]Note, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.Query(
		`SELECT id, category, title, content, priority, done, due_date, COALESCE(protected, 0), COALESCE(keep_forever, 0), archived, COALESCE(archived_at, ''), COALESCE(archived_reason, ''), COALESCE(last_reviewed_at, ''), created_at, updated_at
		 FROM notes WHERE done = 0 AND priority = 3 AND archived = 0
		 ORDER BY CASE WHEN due_date != '' THEN 0 ELSE 1 END, due_date ASC, created_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("get high priority notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Category, &n.Title, &n.Content, &n.Priority, &n.Done, &n.DueDate, &n.Protected, &n.KeepForever, &n.Archived, &n.ArchivedAt, &n.ArchivedReason, &n.LastReviewedAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// DeleteOldDoneNotes removes notes that are marked done and older than daysOld days.
// Returns the number of deleted notes.
func (s *SQLiteMemory) DeleteOldDoneNotes(daysOld int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -daysOld).Format(time.RFC3339)
	res, err := s.db.Exec(`DELETE FROM notes WHERE done = 1 AND updated_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old done notes: %w", err)
	}
	return res.RowsAffected()
}

// ArchiveNote hides a note from normal lists without deleting its history.
func (s *SQLiteMemory) ArchiveNote(id int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		reason = reason[:500]
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE notes
		 SET archived = 1, archived_at = ?, archived_reason = ?, last_reviewed_at = ?, updated_at = ?
		 WHERE id = ?`,
		now, reason, now, now, id,
	)
	if err != nil {
		return fmt.Errorf("archive note: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("archive note rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("note with id %d not found", id)
	}
	return nil
}

// RestoreNote makes an archived note visible again.
func (s *SQLiteMemory) RestoreNote(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`UPDATE notes
		 SET archived = 0, archived_at = '', archived_reason = '', last_reviewed_at = ?, updated_at = ?
		 WHERE id = ?`,
		now, now, id,
	)
	if err != nil {
		return fmt.Errorf("restore note: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("restore note rows affected: %w", err)
	}
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

// GetNotesCount returns the total number of notes.
func (s *SQLiteMemory) GetNotesCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notes WHERE archived = 0`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("notes count: %w", err)
	}
	return count, nil
}
