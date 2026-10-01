package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"aurago/internal/memory"
)

func timeBoundary(left, right string, minimum bool) (string, error) {
	if left == "" {
		if _, err := parseTime(right); err != nil {
			return "", err
		}
		return right, nil
	}
	a, err := parseTime(left)
	if err != nil {
		return "", err
	}
	b, err := parseTime(right)
	if err != nil {
		return "", err
	}
	if (minimum && b.Before(a)) || (!minimum && b.After(a)) {
		return right, nil
	}
	return left, nil
}
func sourceRows(group mergeGroup) (map[string]record, error) {
	result := map[string]record{}
	for _, source := range group.Before.Rows["memory_extraction_sources"] {
		if strings.TrimSpace(text(source, "source_type")) == "" || strings.TrimSpace(text(source, "session_id")) == "" {
			return nil, fmt.Errorf("source observation identity is incomplete")
		}
		key := fingerprint([]string{text(source, "source_type"), text(source, "session_id")})
		current := result[key]
		if current == nil {
			current = record{"source_type": text(source, "source_type"), "session_id": text(source, "session_id")}
		}
		for _, field := range []string{"first_seen_at", "last_seen_at"} {
			value, err := timeBoundary(text(current, field), text(source, field), field == "first_seen_at")
			if err != nil {
				return nil, err
			}
			current[field] = value
		}
		first, _ := parseTime(text(source, "first_seen_at"))
		last, _ := parseTime(text(source, "last_seen_at"))
		if first.After(last) {
			return nil, fmt.Errorf("source observation range is inconsistent")
		}
		result[key] = current
	}
	for _, doc := range group.Before.Documents {
		_, body, _ := memory.AnalysisDocumentParts(doc.Content)
		session, _ := memory.MemoryAnalysisSession(body)
		if session == "" {
			continue
		}
		key := fingerprint([]string{"memory_analysis", session})
		if result[key] == nil {
			// Missing historical dates stay unknown: record this repair observation.
			now := time.Now().UTC().Format(time.RFC3339Nano)
			result[key] = record{"source_type": "memory_analysis", "session_id": session, "first_seen_at": now, "last_seen_at": now}
		}
	}
	return result, nil
}
func transferSourceRows(ctx context.Context, tx *sql.Tx, group mergeGroup) error {
	sources, err := sourceRows(group)
	if err != nil {
		return err
	}
	for _, source := range sources {
		first, _ := parseTime(text(source, "first_seen_at"))
		last, _ := parseTime(text(source, "last_seen_at"))
		if _, err := tx.ExecContext(ctx, `INSERT INTO memory_extraction_sources VALUES (?,?,?,?,?)
			ON CONFLICT(doc_id,source_type,session_id) DO UPDATE SET first_seen_at=excluded.first_seen_at,last_seen_at=excluded.last_seen_at`,
			group.Canonical, text(source, "source_type"), text(source, "session_id"), first.UTC().Format("2006-01-02T15:04:05.000000000Z"), last.UTC().Format("2006-01-02T15:04:05.000000000Z")); err != nil {
			return err
		}
	}
	for _, id := range group.IDs {
		if id != group.Canonical {
			if _, err := tx.ExecContext(ctx, `DELETE FROM memory_extraction_sources WHERE doc_id=?`, id); err != nil {
				return err
			}
		}
	}
	return nil
}
func transferFailureRows(ctx context.Context, tx *sql.Tx, group mergeGroup) error {
	merged := map[string]record{}
	for _, row := range group.Before.Rows["memory_maintenance_failures"] {
		action := text(row, "action")
		current := merged[action]
		if current == nil {
			current = record{"action": action}
		}
		count, previous := number(row, "failure_count"), number(current, "failure_count")
		if count < 0 || previous > math.MaxInt64-count {
			return fmt.Errorf("maintenance failure counter overflows")
		}
		current["failure_count"] = previous + count
		first, err := timeBoundary(text(current, "first_failed_at"), text(row, "first_failed_at"), true)
		if err != nil {
			return err
		}
		current["first_failed_at"] = first
		last, err := timeBoundary(text(current, "last_failed_at"), text(row, "last_failed_at"), false)
		if err != nil {
			return err
		}
		if text(current, "last_failed_at") != last {
			current["last_error"] = text(row, "last_error")
		}
		current["last_failed_at"] = last
		merged[action] = current
	}
	for _, row := range merged {
		if _, err := tx.ExecContext(ctx, `INSERT INTO memory_maintenance_failures VALUES (?,?,?,?,?,?)
			ON CONFLICT(action,target_id) DO UPDATE SET failure_count=excluded.failure_count,last_error=excluded.last_error,
			first_failed_at=excluded.first_failed_at,last_failed_at=excluded.last_failed_at`, text(row, "action"), group.Canonical,
			number(row, "failure_count"), text(row, "last_error"), text(row, "first_failed_at"), text(row, "last_failed_at")); err != nil {
			return err
		}
	}
	for _, id := range group.IDs {
		if id != group.Canonical {
			if _, err := tx.ExecContext(ctx, `DELETE FROM memory_maintenance_failures WHERE target_id=?`, id); err != nil {
				return err
			}
		}
	}
	return nil
}
