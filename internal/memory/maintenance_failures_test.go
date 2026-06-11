package memory

import (
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestMemoryMaintenanceFailureTrackingSkipsAfterThreshold(t *testing.T) {
	stm, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for i := 0; i < 3; i++ {
		if err := stm.RecordMemoryMaintenanceFailure("note_archive", "42", errors.New("boom")); err != nil {
			t.Fatalf("RecordMemoryMaintenanceFailure: %v", err)
		}
	}
	if !stm.ShouldSkipMemoryMaintenanceAction("note_archive", "42", 3) {
		t.Fatal("expected action to be skipped after repeated failures")
	}
	if err := stm.ClearMemoryMaintenanceFailure("note_archive", "42"); err != nil {
		t.Fatalf("ClearMemoryMaintenanceFailure: %v", err)
	}
	if stm.ShouldSkipMemoryMaintenanceAction("note_archive", "42", 3) {
		t.Fatal("expected action not to be skipped after clear")
	}
}
