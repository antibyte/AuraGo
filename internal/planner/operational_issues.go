package planner

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	OperationalIssueTitlePrefix     = "System issue:"
	operationalIssueMarker          = "[aurago:operational_issue]"
	operationalIssueReminderMetaKey = "operational_issue_reminder_last_seen"
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
	Fingerprint      string
	Source           string
	Context          string
	Severity         string
	Title            string
	Detail           string
	Reference        string
	FirstSeen        string
	LastSeen         string
	Occurrences      int
	Status           string
	DetailHash       string
	Revision         int
	NotifiedRevision int
	LastNotifiedAt   string
	ResolvedAt       string
	Resolution       string
	CreatedAt        string
	UpdatedAt        string
}

// OperationalIssueNotice is the safe, versioned representation used by the
// supervisor for deterministic user notifications.
type OperationalIssueNotice struct {
	Fingerprint string
	Revision    int
	Severity    string
	Title       string
	Detail      string
	LastSeen    string
	Occurrences int
}

// OperationalIssueNoticeRef identifies the exact revision that was delivered.
// A newer revision is never marked as notified by an older delivery attempt.
type OperationalIssueNoticeRef struct {
	Fingerprint string
	Revision    int
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
	contentHash := operationalIssueContentHash(normalized)

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

		revision := record.Revision
		if revision < 1 {
			revision = 1
		}
		storedHash := record.DetailHash
		if storedHash == "" {
			storedHash = operationalIssueContentHash(OperationalIssue{
				Source: record.Source, Context: record.Context, Severity: record.Severity,
				Title: record.Title, Detail: record.Detail, Reference: record.Reference,
			})
		}
		if storedHash != contentHash || strings.EqualFold(record.Status, "done") {
			revision++
		}

		_, err := db.Exec(`
			UPDATE operational_issues
			SET source=?, context=?, severity=?, title=?, detail=?, reference=?,
				last_seen=?, occurrences=?, status='open', detail_hash=?, revision=?,
				resolved_at='', resolution='', updated_at=?
			WHERE fingerprint=?`,
			normalized.Source, normalized.Context, normalized.Severity, normalized.Title,
			normalized.Detail, normalized.Reference, now, occurrences, contentHash, revision, now, fingerprint)
		if err != nil {
			return "", fmt.Errorf("update operational issue: %w", err)
		}
		return fingerprint, nil
	}

	_, err = db.Exec(`
		INSERT INTO operational_issues
			(fingerprint, source, context, severity, title, detail, reference,
			 first_seen, last_seen, occurrences, status, detail_hash, revision,
			 notified_revision, last_notified_at, resolved_at, resolution, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 'open', ?, 1, 0, '', '', '', ?, ?)`,
		fingerprint, normalized.Source, normalized.Context, normalized.Severity,
		normalized.Title, normalized.Detail, normalized.Reference, now, now, contentHash, now, now)
	if err != nil {
		return "", fmt.Errorf("insert operational issue: %w", err)
	}
	return fingerprint, nil
}

// ResolveOperationalIssue closes an issue after a verified success. The safe
// resolution text is retained for diagnostics and cleared automatically if the
// same fingerprint reopens later.
func ResolveOperationalIssue(db *sql.DB, fingerprint, resolution string, resolvedAt time.Time) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("planner database not available")
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return false, nil
	}
	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	now := resolvedAt.UTC().Format(time.RFC3339)
	resolution = sanitizeOperationalIssueText(resolution, 400)
	if resolution == "" {
		resolution = "The same operation completed successfully."
	}
	result, err := db.Exec(`
		UPDATE operational_issues
		SET status='done', resolved_at=?, resolution=?, updated_at=?
		WHERE fingerprint=? AND status IN ('open', 'in_progress')`,
		now, resolution, now, fingerprint)
	if err != nil {
		return false, fmt.Errorf("resolve operational issue: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count resolved operational issues: %w", err)
	}
	return rows > 0, nil
}

// ListPendingOperationalIssueNotices returns at most limit supervisor-owned
// notices, ordered by severity, changed revision, and recency. High-severity
// open issues become due again after 24 hours.
func ListPendingOperationalIssueNotices(db *sql.DB, now time.Time, limit int) ([]OperationalIssueNotice, error) {
	if db == nil {
		return nil, fmt.Errorf("planner database not available")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if limit <= 0 || limit > 2 {
		limit = 2
	}
	rows, err := db.Query(`
		SELECT fingerprint, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status, detail_hash, revision,
			notified_revision, last_notified_at, resolved_at, resolution, created_at, updated_at
		FROM operational_issues
		WHERE status IN ('open', 'in_progress')`)
	if err != nil {
		return nil, fmt.Errorf("list pending operational issue notices: %w", err)
	}
	var records []operationalIssueRecord
	for rows.Next() {
		var record operationalIssueRecord
		if err := scanOperationalIssueRecord(rows, &record); err != nil {
			rows.Close()
			return nil, err
		}
		if !operationalIssueVisibleToUser(record) || !operationalIssueNoticeDue(record, now) {
			continue
		}
		records = append(records, record)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close operational issue notice rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational issue notice rows: %w", err)
	}
	sort.SliceStable(records, func(i, j int) bool {
		left, right := records[i], records[j]
		if operationalIssueSeverityRank(left.Severity) != operationalIssueSeverityRank(right.Severity) {
			return operationalIssueSeverityRank(left.Severity) < operationalIssueSeverityRank(right.Severity)
		}
		leftChanged := left.Revision > left.NotifiedRevision
		rightChanged := right.Revision > right.NotifiedRevision
		if leftChanged != rightChanged {
			return leftChanged
		}
		return left.LastSeen > right.LastSeen
	})
	if len(records) > limit {
		records = records[:limit]
	}
	notices := make([]OperationalIssueNotice, 0, len(records))
	for _, record := range records {
		notices = append(notices, OperationalIssueNotice{
			Fingerprint: record.Fingerprint,
			Revision:    record.Revision,
			Severity:    strings.ToLower(strings.TrimSpace(record.Severity)),
			Title:       sanitizeOperationalIssueTitle(record.Title),
			Detail:      sanitizeOperationalIssueText(record.Detail, 240),
			LastSeen:    record.LastSeen,
			Occurrences: record.Occurrences,
		})
	}
	return notices, nil
}

// MarkOperationalIssuesNotified records only revisions that were actually
// delivered or durably persisted for the user.
func MarkOperationalIssuesNotified(db *sql.DB, refs []OperationalIssueNoticeRef, notifiedAt time.Time) error {
	if db == nil || len(refs) == 0 {
		return nil
	}
	if notifiedAt.IsZero() {
		notifiedAt = time.Now()
	}
	now := notifiedAt.UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin operational issue notification update: %w", err)
	}
	defer tx.Rollback()
	for _, ref := range refs {
		if strings.TrimSpace(ref.Fingerprint) == "" || ref.Revision < 1 {
			continue
		}
		if _, err := tx.Exec(`
			UPDATE operational_issues
			SET notified_revision=CASE WHEN notified_revision < ? THEN ? ELSE notified_revision END,
				last_notified_at=?,
				updated_at=CASE WHEN updated_at='' THEN ? ELSE updated_at END
			WHERE fingerprint=? AND revision=?`,
			ref.Revision, ref.Revision, now, now, ref.Fingerprint, ref.Revision); err != nil {
			return fmt.Errorf("mark operational issue notified: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit operational issue notification update: %w", err)
	}
	return nil
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
			first_seen, last_seen, occurrences, status, detail_hash, revision,
			notified_revision, last_notified_at, resolved_at, resolution, created_at, updated_at
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
		if err := scanOperationalIssueRecord(rows, &record); err != nil {
			rows.Close()
			return nil, err
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
	const maxPromptOperationalIssues = 2
	todos = prioritizeOperationalIssueReminderTodos(todos)
	var b strings.Builder
	b.WriteString("Unresolved operational issues detected in background contexts:\n")
	for i, todo := range todos {
		if i >= maxPromptOperationalIssues {
			remaining := len(todos) - maxPromptOperationalIssues
			if remaining > 0 {
				b.WriteString(fmt.Sprintf("- ... %d more unresolved background issue(s) omitted from this prompt\n", remaining))
			}
			break
		}
		b.WriteString("- ")
		b.WriteString(strings.TrimSpace(todo.Title))
		if lastSeen := operationalIssueField(todo.Description, "Last seen"); lastSeen != "" {
			b.WriteString(" (last seen: ")
			b.WriteString(lastSeen)
			b.WriteString(")")
		}
		if detail := operationalIssueField(todo.Description, "Latest detail"); detail != "" {
			b.WriteString("\n  Detail: ")
			b.WriteString(sanitizeOperationalIssueText(detail, 180))
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func prioritizeOperationalIssueReminderTodos(todos []Todo) []Todo {
	if len(todos) < 2 {
		return todos
	}
	out := append([]Todo(nil), todos...)
	sort.SliceStable(out, func(i, j int) bool {
		left := operationalIssueSeverityRank(operationalIssueField(out[i].Description, "Severity"))
		right := operationalIssueSeverityRank(operationalIssueField(out[j].Description, "Severity"))
		if left != right {
			return left < right
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

// ClaimOperationalIssueReminderForDay marks the operational issue reminder as
// shown for the local day. It returns false when the day was already claimed.
func ClaimOperationalIssueReminderForDay(db *sql.DB, now time.Time) (bool, error) {
	if db == nil {
		return false, nil
	}
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("begin operational issue reminder claim: %w", err)
	}
	defer tx.Rollback()

	dayKey := reminderDayKey(now)
	lastSeen, err := getPlannerMetaTx(tx, operationalIssueReminderMetaKey)
	if err != nil {
		return false, err
	}
	if reminderMatchesDay(lastSeen, dayKey) {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit operational issue reminder no-op: %w", err)
		}
		return false, nil
	}
	if err := upsertPlannerMetaTx(tx, operationalIssueReminderMetaKey, dayKey); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit operational issue reminder claim: %w", err)
	}
	return true, nil
}

func compactOperationalIssuePromptDetail(detail string) string {
	return sanitizeOperationalIssueText(detail, 180)
}

func getOperationalIssueRecord(db *sql.DB, fingerprint string) (operationalIssueRecord, bool, error) {
	var record operationalIssueRecord
	row := db.QueryRow(`
		SELECT fingerprint, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status, detail_hash, revision,
			notified_revision, last_notified_at, resolved_at, resolution, created_at, updated_at
		FROM operational_issues
		WHERE fingerprint = ?`, fingerprint)
	err := scanOperationalIssueRecord(row, &record)
	if errors.Is(err, sql.ErrNoRows) {
		return operationalIssueRecord{}, false, nil
	}
	if err != nil {
		return operationalIssueRecord{}, false, fmt.Errorf("get operational issue: %w", err)
	}
	return record, true, nil
}

type operationalIssueScanner interface {
	Scan(dest ...any) error
}

func scanOperationalIssueRecord(scanner operationalIssueScanner, record *operationalIssueRecord) error {
	if err := scanner.Scan(
		&record.Fingerprint, &record.Source, &record.Context, &record.Severity,
		&record.Title, &record.Detail, &record.Reference, &record.FirstSeen,
		&record.LastSeen, &record.Occurrences, &record.Status, &record.DetailHash,
		&record.Revision, &record.NotifiedRevision, &record.LastNotifiedAt,
		&record.ResolvedAt, &record.Resolution, &record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return fmt.Errorf("scan operational issue: %w", err)
	}
	return nil
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

var (
	operationalCredentialAssignment = regexp.MustCompile(`(?i)(["']?(?:api[_ -]?key|access[_ -]?token|refresh[_ -]?token|password|secret|authorization|credential)["']?\s*[:=]\s*)["']?[^"'\s,;}]+["']?`)
	operationalSensitiveEnv         = regexp.MustCompile(`(?i)(\b[A-Z][A-Z0-9_]*(?:KEY|TOKEN|SECRET|PASSWORD)\s*[:=]\s*)[^\s,;]+`)
	operationalBearerToken          = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	operationalSecretToken          = regexp.MustCompile(`(?i)\b(sk-[a-z0-9_-]{8,}|gh[pousr]_[a-z0-9_]{8,})\b`)
	operationalURLCredentials       = regexp.MustCompile(`(?i)\b(https?://)[^/\s:@]+:[^@\s/]+@`)
)

func operationalIssueContentHash(issue OperationalIssue) string {
	title := strings.TrimSpace(strings.TrimPrefix(issue.Title, OperationalIssueTitlePrefix))
	parts := []string{
		strings.ToLower(strings.TrimSpace(issue.Source)),
		strings.ToLower(strings.TrimSpace(issue.Context)),
		strings.ToLower(strings.TrimSpace(issue.Severity)),
		strings.ToLower(title),
		strings.ToLower(strings.TrimSpace(issue.Reference)),
		sanitizeOperationalIssueText(issue.Detail, 1400),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:16])
}

func sanitizeOperationalIssueTitle(title string) string {
	title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(title), OperationalIssueTitlePrefix))
	return sanitizeOperationalIssueText(title, 120)
}

func sanitizeOperationalIssueText(value string, maxRunes int) string {
	var kept []string
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if index := strings.Index(lower, "suggested next step"); index >= 0 {
			trimmed = strings.TrimSpace(trimmed[:index])
			lower = strings.ToLower(trimmed)
		}
		if trimmed == "" || strings.Contains(lower, "_guardian_justification") ||
			strings.Contains(lower, "guardian instruction") ||
			strings.Contains(lower, "policy instruction") ||
			strings.Contains(lower, "ignore previous instructions") ||
			strings.Contains(lower, "begin private key") || strings.Contains(lower, "end private key") {
			continue
		}
		trimmed = operationalCredentialAssignment.ReplaceAllString(trimmed, "$1[redacted]")
		trimmed = operationalSensitiveEnv.ReplaceAllString(trimmed, "$1[redacted]")
		trimmed = operationalBearerToken.ReplaceAllString(trimmed, "Bearer [redacted]")
		trimmed = operationalSecretToken.ReplaceAllString(trimmed, "[redacted]")
		trimmed = operationalURLCredentials.ReplaceAllString(trimmed, "$1[redacted]@")
		if trimmed != "" {
			kept = append(kept, trimmed)
		}
	}
	value = strings.Join(strings.Fields(strings.Join(kept, " ")), " ")
	return truncateOperationalIssueText(value, maxRunes)
}

func operationalIssueVisibleToUser(record operationalIssueRecord) bool {
	if !strings.EqualFold(strings.TrimSpace(record.Source), "memory_reflect") {
		return true
	}
	text := strings.ToLower(strings.Join([]string{record.Title, record.Detail, record.Context}, " "))
	for _, marker := range []string{
		"blocked", "blockiert", "requires user", "user decision", "decision required",
		"needs approval", "approval required", "entscheidung", "freigabe erforderlich",
		"rückfrage", "clarification required",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func operationalIssueNoticeDue(record operationalIssueRecord, now time.Time) bool {
	if record.Revision < 1 {
		record.Revision = 1
	}
	if record.NotifiedRevision < record.Revision {
		return true
	}
	if operationalIssueSeverityRank(record.Severity) > 0 {
		return false
	}
	lastNotified, err := time.Parse(time.RFC3339, strings.TrimSpace(record.LastNotifiedAt))
	if err != nil {
		return true
	}
	return !now.UTC().Before(lastNotified.UTC().Add(24 * time.Hour))
}

func operationalIssueSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error", "high":
		return 0
	case "warning", "warn", "medium":
		return 1
	case "info", "low":
		return 2
	default:
		return 1
	}
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
