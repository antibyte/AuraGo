package main

import (
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/planner"
	"aurago/internal/virtualcomputers"
)

func virtualWorkspaceIssueReporter(db *sql.DB, logger *slog.Logger) func(virtualcomputers.WorkspaceOperationalIssue) {
	return func(issue virtualcomputers.WorkspaceOperationalIssue) {
		if db == nil {
			return
		}
		contextID := strings.TrimSpace(issue.WorkspaceID)
		kind := strings.TrimSpace(issue.Kind)
		fingerprint := "virtual_workspace|" + kind
		if contextID != "" {
			fingerprint += "|" + contextID
		}
		var err error
		if issue.Resolved {
			_, err = planner.ResolveOperationalIssue(db, fingerprint, issue.Detail, time.Now())
		} else {
			_, err = planner.RecordOperationalIssue(db, planner.OperationalIssue{
				Source: "virtual_workspace", Context: contextID,
				Title:  "Virtual workspace " + strings.ReplaceAll(kind, "_", " "),
				Detail: issue.Detail, Severity: issue.Severity, Kind: planner.OperationalIssueKindRuntimeFailure,
				Reference: kind, Fingerprint: fingerprint, OccurredAt: time.Now(),
			})
		}
		if err != nil && logger != nil {
			logger.Warn("Failed to persist virtual workspace operational issue", "error", err)
		}
	}
}
