package tools

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The history cuts long outputs and error messages at a rune boundary, so the stored text
// stays valid UTF-8 (2-byte runes put the old byte offsets 1997 and 497 inside a rune).
func TestC15MissionHistoryCutsAtRuneBoundaries(t *testing.T) {
	db, err := InitMissionHistoryDB(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	text := strings.Repeat("ä", 3000) // 6000 bytes

	okID, err := RecordMissionStart(db, "m1", "Mission", "manual", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordMissionCompletion(db, okID, "success", text); err != nil {
		t.Fatal(err)
	}
	run, err := GetMissionRun(db, okID)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(run.Output) || len(run.Output) > 2000 || !strings.HasSuffix(run.Output, "ä...") {
		t.Fatalf("output: %d bytes, valid UTF-8 %v", len(run.Output), utf8.ValidString(run.Output))
	}

	errID, err := RecordMissionStart(db, "m1", "Mission", "manual", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordMissionError(db, errID, text); err != nil {
		t.Fatal(err)
	}
	if run, err = GetMissionRun(db, errID); err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(run.ErrorMsg) || len(run.ErrorMsg) > 500 || !strings.HasSuffix(run.ErrorMsg, "ä...") {
		t.Fatalf("error: %d bytes, valid UTF-8 %v", len(run.ErrorMsg), utf8.ValidString(run.ErrorMsg))
	}

	// ASCII text keeps the full 2000 and 500 bytes.
	asciiID, _ := RecordMissionStart(db, "m1", "Mission", "manual", "{}")
	if err := RecordMissionCompletion(db, asciiID, "success", strings.Repeat("a", 3000)); err != nil {
		t.Fatal(err)
	}
	if run, _ := GetMissionRun(db, asciiID); len(run.Output) != 2000 {
		t.Fatalf("ASCII output: %d bytes", len(run.Output))
	}
}
