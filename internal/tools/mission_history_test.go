package tools

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// newTestHistoryDB creates a temporary mission history database for testing.
func newTestHistoryDB(t *testing.T) *struct {
	db   *sql.DB
	path string
} {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "mission_history_test.db")
	db, err := InitMissionHistoryDB(dbPath)
	if err != nil {
		t.Fatalf("InitMissionHistoryDB failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &struct {
		db   *sql.DB
		path string
	}{db: db, path: dbPath}
}

func TestInitMissionHistoryDB(t *testing.T) {
	fixture := newTestHistoryDB(t)
	if fixture.db == nil {
		t.Fatal("expected non-nil db")
	}

	// Verify the table exists
	var name string
	err := fixture.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='mission_history'").Scan(&name)
	if err != nil {
		t.Fatalf("mission_history table not found: %v", err)
	}
	if name != "mission_history" {
		t.Errorf("expected table name 'mission_history', got %q", name)
	}

	// Verify indexes exist
	indexes := []string{"idx_mh_mission_id", "idx_mh_status", "idx_mh_trigger_type", "idx_mh_started_at"}
	for _, idx := range indexes {
		var idxName string
		err := fixture.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&idxName)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}

func TestInitMissionHistoryDB_InvalidPath(t *testing.T) {
	// Try to open a DB in a non-existent nested directory
	_, err := InitMissionHistoryDB("/nonexistent/dir/deep/test.db")
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestRecordMissionStart(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID, err := RecordMissionStart(fixture.db, "mission_1", "Test Mission", "manual", `{"user":"admin"}`)
	if err != nil {
		t.Fatalf("RecordMissionStart failed: %v", err)
	}
	if runID == "" {
		t.Error("expected non-empty run ID")
	}

	// Verify the record was inserted
	run, err := GetMissionRun(fixture.db, runID)
	if err != nil {
		t.Fatalf("GetMissionRun failed: %v", err)
	}
	if run == nil {
		t.Fatal("expected run to be found, got nil")
	}
	if run.MissionID != "mission_1" {
		t.Errorf("expected mission_id 'mission_1', got %q", run.MissionID)
	}
	if run.MissionName != "Test Mission" {
		t.Errorf("expected mission_name 'Test Mission', got %q", run.MissionName)
	}
	if run.TriggerType != "manual" {
		t.Errorf("expected trigger_type 'manual', got %q", run.TriggerType)
	}
	if run.Status != "running" {
		t.Errorf("expected status 'running', got %q", run.Status)
	}
	if run.StartedAt.IsZero() {
		t.Error("expected non-zero started_at")
	}
}

func TestRecordMissionStart_NilDB(t *testing.T) {
	_, err := RecordMissionStart(nil, "m1", "test", "manual", "")
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}

func TestRecordMissionCompletion(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID, err := RecordMissionStart(fixture.db, "mission_2", "Completion Test", "cron", "")
	if err != nil {
		t.Fatalf("RecordMissionStart failed: %v", err)
	}

	// Small delay to ensure measurable duration
	time.Sleep(10 * time.Millisecond)

	err = RecordMissionCompletion(fixture.db, runID, "success", "Task completed successfully")
	if err != nil {
		t.Fatalf("RecordMissionCompletion failed: %v", err)
	}

	run, err := GetMissionRun(fixture.db, runID)
	if err != nil {
		t.Fatalf("GetMissionRun failed: %v", err)
	}
	if run.Status != "success" {
		t.Errorf("expected status 'success', got %q", run.Status)
	}
	if run.Output != "Task completed successfully" {
		t.Errorf("unexpected output: %q", run.Output)
	}
	if run.CompletedAt == nil {
		t.Error("expected non-nil completed_at")
	}
	if run.DurationMS <= 0 {
		t.Errorf("expected positive duration_ms, got %d", run.DurationMS)
	}
}

func TestRecordMissionCompletion_OutputTruncation(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID, _ := RecordMissionStart(fixture.db, "m_trunc", "Truncation Test", "manual", "")

	longOutput := make([]byte, 3000)
	for i := range longOutput {
		longOutput[i] = 'A'
	}

	err := RecordMissionCompletion(fixture.db, runID, "success", string(longOutput))
	if err != nil {
		t.Fatalf("RecordMissionCompletion failed: %v", err)
	}

	run, _ := GetMissionRun(fixture.db, runID)
	if len(run.Output) > 2000 {
		t.Errorf("expected output to be truncated to <=2000 chars, got %d", len(run.Output))
	}
}

func TestRecordMissionCompletion_NilDB(t *testing.T) {
	err := RecordMissionCompletion(nil, "run_1", "success", "output")
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}

func TestRecordMissionError(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID, _ := RecordMissionStart(fixture.db, "mission_err", "Error Test", "webhook", `{}`)

	err := RecordMissionError(fixture.db, runID, "Connection timeout after 30s")
	if err != nil {
		t.Fatalf("RecordMissionError failed: %v", err)
	}

	run, _ := GetMissionRun(fixture.db, runID)
	if run.Status != "error" {
		t.Errorf("expected status 'error', got %q", run.Status)
	}
	if run.ErrorMsg != "Connection timeout after 30s" {
		t.Errorf("unexpected error_msg: %q", run.ErrorMsg)
	}
	if run.CompletedAt == nil {
		t.Error("expected non-nil completed_at")
	}
	if run.DurationMS < 0 {
		t.Errorf("expected non-negative duration_ms, got %d", run.DurationMS)
	}
}

func TestRecordMissionError_Truncation(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID, _ := RecordMissionStart(fixture.db, "m_err_trunc", "Error Truncation", "manual", "")

	longError := make([]byte, 800)
	for i := range longError {
		longError[i] = 'E'
	}

	_ = RecordMissionError(fixture.db, runID, string(longError))

	run, _ := GetMissionRun(fixture.db, runID)
	if len(run.ErrorMsg) > 500 {
		t.Errorf("expected error_msg to be truncated to <=500 chars, got %d", len(run.ErrorMsg))
	}
}

func TestRecordMissionError_NilDB(t *testing.T) {
	err := RecordMissionError(nil, "run_1", "error")
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}

func TestQueryMissionHistory_NoFilters(t *testing.T) {
	fixture := newTestHistoryDB(t)

	// Insert test data
	for i := 0; i < 15; i++ {
		triggerType := "manual"
		if i%3 == 0 {
			triggerType = "cron"
		}
		RecordMissionStart(fixture.db, "mission_q", "Query Test", triggerType, "")
	}

	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 15 {
		t.Errorf("expected total 15, got %d", page.Total)
	}
	if page.Limit != 10 {
		t.Errorf("expected default limit 10, got %d", page.Limit)
	}
	if len(page.Entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(page.Entries))
	}
	if page.Offset != 0 {
		t.Errorf("expected offset 0, got %d", page.Offset)
	}
}

func TestQueryMissionHistory_WithPagination(t *testing.T) {
	fixture := newTestHistoryDB(t)

	for i := 0; i < 15; i++ {
		RecordMissionStart(fixture.db, "mission_p", "Page Test", "manual", "")
	}

	// Second page
	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{Limit: 10, Offset: 10})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 15 {
		t.Errorf("expected total 15, got %d", page.Total)
	}
	if len(page.Entries) != 5 {
		t.Errorf("expected 5 entries on second page, got %d", len(page.Entries))
	}
}

func TestQueryMissionHistory_FilterByMissionID(t *testing.T) {
	fixture := newTestHistoryDB(t)

	RecordMissionStart(fixture.db, "mission_a", "Mission A", "manual", "")
	RecordMissionStart(fixture.db, "mission_b", "Mission B", "manual", "")
	RecordMissionStart(fixture.db, "mission_a", "Mission A", "cron", "")

	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{MissionID: "mission_a"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
	for _, entry := range page.Entries {
		if entry.MissionID != "mission_a" {
			t.Errorf("expected mission_id 'mission_a', got %q", entry.MissionID)
		}
	}
}

func TestQueryMissionHistory_FilterByTriggerType(t *testing.T) {
	fixture := newTestHistoryDB(t)

	RecordMissionStart(fixture.db, "m1", "Test", "manual", "")
	RecordMissionStart(fixture.db, "m2", "Test", "cron", "")
	RecordMissionStart(fixture.db, "m3", "Test", "manual", "")

	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{TriggerType: "manual"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
}

func TestQueryMissionHistory_FilterByResult(t *testing.T) {
	fixture := newTestHistoryDB(t)

	runID1, _ := RecordMissionStart(fixture.db, "m1", "Test", "manual", "")
	runID2, _ := RecordMissionStart(fixture.db, "m2", "Test", "manual", "")
	RecordMissionStart(fixture.db, "m3", "Test", "manual", "")

	RecordMissionCompletion(fixture.db, runID1, "success", "ok")
	RecordMissionError(fixture.db, runID2, "failed")

	// Filter for success
	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{Result: "success"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected total 1 for success, got %d", page.Total)
	}

	// Filter for error
	page, err = QueryMissionHistory(fixture.db, MissionHistoryFilter{Result: "error"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected total 1 for error, got %d", page.Total)
	}

	// Filter for running
	page, err = QueryMissionHistory(fixture.db, MissionHistoryFilter{Result: "running"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected total 1 for running, got %d", page.Total)
	}
}

func TestQueryMissionHistory_LimitCapped(t *testing.T) {
	fixture := newTestHistoryDB(t)

	RecordMissionStart(fixture.db, "m1", "Test", "manual", "")

	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{Limit: 500})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Limit != 100 {
		t.Errorf("expected limit capped at 100, got %d", page.Limit)
	}
}

func TestQueryMissionHistory_NilDB(t *testing.T) {
	_, err := QueryMissionHistory(nil, MissionHistoryFilter{})
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}

func TestGetMissionRun_NotFound(t *testing.T) {
	fixture := newTestHistoryDB(t)

	run, err := GetMissionRun(fixture.db, "nonexistent_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run != nil {
		t.Error("expected nil for non-existent run")
	}
}

func TestGetMissionRun_NilDB(t *testing.T) {
	_, err := GetMissionRun(nil, "some_id")
	if err == nil {
		t.Error("expected error for nil db, got nil")
	}
}

func TestCleanOldMissionHistory(t *testing.T) {
	fixture := newTestHistoryDB(t)
	logger := slog.Default()

	// Insert a record with an old started_at timestamp (2 hours ago)
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	_, err := fixture.db.Exec(`
		INSERT INTO mission_history (id, mission_id, mission_name, trigger_type, trigger_data, status, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"run_old", "m_clean", "Clean Test", "manual", "", "success", oldTime)
	if err != nil {
		t.Fatalf("failed to insert old record: %v", err)
	}

	// Insert a recent record that should NOT be cleaned
	RecordMissionStart(fixture.db, "m_recent", "Recent Test", "manual", "")

	// Verify we have 2 entries
	page, _ := QueryMissionHistory(fixture.db, MissionHistoryFilter{})
	if page.Total != 2 {
		t.Fatalf("expected 2 entries before cleanup, got %d", page.Total)
	}

	// Clean entries older than 1 hour - should remove only the old one
	deleted, err := CleanOldMissionHistory(fixture.db, 1*time.Hour, logger)
	if err != nil {
		t.Fatalf("CleanOldMissionHistory failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify only the recent one remains
	page, _ = QueryMissionHistory(fixture.db, MissionHistoryFilter{})
	if page.Total != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", page.Total)
	}
	if len(page.Entries) > 0 && page.Entries[0].MissionID != "m_recent" {
		t.Errorf("expected recent entry to remain, got mission_id %q", page.Entries[0].MissionID)
	}
}

func TestCleanOldMissionHistory_ZeroMaxAge(t *testing.T) {
	fixture := newTestHistoryDB(t)
	logger := slog.Default()

	RecordMissionStart(fixture.db, "m_noclean", "No Clean", "manual", "")

	deleted, err := CleanOldMissionHistory(fixture.db, 0, logger)
	if err != nil {
		t.Fatalf("CleanOldMissionHistory failed: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted with zero maxAge, got %d", deleted)
	}
}

func TestCleanOldMissionHistory_NilDB(t *testing.T) {
	logger := slog.Default()
	deleted, err := CleanOldMissionHistory(nil, time.Hour, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 for nil db, got %d", deleted)
	}
}

func TestFormatMissionHistoryJSON(t *testing.T) {
	page := &MissionHistoryPage{
		Entries: []*MissionRun{
			{
				ID:          "run_1",
				MissionID:   "m1",
				MissionName: "Test",
				TriggerType: "manual",
				Status:      "success",
				StartedAt:   time.Now(),
				DurationMS:  100,
			},
		},
		Total:  1,
		Limit:  10,
		Offset: 0,
	}

	result := FormatMissionHistoryJSON(page)
	if result == "" {
		t.Error("expected non-empty JSON string")
	}

	// Verify it's valid JSON
	var parsed MissionHistoryPage
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if parsed.Total != 1 {
		t.Errorf("expected total 1, got %d", parsed.Total)
	}
	if len(parsed.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(parsed.Entries))
	}
}

func TestFormatMissionHistoryJSON_NilPage(t *testing.T) {
	// This should not panic
	result := FormatMissionHistoryJSON(nil)
	if result != "null" {
		// json.Marshal(nil) returns "null"
		t.Logf("FormatMissionHistoryJSON(nil) = %q", result)
	}
}

func TestMissionHistoryIntegration(t *testing.T) {
	// Full integration test: start -> complete -> query
	fixture := newTestHistoryDB(t)

	// Start multiple missions
	runID1, _ := RecordMissionStart(fixture.db, "backup", "Nightly Backup", "cron", `{"schedule":"0 2 * * *"}`)
	runID2, _ := RecordMissionStart(fixture.db, "cleanup", "Temp Cleanup", "manual", `{"user":"admin"}`)
	runID3, _ := RecordMissionStart(fixture.db, "backup", "Nightly Backup", "cron", `{"schedule":"0 2 * * *"}`)

	time.Sleep(5 * time.Millisecond)

	// Complete some, error one
	RecordMissionCompletion(fixture.db, runID1, "success", "Backup completed: 42 files saved")
	RecordMissionError(fixture.db, runID2, "Permission denied: /tmp/locked")
	// runID3 stays running

	// Query all
	page, err := QueryMissionHistory(fixture.db, MissionHistoryFilter{})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("expected total 3, got %d", page.Total)
	}

	// Query by mission_id
	page, err = QueryMissionHistory(fixture.db, MissionHistoryFilter{MissionID: "backup"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected 2 backup entries, got %d", page.Total)
	}

	// Query by status
	page, err = QueryMissionHistory(fixture.db, MissionHistoryFilter{Result: "success"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected 1 success entry, got %d", page.Total)
	}

	// Query by trigger_type
	page, err = QueryMissionHistory(fixture.db, MissionHistoryFilter{TriggerType: "cron"})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected 2 cron entries, got %d", page.Total)
	}

	// Verify individual run details
	run1, _ := GetMissionRun(fixture.db, runID1)
	if run1.Status != "success" {
		t.Errorf("run1: expected status 'success', got %q", run1.Status)
	}
	if run1.Output != "Backup completed: 42 files saved" {
		t.Errorf("run1: unexpected output: %q", run1.Output)
	}
	if run1.DurationMS <= 0 {
		t.Errorf("run1: expected positive duration, got %d", run1.DurationMS)
	}

	run2, _ := GetMissionRun(fixture.db, runID2)
	if run2.Status != "error" {
		t.Errorf("run2: expected status 'error', got %q", run2.Status)
	}
	if run2.ErrorMsg != "Permission denied: /tmp/locked" {
		t.Errorf("run2: unexpected error_msg: %q", run2.ErrorMsg)
	}

	run3, _ := GetMissionRun(fixture.db, runID3)
	if run3.Status != "running" {
		t.Errorf("run3: expected status 'running', got %q", run3.Status)
	}
	if run3.CompletedAt != nil {
		t.Error("run3: expected nil completed_at for running mission")
	}

	// Format as JSON
	jsonStr := FormatMissionHistoryJSON(page)
	if jsonStr == "" {
		t.Error("expected non-empty JSON output")
	}

	// Clean up old entries (should remove nothing since entries are recent)
	logger := slog.Default()
	deleted, _ := CleanOldMissionHistory(fixture.db, 24*time.Hour, logger)
	if deleted != 0 {
		t.Errorf("expected 0 deleted for recent entries, got %d", deleted)
	}
}

func TestInitMissionHistoryDB_Reopen(t *testing.T) {
	// Test that reopening an existing DB preserves data
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persist_test.db")

	// First open: create and insert
	db1, err := InitMissionHistoryDB(dbPath)
	if err != nil {
		t.Fatalf("first InitMissionHistoryDB failed: %v", err)
	}
	RecordMissionStart(db1, "persist_m", "Persist Test", "manual", "")
	db1.Close()

	// Verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}

	// Second open: verify data persists
	db2, err := InitMissionHistoryDB(dbPath)
	if err != nil {
		t.Fatalf("second InitMissionHistoryDB failed: %v", err)
	}
	defer db2.Close()

	page, err := QueryMissionHistory(db2, MissionHistoryFilter{})
	if err != nil {
		t.Fatalf("QueryMissionHistory failed: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("expected 1 persisted entry, got %d", page.Total)
	}
}
