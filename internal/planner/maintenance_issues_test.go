package planner

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLegacyMaintenanceResolutionMatchesPhaseAndReference(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	issues := []OperationalIssue{
		{Source: "maintenance", Context: "maintenance", Title: "Maintenance agent loop failed", Reference: "daily_maintenance"},
		{Source: "maintenance", Title: "Maintenance prompt could not be read", Reference: "current/maintenance.md"},
		{Source: "maintenance", Title: "Maintenance prompt could not be read", Reference: "other/maintenance.md"},
	}
	var ids []string
	for _, issue := range issues {
		id, err := RecordOperationalIssue(db, issue)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := ResolveLegacyMaintenanceIssues(db, "agent_loop", "", now); err != nil {
		t.Fatal(err)
	}
	if err := ResolveLegacyMaintenanceIssues(db, "prompt_load", "current/maintenance.md", now); err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		var status string
		if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint=?`, id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		want := "done"
		if i == 2 {
			want = "open"
		}
		if status != want {
			t.Fatalf("issue %d status=%s want=%s", i, status, want)
		}
	}
}
