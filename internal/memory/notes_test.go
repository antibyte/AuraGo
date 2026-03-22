package memory

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func newTestNotesDB(t *testing.T) *SQLiteMemory {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	return stm
}

// TestAddNoteUTF8Truncation verifies that truncation of long content does not
// split multi-byte UTF-8 sequences (e.g. emoji, Chinese characters).
func TestAddNoteUTF8Truncation(t *testing.T) {
	stm := newTestNotesDB(t)

	// Build a string that is slightly over maxNoteContentLen runes,
	// using multi-byte characters (日 is 3 bytes in UTF-8).
	runeCount := maxNoteContentLen + 10
	var b strings.Builder
	for i := 0; i < runeCount; i++ {
		b.WriteRune('日')
	}
	longContent := b.String()

	id, err := stm.AddNote("test", "title", longContent, 2, "")
	if err != nil {
		t.Fatalf("AddNote: %v", err)
	}

	notes, err := stm.ListNotes("test", -1)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 || notes[0].ID != id {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}

	content := notes[0].Content
	// Content must be valid UTF-8
	if !utf8.ValidString(content) {
		t.Error("stored content is not valid UTF-8 after truncation")
	}
	// Rune count must not exceed the limit
	rc := utf8.RuneCountInString(content)
	if rc > maxNoteContentLen {
		t.Errorf("content rune count %d exceeds maxNoteContentLen %d", rc, maxNoteContentLen)
	}
}

// TestAddNoteTitleUTF8Truncation verifies that title truncation is also rune-safe.
func TestAddNoteTitleUTF8Truncation(t *testing.T) {
	stm := newTestNotesDB(t)

	runeCount := maxNoteTitleLen + 5
	var b strings.Builder
	for i := 0; i < runeCount; i++ {
		b.WriteRune('ä') // 2 bytes in UTF-8
	}
	longTitle := b.String()

	_, err := stm.AddNote("test", longTitle, "content", 2, "")
	if err != nil {
		t.Fatalf("AddNote with long title: %v", err)
	}

	notes, err := stm.ListNotes("test", -1)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	title := notes[0].Title
	if !utf8.ValidString(title) {
		t.Error("stored title is not valid UTF-8 after truncation")
	}
	if utf8.RuneCountInString(title) > maxNoteTitleLen {
		t.Errorf("title rune count exceeds maxNoteTitleLen")
	}
}
