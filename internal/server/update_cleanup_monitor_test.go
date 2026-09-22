package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/upkeep"
)

func TestUpdateCleanupIssueLifecycle(t *testing.T) {
	s := testOperationalIssueServer(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(sequence int64, failures int, success bool) {
		t.Helper()
		raw, _ := json.Marshal(upkeep.Result{Sequence: sequence, Failures: failures, Success: success})
		if e := os.WriteFile(filepath.Join(root, "data", upkeep.ResultFile), raw, 0600); e != nil {
			t.Fatal(e)
		}
		if e := reconcileUpdateCleanup(s.PlannerDB, root); e != nil {
			t.Fatal(e)
		}
	}
	write(1, 1, false)
	var count int
	if e := s.PlannerDB.QueryRow("SELECT count(*) FROM operational_issues WHERE fingerprint=?", updateCleanupFingerprint).Scan(&count); e != nil || count != 0 {
		t.Fatal(count, e)
	}
	write(2, 2, false)
	write(2, 2, false)
	var occurrences int
	var status string
	query := func() {
		t.Helper()
		if e := s.PlannerDB.QueryRow("SELECT occurrences,status FROM operational_issues WHERE fingerprint=?", updateCleanupFingerprint).Scan(&occurrences, &status); e != nil {
			t.Fatal(e)
		}
	}
	query()
	if occurrences != 1 || status != "open" {
		t.Fatal(occurrences, status)
	}
	write(3, 3, false)
	query()
	if occurrences != 2 {
		t.Fatal("polls were mistaken for failures", occurrences)
	}
	write(4, 0, true)
	query()
	if status != "done" {
		t.Fatal(status)
	}
	write(5, 1, false)
	query()
	if status != "done" {
		t.Fatal("single failure reopened issue")
	}
	write(6, 2, false)
	query()
	if status != "open" {
		t.Fatal("recurrence did not reopen issue")
	}
}
