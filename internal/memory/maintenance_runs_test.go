package memory

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestMaintenanceRunLedgerRoundTrip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	started := time.Date(2026, 6, 10, 4, 0, 0, 0, time.UTC)
	finished := started.Add(12 * time.Minute)
	results := MaintenancePhaseResults{
		JournalRemoved:     2,
		NotesArchived:      1,
		ConsolidationFacts: 4,
		CompressedDeleted:  3,
		KGFilesProcessed:   5,
		Errors:             []string{"file_kg_sync: timeout"},
	}
	if err := stm.InsertMaintenanceRun(started, finished, "partial", results); err != nil {
		t.Fatalf("InsertMaintenanceRun: %v", err)
	}

	record, err := stm.GetLatestMaintenanceRun()
	if err != nil {
		t.Fatalf("GetLatestMaintenanceRun: %v", err)
	}
	if record == nil {
		t.Fatal("expected maintenance run record")
	}
	if record.Status != "partial" {
		t.Fatalf("status = %q, want partial", record.Status)
	}
	if record.PhaseResults.JournalRemoved != 2 || record.PhaseResults.ConsolidationFacts != 4 {
		t.Fatalf("phase results = %+v", record.PhaseResults)
	}
	if len(record.PhaseResults.Errors) != 1 {
		t.Fatalf("errors = %#v, want 1 entry", record.PhaseResults.Errors)
	}
}