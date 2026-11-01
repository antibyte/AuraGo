package memory

import "testing"

func TestDeleteMessagesByIDArchives(t *testing.T) {
	stm := newTestConsolidationDB(t)

	id1, err := stm.InsertMessage("default", "user", "old user", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	id2, err := stm.InsertMessage("default", "assistant", "old assistant", false, false)
	if err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if _, err := stm.InsertMessage("default", "user", "keep me", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	if err := stm.DeleteMessagesByID("default", []int64{id1, id2}); err != nil {
		t.Fatalf("DeleteMessagesByID: %v", err)
	}

	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if got := countArchivedMessages(t, stm); got != 2 {
		t.Fatalf("expected both messages archived, got %d", got)
	}
	if got := countArchivedMessage(t, stm, "assistant", "old assistant"); got != 1 {
		t.Fatalf("assistant archive row was lost: count=%d", got)
	}
	if len(archived) != 1 || archived[0].Content != "old user" {
		t.Fatalf("expected only user-origin messages eligible for consolidation, got %+v", archived)
	}

	msgs, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "keep me" {
		t.Fatalf("unexpected remaining messages: %+v", msgs)
	}
}

func TestClearArchives(t *testing.T) {
	stm := newTestConsolidationDB(t)

	if _, err := stm.InsertMessage("sess-a", "user", "remember this", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}
	if _, err := stm.InsertMessage("sess-a", "assistant", "ack", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	if err := stm.Clear("sess-a"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if got := countArchivedMessages(t, stm); got != 2 {
		t.Fatalf("expected both messages retained in archive after clear, got %d", got)
	}
	if got := countArchivedMessage(t, stm, "assistant", "ack"); got != 1 {
		t.Fatalf("assistant archive row was lost by clear: count=%d", got)
	}
	if len(archived) != 1 || archived[0].Content != "remember this" {
		t.Fatalf("expected only user-origin messages eligible after clear, got %+v", archived)
	}
}

func TestClearSessionArchives(t *testing.T) {
	stm := newTestConsolidationDB(t)

	sess, err := stm.CreateChatSession()
	if err != nil {
		t.Fatalf("CreateChatSession: %v", err)
	}
	if _, err := stm.InsertMessage(sess.ID, "user", "session clear me", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	if err := stm.ClearSession(sess.ID); err != nil {
		t.Fatalf("ClearSession: %v", err)
	}

	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived message after ClearSession, got %d", len(archived))
	}
}

func TestEnforceSTMPRetention(t *testing.T) {
	stm := newTestConsolidationDB(t)

	for i := 0; i < 5; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		if _, err := stm.InsertMessage("default", role, "msg", false, false); err != nil {
			t.Fatalf("InsertMessage: %v", err)
		}
	}
	if _, err := stm.InsertMessage("other", "user", "other session", false, false); err != nil {
		t.Fatalf("InsertMessage: %v", err)
	}

	sessions, err := stm.EnforceSTMPRetention(2)
	if err != nil {
		t.Fatalf("EnforceSTMPRetention: %v", err)
	}
	if sessions != 2 {
		t.Fatalf("expected retention on 2 sessions, got %d", sessions)
	}

	defaultMsgs, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages default: %v", err)
	}
	if len(defaultMsgs) != 2 {
		t.Fatalf("expected 2 remaining default messages, got %d", len(defaultMsgs))
	}

	archived, err := stm.GetUnconsolidatedMessages(100)
	if err != nil {
		t.Fatalf("GetUnconsolidatedMessages: %v", err)
	}
	if got := countArchivedMessages(t, stm); got != 3 {
		t.Fatalf("expected all three evicted messages preserved in archive, got %d", got)
	}
	if got := countArchivedMessage(t, stm, "assistant", "msg"); got != 1 {
		t.Fatalf("evicted assistant row was lost: count=%d", got)
	}
	if len(archived) != 2 {
		t.Fatalf("expected the two archived user-origin messages eligible for consolidation, got %d", len(archived))
	}
}

func countArchivedMessages(t *testing.T, stm *SQLiteMemory) int {
	t.Helper()
	var count int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func countArchivedMessage(t *testing.T, stm *SQLiteMemory, role, content string) int {
	t.Helper()
	var count int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages WHERE role=? AND content=?`, role, content).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
