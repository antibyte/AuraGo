package memory

import (
	"io"
	"log/slog"
	"testing"
)

func TestPurgeChatSessionRemovesTranscriptBearingRecords(t *testing.T) {
	store, err := NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.InitJournalTables(); err != nil {
		t.Fatal(err)
	}
	sessionID := "sip-private-call"
	statements := []string{
		`INSERT INTO chat_sessions(id) VALUES(?)`,
		`INSERT INTO messages(session_id, role, content) VALUES(?, 'user', 'private transcript')`,
		`INSERT INTO archived_messages(session_id, role, content) VALUES(?, 'assistant', 'private transcript')`,
		`INSERT INTO compressed_tool_outputs(session_id, tool_call_id, tool_name, original_content, compressed_content) VALUES(?, 'call-1', 'status', 'private output', 'private output')`,
		`INSERT INTO activity_turns(date, session_id, user_request) VALUES('2026-07-22', ?, 'private transcript')`,
		`INSERT INTO audit_events(timestamp, session_id, summary) VALUES(CURRENT_TIMESTAMP, ?, 'private transcript')`,
		`INSERT INTO journal_entries(entry_type, title, content, date, session_id) VALUES('chat', 'call', 'private transcript', '2026-07-22', ?)`,
		`INSERT INTO episodic_memories(event_date, title, summary, session_id) VALUES('2026-07-22', 'call', 'private transcript', ?)`,
	}
	for _, statement := range statements {
		if _, err := store.db.Exec(statement, sessionID); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}
	if err := store.PurgeChatSession(sessionID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"messages", "archived_messages", "compressed_tool_outputs", "activity_turns", "audit_events", "journal_entries", "episodic_memories"} {
		var count int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE session_id = ?`, sessionID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s retained %d transient records", table, count)
		}
	}
	var sessions int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM chat_sessions WHERE id = ?`, sessionID).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("chat session retained count=%d err=%v", sessions, err)
	}
}
