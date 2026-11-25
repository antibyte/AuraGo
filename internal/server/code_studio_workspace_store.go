package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var errCodeStudioWorkspaceRecordNotFound = errors.New("workspace not found")

type codeStudioWorkspaceRecord struct {
	ID, DeviceID, Root, Label string
	Capabilities              *codeStudioCapabilities
	CreatedAt, LastUsedAt     time.Time
}

type codeStudioWorkspaceStore struct {
	db  *sql.DB
	now func() time.Time
}

var codeStudioWorkspaceSchemas sync.Map // *sql.DB -> struct{}

const codeStudioWorkspaceSchema = `CREATE TABLE IF NOT EXISTS code_studio_workspaces (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	root TEXT NOT NULL,
	label TEXT NOT NULL DEFAULT '',
	capabilities TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_used_at TEXT NOT NULL,
	UNIQUE(device_id, root))`

func openCodeStudioWorkspaceStore(ctx context.Context, db *sql.DB) (*codeStudioWorkspaceStore, error) {
	if db == nil {
		return nil, errors.New("desktop database is not available")
	}
	if _, done := codeStudioWorkspaceSchemas.Load(db); !done {
		if _, err := db.ExecContext(ctx, codeStudioWorkspaceSchema); err != nil {
			return nil, fmt.Errorf("create code studio workspace table: %w", err)
		}
		codeStudioWorkspaceSchemas.Store(db, struct{}{})
	}
	return &codeStudioWorkspaceStore{db: db, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (h codeStudioHandlers) workspaceStore(ctx context.Context) (*codeStudioWorkspaceStore, error) {
	if h.server == nil {
		return nil, errors.New("code studio server is not available")
	}
	svc, _, err := h.server.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	if svc.DB() == nil { // services built outside getDesktopService (tests) are not initialised yet
		if err := svc.Init(ctx); err != nil {
			return nil, err
		}
	}
	return openCodeStudioWorkspaceStore(ctx, svc.DB())
}

func newCodeStudioWorkspaceID() string { return "ssh_" + strings.ToLower(rand.Text()) }

const codeStudioWorkspaceColumns = `id, device_id, root, label, capabilities, created_at, last_used_at`

func scanCodeStudioWorkspace(row interface{ Scan(...any) error }) (codeStudioWorkspaceRecord, error) {
	var rec codeStudioWorkspaceRecord
	var caps, created, used string
	if err := row.Scan(&rec.ID, &rec.DeviceID, &rec.Root, &rec.Label, &caps, &created, &used); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return rec, errCodeStudioWorkspaceRecordNotFound
		}
		return rec, err
	}
	rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	rec.LastUsedAt, _ = time.Parse(time.RFC3339Nano, used)
	if caps != "" {
		var c codeStudioCapabilities
		if json.Unmarshal([]byte(caps), &c) == nil {
			rec.Capabilities = &c
		}
	}
	return rec, nil
}

func (s *codeStudioWorkspaceStore) List(ctx context.Context) ([]codeStudioWorkspaceRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces ORDER BY last_used_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []codeStudioWorkspaceRecord{}
	for rows.Next() {
		rec, err := scanCodeStudioWorkspace(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *codeStudioWorkspaceStore) Get(ctx context.Context, id string) (codeStudioWorkspaceRecord, error) {
	return scanCodeStudioWorkspace(s.db.QueryRowContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces WHERE id = ?`, id))
}

func capsJSON(c *codeStudioCapabilities) string {
	if c == nil {
		return ""
	}
	raw, _ := json.Marshal(c)
	return string(raw)
}

// Upsert saves device+root; an existing entry keeps its id and label unless a new label is given.
func (s *codeStudioWorkspaceStore) Upsert(ctx context.Context, deviceID, root, label string, caps *codeStudioCapabilities) (codeStudioWorkspaceRecord, error) {
	now := s.now().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO code_studio_workspaces (id, device_id, root, label, capabilities, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id, root) DO UPDATE SET
			label = CASE WHEN excluded.label <> '' THEN excluded.label ELSE code_studio_workspaces.label END,
			capabilities = CASE WHEN excluded.capabilities <> '' THEN excluded.capabilities ELSE code_studio_workspaces.capabilities END,
			last_used_at = excluded.last_used_at`,
		newCodeStudioWorkspaceID(), deviceID, root, label, capsJSON(caps), now, now)
	if err != nil {
		return codeStudioWorkspaceRecord{}, fmt.Errorf("save code studio workspace: %w", err)
	}
	return scanCodeStudioWorkspace(s.db.QueryRowContext(ctx, `SELECT `+codeStudioWorkspaceColumns+` FROM code_studio_workspaces WHERE device_id = ? AND root = ?`, deviceID, root))
}

func (s *codeStudioWorkspaceStore) Touch(ctx context.Context, id string, caps *codeStudioCapabilities) error {
	_, err := s.db.ExecContext(ctx, `UPDATE code_studio_workspaces SET last_used_at = ?,
		capabilities = CASE WHEN ? <> '' THEN ? ELSE capabilities END WHERE id = ?`,
		s.now().Format(time.RFC3339Nano), capsJSON(caps), capsJSON(caps), id)
	return err
}

func (s *codeStudioWorkspaceStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM code_studio_workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errCodeStudioWorkspaceRecordNotFound
	}
	return nil
}

func (s *codeStudioWorkspaceStore) CountDevice(ctx context.Context, deviceID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM code_studio_workspaces WHERE device_id = ?`, deviceID).Scan(&n)
	return n, err
}
