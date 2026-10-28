package dockerstacks

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/dbutil"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := dbutil.Open(filepath.Join(t.TempDir(), "docker_manager.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newTestJournal(t *testing.T) *journal {
	t.Helper()
	j, err := newJournal(openTestDB(t))
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func testOp(id, host, stack, kind, state string, created time.Time) Operation {
	return Operation{ID: id, HostID: host, Stack: stack, Kind: kind, State: state, CreatedAt: created}
}

func TestJournalRoundTripKeepsEveryField(t *testing.T) {
	ctx := context.Background()
	j := newTestJournal(t)
	created := time.Date(2026, 10, 27, 12, 0, 0, 123456789, time.UTC)
	started := created.Add(time.Second)
	finished := created.Add(time.Minute)
	exit := 17

	op := testOp("op_0123456789abcdef", "h_1", "web", KindDown, StateQueued, created)
	op.Options = OpOptions{Volumes: true}
	if err := j.insert(ctx, op); err != nil {
		t.Fatal(err)
	}
	got, err := j.get(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateQueued || got.StartedAt != nil || got.FinishedAt != nil || got.ExitCode != nil || !got.Options.Volumes {
		t.Fatalf("inserted row = %+v", got)
	}

	op.State, op.RunnerID, op.ExitCode, op.Code, op.Error, op.ErrorLine = StateFailed, "runner-id", &exit, "runner_failed", "yaml: line 12: boom", 12
	op.Warning, op.Output, op.StartedAt, op.FinishedAt = "db is unhealthy", "line one\nline two\n", &started, &finished
	op.Ports = []PortLink{{Service: "web", PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"}}
	if err := j.save(ctx, op); err != nil {
		t.Fatal(err)
	}
	got, err = j.get(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateFailed || got.RunnerID != "runner-id" || got.ExitCode == nil || *got.ExitCode != 17 ||
		got.Code != "runner_failed" || got.Error != "yaml: line 12: boom" || got.ErrorLine != 12 ||
		got.Warning != "db is unhealthy" || got.Output != "line one\nline two\n" ||
		len(got.Ports) != 1 || got.Ports[0].PublicPort != 8080 {
		t.Fatalf("saved row = %+v", got)
	}
	if !got.CreatedAt.Equal(created) || got.StartedAt == nil || !got.StartedAt.Equal(started) || got.FinishedAt == nil || !got.FinishedAt.Equal(finished) {
		t.Fatalf("times = %v %v %v", got.CreatedAt, got.StartedAt, got.FinishedAt)
	}
	if !got.Done() {
		t.Fatal("a failed operation must report Done")
	}

	if _, err := j.get(ctx, "op_missing"); CodeOf(err) != CodeOperationNotFound {
		t.Fatalf("get of an unknown id = %v, want %s", err, CodeOperationNotFound)
	}
}

func TestJournalListFiltersAndOrdersNewestFirst(t *testing.T) {
	ctx := context.Background()
	j := newTestJournal(t)
	base := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	rows := []Operation{
		testOp("op_01", "h_1", "web", KindUp, StateSucceeded, base),
		testOp("op_02", "h_1", "web", KindDown, StateRunning, base.Add(time.Second)),
		testOp("op_03", "h_1", "db", KindUp, StateQueued, base.Add(2*time.Second)),
		testOp("op_04", "h_2", "web", KindUp, StateHealthWait, base.Add(3*time.Second)),
		testOp("op_05", "h_1", "", KindPrepare, StateFailed, base.Add(4*time.Second)),
	}
	for _, op := range rows {
		op.Output = "must not be listed"
		if err := j.insert(ctx, op); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(f OperationFilter) string {
		ops, err := j.list(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		s := ""
		for _, op := range ops {
			if op.Output != "" {
				t.Fatalf("list returned the output of %s", op.ID)
			}
			s += op.ID[3:] + " "
		}
		return s
	}
	if got := ids(OperationFilter{}); got != "05 04 03 02 01 " {
		t.Fatalf("all = %q, want newest first", got)
	}
	if got := ids(OperationFilter{HostID: "h_1", Stack: "web"}); got != "02 01 " {
		t.Fatalf("host+stack = %q", got)
	}
	if got := ids(OperationFilter{Active: true}); got != "04 03 02 " {
		t.Fatalf("active = %q, want queued, running and health_wait only", got)
	}
	if got := ids(OperationFilter{Limit: 2}); got != "05 04 " {
		t.Fatalf("limit 2 = %q", got)
	}
	active, err := j.active(ctx)
	if err != nil || len(active) != 3 || active[0].ID != "op_02" || active[0].Output != "must not be listed" {
		t.Fatalf("active() = (%v, %v), want three rows oldest first, with their output", active, err)
	}
}

func TestJournalPruneKeepsTheNewest50AndNeverActiveRows(t *testing.T) {
	ctx := context.Background()
	j := newTestJournal(t)
	base := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		state := StateSucceeded
		if i == 0 {
			state = StateRunning // the oldest row is still active and must survive
		}
		if err := j.insert(ctx, testOp(fmt.Sprintf("op_%02d", i), "h_1", "web", KindUp, state, base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.insert(ctx, testOp("op_other", "h_1", "db", KindUp, StateSucceeded, base)); err != nil {
		t.Fatal(err)
	}
	if err := j.prune(ctx, "h_1", "web", journalKeep); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := j.db.QueryRow("SELECT count(*) FROM docker_operations WHERE host_id = 'h_1' AND stack = 'web'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 51 { // the newest 50 plus the old active row
		t.Fatalf("rows after prune = %d, want 51", n)
	}
	if _, err := j.get(ctx, "op_00"); err != nil {
		t.Fatalf("the active row was pruned: %v", err)
	}
	if _, err := j.get(ctx, "op_09"); CodeOf(err) != CodeOperationNotFound {
		t.Fatalf("op_09 is older than the newest 50 and must be gone, got %v", err)
	}
	if _, err := j.get(ctx, "op_10"); err != nil {
		t.Fatalf("op_10 is the 50th newest and must stay: %v", err)
	}
	if _, err := j.get(ctx, "op_other"); err != nil {
		t.Fatalf("another stack's row must be untouched: %v", err)
	}
}

func TestJournalSchemaIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 2; i++ {
		if _, err := newJournal(db); err != nil {
			t.Fatalf("newJournal #%d: %v", i+1, err)
		}
	}
}
