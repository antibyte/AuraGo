package memory

import (
	"log/slog"
	"os"
	"testing"
)

func newTestErrorLearningDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitErrorLearningTable(); err != nil {
		t.Fatalf("InitErrorLearningTable: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

// TestNormalizeErrorMsg verifies that variable parts are replaced with stable placeholders.
func TestNormalizeErrorMsg(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			"file not found: /tmp/12345.txt",
			"file not found: <PATH>",
		},
		{
			"file not found: /tmp/67890.txt",
			"file not found: <PATH>",
		},
		{
			"failed at 2024-01-15T09:30:00: timeout",
			"failed at <TIMESTAMP>: timeout",
		},
		{
			"request id 99999999 rejected",
			"request id <ID> rejected",
		},
		{
			"hash mismatch: deadbeefcafe1234 vs expected",
			"hash mismatch: <HEX> vs expected",
		},
		{
			"simple error without variables",
			"simple error without variables",
		},
	}
	for _, c := range cases {
		got := normalizeErrorMsg(c.input)
		if got != c.want {
			t.Errorf("normalizeErrorMsg(%q)\n  got:  %q\n  want: %q", c.input, got, c.want)
		}
	}
}

// TestRecordErrorGroupsSimilarErrors verifies that errors differing only in variable parts
// are grouped as a single pattern (occurrence_count incremented).
func TestRecordErrorGroupsSimilarErrors(t *testing.T) {
	stm := newTestErrorLearningDB(t)

	// Both errors are semantically identical after normalization
	if err := stm.RecordError("shell", "file not found: /tmp/12345.txt"); err != nil {
		t.Fatal(err)
	}
	if err := stm.RecordError("shell", "file not found: /tmp/67890.txt"); err != nil {
		t.Fatal(err)
	}

	patterns, err := stm.GetFrequentErrors("shell", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 grouped pattern, got %d", len(patterns))
	}
	if patterns[0].OccurrenceCount != 2 {
		t.Errorf("expected occurrence_count=2, got %d", patterns[0].OccurrenceCount)
	}
}

// TestRecordErrorDistinctPatterns verifies that genuinely different errors stay separate.
func TestRecordErrorDistinctPatterns(t *testing.T) {
	stm := newTestErrorLearningDB(t)

	_ = stm.RecordError("shell", "permission denied")
	_ = stm.RecordError("shell", "connection timeout")

	patterns, err := stm.GetFrequentErrors("shell", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 2 {
		t.Errorf("expected 2 distinct patterns, got %d", len(patterns))
	}
}

// TestLookupErrorResolutionNormalized verifies that lookup works after normalization.
func TestLookupErrorResolutionNormalized(t *testing.T) {
	stm := newTestErrorLearningDB(t)

	_ = stm.RecordError("python", "file not found: /home/user/12345.py")
	_ = stm.RecordResolution("python", "file not found: /home/user/99999.py", "check file permissions")

	res, err := stm.LookupErrorResolution("python", "file not found: /tmp/00001.py")
	if err != nil {
		t.Fatal(err)
	}
	if res != "check file permissions" {
		t.Errorf("expected resolution 'check file permissions', got %q", res)
	}
}
