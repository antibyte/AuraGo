package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// HomepageHistoryEntry represents a single chronological note/decision/observation
// for a homepage project. It is separate from file revisions and event logs.
type HomepageHistoryEntry struct {
	ID        int64    `json:"id"`
	ProjectID int64    `json:"project_id"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Author    string   `json:"author"`
	EntryType string   `json:"entry_type"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	Source    string   `json:"source"`
}

const hhSelectCols = "id, project_id, created_at, updated_at, author, entry_type, content, tags, source"

func scanHistoryEntry(row interface {
	Scan(dest ...interface{}) error
}) (*HomepageHistoryEntry, error) {
	var e HomepageHistoryEntry
	var tagsStr string
	err := row.Scan(&e.ID, &e.ProjectID, &e.CreatedAt, &e.UpdatedAt, &e.Author,
		&e.EntryType, &e.Content, &tagsStr, &e.Source)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(tagsStr), &e.Tags); err != nil {
		e.Tags = []string{}
	}
	if e.Tags == nil {
		e.Tags = []string{}
	}
	return &e, nil
}

func normalizeHistoryEntryType(entryType string) string {
	switch entryType {
	case "decision", "question", "feedback", "milestone", "observation":
		return entryType
	default:
		return "note"
	}
}

func marshalTags(tags []string) string {
	if tags == nil {
		return "[]"
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

// AddHomepageHistoryEntry inserts a new history entry for a project.
func AddHomepageHistoryEntry(db *sql.DB, projectID int64, entryType, content, source string, tags []string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("homepage registry DB not initialized")
	}
	if projectID <= 0 {
		return 0, fmt.Errorf("project_id is required")
	}
	if content == "" {
		return 0, fmt.Errorf("content is required")
	}
	entryType = normalizeHistoryEntryType(entryType)
	res, err := db.Exec(`
		INSERT INTO homepage_history (project_id, author, entry_type, content, tags, source)
		VALUES (?, 'agent', ?, ?, ?, ?)`,
		projectID, entryType, content, marshalTags(tags), source)
	if err != nil {
		return 0, fmt.Errorf("failed to add homepage history entry: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetHomepageHistoryEntry retrieves a single history entry by ID.
func GetHomepageHistoryEntry(db *sql.DB, id int64) (*HomepageHistoryEntry, error) {
	if db == nil {
		return nil, fmt.Errorf("homepage registry DB not initialized")
	}
	row := db.QueryRow("SELECT "+hhSelectCols+" FROM homepage_history WHERE id = ?", id)
	e, err := scanHistoryEntry(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("history entry %d not found", id)
		}
		return nil, fmt.Errorf("failed to get history entry: %w", err)
	}
	return e, nil
}

// ListHomepageHistoryEntries returns history entries for a project, newest first.
func ListHomepageHistoryEntries(db *sql.DB, projectID int64, entryType string, tags []string, limit, offset int) ([]HomepageHistoryEntry, int, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("homepage registry DB not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	conditions = append(conditions, "project_id = ?")
	args = append(args, projectID)

	if entryType != "" {
		conditions = append(conditions, "entry_type = ?")
		args = append(args, entryType)
	}
	for _, t := range tags {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%\""+t+"\"%")
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_history WHERE "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count history entries: %w", err)
	}

	queryArgs := append(args, limit, offset)
	rows, err := db.Query("SELECT "+hhSelectCols+" FROM homepage_history WHERE "+where+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list history entries: %w", err)
	}
	defer rows.Close()

	var entries []HomepageHistoryEntry
	for rows.Next() {
		e, err := scanHistoryEntry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan history entry: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, total, nil
}

// SearchHomepageHistoryEntries searches history entries by content across all projects or one project.
func SearchHomepageHistoryEntries(db *sql.DB, projectID int64, query, entryType string, tags []string, limit, offset int) ([]HomepageHistoryEntry, int, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("homepage registry DB not initialized")
	}
	if limit <= 0 {
		limit = 20
	}

	var conditions []string
	var args []interface{}

	if projectID > 0 {
		conditions = append(conditions, "project_id = ?")
		args = append(args, projectID)
	}
	if query != "" {
		conditions = append(conditions, "(content LIKE ? OR source LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q)
	}
	if entryType != "" {
		conditions = append(conditions, "entry_type = ?")
		args = append(args, entryType)
	}
	for _, t := range tags {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%\""+t+"\"%")
	}

	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM homepage_history WHERE "+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count history entries: %w", err)
	}

	queryArgs := append(args, limit, offset)
	rows, err := db.Query("SELECT "+hhSelectCols+" FROM homepage_history WHERE "+where+" ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search history entries: %w", err)
	}
	defer rows.Close()

	var entries []HomepageHistoryEntry
	for rows.Next() {
		e, err := scanHistoryEntry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan history entry: %w", err)
		}
		entries = append(entries, *e)
	}
	return entries, total, nil
}

// UpdateHomepageHistoryEntry updates content and tags of an existing entry.
func UpdateHomepageHistoryEntry(db *sql.DB, id int64, content string, tags []string) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("history entry id is required")
	}
	if content == "" {
		return fmt.Errorf("content is required")
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec("UPDATE homepage_history SET content = ?, tags = ?, updated_at = ? WHERE id = ?",
		content, marshalTags(tags), now, id)
	if err != nil {
		return fmt.Errorf("failed to update history entry: %w", err)
	}
	return nil
}

// DeleteHomepageHistoryEntry deletes a single history entry.
func DeleteHomepageHistoryEntry(db *sql.DB, id int64) error {
	if db == nil {
		return fmt.Errorf("homepage registry DB not initialized")
	}
	if id <= 0 {
		return fmt.Errorf("history entry id is required")
	}
	_, err := db.Exec("DELETE FROM homepage_history WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete history entry: %w", err)
	}
	return nil
}

// DispatchHomepageHistory handles homepage_registry history operations.
func DispatchHomepageHistory(db *sql.DB, operation string, id, projectID int64, entryType, content, query, source string, tags []string, limit, offset int) string {
	switch operation {
	case "add_history":
		if projectID <= 0 {
			return `{"status":"error","message":"'id' (project_id) is required for add_history."}`
		}
		if content == "" {
			return `{"status":"error","message":"'content' is required for add_history."}`
		}
		newID, err := AddHomepageHistoryEntry(db, projectID, entryType, content, source, tags)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return fmt.Sprintf(`{"status":"success","id":%d,"message":"History entry added."}`, newID)

	case "get_history":
		if id <= 0 {
			return `{"status":"error","message":"'history_id' is required for get_history."}`
		}
		e, err := GetHomepageHistoryEntry(db, id)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(e)
		return fmt.Sprintf(`{"status":"success","entry":%s}`, string(b))

	case "list_history":
		if projectID <= 0 {
			return `{"status":"error","message":"'id' (project_id) is required for list_history."}`
		}
		entries, total, err := ListHomepageHistoryEntries(db, projectID, entryType, tags, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(entries)
		return fmt.Sprintf(`{"status":"success","total":%d,"entries":%s}`, total, string(b))

	case "search_history":
		entries, total, err := SearchHomepageHistoryEntries(db, projectID, query, entryType, tags, limit, offset)
		if err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		b, _ := json.Marshal(entries)
		return fmt.Sprintf(`{"status":"success","total":%d,"entries":%s}`, total, string(b))

	case "update_history":
		if id <= 0 {
			return `{"status":"error","message":"'history_id' is required for update_history."}`
		}
		if content == "" {
			return `{"status":"error","message":"'content' is required for update_history."}`
		}
		if err := UpdateHomepageHistoryEntry(db, id, content, tags); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"History entry updated."}`

	case "delete_history":
		if id <= 0 {
			return `{"status":"error","message":"'history_id' is required for delete_history."}`
		}
		if err := DeleteHomepageHistoryEntry(db, id); err != nil {
			return fmt.Sprintf(`{"status":"error","message":"%s"}`, err.Error())
		}
		return `{"status":"success","message":"History entry deleted."}`

	default:
		return fmt.Sprintf(`{"status":"error","message":"Unknown history operation '%s'. Use: add_history, list_history, get_history, search_history, update_history, delete_history."}`, operation)
	}
}
