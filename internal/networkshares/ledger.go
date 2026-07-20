package networkshares

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const ledgerSchemaVersion = 1

type ledgerRecord struct {
	Spec      ShareSpec
	Drift     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Ledger owns AuraGo's durable share identities and desired state.
type Ledger struct {
	db *sql.DB
}

// OpenLedger opens and migrates the network-share ownership ledger.
func OpenLedger(path string) (*Ledger, error) {
	if path == "" {
		return nil, fmt.Errorf("network shares ledger path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create network shares ledger directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open network shares ledger: %w", err)
	}
	db.SetMaxOpenConns(1)
	ledger := &Ledger{db: db}
	if err := ledger.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) migrate(ctx context.Context) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin network shares ledger migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			version INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS managed_shares (
			id TEXT PRIMARY KEY,
			protocol TEXT NOT NULL,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			spec_json TEXT NOT NULL,
			drift TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(protocol, name)
		);
		CREATE INDEX IF NOT EXISTS idx_managed_shares_path ON managed_shares(protocol, path);
	`); err != nil {
		return fmt.Errorf("migrate network shares ledger schema: %w", err)
	}
	var version int
	err = tx.QueryRowContext(ctx, `SELECT version FROM schema_meta WHERE id = 1`).Scan(&version)
	switch {
	case err == sql.ErrNoRows:
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta(id, version) VALUES(1, ?)`, ledgerSchemaVersion); err != nil {
			return fmt.Errorf("record network shares ledger schema: %w", err)
		}
	case err != nil:
		return fmt.Errorf("read network shares ledger schema: %w", err)
	case version > ledgerSchemaVersion:
		return fmt.Errorf("network shares ledger schema %d is newer than supported version %d", version, ledgerSchemaVersion)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit network shares ledger migration: %w", err)
	}
	return nil
}

func (l *Ledger) list(ctx context.Context) ([]ledgerRecord, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT spec_json, drift, created_at, updated_at FROM managed_shares ORDER BY protocol, name`)
	if err != nil {
		return nil, fmt.Errorf("list managed network shares: %w", err)
	}
	defer rows.Close()
	var records []ledgerRecord
	for rows.Next() {
		var raw, drift, created, updated string
		if err := rows.Scan(&raw, &drift, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan managed network share: %w", err)
		}
		var spec ShareSpec
		if err := json.Unmarshal([]byte(raw), &spec); err != nil {
			return nil, fmt.Errorf("decode managed network share %q: %w", raw, err)
		}
		records = append(records, ledgerRecord{
			Spec:      spec,
			Drift:     drift,
			CreatedAt: parseLedgerTime(created),
			UpdatedAt: parseLedgerTime(updated),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed network shares: %w", err)
	}
	return records, nil
}

func (l *Ledger) get(ctx context.Context, id string) (ledgerRecord, error) {
	var raw, drift, created, updated string
	err := l.db.QueryRowContext(ctx, `SELECT spec_json, drift, created_at, updated_at FROM managed_shares WHERE id = ?`, id).
		Scan(&raw, &drift, &created, &updated)
	if err == sql.ErrNoRows {
		return ledgerRecord{}, codedError(ErrorNotFound, "The managed network share was not found.", err)
	}
	if err != nil {
		return ledgerRecord{}, fmt.Errorf("get managed network share: %w", err)
	}
	var spec ShareSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return ledgerRecord{}, fmt.Errorf("decode managed network share: %w", err)
	}
	return ledgerRecord{Spec: spec, Drift: drift, CreatedAt: parseLedgerTime(created), UpdatedAt: parseLedgerTime(updated)}, nil
}

func (l *Ledger) put(ctx context.Context, spec ShareSpec, drift string) error {
	raw, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode managed network share: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = l.db.ExecContext(ctx, `
		INSERT INTO managed_shares(id, protocol, name, path, spec_json, drift, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			protocol = excluded.protocol,
			name = excluded.name,
			path = excluded.path,
			spec_json = excluded.spec_json,
			drift = excluded.drift,
			updated_at = excluded.updated_at
	`, spec.ID, spec.Protocol, spec.Name, spec.Path, string(raw), drift, now, now)
	if err != nil {
		return fmt.Errorf("store managed network share: %w", err)
	}
	return nil
}

func (l *Ledger) setDrift(ctx context.Context, id, drift string) error {
	_, err := l.db.ExecContext(ctx, `UPDATE managed_shares SET drift = ?, updated_at = ? WHERE id = ?`,
		drift, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update managed network share drift: %w", err)
	}
	return nil
}

func (l *Ledger) delete(ctx context.Context, id string) error {
	result, err := l.db.ExecContext(ctx, `DELETE FROM managed_shares WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete managed network share: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return codedError(ErrorNotFound, "The managed network share was not found.", nil)
	}
	return nil
}

func parseLedgerTime(raw string) time.Time {
	value, _ := time.Parse(time.RFC3339Nano, raw)
	return value
}

// Close releases the SQLite ledger.
func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}
