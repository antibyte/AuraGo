package flows

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
)

// blockDBConn temporarily shortens only this checked-out connection's SQLite
// busy wait. The pool's original timeout is restored before returning it.
func blockDBConn(ctx context.Context, db *sql.DB) (*sql.Conn, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	var previous int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&previous); err != nil {
		conn.Close()
		return nil, nil, err
	}
	release := func() {
		if _, err := conn.ExecContext(context.Background(), fmt.Sprintf(`PRAGMA busy_timeout=%d`, previous)); err != nil {
			// Never return a connection with unknown per-connection policy to the pool.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout=50`); err != nil {
		release()
		return nil, nil, err
	}
	return conn, release, nil
}

func blockDBBusy(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code() & 0xff
		return code == 5 || code == 6
	}
	return false
}

func discardBlockDBConn(conn *sql.Conn) { _ = conn.Raw(func(any) error { return driver.ErrBadConn }) }

func waitBlockDBRetry(ctx context.Context) error {
	timer := time.NewTimer(25 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// beginBlockWrite retries only acquisition of a write lock, before any state
// change. Once a lock is acquired, later errors or an uncertain commit are not retried.
func beginBlockWrite(ctx context.Context, db *sql.DB, flowID string) (*sql.Tx, func(), error) {
	conn, release, err := blockDBConn(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	for {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			release()
			return nil, nil, err
		}
		res, err := tx.ExecContext(ctx, `UPDATE flows SET kind=kind WHERE id=? AND kind='flow'`, flowID)
		if err == nil {
			if err = requireRow(res, ErrNotFound); err == nil {
				return tx, release, nil
			}
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			discardBlockDBConn(conn)
			release()
			return nil, nil, errors.Join(err, rollbackErr)
		}
		if !blockDBBusy(err) {
			release()
			return nil, nil, err
		}
		if err := waitBlockDBRetry(ctx); err != nil {
			release()
			return nil, nil, err
		}
	}
}

// blockReadSnapshot retries a busy read-only snapshot without materializing a
// partial plan or invoking any node; cancellation bounds the whole operation.
func blockReadSnapshot(ctx context.Context, db *sql.DB, read func(*sql.Tx) error) error {
	conn, release, err := blockDBConn(ctx, db)
	if err != nil {
		return err
	}
	defer release()
	for {
		tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return err
		}
		err = read(tx)
		if err == nil {
			if err := tx.Commit(); err != nil {
				discardBlockDBConn(conn)
				return err
			}
			return nil
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			discardBlockDBConn(conn)
			return errors.Join(err, rollbackErr)
		}
		if !blockDBBusy(err) {
			return err
		}
		if err := waitBlockDBRetry(ctx); err != nil {
			return err
		}
	}
}
