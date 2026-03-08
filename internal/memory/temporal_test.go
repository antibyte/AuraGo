package memory

import (
	"log/slog"
	"os"
	"testing"
)

func newTestSQLiteMemory(t *testing.T) (*SQLiteMemory, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "aurago_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	stm, err := NewSQLiteMemory(tmpFile.Name(), logger)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatal(err)
	}

	cleanup := func() {
		stm.Close()
		os.Remove(tmpFile.Name())
	}
	return stm, cleanup
}

// ── Phase A: Temporal Memory Tests ──────────────────────────────────

func TestRecordInteraction(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Empty topic should be a no-op
	if err := stm.RecordInteraction(""); err != nil {
		t.Errorf("Expected nil for empty topic, got %v", err)
	}

	// Record some interactions
	if err := stm.RecordInteraction("server status check"); err != nil {
		t.Fatalf("RecordInteraction failed: %v", err)
	}
	if err := stm.RecordInteraction("server status check"); err != nil {
		t.Fatalf("RecordInteraction (2nd) failed: %v", err)
	}
	if err := stm.RecordInteraction("backup files"); err != nil {
		t.Fatalf("RecordInteraction (different topic) failed: %v", err)
	}
}

func TestRecordInteractionTruncation(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	longTopic := ""
	for i := 0; i < 200; i++ {
		longTopic += "a"
	}

	// Should not error — truncated to 120 chars
	if err := stm.RecordInteraction(longTopic); err != nil {
		t.Fatalf("RecordInteraction with long topic failed: %v", err)
	}
}

func TestGetTopPatterns(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Manually insert patterns with known hour/day for deterministic testing
	stmt := `INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
	         VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP);`

	stm.db.Exec(stmt, 9, 1, "server status", 10)
	stm.db.Exec(stmt, 9, 1, "backup files", 5)
	stm.db.Exec(stmt, 9, 1, "deploy app", 3)
	stm.db.Exec(stmt, 14, 1, "other task", 20)

	topics, err := stm.GetTopPatterns(9, 1, 2)
	if err != nil {
		t.Fatalf("GetTopPatterns failed: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("Expected 2 topics, got %d", len(topics))
	}
	if topics[0] != "server status" {
		t.Errorf("Expected 'server status' first (count=10), got %q", topics[0])
	}
	if topics[1] != "backup files" {
		t.Errorf("Expected 'backup files' second (count=5), got %q", topics[1])
	}
}

func TestGetTopPatternsEmpty(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	topics, err := stm.GetTopPatterns(3, 0, 5)
	if err != nil {
		t.Fatalf("GetTopPatterns on empty DB failed: %v", err)
	}
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics, got %d", len(topics))
	}
}

func TestCleanOldPatterns(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Insert old pattern (200 days ago)
	stm.db.Exec(
		`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
		 VALUES (10, 3, 'old task', 5, datetime('now', '-200 days'));`)

	// Insert recent pattern
	stm.db.Exec(
		`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
		 VALUES (10, 3, 'new task', 2, CURRENT_TIMESTAMP);`)

	deleted, err := stm.CleanOldPatterns(90)
	if err != nil {
		t.Fatalf("CleanOldPatterns failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	// Verify the recent one survived
	topics, _ := stm.GetTopPatterns(10, 3, 5)
	if len(topics) != 1 || topics[0] != "new task" {
		t.Errorf("Expected ['new task'] to survive, got %v", topics)
	}
}

// ── Phase B: Predictive Memory Tests ─────────────────────────────────

func TestPredictNextQuery_TemporalOnly(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Seed temporal patterns at hour=14, weekday=2
	stm.db.Exec(
		`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
		 VALUES (14, 2, 'deploy app', 8, CURRENT_TIMESTAMP);`)
	stm.db.Exec(
		`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
		 VALUES (14, 2, 'check logs', 4, CURRENT_TIMESTAMP);`)

	predictions, err := stm.PredictNextQuery("", 14, 2, 3)
	if err != nil {
		t.Fatalf("PredictNextQuery failed: %v", err)
	}
	if len(predictions) != 2 {
		t.Fatalf("Expected 2 predictions, got %d: %v", len(predictions), predictions)
	}
	if predictions[0] != "deploy app" {
		t.Errorf("Expected 'deploy app' first, got %q", predictions[0])
	}
}

func TestPredictNextQuery_WithToolTransition(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Seed tool transition: after "execute_python", usually "filesystem" follows
	stm.RecordToolTransition("execute_python", "filesystem")
	stm.RecordToolTransition("execute_python", "filesystem")
	stm.RecordToolTransition("execute_python", "query_memory")

	predictions, err := stm.PredictNextQuery("execute_python", 3, 0, 5)
	if err != nil {
		t.Fatalf("PredictNextQuery failed: %v", err)
	}

	// Should include "filesystem" from tool transition even with no temporal data at 3am Sunday
	found := false
	for _, p := range predictions {
		if p == "filesystem" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'filesystem' in predictions from tool transitions, got %v", predictions)
	}
}

func TestPredictNextQuery_Dedup(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Temporal and tool transition predict the same topic
	stm.db.Exec(
		`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
		 VALUES (10, 1, 'filesystem', 5, CURRENT_TIMESTAMP);`)
	stm.RecordToolTransition("execute_python", "filesystem")

	predictions, err := stm.PredictNextQuery("execute_python", 10, 1, 5)
	if err != nil {
		t.Fatalf("PredictNextQuery failed: %v", err)
	}

	// "filesystem" should appear only once (deduplication)
	count := 0
	for _, p := range predictions {
		if p == "filesystem" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected 'filesystem' exactly once (dedup), got %d in %v", count, predictions)
	}
}

func TestPredictNextQuery_LimitRespected(t *testing.T) {
	stm, cleanup := newTestSQLiteMemory(t)
	defer cleanup()

	// Seed many patterns
	for i := 0; i < 10; i++ {
		stm.db.Exec(
			`INSERT INTO interaction_patterns (hour_of_day, day_of_week, topic, count, last_seen)
			 VALUES (8, 5, ?, ?, CURRENT_TIMESTAMP);`, "topic_"+string(rune('a'+i)), 10-i)
	}

	predictions, err := stm.PredictNextQuery("", 8, 5, 3)
	if err != nil {
		t.Fatalf("PredictNextQuery failed: %v", err)
	}
	if len(predictions) > 3 {
		t.Errorf("Expected max 3 predictions, got %d", len(predictions))
	}
}
