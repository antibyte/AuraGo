package main

import (
	"path/filepath"
	"testing"

	"aurago/internal/planner"
	"aurago/internal/virtualcomputers"
)

func TestVirtualWorkspaceIssueReporterResolvesOnlyMatchingFailure(t *testing.T) {
	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	report := virtualWorkspaceIssueReporter(db, nil)
	issue := virtualcomputers.WorkspaceOperationalIssue{Kind: "lease_close_failed", WorkspaceID: "workspace-1", Severity: "warning", Detail: "delete failed"}
	report(issue)
	report(issue)
	other := issue
	other.WorkspaceID = "workspace-2"
	report(other)
	issue.Resolved = true
	issue.Detail = "Machine absent and workspace closed."
	report(issue)
	var status string
	var occurrences int
	if err := db.QueryRow(`SELECT status, occurrences FROM operational_issues WHERE fingerprint = ?`, "virtual_workspace|lease_close_failed|workspace-1").Scan(&status, &occurrences); err != nil {
		t.Fatal(err)
	}
	if status != "done" || occurrences != 2 {
		t.Fatalf("resolution lost history: %s %d", status, occurrences)
	}
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint = ?`, "virtual_workspace|lease_close_failed|workspace-2").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "open" {
		t.Fatalf("unrelated workspace issue resolved: %s", status)
	}
	issue.Resolved = false
	report(issue)
	if err := db.QueryRow(`SELECT status, occurrences FROM operational_issues WHERE fingerprint = ?`, "virtual_workspace|lease_close_failed|workspace-1").Scan(&status, &occurrences); err != nil {
		t.Fatal(err)
	}
	if status != "open" || occurrences != 3 {
		t.Fatalf("recurrence not retained: %s %d", status, occurrences)
	}
}
