package planner

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	OperationalIssueTitlePrefix = "System issue:"
	operationalIssueMarker      = "[aurago:operational_issue]"
)

// OperationalIssue describes a problem detected by a background agent context.
// The recorder stores a visible planner todo plus a dedicated indexed record.
type OperationalIssue struct {
	Source      string
	Context     string
	Title       string
	Detail      string
	Severity    string
	Reference   string
	Fingerprint string
	OccurredAt  time.Time
}

type operationalIssueRecord struct {
	Fingerprint string
	TodoID      string
	Source      string
	Context     string
	Severity    string
	Title       string
	Detail      string
	Reference   string
	FirstSeen   string
	LastSeen    string
	Occurrences int
	Status      string
}

// RecordOperationalIssue creates or updates a deduplicated open todo for a
// background problem. Repeated occurrences update the same todo instead of
// flooding the planner.
func RecordOperationalIssue(db *sql.DB, issue OperationalIssue) (string, error) {
	if db == nil {
		return "", fmt.Errorf("planner database not available")
	}
	normalized := normalizeOperationalIssue(issue)
	fingerprint := normalized.Fingerprint
	now := normalized.OccurredAt.UTC().Format(time.RFC3339)

	record, found, err := getOperationalIssueRecord(db, fingerprint)
	if err != nil {
		return "", err
	}
	if found {
		firstSeen := record.FirstSeen
		if firstSeen == "" {
			firstSeen = now
		}
		occurrences := record.Occurrences + 1
		if occurrences < 1 {
			occurrences = 1
		}
		description := buildOperationalIssueDescription(normalized, firstSeen, occurrences)

		todo, todoErr := GetTodo(db, record.TodoID)
		if todoErr != nil {
			record.TodoID, todoErr = CreateTodo(db, Todo{
				Title:       normalized.Title,
				Description: description,
				Priority:    operationalIssuePriority(normalized.Severity),
				Status:      "open",
				RemindDaily: true,
			})
			if todoErr != nil {
				return "", todoErr
			}
		} else {
			todo.Title = normalized.Title
			todo.Description = description
			todo.Priority = operationalIssuePriority(normalized.Severity)
			todo.Status = "open"
			todo.RemindDaily = true
			if err := UpdateTodo(db, *todo); err != nil {
				return "", err
			}
		}

		_, err := db.Exec(`
			UPDATE operational_issues
			SET todo_id=?, source=?, context=?, severity=?, title=?, detail=?, reference=?,
				last_seen=?, occurrences=?, status='open', updated_at=?
			WHERE fingerprint=?`,
			record.TodoID, normalized.Source, normalized.Context, normalized.Severity, normalized.Title,
			normalized.Detail, normalized.Reference, now, occurrences, now, fingerprint)
		if err != nil {
			return "", fmt.Errorf("update operational issue: %w", err)
		}
		return record.TodoID, nil
	}

	id, err := CreateTodo(db, Todo{
		Title:       normalized.Title,
		Description: buildOperationalIssueDescription(normalized, now, 1),
		Priority:    operationalIssuePriority(normalized.Severity),
		Status:      "open",
		RemindDaily: true,
	})
	if err != nil {
		return "", err
	}

	_, err = db.Exec(`
		INSERT INTO operational_issues
			(fingerprint, todo_id, source, context, severity, title, detail, reference,
			 first_seen, last_seen, occurrences, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'open', ?, ?)`,
		fingerprint, id, normalized.Source, normalized.Context, normalized.Severity,
		normalized.Title, normalized.Detail, normalized.Reference, now, now, now, now)
	if err != nil {
		return "", fmt.Errorf("insert operational issue: %w", err)
	}
	return id, nil
}

// ListOperationalIssueTodos returns unresolved operational issue todos.
func ListOperationalIssueTodos(db *sql.DB, limit int) ([]Todo, error) {
	if db == nil {
		return nil, fmt.Errorf("planner database not available")
	}
	query := `
		SELECT oi.todo_id
		FROM operational_issues oi
		JOIN todos t ON t.id = oi.todo_id
		WHERE t.status IN ('open', 'in_progress')
		ORDER BY oi.last_seen DESC`
	args := []interface{}{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list operational issue ids: %w", err)
	}

	todoIDs := []string{}
	for rows.Next() {
		var todoID string
		if err := rows.Scan(&todoID); err != nil {
			return nil, fmt.Errorf("scan operational issue id: %w", err)
		}
		todoIDs = append(todoIDs, todoID)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close operational issue id rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational issue ids: %w", err)
	}

	result := make([]Todo, 0, len(todoIDs))
	for _, todoID := range todoIDs {
		todo, err := GetTodo(db, todoID)
		if err != nil {
			return nil, err
		}
		result = append(result, *todo)
	}
	return result, nil
}

// CleanupOperationalIssues deletes completed operational issue todos older than maxDoneAge.
func CleanupOperationalIssues(db *sql.DB, maxDoneAge time.Duration) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("planner database not available")
	}
	if maxDoneAge <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-maxDoneAge).Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin operational issue cleanup: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT oi.todo_id
		FROM operational_issues oi
		JOIN todos t ON t.id = oi.todo_id
		WHERE t.status = 'done'
			AND COALESCE(NULLIF(t.completed_at, ''), NULLIF(t.updated_at, ''), oi.last_seen) < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("query old operational issues: %w", err)
	}
	var todoIDs []string
	for rows.Next() {
		var todoID string
		if err := rows.Scan(&todoID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan old operational issue: %w", err)
		}
		todoIDs = append(todoIDs, todoID)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close operational issue cleanup rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate old operational issues: %w", err)
	}

	for _, todoID := range todoIDs {
		if _, err := tx.Exec(`DELETE FROM todo_items WHERE todo_id = ?`, todoID); err != nil {
			return 0, fmt.Errorf("delete operational issue todo items: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM todos WHERE id = ?`, todoID); err != nil {
			return 0, fmt.Errorf("delete operational issue todo: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM operational_issues WHERE todo_id NOT IN (SELECT id FROM todos)`); err != nil {
		return 0, fmt.Errorf("delete orphaned operational issues: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit operational issue cleanup: %w", err)
	}
	return int64(len(todoIDs)), nil
}

// BuildOperationalIssueReminderText formats unresolved background issues for
// prompt injection at the user's next direct contact.
func BuildOperationalIssueReminderText(todos []Todo) string {
	if len(todos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Unresolved operational issues detected in background contexts:\n")
	for _, todo := range todos {
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(todo.Title))
		if lastSeen := operationalIssueField(todo.Description, "Last seen"); lastSeen != "" {
			b.WriteString(" (last seen: ")
			b.WriteString(lastSeen)
			b.WriteString(")")
		}
		if detail := operationalIssueField(todo.Description, "Latest detail"); detail != "" {
			b.WriteString("\n  Detail: ")
			b.WriteString(detail)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func getOperationalIssueRecord(db *sql.DB, fingerprint string) (operationalIssueRecord, bool, error) {
	var record operationalIssueRecord
	err := db.QueryRow(`
		SELECT fingerprint, todo_id, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status
		FROM operational_issues
		WHERE fingerprint = ?`, fingerprint).Scan(
		&record.Fingerprint, &record.TodoID, &record.Source, &record.Context, &record.Severity,
		&record.Title, &record.Detail, &record.Reference, &record.FirstSeen, &record.LastSeen,
		&record.Occurrences, &record.Status,
	)
	if err == sql.ErrNoRows {
		return operationalIssueRecord{}, false, nil
	}
	if err != nil {
		return operationalIssueRecord{}, false, fmt.Errorf("get operational issue: %w", err)
	}
	return record, true, nil
}

func backfillOperationalIssuesFromTodos(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, title, description, status, created_at, updated_at
		FROM todos
		WHERE description LIKE ?`, "%"+operationalIssueMarker+"%")
	if err != nil {
		return fmt.Errorf("query legacy operational issue todos: %w", err)
	}

	var todos []Todo
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.Title, &todo.Description, &todo.Status, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			return fmt.Errorf("scan legacy operational issue todo: %w", err)
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy operational issue todos: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close legacy operational issue todos: %w", err)
	}

	for _, todo := range todos {
		fingerprint := operationalIssueField(todo.Description, "Fingerprint")
		if fingerprint == "" {
			fingerprint = operationalIssueFingerprint(OperationalIssue{
				Source:    operationalIssueField(todo.Description, "Source"),
				Context:   operationalIssueField(todo.Description, "Context"),
				Title:     strings.TrimSpace(strings.TrimPrefix(todo.Title, OperationalIssueTitlePrefix)),
				Reference: operationalIssueField(todo.Description, "Reference"),
			})
		}
		firstSeen := operationalIssueField(todo.Description, "First seen")
		if firstSeen == "" {
			firstSeen = defaultTrimmed(todo.CreatedAt, time.Now().UTC().Format(time.RFC3339))
		}
		lastSeen := operationalIssueField(todo.Description, "Last seen")
		if lastSeen == "" {
			lastSeen = defaultTrimmed(todo.UpdatedAt, firstSeen)
		}
		occurrences := parseOperationalIssueOccurrences(todo.Description)
		if occurrences < 1 {
			occurrences = 1
		}
		detail := operationalIssueDetails(todo.Description)
		if detail == "" {
			detail = operationalIssueField(todo.Description, "Latest detail")
		}

		_, err := db.Exec(`
			INSERT OR IGNORE INTO operational_issues
				(fingerprint, todo_id, source, context, severity, title, detail, reference,
				 first_seen, last_seen, occurrences, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fingerprint, todo.ID,
			operationalIssueField(todo.Description, "Source"),
			operationalIssueField(todo.Description, "Context"),
			defaultTrimmed(operationalIssueField(todo.Description, "Severity"), "warning"),
			todo.Title,
			detail,
			operationalIssueField(todo.Description, "Reference"),
			firstSeen, lastSeen, occurrences, todo.Status,
			defaultTrimmed(todo.CreatedAt, firstSeen), defaultTrimmed(todo.UpdatedAt, lastSeen),
		)
		if err != nil {
			return fmt.Errorf("backfill operational issue: %w", err)
		}
	}
	return nil
}

func normalizeOperationalIssue(issue OperationalIssue) OperationalIssue {
	if issue.OccurredAt.IsZero() {
		issue.OccurredAt = time.Now()
	}
	issue.Source = defaultTrimmed(issue.Source, "background")
	issue.Context = strings.TrimSpace(issue.Context)
	issue.Severity = defaultTrimmed(issue.Severity, "warning")
	issue.Reference = strings.TrimSpace(issue.Reference)
	issue.Detail = truncateOperationalIssueText(strings.TrimSpace(issue.Detail), 1400)
	if issue.Detail == "" {
		issue.Detail = "No additional detail was captured."
	}
	baseTitle := defaultTrimmed(issue.Title, "Background problem detected")
	baseTitle = strings.TrimSpace(strings.TrimPrefix(baseTitle, OperationalIssueTitlePrefix))
	if issue.Fingerprint == "" {
		issue.Fingerprint = operationalIssueFingerprint(OperationalIssue{
			Source:    issue.Source,
			Context:   issue.Context,
			Title:     baseTitle,
			Reference: issue.Reference,
		})
	} else {
		issue.Fingerprint = strings.TrimSpace(issue.Fingerprint)
	}
	issue.Title = OperationalIssueTitlePrefix + " " + baseTitle
	return issue
}

func operationalIssueFingerprint(issue OperationalIssue) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(issue.Source)),
		strings.ToLower(strings.TrimSpace(issue.Context)),
		strings.ToLower(strings.TrimSpace(issue.Title)),
		strings.ToLower(strings.TrimSpace(issue.Reference)),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func buildOperationalIssueDescription(issue OperationalIssue, firstSeen string, occurrences int) string {
	if occurrences < 1 {
		occurrences = 1
	}
	lines := []string{
		operationalIssueMarker,
		"Fingerprint: " + issue.Fingerprint,
		"Source: " + issue.Source,
		"Context: " + emptyDash(issue.Context),
		"Severity: " + issue.Severity,
		"Reference: " + emptyDash(issue.Reference),
		"First seen: " + emptyDash(firstSeen),
		"Last seen: " + issue.OccurredAt.UTC().Format(time.RFC3339),
		fmt.Sprintf("Occurrences: %d", occurrences),
		"Latest detail: " + singleLineOperationalDetail(issue.Detail),
		"",
		"Details:",
		issue.Detail,
	}
	return strings.Join(lines, "\n")
}

func operationalIssuePriority(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error", "high":
		return "high"
	case "info", "low":
		return "low"
	default:
		return "medium"
	}
}

func operationalIssueField(description, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func operationalIssueDetails(description string) string {
	parts := strings.SplitN(description, "\nDetails:\n", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseOperationalIssueOccurrences(description string) int {
	raw := operationalIssueField(description, "Occurrences")
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func defaultTrimmed(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func singleLineOperationalDetail(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	return truncateOperationalIssueText(value, 240)
}

func truncateOperationalIssueText(value string, max int) string {
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	if max <= 3 {
		return strings.Repeat(".", max)
	}
	return strings.TrimSpace(string(runes[:max-3])) + "..."
}
