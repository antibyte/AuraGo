package virtualcomputers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Ledger struct {
	db   *sql.DB
	path string
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
	// A single connection keeps concurrent task and volume writers serialized.
	// HTTP capability checks remain parallel and queue only their short DB updates.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ledger := &Ledger{db: db, path: path}
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
	needsBackup, err := l.needsV2Migration(ctx)
	if err != nil {
		return err
	}
	if needsBackup {
		if err := l.backupV1(ctx); err != nil {
			return err
		}
	}
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin virtual computers migration: %w", err)
	}
	defer tx.Rollback()
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
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
			created_at TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL DEFAULT '',
			quota_mb INTEGER NOT NULL DEFAULT 0,
			last_verified_at TEXT NOT NULL DEFAULT '',
			verification_status TEXT NOT NULL DEFAULT 'tracked',
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
		`CREATE TABLE IF NOT EXISTS agent_tasks (
			id TEXT PRIMARY KEY,
			machine_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			instruction TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			preview_port INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			events_truncated INTEGER NOT NULL DEFAULT 0,
			event_count INTEGER NOT NULL DEFAULT 0,
			event_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			started_at TEXT NOT NULL DEFAULT '',
			completed_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS agent_task_events (
			task_id TEXT NOT NULL,
			sequence INTEGER NOT NULL,
			type TEXT NOT NULL,
			text TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			PRIMARY KEY(task_id, sequence),
			FOREIGN KEY(task_id) REFERENCES agent_tasks(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_tasks_created_at ON agent_tasks(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_tasks_machine_id ON agent_tasks(machine_id, created_at DESC)`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate virtual computers ledger: %w", err)
		}
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "created_at", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "expires_at", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "quota_mb", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "last_verified_at", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "verification_status", definition: "TEXT NOT NULL DEFAULT 'tracked'"},
	} {
		if err := ensureColumn(ctx, tx, "volumes", column.name, column.definition); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_meta(key, value) VALUES ('schema_version', '2')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		return fmt.Errorf("write virtual computers schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit virtual computers migration: %w", err)
	}
	return nil
}

func (l *Ledger) needsV2Migration(ctx context.Context) (bool, error) {
	var tableCount int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&tableCount); err != nil {
		return false, fmt.Errorf("inspect virtual computers schema: %w", err)
	}
	if tableCount == 0 {
		return false, nil
	}
	var metaCount int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_meta'`).Scan(&metaCount); err != nil {
		return false, fmt.Errorf("inspect virtual computers schema metadata: %w", err)
	}
	if metaCount == 0 {
		return true, nil
	}
	var version string
	err := l.db.QueryRowContext(ctx, `SELECT value FROM schema_meta WHERE key = 'schema_version'`).Scan(&version)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read virtual computers schema version: %w", err)
	}
	return version != "2", nil
}

func (l *Ledger) backupV1(ctx context.Context) error {
	if strings.TrimSpace(l.path) == "" || l.path == ":memory:" {
		return nil
	}
	backupPath := l.path + ".v1.bak"
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect virtual computers migration backup: %w", err)
	}
	escaped := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := l.db.ExecContext(ctx, "VACUUM INTO '"+escaped+"'"); err != nil {
		return fmt.Errorf("backup virtual computers database before migration: %w", err)
	}
	return nil
}

func ensureColumn(ctx context.Context, tx *sql.Tx, table, column, definition string) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect %s schema: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan %s schema: %w", table, err)
		}
		if name == column {
			found = true
		}
	}
	_ = rows.Close()
	if found {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}

func (l *Ledger) InsertAgentTask(ctx context.Context, task AgentTask) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("virtual computers ledger is not open")
	}
	if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.MachineID) == "" {
		return fmt.Errorf("agent task id and machine id are required")
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now().UTC()
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO agent_tasks
		(id, machine_id, kind, instruction, status, preview_port, error, events_truncated,
		 event_count, event_bytes, created_at, updated_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?, ?, ?)`,
		task.ID, task.MachineID, task.Kind, task.Instruction, task.Status, task.PreviewPort,
		task.Error, boolInt(task.EventsTruncated), timeText(task.CreatedAt), timeText(task.UpdatedAt),
		timePtrText(task.StartedAt), timePtrText(task.CompletedAt))
	if err != nil {
		return fmt.Errorf("insert virtual computer agent task: %w", err)
	}
	return nil
}

func (l *Ledger) UpdateAgentTaskRunning(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := l.db.ExecContext(ctx, `UPDATE agent_tasks
		SET status = ?, started_at = ?, updated_at = ?, error = ''
		WHERE id = ? AND status = ?`, AgentTaskStatusRunning, timeText(now), timeText(now), id, AgentTaskStatusQueued)
	if err != nil {
		return fmt.Errorf("mark virtual computer agent task running: %w", err)
	}
	return nil
}

func (l *Ledger) FinishAgentTask(ctx context.Context, id, status, errText string) error {
	now := time.Now().UTC()
	_, err := l.db.ExecContext(ctx, `UPDATE agent_tasks
		SET status = ?, error = ?, completed_at = ?, updated_at = ?
		WHERE id = ? AND status IN (?, ?)`, status, errText, timeText(now), timeText(now), id,
		AgentTaskStatusQueued, AgentTaskStatusRunning)
	if err != nil {
		return fmt.Errorf("finish virtual computer agent task: %w", err)
	}
	return nil
}

func (l *Ledger) AppendAgentTaskEvent(ctx context.Context, taskID, eventType, text string, maxEvents, maxBytes int) error {
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin agent task event transaction: %w", err)
	}
	defer tx.Rollback()
	var count, size int
	if err := tx.QueryRowContext(ctx, `SELECT event_count, event_bytes FROM agent_tasks WHERE id = ?`, taskID).Scan(&count, &size); err != nil {
		return fmt.Errorf("read agent task event limits: %w", err)
	}
	if count >= maxEvents || size+len(text) > maxBytes {
		if _, err := tx.ExecContext(ctx, `UPDATE agent_tasks SET events_truncated = 1, updated_at = ? WHERE id = ?`, nowText(), taskID); err != nil {
			return fmt.Errorf("mark agent task events truncated: %w", err)
		}
		return tx.Commit()
	}
	sequence := int64(count + 1)
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO agent_task_events(task_id, sequence, type, text, created_at)
		VALUES (?, ?, ?, ?, ?)`, taskID, sequence, eventType, text, timeText(now)); err != nil {
		return fmt.Errorf("insert agent task event: %w", err)
	}
	previewPort := 0
	if eventType == "preview" {
		previewPort, _ = strconv.Atoi(strings.TrimSpace(text))
		if previewPort < 1 || previewPort > 65535 {
			previewPort = 0
		}
	}
	if previewPort > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE agent_tasks SET event_count = ?, event_bytes = ?, preview_port = ?, updated_at = ? WHERE id = ?`, count+1, size+len(text), previewPort, timeText(now), taskID)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE agent_tasks SET event_count = ?, event_bytes = ?, updated_at = ? WHERE id = ?`, count+1, size+len(text), timeText(now), taskID)
	}
	if err != nil {
		return fmt.Errorf("update agent task event counters: %w", err)
	}
	return tx.Commit()
}

func (l *Ledger) GetAgentTask(ctx context.Context, id string) (AgentTask, bool, error) {
	var task AgentTask
	var createdAt, updatedAt, startedAt, completedAt string
	var truncated int
	err := l.db.QueryRowContext(ctx, `SELECT id, machine_id, kind, instruction, status, preview_port,
		error, events_truncated, created_at, updated_at, started_at, completed_at
		FROM agent_tasks WHERE id = ?`, id).Scan(&task.ID, &task.MachineID, &task.Kind, &task.Instruction,
		&task.Status, &task.PreviewPort, &task.Error, &truncated, &createdAt, &updatedAt, &startedAt, &completedAt)
	if err == sql.ErrNoRows {
		return AgentTask{}, false, nil
	}
	if err != nil {
		return AgentTask{}, false, fmt.Errorf("read virtual computer agent task: %w", err)
	}
	task.EventsTruncated = truncated != 0
	task.CreatedAt = parseStoredTime(createdAt)
	task.UpdatedAt = parseStoredTime(updatedAt)
	task.StartedAt = parseStoredOptionalTime(startedAt)
	task.CompletedAt = parseStoredOptionalTime(completedAt)
	rows, err := l.db.QueryContext(ctx, `SELECT sequence, type, text, created_at FROM agent_task_events WHERE task_id = ? ORDER BY sequence`, id)
	if err != nil {
		return AgentTask{}, false, fmt.Errorf("read virtual computer agent task events: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var event AgentTaskEvent
		var eventTime string
		if err := rows.Scan(&event.Sequence, &event.Type, &event.Text, &eventTime); err != nil {
			return AgentTask{}, false, fmt.Errorf("scan virtual computer agent task event: %w", err)
		}
		event.CreatedAt = parseStoredTime(eventTime)
		task.Events = append(task.Events, event)
	}
	return task, true, rows.Err()
}

func (l *Ledger) ListAgentTasks(ctx context.Context, machineID string, limit int) ([]AgentTask, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := `SELECT id FROM agent_tasks`
	args := []interface{}{}
	if strings.TrimSpace(machineID) != "" {
		query += ` WHERE machine_id = ?`
		args = append(args, strings.TrimSpace(machineID))
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list virtual computer agent tasks: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	out := make([]AgentTask, 0, len(ids))
	for _, id := range ids {
		task, ok, err := l.GetAgentTask(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, task)
		}
	}
	return out, nil
}

func (l *Ledger) InterruptActiveAgentTasks(ctx context.Context) error {
	now := nowText()
	_, err := l.db.ExecContext(ctx, `UPDATE agent_tasks
		SET status = ?, error = ?, completed_at = ?, updated_at = ?
		WHERE status IN (?, ?)`, AgentTaskStatusInterrupted, "interrupted by AuraGo restart", now, now,
		AgentTaskStatusQueued, AgentTaskStatusRunning)
	if err != nil {
		return fmt.Errorf("interrupt active virtual computer agent tasks: %w", err)
	}
	return nil
}

func (l *Ledger) CleanupAgentTasks(ctx context.Context, before time.Time) error {
	_, err := l.db.ExecContext(ctx, `DELETE FROM agent_tasks
		WHERE completed_at <> '' AND completed_at < ? AND status NOT IN (?, ?)`, timeText(before),
		AgentTaskStatusQueued, AgentTaskStatusRunning)
	if err != nil {
		return fmt.Errorf("cleanup virtual computer agent tasks: %w", err)
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
		timePtrText(machine.CreatedAt), timePtrText(machine.ExpiresAt), raw, nowText())
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
	status := strings.TrimSpace(volume.VerificationStatus)
	if status == "" {
		status = "tracked"
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO volumes
		(id, name, size_bytes, created_at, expires_at, quota_mb, last_verified_at, verification_status, raw_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			size_bytes = excluded.size_bytes,
			created_at = excluded.created_at,
			expires_at = excluded.expires_at,
			quota_mb = excluded.quota_mb,
			last_verified_at = excluded.last_verified_at,
			verification_status = excluded.verification_status,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at`,
		volume.ID, volume.Name, volume.SizeBytes, timePtrText(volume.CreatedAt), timePtrText(volume.ExpiresAt),
		volume.QuotaMB, timePtrText(volume.LastVerifiedAt), status, mustJSON(volume), nowText())
	if err != nil {
		return fmt.Errorf("upsert virtual computer volume: %w", err)
	}
	return nil
}

func (l *Ledger) ListVolumes(ctx context.Context) ([]Volume, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT id, name, size_bytes, created_at, expires_at, quota_mb,
		last_verified_at, verification_status, raw_json FROM volumes ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tracked virtual computer volumes: %w", err)
	}
	defer rows.Close()
	var volumes []Volume
	for rows.Next() {
		var volume Volume
		var createdAt, expiresAt, verifiedAt, raw string
		if err := rows.Scan(&volume.ID, &volume.Name, &volume.SizeBytes, &createdAt, &expiresAt,
			&volume.QuotaMB, &verifiedAt, &volume.VerificationStatus, &raw); err != nil {
			return nil, fmt.Errorf("scan tracked virtual computer volume: %w", err)
		}
		volume.CreatedAt = parseStoredOptionalTime(createdAt)
		volume.ExpiresAt = parseStoredOptionalTime(expiresAt)
		volume.LastVerifiedAt = parseStoredOptionalTime(verifiedAt)
		_ = json.Unmarshal([]byte(raw), &volume.Raw)
		volumes = append(volumes, volume)
	}
	return volumes, rows.Err()
}

func (l *Ledger) DeleteVolume(ctx context.Context, id string) error {
	_, err := l.db.ExecContext(ctx, `DELETE FROM volumes WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("delete tracked virtual computer volume: %w", err)
	}
	return nil
}

func (l *Ledger) MarkVolumeStale(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := l.db.ExecContext(ctx, `UPDATE volumes SET verification_status = 'stale', last_verified_at = ?, updated_at = ? WHERE id = ?`,
		timeText(now), timeText(now), strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("mark tracked virtual computer volume stale: %w", err)
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

func timePtrText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return timeText(*t)
}

func parseStoredTime(value string) time.Time {
	parsed := parseOptionalTime(value)
	if parsed == nil {
		return time.Time{}
	}
	return *parsed
}

func parseStoredOptionalTime(value string) *time.Time {
	return parseOptionalTime(value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
