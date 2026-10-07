package memory

import "testing"

func TestExcludedArchiveRowsExpireAndIncompleteRowsSurvive(t *testing.T) {
	stm := newTestConsolidationDB(t)
	for _, msg := range []struct {
		session, role string
		internal      bool
	}{{"default", "tool", true}, {"maintenance", "assistant", true}, {"default", "user", false}} {
		if _, err := stm.InsertMessage(msg.session, msg.role, "synthetic event", false, msg.internal); err != nil {
			t.Fatal(err)
		}
	}
	for _, session := range []string{"default", "maintenance"} {
		if err := stm.Clear(session); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stm.FinalizeIneligibleConsolidationCandidates(); err != nil {
		t.Fatal(err)
	}
	rows, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil || len(rows) != 1 {
		t.Fatalf("ordinary candidate: %v %v", rows, err)
	}
	if err := stm.MarkConsolidationSuccess([]int64{rows[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := stm.db.Exec(`UPDATE archived_messages SET archived_at = datetime('now', '-60 days')`); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"pending", "processing", "failed", "done", "excluded"} {
		if _, err := stm.db.Exec(`INSERT INTO archived_messages(session_id, role, content, consolidation_status, consolidated, archived_at)
			VALUES ('default', 'user', ?, ?, 0, datetime('now', '-60 days'))`, status, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stm.db.Exec(`INSERT INTO archived_messages(session_id, role, content, consolidation_status, consolidated)
		VALUES ('default', 'tool', 'fresh excluded', 'excluded', 1)`); err != nil {
		t.Fatal(err)
	}
	removed, err := stm.CleanOldArchivedMessages(30)
	if err != nil || removed != 3 {
		t.Fatalf("terminal retention: removed=%d err=%v", removed, err)
	}
	var remaining int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages`).Scan(&remaining); err != nil || remaining != 6 {
		t.Fatalf("incomplete or fresh rows removed: remaining=%d err=%v", remaining, err)
	}
}
