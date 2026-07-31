package planner

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	OperationalIssueKindRuntimeFailure = "runtime_failure"
	OperationalIssueKindToolFailure    = "tool_failure"
	OperationalIssueKindReviewRequired = "review_required"

	operationalIssueSingleWarningAge = 7 * 24 * time.Hour
	operationalIssueRecurringAge     = 30 * 24 * time.Hour
)

// OperationalIssueListFilter controls the bounded administrative issue list.
type OperationalIssueListFilter struct {
	Status   string
	Kind     string
	Source   string
	Severity string
	Limit    int
	Offset   int
}

// OperationalIssueListItem is the sanitized administrative read model. The
// internal fingerprint is retained for server-side opaque ID generation only.
type OperationalIssueListItem struct {
	Fingerprint   string `json:"-"`
	Kind          string `json:"kind"`
	Source        string `json:"source"`
	Severity      string `json:"severity"`
	Title         string `json:"title"`
	Detail        string `json:"detail"`
	FirstSeen     string `json:"first_seen"`
	LastSeen      string `json:"last_seen"`
	Occurrences   int    `json:"occurrences"`
	Status        string `json:"status"`
	Revision      int    `json:"revision"`
	ResolvedAt    string `json:"resolved_at,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
	ArchivedAt    string `json:"archived_at,omitempty"`
	ArchiveReason string `json:"archive_reason,omitempty"`
}

// OperationalIssueListPage contains one bounded page and status totals.
type OperationalIssueListPage struct {
	Items        []OperationalIssueListItem `json:"items"`
	Total        int                        `json:"total"`
	Limit        int                        `json:"limit"`
	Offset       int                        `json:"offset"`
	StatusCounts map[string]int             `json:"status_counts"`
}

// OperationalIssuePublicID returns a non-reversible stable identifier for
// administrative API routes without exposing the internal fingerprint.
func OperationalIssuePublicID(fingerprint string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(fingerprint)))
	return hex.EncodeToString(sum[:])
}

// FindOperationalIssueFingerprintByPublicID resolves a public identifier
// server-side. The fingerprint never needs to cross the API boundary.
func FindOperationalIssueFingerprintByPublicID(db *sql.DB, publicID string) (string, bool, error) {
	if db == nil {
		return "", false, fmt.Errorf("planner database not available")
	}
	publicID = strings.ToLower(strings.TrimSpace(publicID))
	if len(publicID) != sha256.Size*2 {
		return "", false, nil
	}
	if _, err := hex.DecodeString(publicID); err != nil {
		return "", false, nil
	}
	rows, err := db.Query(`SELECT fingerprint FROM operational_issues`)
	if err != nil {
		return "", false, fmt.Errorf("list operational issue identifiers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var fingerprint string
		if err := rows.Scan(&fingerprint); err != nil {
			return "", false, fmt.Errorf("scan operational issue identifier: %w", err)
		}
		if OperationalIssuePublicID(fingerprint) == publicID {
			return fingerprint, true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", false, fmt.Errorf("iterate operational issue identifiers: %w", err)
	}
	return "", false, nil
}

func inferOperationalIssueKind(issue OperationalIssue) string {
	switch strings.ToLower(strings.TrimSpace(issue.Kind)) {
	case OperationalIssueKindRuntimeFailure, OperationalIssueKindToolFailure, OperationalIssueKindReviewRequired:
		return strings.ToLower(strings.TrimSpace(issue.Kind))
	}
	if operationalIssueFingerprintIsToolFailure(issue.Fingerprint) {
		return OperationalIssueKindToolFailure
	}
	if operationalIssueRequiresUserActionText(issue.Source, issue.Context, issue.Title, issue.Detail) {
		return OperationalIssueKindReviewRequired
	}
	return OperationalIssueKindRuntimeFailure
}

func operationalIssueFingerprintIsToolFailure(fingerprint string) bool {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(fingerprint)), "|")
	for _, part := range parts {
		if strings.TrimSpace(part) == "tool" {
			return true
		}
	}
	return false
}

func operationalIssueRequiresUserActionText(values ...string) bool {
	text := strings.ToLower(strings.Join(values, " "))
	for _, marker := range []string{
		"requires user", "user decision", "decision required", "awaiting user",
		"needs approval", "approval required", "entscheidung", "freigabe erforderlich",
		"rückfrage", "clarification required",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func operationalIssueRequiresUserAction(record operationalIssueRecord) bool {
	if strings.EqualFold(strings.TrimSpace(record.Kind), OperationalIssueKindReviewRequired) {
		return true
	}
	return operationalIssueRequiresUserActionText(record.Source, record.Context, record.Title, record.Detail)
}

func operationalIssueStaleReason(record operationalIssueRecord, now time.Time) (string, bool) {
	if operationalIssueRequiresUserAction(record) {
		return "", false
	}
	lastSeen, err := time.Parse(time.RFC3339, strings.TrimSpace(record.LastSeen))
	if err != nil || lastSeen.IsZero() {
		return "", false
	}
	if now.IsZero() {
		now = time.Now()
	}
	age := operationalIssueRecurringAge
	reason := "stale_recurring_or_error_30d"
	if record.Occurrences <= 1 && operationalIssueSeverityRank(record.Severity) > 0 {
		age = operationalIssueSingleWarningAge
		reason = "stale_single_warning_7d"
	}
	if now.UTC().Before(lastSeen.UTC().Add(age)) {
		return "", false
	}
	return reason, true
}

// ArchiveStaleOperationalIssues losslessly archives old active records. Each
// update includes the observed generation fields, so a concurrent recurrence
// either prevents archival or reopens the record through RecordOperationalIssue.
func ArchiveStaleOperationalIssues(db *sql.DB, now time.Time) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("planner database not available")
	}
	if now.IsZero() {
		now = time.Now()
	}
	records, err := listOperationalIssueRecords(db, operationalIssueRecordQuery{
		status: "active",
		limit:  10000,
	})
	if err != nil {
		return 0, err
	}
	archivedAt := now.UTC().Format(time.RFC3339)
	var archived int64
	for _, record := range records {
		reason, stale := operationalIssueStaleReason(record, now)
		if !stale {
			continue
		}
		result, err := db.Exec(`
			UPDATE operational_issues
			SET status='archived', archived_at=?, archive_reason=?, updated_at=?
			WHERE fingerprint=? AND status IN ('open', 'in_progress')
				AND last_seen=? AND occurrences=? AND revision=?`,
			archivedAt, reason, archivedAt, record.Fingerprint,
			record.LastSeen, record.Occurrences, record.Revision)
		if err != nil {
			return archived, fmt.Errorf("archive stale operational issue: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return archived, fmt.Errorf("count archived stale operational issue: %w", err)
		}
		archived += rows
	}
	return archived, nil
}

// ArchiveOperationalIssue archives one active record without deleting it.
func ArchiveOperationalIssue(db *sql.DB, fingerprint, reason string, archivedAt time.Time) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("planner database not available")
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return false, nil
	}
	if archivedAt.IsZero() {
		archivedAt = time.Now()
	}
	reason = sanitizeOperationalIssueText(reason, 240)
	if reason == "" {
		reason = "Archived by administrator."
	}
	now := archivedAt.UTC().Format(time.RFC3339)
	result, err := db.Exec(`
		UPDATE operational_issues
		SET status='archived', archived_at=?, archive_reason=?,
			resolved_at='', resolution='', updated_at=?
		WHERE fingerprint=? AND status IN ('open', 'in_progress')`,
		now, reason, now, fingerprint)
	if err != nil {
		return false, fmt.Errorf("archive operational issue: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count archived operational issues: %w", err)
	}
	return rows > 0, nil
}

// PreviewStaleOperationalIssues returns the current lossless archive candidates.
func PreviewStaleOperationalIssues(db *sql.DB, now time.Time, limit int) ([]OperationalIssueListItem, int, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("planner database not available")
	}
	if now.IsZero() {
		now = time.Now()
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	records, err := listOperationalIssueRecords(db, operationalIssueRecordQuery{status: "active", limit: 10000})
	if err != nil {
		return nil, 0, err
	}
	items := make([]OperationalIssueListItem, 0, limit)
	total := 0
	for _, record := range records {
		if _, stale := operationalIssueStaleReason(record, now); !stale {
			continue
		}
		total++
		if len(items) < limit {
			items = append(items, operationalIssueListItem(record))
		}
	}
	return items, total, nil
}

// ListOperationalIssues returns a filtered administrative page.
func ListOperationalIssues(db *sql.DB, filter OperationalIssueListFilter) (OperationalIssueListPage, error) {
	if db == nil {
		return OperationalIssueListPage{}, fmt.Errorf("planner database not available")
	}
	filter.Status = normalizeOperationalIssueStatusFilter(filter.Status)
	filter.Kind = normalizeOperationalIssueKindFilter(filter.Kind)
	filter.Severity = normalizeOperationalIssueSeverityFilter(filter.Severity)
	filter.Source = truncateOperationalIssueText(strings.TrimSpace(filter.Source), 64)
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 || filter.Offset > 100000 {
		filter.Offset = 0
	}
	records, err := listOperationalIssueRecords(db, operationalIssueRecordQuery{
		status:   filter.Status,
		kind:     filter.Kind,
		source:   filter.Source,
		severity: filter.Severity,
		limit:    filter.Limit,
		offset:   filter.Offset,
	})
	if err != nil {
		return OperationalIssueListPage{}, err
	}
	total, err := countOperationalIssueRecords(db, operationalIssueRecordQuery{
		status: filter.Status, kind: filter.Kind, source: filter.Source, severity: filter.Severity,
	})
	if err != nil {
		return OperationalIssueListPage{}, err
	}
	counts, err := countOperationalIssueStatuses(db)
	if err != nil {
		return OperationalIssueListPage{}, err
	}
	items := make([]OperationalIssueListItem, 0, len(records))
	for _, record := range records {
		items = append(items, operationalIssueListItem(record))
	}
	return OperationalIssueListPage{
		Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset, StatusCounts: counts,
	}, nil
}

type operationalIssueRecordQuery struct {
	status   string
	kind     string
	source   string
	severity string
	limit    int
	offset   int
}

func listOperationalIssueRecords(db *sql.DB, query operationalIssueRecordQuery) ([]operationalIssueRecord, error) {
	where, args := operationalIssueWhere(query)
	sqlQuery := `
		SELECT fingerprint, source, context, severity, title, detail, reference,
			first_seen, last_seen, occurrences, status, kind, detail_hash, revision,
			notified_revision, last_notified_at, resolved_at, resolution,
			archived_at, archive_reason, created_at, updated_at
		FROM operational_issues` + where + `
		ORDER BY
			CASE lower(severity)
				WHEN 'critical' THEN 0 WHEN 'error' THEN 0 WHEN 'high' THEN 0
				WHEN 'warning' THEN 1 WHEN 'warn' THEN 1 WHEN 'medium' THEN 1
				ELSE 2 END,
			last_seen DESC`
	if query.limit > 0 {
		sqlQuery += " LIMIT ? OFFSET ?"
		args = append(args, query.limit, query.offset)
	}
	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list operational issue records: %w", err)
	}
	defer rows.Close()
	var records []operationalIssueRecord
	for rows.Next() {
		var record operationalIssueRecord
		if err := scanOperationalIssueRecord(rows, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational issue records: %w", err)
	}
	return records, nil
}

func countOperationalIssueRecords(db *sql.DB, query operationalIssueRecordQuery) (int, error) {
	where, args := operationalIssueWhere(query)
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM operational_issues`+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count operational issue records: %w", err)
	}
	return total, nil
}

func countOperationalIssueStatuses(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM operational_issues GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count operational issue statuses: %w", err)
	}
	defer rows.Close()
	counts := map[string]int{"active": 0, "open": 0, "in_progress": 0, "done": 0, "archived": 0}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan operational issue status count: %w", err)
		}
		status = strings.ToLower(strings.TrimSpace(status))
		counts[status] = count
		if status == "open" || status == "in_progress" {
			counts["active"] += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate operational issue status counts: %w", err)
	}
	return counts, nil
}

func operationalIssueWhere(query operationalIssueRecordQuery) (string, []any) {
	var clauses []string
	var args []any
	switch query.status {
	case "active":
		clauses = append(clauses, "status IN ('open', 'in_progress')")
	case "open", "in_progress", "done", "archived":
		clauses = append(clauses, "status=?")
		args = append(args, query.status)
	}
	if query.kind != "" {
		clauses = append(clauses, "kind=?")
		args = append(args, query.kind)
	}
	if query.source != "" {
		clauses = append(clauses, "source=?")
		args = append(args, query.source)
	}
	if query.severity != "" {
		switch query.severity {
		case "error":
			clauses = append(clauses, "lower(severity) IN ('critical', 'error', 'high')")
		case "warning":
			clauses = append(clauses, "lower(severity) IN ('warning', 'warn', 'medium')")
		case "info":
			clauses = append(clauses, "lower(severity) IN ('info', 'low')")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func normalizeOperationalIssueStatusFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "active":
		return "active"
	case "all", "open", "in_progress", "done", "archived":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "active"
	}
}

func normalizeOperationalIssueKindFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case OperationalIssueKindRuntimeFailure, OperationalIssueKindToolFailure, OperationalIssueKindReviewRequired:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeOperationalIssueSeverityFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical", "error", "high":
		return "error"
	case "warning", "warn", "medium":
		return "warning"
	case "info", "low":
		return "info"
	default:
		return ""
	}
}

func operationalIssueListItem(record operationalIssueRecord) OperationalIssueListItem {
	return OperationalIssueListItem{
		Fingerprint:   record.Fingerprint,
		Kind:          inferOperationalIssueKind(OperationalIssue{Kind: record.Kind, Fingerprint: record.Fingerprint}),
		Source:        sanitizeOperationalIssueText(record.Source, 64),
		Severity:      strings.ToLower(sanitizeOperationalIssueText(record.Severity, 16)),
		Title:         sanitizeOperationalIssueTitle(record.Title),
		Detail:        sanitizeOperationalIssueText(record.Detail, 400),
		FirstSeen:     safeOperationalIssueTimestamp(record.FirstSeen),
		LastSeen:      safeOperationalIssueTimestamp(record.LastSeen),
		Occurrences:   record.Occurrences,
		Status:        strings.ToLower(strings.TrimSpace(record.Status)),
		Revision:      record.Revision,
		ResolvedAt:    safeOperationalIssueTimestamp(record.ResolvedAt),
		Resolution:    sanitizeOperationalIssueText(record.Resolution, 240),
		ArchivedAt:    safeOperationalIssueTimestamp(record.ArchivedAt),
		ArchiveReason: sanitizeOperationalIssueText(record.ArchiveReason, 240),
	}
}

func safeOperationalIssueTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}
