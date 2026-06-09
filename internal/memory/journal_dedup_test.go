package memory

import (
	"testing"
)

func TestJournalErrorRecentlyLogged(t *testing.T) {
	stm := newTestJournalDB(t)

	title := "Error in send_telegram"
	content := "message is required"
	if _, err := stm.InsertJournalEntry(JournalEntry{
		EntryType: "error_learned",
		Title:     title,
		Content:   content,
	}); err != nil {
		t.Fatalf("InsertJournalEntry: %v", err)
	}

	logged, err := stm.JournalErrorRecentlyLogged("error_learned", title, content, 24)
	if err != nil {
		t.Fatalf("JournalErrorRecentlyLogged: %v", err)
	}
	if !logged {
		t.Fatal("expected recent identical journal error to be detected")
	}

	logged, err = stm.JournalErrorRecentlyLogged("error_learned", title, "different error", 24)
	if err != nil {
		t.Fatalf("JournalErrorRecentlyLogged different content: %v", err)
	}
	if logged {
		t.Fatal("expected different content not to match")
	}
}