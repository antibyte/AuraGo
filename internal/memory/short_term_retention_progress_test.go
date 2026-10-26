package memory

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSTMRetentionPrunesByTimestampRatherThanID(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*SQLiteMemory, string) error
	}{
		{
			name: "global",
			run: func(stm *SQLiteMemory, session string) error {
				_, err := stm.EnforceSTMPRetentionContext(context.Background(), 2, 100)
				return err
			},
		},
		{
			name: "per-session",
			run: func(stm *SQLiteMemory, session string) error {
				return stm.EnforceSTMPRetentionForSessionContext(context.Background(), session, 2, 100)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stm := newMaintenanceProgressStore(t, ":memory:")
			session := "out-of-order"
			for _, row := range []struct {
				content string
				day     int
			}{
				{content: "newest", day: 3},
				{content: "oldest", day: 1},
				{content: "middle", day: 2},
			} {
				stamp := time.Date(2026, time.January, row.day, 0, 0, 0, 0, time.UTC)
				if _, err := stm.db.Exec(`INSERT INTO messages(session_id,role,content,timestamp) VALUES(?,?,?,?)`, session, "user", row.content, stamp); err != nil {
					t.Fatalf("insert %s: %v", row.content, err)
				}
			}
			if err := test.run(stm, session); err != nil {
				t.Fatalf("retention: %v", err)
			}
			rows, err := stm.db.Query(`SELECT content FROM messages WHERE session_id=? ORDER BY timestamp DESC,id DESC`, session)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var remaining []string
			for rows.Next() {
				var content string
				if err := rows.Scan(&content); err != nil {
					t.Fatal(err)
				}
				remaining = append(remaining, content)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if len(remaining) != 2 || remaining[0] != "newest" || remaining[1] != "middle" {
				t.Fatalf("remaining messages=%v; want [newest middle]", remaining)
			}
		})
	}
}

func TestSTMRetentionReturnedSessionCountExcludesRolledBackPage(t *testing.T) {
	stm := newMaintenanceProgressStore(t, ":memory:")
	for i := 0; i < stmRetentionPageSize; i++ {
		if _, err := stm.db.Exec(`INSERT INTO messages(session_id,role,content,timestamp) VALUES(?,?,?,?)`, fmt.Sprintf("page-%03d", i), "user", "single", "2026-01-01 00:00:00"); err != nil {
			t.Fatal(err)
		}
	}
	for i, stamp := range []string{"2026-01-01 00:00:00", "2026-01-02 00:00:00"} {
		if _, err := stm.db.Exec(`INSERT INTO messages(session_id,role,content,timestamp) VALUES('rollback-page','user',?,?)`, fmt.Sprintf("ordered-%d", i), stamp); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := stm.db.Exec(`CREATE TRIGGER fail_retention_archive BEFORE INSERT ON archived_messages WHEN NEW.session_id='rollback-page' BEGIN SELECT RAISE(ABORT,'injected retention failure'); END`); err != nil {
		t.Fatal(err)
	}
	sessions, err := stm.EnforceSTMPRetentionContext(context.Background(), 1, stmRetentionPageSize+1)
	if err == nil {
		t.Fatal("retention error=nil; want injected archive failure")
	}
	if sessions != stmRetentionPageSize {
		t.Fatalf("processed session count=%d; want only %d committed page sessions", sessions, stmRetentionPageSize)
	}
	var rollbackRows int
	if err := stm.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id='rollback-page'`).Scan(&rollbackRows); err != nil || rollbackRows != 2 {
		t.Fatalf("rolled back page left %d rows, err=%v; want 2", rollbackRows, err)
	}
}

func TestSTMRetentionUsesSQLiteTimestampTextForCutoff(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(*SQLiteMemory, string) error
	}{
		{
			name: "global",
			run: func(stm *SQLiteMemory, _ string) error {
				_, err := stm.EnforceSTMPRetentionContext(context.Background(), 2, 100)
				return err
			},
		},
		{
			name: "per-session",
			run: func(stm *SQLiteMemory, session string) error {
				return stm.EnforceSTMPRetentionForSessionContext(context.Background(), session, 2, 100)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stm := newMaintenanceProgressStore(t, ":memory:")
			const session = "sqlite-timestamp-text"
			for index := 0; index < 4; index++ {
				if _, err := stm.db.Exec(`INSERT INTO messages(session_id,role,content,timestamp) VALUES(?,?,?,CURRENT_TIMESTAMP)`, session, "user", fmt.Sprintf("message-%d", index)); err != nil {
					t.Fatalf("insert message %d: %v", index, err)
				}
			}
			if err := test.run(stm, session); err != nil {
				t.Fatalf("retention: %v", err)
			}
			var active, archived int
			if err := stm.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE session_id=?`, session).Scan(&active); err != nil {
				t.Fatal(err)
			}
			if err := stm.db.QueryRow(`SELECT COUNT(*) FROM archived_messages WHERE session_id=?`, session).Scan(&archived); err != nil {
				t.Fatal(err)
			}
			if active != 2 || archived != 2 {
				t.Fatalf("active=%d archived=%d; want newest 2 active and 2 archived", active, archived)
			}
		})
	}
}
