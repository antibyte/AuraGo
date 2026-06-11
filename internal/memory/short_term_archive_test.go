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
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived messages, got %d", len(archived))
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
	if len(archived) != 2 {
		t.Fatalf("expected 2 archived messages after clear, got %d", len(archived))
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
	if len(archived) < 3 {
		t.Fatalf("expected at least 3 archived messages, got %d", len(archived))
	}
}