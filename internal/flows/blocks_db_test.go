package flows

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBlockAdmissionRetriesBeforeWriteAndRestoresBusyTimeout(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	block := publishedGreetingBlock(t, s)
	parent, _ := blockParent(t, s, block, "Parent")
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	type answer struct {
		result StartResult
		err    error
	}
	done := make(chan answer, 1)
	go func() {
		result, err := s.runner.StartContext(ctx, StartRequest{Flow: parent.Draft, Revision: parent.DraftRevision, Mode: ModeTest, TriggerNode: parent.Draft.Nodes[0].ID})
		done <- answer{result, err}
	}()
	time.Sleep(150 * time.Millisecond)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("retry after lock release: %v", got.err)
		}
		if run := waitRun(t, s, got.result.RunID); run.Status != RunSuccess {
			t.Fatalf("run: %+v", run)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("admission did not recover after lock release")
	}
	var runs, pins int
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_runs WHERE flow_id=?`, parent.ID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := s.store.db.QueryRow(`SELECT count(*) FROM flow_block_run_pins`).Scan(&pins); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || pins != 1 {
		t.Fatalf("retry duplicated records: runs=%d pins=%d", runs, pins)
	}
	s.store.db.SetMaxOpenConns(1)
	s.store.db.SetMaxIdleConns(1)
	conn, release, err := blockDBConn(ctx, s.store.db)
	if err != nil {
		t.Fatal(err)
	}
	var timeout int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != 50 {
		release()
		t.Fatalf("short timeout: %d, %v", timeout, err)
	}
	release()
	conn, err = s.store.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatalf("restored timeout: %d, %v", timeout, err)
	}
}

type blockTimeoutFaultDriver struct{ opened, closed atomic.Int32 }
type blockTimeoutFaultConn struct{ owner *blockTimeoutFaultDriver }
type blockTimeoutRows struct{ sent bool }

func (d *blockTimeoutFaultDriver) Open(string) (driver.Conn, error) {
	d.opened.Add(1)
	return &blockTimeoutFaultConn{owner: d}, nil
}
func (c *blockTimeoutFaultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unsupported")
}
func (c *blockTimeoutFaultConn) Close() error              { c.owner.closed.Add(1); return nil }
func (c *blockTimeoutFaultConn) Begin() (driver.Tx, error) { return nil, errors.New("unsupported") }
func (c *blockTimeoutFaultConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &blockTimeoutRows{}, nil
}
func (c *blockTimeoutFaultConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "=5000") {
		return nil, errors.New("restore failed")
	}
	return driver.RowsAffected(0), nil
}
func (r *blockTimeoutRows) Columns() []string { return []string{"busy_timeout"} }
func (r *blockTimeoutRows) Close() error      { return nil }
func (r *blockTimeoutRows) Next(dest []driver.Value) error {
	if r.sent {
		return io.EOF
	}
	r.sent = true
	dest[0] = int64(5000)
	return nil
}

var blockTimeoutDriverID atomic.Int32

func TestBlockAdmissionDiscardsConnectionWhenTimeoutRestoreFails(t *testing.T) {
	d := &blockTimeoutFaultDriver{}
	name := fmt.Sprintf("flows-block-timeout-fault-%d", blockTimeoutDriverID.Add(1))
	sql.Register(name, d)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, release, err := blockDBConn(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if d.closed.Load() != 1 || db.Stats().Idle != 0 {
		t.Fatalf("bad connection retained: opened=%d closed=%d stats=%+v", d.opened.Load(), d.closed.Load(), db.Stats())
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if d.opened.Load() != 2 {
		t.Fatalf("expected fresh connection, opened=%d", d.opened.Load())
	}
}

type blockCommitFaultDriver struct{ opened, closed, commits atomic.Int32 }
type blockCommitFaultConn struct{ owner *blockCommitFaultDriver }
type blockCommitFaultTx struct{ owner *blockCommitFaultDriver }

func (d *blockCommitFaultDriver) Open(string) (driver.Conn, error) {
	d.opened.Add(1)
	return &blockCommitFaultConn{owner: d}, nil
}
func (c *blockCommitFaultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unsupported")
}
func (c *blockCommitFaultConn) Close() error { c.owner.closed.Add(1); return nil }
func (c *blockCommitFaultConn) Begin() (driver.Tx, error) {
	return &blockCommitFaultTx{owner: c.owner}, nil
}
func (c *blockCommitFaultConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &blockCommitFaultTx{owner: c.owner}, nil
}
func (c *blockCommitFaultConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &blockTimeoutRows{}, nil
}
func (c *blockCommitFaultConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}
func (tx *blockCommitFaultTx) Commit() error {
	tx.owner.commits.Add(1)
	return errors.New("commit failed")
}
func (tx *blockCommitFaultTx) Rollback() error { return nil }

func TestBlockReadSnapshotNeverRetriesFailedCommit(t *testing.T) {
	d := &blockCommitFaultDriver{}
	name := fmt.Sprintf("flows-block-commit-fault-%d", blockTimeoutDriverID.Add(1))
	sql.Register(name, d)
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var reads int
	err = blockReadSnapshot(context.Background(), db, func(*sql.Tx) error { reads++; return nil })
	if err == nil || reads != 1 || d.commits.Load() != 1 || d.closed.Load() != 1 || db.Stats().Idle != 0 {
		t.Fatalf("failed commit was retried or pooled: err=%v reads=%d commits=%d closed=%d stats=%+v", err, reads, d.commits.Load(), d.closed.Load(), db.Stats())
	}
}
