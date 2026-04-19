package memory

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func newTestInnerVoiceDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitEmotionTables(); err != nil {
		t.Fatalf("InitEmotionTables: %v", err)
	}
	if err := stm.InitInnerVoiceTables(); err != nil {
		t.Fatalf("InitInnerVoiceTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

func TestInitInnerVoiceTables_Idempotent(t *testing.T) {
	stm := newTestInnerVoiceDB(t)
	// Calling again should be safe
	if err := stm.InitInnerVoiceTables(); err != nil {
		t.Fatalf("second InitInnerVoiceTables: %v", err)
	}
}

func TestStoreInnerVoice_EmptyThought(t *testing.T) {
	stm := newTestInnerVoiceDB(t)
	// Storing empty thought should be a no-op
	if err := stm.StoreInnerVoice("", "caution"); err != nil {
		t.Fatalf("StoreInnerVoice empty: %v", err)
	}
}

func TestStoreAndGetInnerVoices(t *testing.T) {
	stm := newTestInnerVoiceDB(t)

	// Insert some emotion_history rows first (inner voice piggybacks on them)
	for i := 0; i < 3; i++ {
		_, err := stm.db.Exec(
			`INSERT INTO emotion_history (description, primary_mood, secondary_mood, valence, arousal, confidence, cause, recommended_response_style)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"feeling good", "happy", "calm", 0.5, 0.3, 0.8, "test", "friendly",
		)
		if err != nil {
			t.Fatalf("insert emotion_history row %d: %v", i, err)
		}
		// Store inner voice on latest row
		thought := ""
		category := ""
		if i == 1 {
			thought = "Maybe try a different approach"
			category = "suggestion"
		}
		if i == 2 {
			thought = "This is going well, keep the momentum"
			category = "encouragement"
		}
		if thought != "" {
			if err := stm.StoreInnerVoice(thought, category); err != nil {
				t.Fatalf("StoreInnerVoice %d: %v", i, err)
			}
		}
	}

	// GetRecentInnerVoices should return the 2 non-empty entries
	entries, err := stm.GetRecentInnerVoices(5)
	if err != nil {
		t.Fatalf("GetRecentInnerVoices: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 inner voice entries, got %d", len(entries))
	}
	// Most recent first
	if entries[0].InnerThought != "This is going well, keep the momentum" {
		t.Fatalf("unexpected first entry: %q", entries[0].InnerThought)
	}
	if entries[0].NudgeCategory != "encouragement" {
		t.Fatalf("unexpected first category: %q", entries[0].NudgeCategory)
	}
	if entries[1].InnerThought != "Maybe try a different approach" {
		t.Fatalf("unexpected second entry: %q", entries[1].InnerThought)
	}
}

func TestGetRecentInnerVoices_DefaultLimit(t *testing.T) {
	stm := newTestInnerVoiceDB(t)
	// With 0 limit, should default to 5 and not error
	entries, err := stm.GetRecentInnerVoices(0)
	if err != nil {
		t.Fatalf("GetRecentInnerVoices(0): %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries from empty db, got %d", len(entries))
	}
}

func TestGetTodayInnerVoiceSummary_Empty(t *testing.T) {
	stm := newTestInnerVoiceDB(t)
	entries, err := stm.GetTodayInnerVoiceSummary()
	if err != nil {
		t.Fatalf("GetTodayInnerVoiceSummary: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetTodayInnerVoiceSummary_ReturnsToday(t *testing.T) {
	stm := newTestInnerVoiceDB(t)

	// Insert emotion_history row with inner voice (timestamp defaults to now)
	_, err := stm.db.Exec(
		`INSERT INTO emotion_history (description, primary_mood, secondary_mood, valence, arousal, confidence, cause, recommended_response_style)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"alert", "focused", "calm", 0.6, 0.4, 0.9, "work", "precise",
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := stm.StoreInnerVoice("Stay focused on the goal", "focus"); err != nil {
		t.Fatalf("StoreInnerVoice: %v", err)
	}

	entries, err := stm.GetTodayInnerVoiceSummary()
	if err != nil {
		t.Fatalf("GetTodayInnerVoiceSummary: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].InnerThought != "Stay focused on the goal" {
		t.Fatalf("unexpected thought: %q", entries[0].InnerThought)
	}
}

func TestFormatInnerVoiceHistory_Empty(t *testing.T) {
	result := FormatInnerVoiceHistory(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil, got %q", result)
	}
	result = FormatInnerVoiceHistory([]InnerVoiceEntry{})
	if result != "" {
		t.Fatalf("expected empty string for empty slice, got %q", result)
	}
}

func TestFormatInnerVoiceHistory_Formats(t *testing.T) {
	entries := []InnerVoiceEntry{
		{InnerThought: "Stay focused", NudgeCategory: "focus"},
		{InnerThought: "Be patient with the user", NudgeCategory: "patience"},
	}
	result := FormatInnerVoiceHistory(entries)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if !strings.Contains(result, "Stay focused") {
		t.Fatalf("expected 'Stay focused' in result, got %q", result)
	}
	if !strings.Contains(result, "[focus]") {
		t.Fatalf("expected '[focus]' in result, got %q", result)
	}
	if !strings.Contains(result, "[patience]") {
		t.Fatalf("expected '[patience]' in result, got %q", result)
	}
}

func TestFormatInnerVoiceHistory_Truncates(t *testing.T) {
	longThought := ""
	for i := 0; i < 200; i++ {
		longThought += "x"
	}
	entries := []InnerVoiceEntry{
		{InnerThought: longThought, NudgeCategory: "caution"},
	}
	result := FormatInnerVoiceHistory(entries)
	if !strings.Contains(result, "...") {
		t.Fatalf("expected truncation marker in result, got %q", result)
	}
}

func TestFormatInnerVoiceHistory_LimitsToThree(t *testing.T) {
	entries := []InnerVoiceEntry{
		{InnerThought: "first", NudgeCategory: "a"},
		{InnerThought: "second", NudgeCategory: "b"},
		{InnerThought: "third", NudgeCategory: "c"},
		{InnerThought: "fourth", NudgeCategory: "d"},
	}
	result := FormatInnerVoiceHistory(entries)
	if strings.Contains(result, "fourth") {
		t.Fatalf("expected only 3 entries, but 'fourth' appeared in result: %q", result)
	}
	if !strings.Contains(result, "first") {
		t.Fatalf("expected 'first' in result, got %q", result)
	}
}

