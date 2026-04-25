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
// The recorder stores it in the internal operational issue table so it can be
// surfaced to the agent without polluting the user's visible planner todos.
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
	CreatedAt   string
	UpdatedAt   string
}

// RecordOperationalIssue creates or updates a deduplicated internal record for
// a background problem. Repeated occurrences update the same record instead of
// flooding the user's visible planner todos.
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

		_, err := db.Exec(`
			UPDATE operational_issues
			SET source=?, context=?, severity=?, title=?, detail=?, reference=?,
				last_seen=?, occurrences=?, status='open', updated_at=?
			WHERE fingerprint=?`,
			normalized.Source, normalized.Context, normalized.Severity, normalized.Title,
			normalized.Detail, normalized.Reference, now, occurrences, now, fingerprint)
		if err != nil {
			return "", fmt.Errorf("update operational issue: %w", err)
		}
		return fingerprint, nil
	}

	_, err = db.Exec(`
		INSERT INTO operational_issues
			(fingerprint, source, context, severity, title, detail, reference,
			 first_seen, last_seen, occurrences, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'open', ?, ?)`,
		fingerprint, normalized.Source, normalized.Context, normalized.Severity,
		normalized.Title, normalized.Detail, normalized.Reference, now, now, now, now)
	if err != nil {
		return "", fmt.Errorf("insert operational issue: %w", err)
	}
	return fingerprint, nil
}

// ListOperationalIssueTodos returns unresolved operational issues as synthetic
// Todo values for prompt-reminder compatibility. These records are not stored
// in the visible todos table.
func ListOperationalIssueTodos(db *sql.DB, limit int) ([]Todo, error) {
	if db == nil {
		return nil, fmt.Errorf("planner database not available")
	}
	query := `
		SELECT fingerprint, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status, created_at, updated_at
		FROM operational_issues
		WHERE status IN ('open', 'in_progress')
		ORDER BY last_seen DESC`
	args := []interface{}{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list operational issues: %w", err)
	}

	result := []Todo{}
	for rows.Next() {
		var record operationalIssueRecord
		if err := rows.Scan(
			&record.Fingerprint, &record.Source, &record.Context, &record.Severity,
			&record.Title, &record.Detail, &record.Reference, &record.FirstSeen,
			&record.LastSeen, &record.Occurrences, &record.Status, &record.CreatedAt,
			&record.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan operational issue: %w", err)
		}
		result = append(result, syntheticOperationalIssueTodo(record))
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close operational issue rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational issues: %w", err)
	}
	return result, nil
}

// CleanupOperationalIssues deletes completed internal operational issues older than maxDoneAge.
func CleanupOperationalIssues(db *sql.DB, maxDoneAge time.Duration) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("planner database not available")
	}
	if maxDoneAge <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-maxDoneAge).Format(time.RFC3339)

	result, err := db.Exec(`
		DELETE FROM operational_issues
		WHERE status = 'done'
			AND COALESCE(NULLIF(updated_at, ''), NULLIF(last_seen, ''), created_at) < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old operational issues: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted operational issues: %w", err)
	}
	return rows, nil
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
		SELECT fingerprint, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status
		FROM operational_issues
		WHERE fingerprint = ?`, fingerprint).Scan(
		&record.Fingerprint, &record.Source, &record.Context, &record.Severity,
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
				(fingerprint, source, context, severity, title, detail, reference,
				 first_seen, last_seen, occurrences, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fingerprint,
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
	return deleteLegacyOperationalIssueTodos(db)
}

type operationalIssueExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func deleteLegacyOperationalIssueTodos(exec operationalIssueExecer) error {
	if _, err := exec.Exec(`
		DELETE FROM todo_items
		WHERE todo_id IN (
			SELECT id FROM todos WHERE description LIKE ?
		)`, "%"+operationalIssueMarker+"%"); err != nil {
		return fmt.Errorf("delete legacy operational issue todo items: %w", err)
	}
	if _, err := exec.Exec(`DELETE FROM todos WHERE description LIKE ?`, "%"+operationalIssueMarker+"%"); err != nil {
		return fmt.Errorf("delete legacy operational issue todos: %w", err)
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

func syntheticOperationalIssueTodo(record operationalIssueRecord) Todo {
	occurredAt := parseOperationalIssueTime(record.LastSeen)
	issue := OperationalIssue{
		Source:      record.Source,
		Context:     record.Context,
		Title:       defaultTrimmed(record.Title, OperationalIssueTitlePrefix+" Background problem detected"),
		Detail:      record.Detail,
		Severity:    record.Severity,
		Reference:   record.Reference,
		Fingerprint: record.Fingerprint,
		OccurredAt:  occurredAt,
	}
	description := buildOperationalIssueDescription(issue, defaultTrimmed(record.FirstSeen, record.CreatedAt), record.Occurrences)
	return Todo{
		ID:          record.Fingerprint,
		Title:       issue.Title,
		Description: description,
		Priority:    operationalIssuePriority(record.Severity),
		Status:      operationalIssueTodoStatus(record.Status),
		RemindDaily: true,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func parseOperationalIssueTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed
	}
	return time.Now().UTC()
}

func operationalIssueTodoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "open", "in_progress", "done":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "open"
	}
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
