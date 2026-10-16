package mailer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/dbutil"
	_ "modernc.org/sqlite"
)

const schemaVersion = 8

var (
	ErrConflict    = errors.New("mailer revision conflict")
	ErrNotFound    = errors.New("mailer item not found")
	ErrReadOnly    = errors.New("mailer is read-only")
	ErrCapability  = errors.New("mailer operation is not supported by this account")
	ErrInvalid     = errors.New("invalid mailer request")
	ErrSyncFailed  = errors.New("mail account sync failed")
	ErrStorageFull = errors.New("mailer storage quota exceeded")
)

type Store struct{ db *sql.DB }

func OpenStore(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create mailer data directory: %w", err)
		}
	}
	db, err := dbutil.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mailer database: %w", err)
	}
	closeOnError := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	if path != ":memory:" {
		if err := os.Chmod(path, 0o600); err != nil {
			return closeOnError(fmt.Errorf("restrict mailer database permissions: %w", err))
		}
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return closeOnError(fmt.Errorf("read mailer schema version: %w", err))
	}
	if version > schemaVersion {
		return closeOnError(fmt.Errorf("mailer database schema %d is newer than supported schema %d", version, schemaVersion))
	}
	if version > 0 && version < schemaVersion && path != ":memory:" {
		backup := fmt.Sprintf("%s.pre-v%d-%d.bak", path, schemaVersion, time.Now().UTC().UnixNano())
		if _, err := db.Exec("VACUUM INTO ?", backup); err != nil {
			return closeOnError(fmt.Errorf("back up mailer database before migration: %w", err))
		}
		if err := os.Chmod(backup, 0o600); err != nil {
			return closeOnError(fmt.Errorf("restrict mailer migration backup permissions: %w", err))
		}
	}
	if version < schemaVersion {
		tx, err := db.Begin()
		if err != nil {
			return closeOnError(fmt.Errorf("begin mailer schema migration: %w", err))
		}
		if _, err = tx.Exec(mailSchema); err == nil && version > 0 {
			err = ensureColumn(context.Background(), tx, "mailer_messages", "raw", "BLOB")
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_cursors", "uid_validity", "INTEGER NOT NULL DEFAULT 0")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_accounts", "protocol", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_accounts", "sync_status", "TEXT NOT NULL DEFAULT 'idle'")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_accounts", "sync_error", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_accounts", "sync_checked_at", "INTEGER NOT NULL DEFAULT 0")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_outbox", "timezone", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_outbox", "append_error", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_blobs", "owner_kind", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_blobs", "owner_id", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_messages", "message_id", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = ensureColumn(context.Background(), tx, "mailer_messages", "in_reply_to", "TEXT NOT NULL DEFAULT ''")
			}
			if err == nil {
				err = backfillMessageHeaders(context.Background(), tx)
			}
		}
		if err == nil {
			_, err = tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", schemaVersion))
		}
		if err != nil {
			_ = tx.Rollback()
			return closeOnError(fmt.Errorf("migrate mailer database: %w", err))
		}
		if err := tx.Commit(); err != nil {
			return closeOnError(fmt.Errorf("commit mailer database migration: %w", err))
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS mailer_messages_message_id ON mailer_messages(account_id,message_id);
CREATE INDEX IF NOT EXISTS mailer_messages_in_reply_to ON mailer_messages(account_id,in_reply_to);
CREATE INDEX IF NOT EXISTS mailer_message_refs_by_reference ON mailer_message_refs(account_id,referenced_id);`); err != nil {
		return closeOnError(fmt.Errorf("index mailer message headers: %w", err))
	}
	store := &Store{db: db}
	if err := store.RecoverSending(context.Background()); err != nil {
		return closeOnError(err)
	}
	if err := store.RecoverRuleRuns(context.Background()); err != nil {
		return closeOnError(err)
	}
	return store, nil
}

const mailSchema = `
CREATE TABLE IF NOT EXISTS mailer_settings (id INTEGER PRIMARY KEY CHECK(id=1), body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_accounts (id TEXT PRIMARY KEY, source TEXT NOT NULL, protocol TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, address TEXT NOT NULL, capabilities BLOB NOT NULL, disabled INTEGER NOT NULL, reason TEXT NOT NULL, agent_access INTEGER NOT NULL, read_only INTEGER NOT NULL, sync_status TEXT NOT NULL DEFAULT 'idle', sync_error TEXT NOT NULL DEFAULT '', sync_checked_at INTEGER NOT NULL DEFAULT 0, linked INTEGER NOT NULL DEFAULT 1);
CREATE TABLE IF NOT EXISTS mailer_account_settings (account_id TEXT PRIMARY KEY REFERENCES mailer_accounts(id) ON DELETE CASCADE, body BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_folders (account_id TEXT NOT NULL REFERENCES mailer_accounts(id) ON DELETE CASCADE, id TEXT NOT NULL, provider_id TEXT NOT NULL, name TEXT NOT NULL, kind TEXT NOT NULL, read_only INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(account_id,id));
CREATE TABLE IF NOT EXISTS mailer_cursors (account_id TEXT NOT NULL, folder_id TEXT NOT NULL, cursor TEXT NOT NULL, uid_validity INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL, PRIMARY KEY(account_id,folder_id));
CREATE TABLE IF NOT EXISTS mailer_messages (id TEXT PRIMARY KEY, account_id TEXT NOT NULL, folder_id TEXT NOT NULL, remote_id TEXT NOT NULL, thread_id TEXT NOT NULL, message_id TEXT NOT NULL DEFAULT '', in_reply_to TEXT NOT NULL DEFAULT '', subject TEXT NOT NULL, sender TEXT NOT NULL, date INTEGER NOT NULL, unread INTEGER NOT NULL, starred INTEGER NOT NULL, labels BLOB NOT NULL, snoozed_until INTEGER NOT NULL DEFAULT 0, body BLOB NOT NULL, raw BLOB, UNIQUE(account_id,folder_id,remote_id));
CREATE INDEX IF NOT EXISTS mailer_messages_folder_date ON mailer_messages(account_id,folder_id,date DESC,id DESC);
CREATE INDEX IF NOT EXISTS mailer_messages_thread ON mailer_messages(account_id,thread_id,date,id);
CREATE TABLE IF NOT EXISTS mailer_message_refs (account_id TEXT NOT NULL, referenced_id TEXT NOT NULL, message_id TEXT NOT NULL, PRIMARY KEY(account_id,referenced_id,message_id));
CREATE INDEX IF NOT EXISTS mailer_messages_unread ON mailer_messages(account_id,folder_id,unread,starred);
CREATE TABLE IF NOT EXISTS mailer_drafts (id TEXT PRIMARY KEY, account_id TEXT NOT NULL, revision INTEGER NOT NULL, body BLOB NOT NULL, updated_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS mailer_drafts_account ON mailer_drafts(account_id,updated_at DESC);
CREATE TABLE IF NOT EXISTS mailer_templates (id TEXT PRIMARY KEY, account_id TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL, name TEXT NOT NULL, revision INTEGER NOT NULL, body BLOB NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_rules (id TEXT PRIMARY KEY, account_id TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL, enabled INTEGER NOT NULL, body BLOB NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_rule_runs (message_id TEXT NOT NULL, account_id TEXT NOT NULL, rule_id TEXT NOT NULL, rule_revision INTEGER NOT NULL, rule_body BLOB NOT NULL, action_index INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '', updated_at INTEGER NOT NULL, PRIMARY KEY(message_id,rule_id));
CREATE INDEX IF NOT EXISTS mailer_rule_runs_pending ON mailer_rule_runs(account_id,status,updated_at);
CREATE TABLE IF NOT EXISTS mailer_outbox (id TEXT PRIMARY KEY, account_id TEXT NOT NULL, draft_id TEXT NOT NULL, revision INTEGER NOT NULL, timezone TEXT NOT NULL DEFAULT '', send_at INTEGER NOT NULL, undo_until INTEGER NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '', remote_id TEXT NOT NULL DEFAULT '', append_error TEXT NOT NULL DEFAULT '', body BLOB NOT NULL, created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS mailer_outbox_due ON mailer_outbox(status,send_at,created_at);
CREATE UNIQUE INDEX IF NOT EXISTS mailer_outbox_revision ON mailer_outbox(draft_id,revision) WHERE status!='cancelled';
CREATE TABLE IF NOT EXISTS mailer_outbox_blobs (outbox_id TEXT NOT NULL, blob_id TEXT NOT NULL, PRIMARY KEY(outbox_id,blob_id));
CREATE TABLE IF NOT EXISTS mailer_blobs (id TEXT PRIMARY KEY, filename TEXT NOT NULL, content_type TEXT NOT NULL, size INTEGER NOT NULL, digest TEXT NOT NULL, owner_kind TEXT NOT NULL, owner_id TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_snoozes (message_id TEXT PRIMARY KEY REFERENCES mailer_messages(id) ON DELETE CASCADE, until_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS mailer_snapshot_seen (account_id TEXT NOT NULL, folder_id TEXT NOT NULL, remote_id TEXT NOT NULL, PRIMARY KEY(account_id,folder_id,remote_id));
CREATE TABLE IF NOT EXISTS mailer_snapshots (account_id TEXT NOT NULL, folder_id TEXT NOT NULL, active INTEGER NOT NULL DEFAULT 1, PRIMARY KEY(account_id,folder_id));
`

func ensureColumn(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect mailer schema %s: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notnull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition); err != nil {
		return fmt.Errorf("add mailer schema column %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func marshal(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode mailer data: %w", err)
	}
	return b, nil
}

func unmarshal(b []byte, v any) error {
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decode mailer data: %w", err)
	}
	return nil
}

func timeNS(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixNano()
}

func fromNS(ns int64) time.Time {
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns).UTC()
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var b []byte
	if err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_settings WHERE id=1").Scan(&b); errors.Is(err, sql.ErrNoRows) {
		return Settings{}, nil
	} else if err != nil {
		return Settings{}, fmt.Errorf("read mailer settings: %w", err)
	}
	var settings Settings
	if err := unmarshal(b, &settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func (s *Store) UpdateSettings(ctx context.Context, settings Settings) error {
	b, err := marshal(settings)
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "INSERT INTO mailer_settings(id,body) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", b); err != nil {
		return fmt.Errorf("save mailer settings: %w", err)
	}
	return nil
}

func (s *Store) PutAccount(ctx context.Context, account Account) error {
	capabilities, err := marshal(account.Capabilities)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO mailer_accounts(id,source,protocol,name,address,capabilities,disabled,reason,agent_access,read_only,linked)
VALUES(?,?,?,?,?,?,?,?,?,?,1) ON CONFLICT(id) DO UPDATE SET source=excluded.source,protocol=excluded.protocol,name=excluded.name,address=excluded.address,capabilities=excluded.capabilities,disabled=excluded.disabled,reason=excluded.reason,agent_access=excluded.agent_access,read_only=excluded.read_only,linked=1`,
		account.ID, account.Source, account.Protocol, account.Name, account.Address, capabilities, account.Disabled, account.Reason, account.AgentAccess, account.ReadOnly)
	if err != nil {
		return fmt.Errorf("save linked mail account: %w", err)
	}
	return nil
}

func (s *Store) LinkedAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source,protocol,name,address,capabilities,disabled,reason,agent_access,read_only,sync_status,sync_error,sync_checked_at FROM mailer_accounts WHERE linked=1 ORDER BY name,id`)
	if err != nil {
		return nil, fmt.Errorf("list linked mail accounts: %w", err)
	}
	defer rows.Close()
	accounts := make([]Account, 0)
	for rows.Next() {
		var a Account
		var capabilities []byte
		var syncAt int64
		if err := rows.Scan(&a.ID, &a.Source, &a.Protocol, &a.Name, &a.Address, &capabilities, &a.Disabled, &a.Reason, &a.AgentAccess, &a.ReadOnly, &a.SyncStatus, &a.SyncError, &syncAt); err != nil {
			return nil, err
		}
		a.SyncCheckedAt = fromNS(syncAt)
		if err := unmarshal(capabilities, &a.Capabilities); err != nil {
			return nil, err
		}
		a.Linked = true
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *Store) UnlinkAccount(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE mailer_rule_runs SET status='skipped',error='account was disconnected',updated_at=?
WHERE account_id=? AND status='pending'`, timeNS(time.Now()), id); err != nil {
		return fmt.Errorf("skip disconnected account rule actions: %w", err)
	}
	res, err := tx.ExecContext(ctx, "UPDATE mailer_accounts SET linked=0 WHERE id=? AND linked=1", id)
	if err != nil {
		return fmt.Errorf("unlink mail account: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) IsLinked(ctx context.Context, id string) (bool, error) {
	var linked bool
	err := s.db.QueryRowContext(ctx, "SELECT linked FROM mailer_accounts WHERE id=?", id).Scan(&linked)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return linked, err
}

func (s *Store) AccountSettings(ctx context.Context, id string) (AccountSettings, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, "SELECT body FROM mailer_account_settings WHERE account_id=?", id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountSettings{}, nil
	}
	if err != nil {
		return AccountSettings{}, fmt.Errorf("read account settings: %w", err)
	}
	var settings AccountSettings
	if err := unmarshal(b, &settings); err != nil {
		return AccountSettings{}, err
	}
	return settings, nil
}

func (s *Store) SetAccountSettings(ctx context.Context, id string, settings AccountSettings) error {
	b, err := marshal(settings)
	if err != nil {
		return err
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO mailer_account_settings(account_id,body) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET body=excluded.body`, id, b); err != nil {
		return fmt.Errorf("save account settings: %w", err)
	}
	return nil
}

func (s *Store) Account(ctx context.Context, id string) (Account, error) {
	var a Account
	var capabilities []byte
	var syncAt int64
	err := s.db.QueryRowContext(ctx, "SELECT id,source,protocol,name,address,capabilities,disabled,reason,agent_access,read_only,sync_status,sync_error,sync_checked_at,linked FROM mailer_accounts WHERE id=?", id).Scan(&a.ID, &a.Source, &a.Protocol, &a.Name, &a.Address, &capabilities, &a.Disabled, &a.Reason, &a.AgentAccess, &a.ReadOnly, &a.SyncStatus, &a.SyncError, &syncAt, &a.Linked)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	if err := unmarshal(capabilities, &a.Capabilities); err != nil {
		return Account{}, err
	}
	a.SyncCheckedAt = fromNS(syncAt)
	return a, nil
}

func (s *Store) SetAccountSyncStatus(ctx context.Context, id, status, code string, at time.Time) error {
	if status != "idle" && status != "ok" && status != "error" {
		return ErrInvalid
	}
	if len(code) > 64 {
		return ErrInvalid
	}
	res, err := s.db.ExecContext(ctx, "UPDATE mailer_accounts SET sync_status=?,sync_error=?,sync_checked_at=? WHERE id=?", status, code, timeNS(at), id)
	if err != nil {
		return fmt.Errorf("save mail account sync state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) PutFolders(ctx context.Context, accountID string, folders []Folder) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range folders {
		f.AccountID = accountID
		if _, err := tx.ExecContext(ctx, `INSERT INTO mailer_folders(account_id,id,provider_id,name,kind,read_only) VALUES(?,?,?,?,?,?) ON CONFLICT(account_id,id) DO UPDATE SET provider_id=excluded.provider_id,name=excluded.name,kind=excluded.kind,read_only=excluded.read_only`, accountID, f.ID, f.ProviderID, f.Name, f.Kind, f.ReadOnly); err != nil {
			return fmt.Errorf("save mail folder: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mail folders: %w", err)
	}
	return nil
}

func (s *Store) Folders(ctx context.Context, accountID string) ([]Folder, error) {
	query := `SELECT id,account_id,provider_id,name,kind,read_only FROM mailer_folders ORDER BY account_id,name,id`
	args := []any{}
	if accountID != "" {
		query = `SELECT id,account_id,provider_id,name,kind,read_only FROM mailer_folders WHERE account_id=? ORDER BY name,id`
		args = append(args, accountID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mail folders: %w", err)
	}
	defer rows.Close()
	folders := make([]Folder, 0)
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.AccountID, &f.ProviderID, &f.Name, &f.Kind, &f.ReadOnly); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (s *Store) Cursor(ctx context.Context, accountID, folderID string) (string, error) {
	var cursor string
	err := s.db.QueryRowContext(ctx, "SELECT cursor FROM mailer_cursors WHERE account_id=? AND folder_id=?", accountID, folderID).Scan(&cursor)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return cursor, err
}

func (s *Store) CursorValidity(ctx context.Context, accountID, folderID string) (uint32, error) {
	var value uint32
	err := s.db.QueryRowContext(ctx, "SELECT uid_validity FROM mailer_cursors WHERE account_id=? AND folder_id=?", accountID, folderID).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return value, err
}

func (s *Store) SetCursor(ctx context.Context, accountID, folderID, cursor string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mailer_cursors(account_id,folder_id,cursor,updated_at) VALUES(?,?,?,?) ON CONFLICT(account_id,folder_id) DO UPDATE SET cursor=excluded.cursor,updated_at=excluded.updated_at`, accountID, folderID, cursor, timeNS(time.Now()))
	if err != nil {
		return fmt.Errorf("save folder cursor: %w", err)
	}
	return nil
}

func (s *Store) SetCursorValidity(ctx context.Context, accountID, folderID string, validity uint32) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO mailer_cursors(account_id,folder_id,cursor,uid_validity,updated_at) VALUES(?,?, '', ?, ?) ON CONFLICT(account_id,folder_id) DO UPDATE SET uid_validity=excluded.uid_validity,updated_at=excluded.updated_at`, accountID, folderID, validity, timeNS(time.Now()))
	if err != nil {
		return fmt.Errorf("save folder UID validity: %w", err)
	}
	return nil
}
