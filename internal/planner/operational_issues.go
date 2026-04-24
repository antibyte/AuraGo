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
// The recorder stores these as regular planner todos so they survive sessions
// and can be surfaced on the user's next contact.
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

// RecordOperationalIssue creates or updates a deduplicated open todo for a
// background problem. Repeated occurrences update the same todo instead of
// flooding the planner.
func RecordOperationalIssue(db *sql.DB, issue OperationalIssue) (string, error) {
	if db == nil {
		return "", fmt.Errorf("planner database not available")
	}
	normalized := normalizeOperationalIssue(issue)
	fingerprint := normalized.Fingerprint

	existing, err := ListOperationalIssueTodos(db, 100)
	if err != nil {
		return "", err
	}
	for _, todo := range existing {
		if operationalIssueField(todo.Description, "Fingerprint") != fingerprint {
			continue
		}
		firstSeen := operationalIssueField(todo.Description, "First seen")
		if firstSeen == "" {
			firstSeen = normalized.OccurredAt.UTC().Format(time.RFC3339)
		}
		occurrences := parseOperationalIssueOccurrences(todo.Description) + 1
		todo.Title = normalized.Title
		todo.Description = buildOperationalIssueDescription(normalized, firstSeen, occurrences)
		todo.Priority = operationalIssuePriority(normalized.Severity)
		todo.Status = "open"
		todo.RemindDaily = true
		if err := UpdateTodo(db, todo); err != nil {
			return "", err
		}
		return todo.ID, nil
	}

	id, err := CreateTodo(db, Todo{
		Title:       normalized.Title,
		Description: buildOperationalIssueDescription(normalized, normalized.OccurredAt.UTC().Format(time.RFC3339), 1),
		Priority:    operationalIssuePriority(normalized.Severity),
		Status:      "open",
		RemindDaily: true,
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// ListOperationalIssueTodos returns unresolved operational issue todos.
func ListOperationalIssueTodos(db *sql.DB, limit int) ([]Todo, error) {
	if db == nil {
		return nil, nil
	}
	todos, err := ListTodos(db, operationalIssueMarker, "")
	if err != nil {
		return nil, err
	}
	result := make([]Todo, 0, len(todos))
	for _, todo := range todos {
		if todo.Status == "done" {
			continue
		}
		if !strings.Contains(todo.Description, operationalIssueMarker) {
			continue
		}
		result = append(result, todo)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
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
	baseTitle = strings.TrimPrefix(baseTitle, OperationalIssueTitlePrefix)
	issue.Title = OperationalIssueTitlePrefix + " " + strings.TrimSpace(baseTitle)
	issue.Fingerprint = defaultTrimmed(issue.Fingerprint, operationalIssueFingerprint(issue))
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
	if max <= 0 || len([]rune(value)) <= max {
		return value
	}
	if max <= 3 {
		return string([]rune(value)[:max])
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:max-3])) + "..."
}
