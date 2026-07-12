package virtualcomputers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Ledger struct {
	db *sql.DB
}

type ActionRecord struct {
	Actor      string                 `json:"actor,omitempty"`
	Action     string                 `json:"action"`
	TargetType string                 `json:"target_type,omitempty"`
	TargetID   string                 `json:"target_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type ExposureRecord struct {
	MachineID string `json:"machine_id"`
	Channel   string `json:"channel"`
	URL       string `json:"url,omitempty"`
	Active    bool   `json:"active"`
}

func OpenLedger(path string) (*Ledger, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("virtual computers ledger path is empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create virtual computers ledger dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open virtual computers ledger: %w", err)
	}
	ledger := &Ledger{db: db}
	if err := ledger.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	return l.db.Close()
}

func (l *Ledger) Migrate(ctx context.Context) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("virtual computers ledger is not open")
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS setup_state (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS machines (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			template TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			ttl_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT '',
			raw_json TEXT NOT NULL DEFAULT '{}',
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			raw_json TEXT NOT NULL DEFAULT '{}',
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS volumes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			raw_json TEXT NOT NULL DEFAULT '{}',
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS actions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			target_type TEXT NOT NULL DEFAULT '',
			target_id TEXT NOT NULL DEFAULT '',
			metadata_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS exposure_status (
			machine_id TEXT NOT NULL,
			channel TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			active INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(machine_id, channel)
		)`,
	} {
		if _, err := l.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate virtual computers ledger: %w", err)
		}
	}
	return nil
}

func (l *Ledger) SetSetupState(ctx context.Context, key, value string) error {
	if l == nil || l.db == nil {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO setup_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		strings.TrimSpace(key), value, nowText())
	if err != nil {
		return fmt.Errorf("write virtual computers setup state: %w", err)
	}
	return nil
}

func (l *Ledger) UpsertMachine(ctx context.Context, machine Machine) error {
	if l == nil || l.db == nil || strings.TrimSpace(machine.ID) == "" {
		return nil
	}
	raw := mustJSON(machine)
	_, err := l.db.ExecContext(ctx, `INSERT INTO machines
		(id, name, template, status, ttl_seconds, created_at, expires_at, raw_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			template = excluded.template,
			status = excluded.status,
			ttl_seconds = excluded.ttl_seconds,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at`,
		machine.ID, machine.Name, machine.Template, machine.Status, machine.TTLSeconds,
		timeText(machine.CreatedAt), timeText(machine.ExpiresAt), raw, nowText())
	if err != nil {
		return fmt.Errorf("upsert virtual computer machine: %w", err)
	}
	return nil
}

func (l *Ledger) DeleteMachine(ctx context.Context, id string) error {
	if l == nil || l.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `DELETE FROM machines WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete virtual computer machine ledger row: %w", err)
	}
	return nil
}

func (l *Ledger) UpsertTemplate(ctx context.Context, template Template) error {
	if l == nil || l.db == nil || strings.TrimSpace(template.ID) == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO templates (id, name, description, raw_json, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at`,
		template.ID, template.Name, template.Description, mustJSON(template), nowText())
	if err != nil {
		return fmt.Errorf("upsert virtual computer template: %w", err)
	}
	return nil
}

func (l *Ledger) UpsertVolume(ctx context.Context, volume Volume) error {
	if l == nil || l.db == nil || strings.TrimSpace(volume.ID) == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO volumes (id, name, size_bytes, raw_json, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			size_bytes = excluded.size_bytes,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at`,
		volume.ID, volume.Name, volume.SizeBytes, mustJSON(volume), nowText())
	if err != nil {
		return fmt.Errorf("upsert virtual computer volume: %w", err)
	}
	return nil
}

func (l *Ledger) RecordAction(ctx context.Context, action ActionRecord) error {
	if l == nil || l.db == nil || strings.TrimSpace(action.Action) == "" {
		return nil
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO actions
		(actor, action, target_type, target_id, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		action.Actor, action.Action, action.TargetType, action.TargetID, mustJSON(action.Metadata), nowText())
	if err != nil {
		return fmt.Errorf("record virtual computer action: %w", err)
	}
	return nil
}

func (l *Ledger) SetExposure(ctx context.Context, exposure ExposureRecord) error {
	if l == nil || l.db == nil || strings.TrimSpace(exposure.MachineID) == "" || strings.TrimSpace(exposure.Channel) == "" {
		return nil
	}
	active := 0
	if exposure.Active {
		active = 1
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO exposure_status
		(machine_id, channel, url, active, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(machine_id, channel) DO UPDATE SET
			url = excluded.url,
			active = excluded.active,
			updated_at = excluded.updated_at`,
		exposure.MachineID, exposure.Channel, exposure.URL, active, nowText())
	if err != nil {
		return fmt.Errorf("write virtual computer exposure: %w", err)
	}
	return nil
}

func mustJSON(v interface{}) string {
	if v == nil {
		return "{}"
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) == 0 {
		return "{}"
	}
	return string(data)
}

func nowText() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func timeText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
