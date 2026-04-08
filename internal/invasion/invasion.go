package invasion

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// NestRecord represents a deployment target (server, VM, or Docker container).
type NestRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Notes         string `json:"notes"`
	AccessType    string `json:"access_type"` // "ssh" | "docker" | "local"
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	VaultSecretID string `json:"vault_secret_id"` // reference to vault entry
	Active        bool   `json:"active"`
	EggID         string `json:"egg_id"` // assigned egg (empty = none)
	// ── Hatch / deploy state ──
	HatchStatus string `json:"hatch_status"`  // "idle" | "hatching" | "running" | "failed" | "stopped"
	LastHatchAt string `json:"last_hatch_at"` // ISO 8601
	HatchError  string `json:"hatch_error"`   // last error message
	// ── Network route ──
	Route       string `json:"route"`        // "direct" | "ssh_tunnel" | "tailscale" | "wireguard" | "custom"
	RouteConfig string `json:"route_config"` // JSON with route-specific params
	// ── Deployment settings ──
	DeployMethod string `json:"deploy_method"` // "ssh" | "docker_remote" | "docker_local"
	TargetArch   string `json:"target_arch"`   // "linux/amd64" | "linux/arm64" | etc.

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// EggRecord represents a sub-agent configuration template.
type EggRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model"`
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url"`
	APIKeyRef   string `json:"api_key_ref"` // vault reference for API key
	Active      bool   `json:"active"`
	// ── Deployment settings ──
	Permanent    bool   `json:"permanent"`     // install as systemd service vs. run once
	IncludeVault bool   `json:"include_vault"` // ship the master vault (target must be secure!)
	InheritLLM   bool   `json:"inherit_llm"`   // inherit master LLM config instead of own fields
	EggPort      int    `json:"egg_port"`      // port for the egg HTTP/WS server (default 8099)
	AllowedTools string `json:"allowed_tools"` // JSON array of allowed tool IDs (empty = default set)

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// InitDB initializes the invasion SQLite database with nests and eggs tables.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open invasion database: %w", err)
	}

	nestsSchema := `
	CREATE TABLE IF NOT EXISTS nests (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		notes           TEXT DEFAULT '',
		access_type     TEXT NOT NULL DEFAULT 'ssh',
		host            TEXT NOT NULL DEFAULT '',
		port            INTEGER NOT NULL DEFAULT 22,
		username        TEXT DEFAULT '',
		vault_secret_id TEXT DEFAULT '',
		active          INTEGER NOT NULL DEFAULT 1,
		egg_id          TEXT DEFAULT '',
		hatch_status    TEXT DEFAULT 'idle',
		last_hatch_at   TEXT DEFAULT '',
		hatch_error     TEXT DEFAULT '',
		route           TEXT DEFAULT 'direct',
		route_config    TEXT DEFAULT '',
		deploy_method   TEXT DEFAULT 'ssh',
		target_arch     TEXT DEFAULT 'linux/amd64',
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	);`

	if _, err := db.Exec(nestsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create nests schema: %w", err)
	}

	eggsSchema := `
	CREATE TABLE IF NOT EXISTS eggs (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		description   TEXT DEFAULT '',
		model         TEXT DEFAULT '',
		provider      TEXT DEFAULT '',
		base_url      TEXT DEFAULT '',
		api_key_ref   TEXT DEFAULT '',
		active        INTEGER NOT NULL DEFAULT 1,
		permanent     INTEGER NOT NULL DEFAULT 0,
		include_vault INTEGER NOT NULL DEFAULT 0,
		inherit_llm   INTEGER NOT NULL DEFAULT 0,
		egg_port      INTEGER NOT NULL DEFAULT 8099,
		allowed_tools TEXT DEFAULT '',
		created_at    TEXT NOT NULL,
		updated_at    TEXT NOT NULL
	);`

	if _, err := db.Exec(eggsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create eggs schema: %w", err)
	}

	tasksSchema := `
	CREATE TABLE IF NOT EXISTS invasion_tasks (
		id            TEXT PRIMARY KEY,
		nest_id       TEXT NOT NULL,
		egg_id        TEXT DEFAULT '',
		description   TEXT NOT NULL,
		timeout       INTEGER DEFAULT 0,
		status        TEXT NOT NULL DEFAULT 'pending',
		result_output TEXT DEFAULT '',
		result_error  TEXT DEFAULT '',
		created_at    TEXT NOT NULL,
		sent_at       TEXT DEFAULT '',
		completed_at  TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_invasion_tasks_nest_status ON invasion_tasks(nest_id, status);
	`

	if _, err := db.Exec(tasksSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create invasion_tasks schema: %w", err)
	}

	deployHistorySchema := `
	CREATE TABLE IF NOT EXISTS deployment_history (
		id              TEXT PRIMARY KEY,
		nest_id         TEXT NOT NULL,
		egg_id          TEXT DEFAULT '',
		status          TEXT NOT NULL DEFAULT 'started',
		binary_hash     TEXT DEFAULT '',
		config_hash     TEXT DEFAULT '',
		deploy_method   TEXT DEFAULT '',
		created_at      TEXT NOT NULL,
		deployed_at     TEXT DEFAULT '',
		verified_at     TEXT DEFAULT '',
		rolled_back_at  TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_deployment_history_nest ON deployment_history(nest_id, created_at);
	`

	if _, err := db.Exec(deployHistorySchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create deployment_history schema: %w", err)
	}

	// ── Migrations — add columns that may be missing on older DBs ──
	migrations := []string{
		"ALTER TABLE nests ADD COLUMN hatch_status TEXT DEFAULT 'idle'",
		"ALTER TABLE nests ADD COLUMN last_hatch_at TEXT DEFAULT ''",
		"ALTER TABLE nests ADD COLUMN hatch_error TEXT DEFAULT ''",
		"ALTER TABLE nests ADD COLUMN route TEXT DEFAULT 'direct'",
		"ALTER TABLE nests ADD COLUMN route_config TEXT DEFAULT ''",
		"ALTER TABLE nests ADD COLUMN deploy_method TEXT DEFAULT 'ssh'",
		"ALTER TABLE nests ADD COLUMN target_arch TEXT DEFAULT 'linux/amd64'",
		"ALTER TABLE eggs ADD COLUMN permanent INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE eggs ADD COLUMN include_vault INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE eggs ADD COLUMN inherit_llm INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE eggs ADD COLUMN egg_port INTEGER NOT NULL DEFAULT 8099",
		"ALTER TABLE eggs ADD COLUMN allowed_tools TEXT DEFAULT ''",
	}
	for _, m := range migrations {
		_, err := db.Exec(m)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate column") {
				// Expected when column already exists, ignore
				continue
			}
			slog.Warn("Invasion migration failed", "error", err, "migration", m)
		}
	}

	// Drop the personality column if it still exists (eggs no longer use personality)
	// SQLite doesn't support DROP COLUMN before 3.35.0, so we just ignore it.

	return db, nil
}

// ── Nests CRUD ──────────────────────────────────────────────────────────────

// CreateNest generates a UUID and inserts a new nest record.
func CreateNest(db *sql.DB, n NestRecord) (string, error) {
	n.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	n.CreatedAt = now
	n.UpdatedAt = now
	if err := insertNest(db, n); err != nil {
		return "", err
	}
	return n.ID, nil
}

func insertNest(db *sql.DB, n NestRecord) error {
	query := `INSERT INTO nests (id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	           hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if n.HatchStatus == "" {
		n.HatchStatus = "idle"
	}
	if n.Route == "" {
		n.Route = "direct"
	}
	if n.DeployMethod == "" {
		n.DeployMethod = "ssh"
	}
	if n.TargetArch == "" {
		n.TargetArch = "linux/amd64"
	}
	_, err := db.Exec(query, n.ID, n.Name, n.Notes, n.AccessType, n.Host, n.Port, n.Username, n.VaultSecretID,
		dbutil.BoolToInt(n.Active), n.EggID, n.HatchStatus, n.LastHatchAt, n.HatchError, n.Route, n.RouteConfig, n.DeployMethod, n.TargetArch, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert nest: %w", err)
	}
	return nil
}

// nestScanner is an interface satisfied by both *sql.Row and per-row scanning helpers.
type nestScanner interface {
	Scan(dest ...interface{}) error
}

// scanNestRow scans a single nest row from any scanner (Row or Rows).
func scanNestRow(s nestScanner) (NestRecord, error) {
	var n NestRecord
	var active int
	var notesNull, hostNull, userNull, secretNull, eggNull sql.NullString
	var hatchStatusNull, lastHatchNull, hatchErrNull, routeNull, routeCfgNull, deployNull, archNull sql.NullString
	if err := s.Scan(&n.ID, &n.Name, &notesNull, &n.AccessType, &hostNull, &n.Port, &userNull, &secretNull, &active, &eggNull,
		&hatchStatusNull, &lastHatchNull, &hatchErrNull, &routeNull, &routeCfgNull, &deployNull, &archNull, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return NestRecord{}, err
	}
	n.Notes = nullStr(notesNull)
	n.Host = nullStr(hostNull)
	n.Username = nullStr(userNull)
	n.VaultSecretID = nullStr(secretNull)
	n.EggID = nullStr(eggNull)
	n.HatchStatus = nullStr(hatchStatusNull)
	n.LastHatchAt = nullStr(lastHatchNull)
	n.HatchError = nullStr(hatchErrNull)
	n.Route = nullStr(routeNull)
	n.RouteConfig = nullStr(routeCfgNull)
	n.DeployMethod = nullStr(deployNull)
	n.TargetArch = nullStr(archNull)
	n.Active = active != 0
	if n.HatchStatus == "" {
		n.HatchStatus = "idle"
	}
	if n.Route == "" {
		n.Route = "direct"
	}
	if n.DeployMethod == "" {
		n.DeployMethod = "ssh"
	}
	if n.TargetArch == "" {
		n.TargetArch = "linux/amd64"
	}
	return n, nil
}

// GetNest retrieves a single nest by ID.
func GetNest(db *sql.DB, id string) (NestRecord, error) {
	query := `SELECT id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	          hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at FROM nests WHERE id = ?`
	n, err := scanNestRow(db.QueryRow(query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return NestRecord{}, fmt.Errorf("nest not found: %s", id)
		}
		return NestRecord{}, fmt.Errorf("failed to get nest: %w", err)
	}
	return n, nil
}

// ListNests returns all nest records.
func ListNests(db *sql.DB) ([]NestRecord, error) {
	query := `SELECT id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	          hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at FROM nests ORDER BY name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list nests: %w", err)
	}
	defer rows.Close()
	return scanNests(rows)
}

// ListActiveNests returns only active nest records.
func ListActiveNests(db *sql.DB) ([]NestRecord, error) {
	query := `SELECT id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	          hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at FROM nests WHERE active = 1 ORDER BY name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list active nests: %w", err)
	}
	defer rows.Close()
	return scanNests(rows)
}

// UpdateNest updates an existing nest record (updates updated_at automatically).
func UpdateNest(db *sql.DB, n NestRecord) error {
	n.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE nests SET name=?, notes=?, access_type=?, host=?, port=?, username=?, vault_secret_id=?, active=?, egg_id=?,
	          hatch_status=?, last_hatch_at=?, hatch_error=?, route=?, route_config=?, deploy_method=?, target_arch=?, updated_at=? WHERE id=?`
	res, err := db.Exec(query, n.Name, n.Notes, n.AccessType, n.Host, n.Port, n.Username, n.VaultSecretID, dbutil.BoolToInt(n.Active), n.EggID,
		n.HatchStatus, n.LastHatchAt, n.HatchError, n.Route, n.RouteConfig, n.DeployMethod, n.TargetArch, n.UpdatedAt, n.ID)
	if err != nil {
		return fmt.Errorf("failed to update nest: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("nest not found: %s", n.ID)
	}
	return nil
}

// DeleteNest removes a nest record by ID.
func DeleteNest(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM nests WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete nest: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("nest not found: %s", id)
	}
	return nil
}

// ToggleNestActive sets the active flag for a nest.
func ToggleNestActive(db *sql.DB, id string, active bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(`UPDATE nests SET active=?, updated_at=? WHERE id=?`, dbutil.BoolToInt(active), now, id)
	if err != nil {
		return fmt.Errorf("failed to toggle nest: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("nest not found: %s", id)
	}
	return nil
}

// ── Eggs CRUD ───────────────────────────────────────────────────────────────

// CreateEgg generates a UUID and inserts a new egg record.
func CreateEgg(db *sql.DB, e EggRecord) (string, error) {
	e.ID = uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := insertEgg(db, e); err != nil {
		return "", err
	}
	return e.ID, nil
}

func insertEgg(db *sql.DB, e EggRecord) error {
	query := `INSERT INTO eggs (id, name, description, model, provider, base_url, api_key_ref, active,
	           permanent, include_vault, inherit_llm, egg_port, allowed_tools, created_at, updated_at)
	           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if e.EggPort <= 0 {
		e.EggPort = 8099
	}
	_, err := db.Exec(query, e.ID, e.Name, e.Description, e.Model, e.Provider, e.BaseURL, e.APIKeyRef,
		dbutil.BoolToInt(e.Active), dbutil.BoolToInt(e.Permanent), dbutil.BoolToInt(e.IncludeVault), dbutil.BoolToInt(e.InheritLLM), e.EggPort, e.AllowedTools, e.CreatedAt, e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert egg: %w", err)
	}
	return nil
}

// GetEgg retrieves a single egg by ID.
func GetEgg(db *sql.DB, id string) (EggRecord, error) {
	query := `SELECT id, name, description, model, provider, base_url, api_key_ref, active,
	          permanent, include_vault, inherit_llm, egg_port, allowed_tools, created_at, updated_at FROM eggs WHERE id = ?`
	var e EggRecord
	var active, permanent, includeVault, inheritLLM int
	var descNull, modelNull, provNull, urlNull, keyNull, toolsNull sql.NullString
	err := db.QueryRow(query, id).Scan(&e.ID, &e.Name, &descNull, &modelNull, &provNull, &urlNull, &keyNull, &active,
		&permanent, &includeVault, &inheritLLM, &e.EggPort, &toolsNull, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return EggRecord{}, fmt.Errorf("egg not found: %s", id)
		}
		return EggRecord{}, fmt.Errorf("failed to get egg: %w", err)
	}
	e.Description = nullStr(descNull)
	e.Model = nullStr(modelNull)
	e.Provider = nullStr(provNull)
	e.BaseURL = nullStr(urlNull)
	e.APIKeyRef = nullStr(keyNull)
	e.AllowedTools = nullStr(toolsNull)
	e.Active = active != 0
	e.Permanent = permanent != 0
	e.IncludeVault = includeVault != 0
	e.InheritLLM = inheritLLM != 0
	if e.EggPort <= 0 {
		e.EggPort = 8099
	}
	return e, nil
}

// ListEggs returns all egg records.
func ListEggs(db *sql.DB) ([]EggRecord, error) {
	query := `SELECT id, name, description, model, provider, base_url, api_key_ref, active,
	          permanent, include_vault, inherit_llm, egg_port, allowed_tools, created_at, updated_at FROM eggs ORDER BY name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list eggs: %w", err)
	}
	defer rows.Close()
	return scanEggs(rows)
}

// UpdateEgg updates an existing egg record.
func UpdateEgg(db *sql.DB, e EggRecord) error {
	e.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE eggs SET name=?, description=?, model=?, provider=?, base_url=?, api_key_ref=?, active=?,
	          permanent=?, include_vault=?, inherit_llm=?, egg_port=?, allowed_tools=?, updated_at=? WHERE id=?`
	if e.EggPort <= 0 {
		e.EggPort = 8099
	}
	res, err := db.Exec(query, e.Name, e.Description, e.Model, e.Provider, e.BaseURL, e.APIKeyRef, dbutil.BoolToInt(e.Active),
		dbutil.BoolToInt(e.Permanent), dbutil.BoolToInt(e.IncludeVault), dbutil.BoolToInt(e.InheritLLM), e.EggPort, e.AllowedTools, e.UpdatedAt, e.ID)
	if err != nil {
		return fmt.Errorf("failed to update egg: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("egg not found: %s", e.ID)
	}
	return nil
}

// DeleteEgg removes an egg record by ID.
func DeleteEgg(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM eggs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete egg: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("egg not found: %s", id)
	}
	return nil
}

// ToggleEggActive sets the active flag for an egg.
func ToggleEggActive(db *sql.DB, id string, active bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(`UPDATE eggs SET active=?, updated_at=? WHERE id=?`, dbutil.BoolToInt(active), now, id)
	if err != nil {
		return fmt.Errorf("failed to toggle egg: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("egg not found: %s", id)
	}
	return nil
}

// GetNestByName retrieves a nest by its name (case-insensitive).
func GetNestByName(db *sql.DB, name string) (NestRecord, error) {
	query := `SELECT id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	          hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at FROM nests WHERE LOWER(name) = LOWER(?)`
	n, err := scanNestRow(db.QueryRow(query, name))
	if err != nil {
		if err == sql.ErrNoRows {
			return NestRecord{}, fmt.Errorf("nest not found by name: %s", name)
		}
		return NestRecord{}, fmt.Errorf("failed to get nest by name: %w", err)
	}
	return n, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func scanNests(rows *sql.Rows) ([]NestRecord, error) {
	var nests []NestRecord
	for rows.Next() {
		n, err := scanNestRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nest row: %w", err)
		}
		nests = append(nests, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("nest rows iteration: %w", err)
	}
	return nests, nil
}

func scanEggs(rows *sql.Rows) ([]EggRecord, error) {
	var eggs []EggRecord
	for rows.Next() {
		var e EggRecord
		var active, permanent, includeVault, inheritLLM int
		var descNull, modelNull, provNull, urlNull, keyNull, toolsNull sql.NullString
		if err := rows.Scan(&e.ID, &e.Name, &descNull, &modelNull, &provNull, &urlNull, &keyNull, &active,
			&permanent, &includeVault, &inheritLLM, &e.EggPort, &toolsNull, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan egg row: %w", err)
		}
		e.Description = nullStr(descNull)
		e.Model = nullStr(modelNull)
		e.Provider = nullStr(provNull)
		e.BaseURL = nullStr(urlNull)
		e.APIKeyRef = nullStr(keyNull)
		e.AllowedTools = nullStr(toolsNull)
		e.Active = active != 0
		e.Permanent = permanent != 0
		e.IncludeVault = includeVault != 0
		e.InheritLLM = inheritLLM != 0
		if e.EggPort <= 0 {
			e.EggPort = 8099
		}
		eggs = append(eggs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("egg rows iteration: %w", err)
	}
	return eggs, nil
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// ── Hatch helpers ───────────────────────────────────────────────────────────

// UpdateNestHatchStatus atomically updates the deployment state of a nest.
func UpdateNestHatchStatus(db *sql.DB, id, status, hatchError string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var query string
	var args []interface{}
	switch status {
	case "running":
		query = `UPDATE nests SET hatch_status=?, hatch_error='', last_hatch_at=?, updated_at=? WHERE id=?`
		args = []interface{}{status, now, now, id}
	case "failed":
		query = `UPDATE nests SET hatch_status=?, hatch_error=?, updated_at=? WHERE id=?`
		args = []interface{}{status, hatchError, now, id}
	default: // idle, hatching, stopped
		query = `UPDATE nests SET hatch_status=?, updated_at=? WHERE id=?`
		args = []interface{}{status, now, id}
	}
	res, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update hatch status: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("nest not found: %s", id)
	}
	return nil
}

// ── Task tracking ───────────────────────────────────────────────────────────

// TaskRecord represents a tracked task sent to an egg.
type TaskRecord struct {
	ID           string `json:"id"`
	NestID       string `json:"nest_id"`
	EggID        string `json:"egg_id"`
	Description  string `json:"description"`
	Timeout      int    `json:"timeout"`
	Status       string `json:"status"` // pending, sent, acked, completed, failed, timeout
	ResultOutput string `json:"result_output"`
	ResultError  string `json:"result_error"`
	CreatedAt    string `json:"created_at"`
	SentAt       string `json:"sent_at"`
	CompletedAt  string `json:"completed_at"`
}

// CreateTask inserts a new task record with status "pending".
func CreateTask(db *sql.DB, nestID, eggID, description string, timeout int) (string, error) {
	id := uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO invasion_tasks (id, nest_id, egg_id, description, timeout, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'pending', ?)`, id, nestID, eggID, description, timeout, now)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}
	return id, nil
}

// UpdateTaskStatus transitions a task to a new status.
func UpdateTaskStatus(db *sql.DB, taskID, status, output, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var query string
	var args []interface{}
	switch status {
	case "sent":
		query = `UPDATE invasion_tasks SET status=?, sent_at=? WHERE id=?`
		args = []interface{}{status, now, taskID}
	case "completed", "failed", "timeout":
		query = `UPDATE invasion_tasks SET status=?, result_output=?, result_error=?, completed_at=? WHERE id=?`
		args = []interface{}{status, output, errMsg, now, taskID}
	default: // acked, etc.
		query = `UPDATE invasion_tasks SET status=? WHERE id=?`
		args = []interface{}{status, taskID}
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}
	return nil
}

// GetPendingTasks returns all tasks for a nest that are in a recoverable state.
func GetPendingTasks(db *sql.DB, nestID string) ([]TaskRecord, error) {
	rows, err := db.Query(`SELECT id, nest_id, egg_id, description, timeout, status,
		result_output, result_error, created_at, sent_at, completed_at
		FROM invasion_tasks WHERE nest_id=? AND status IN ('pending','sent') ORDER BY created_at ASC`, nestID)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

// GetTasksByNest returns all tasks for a nest, ordered by creation time (newest first).
func GetTasksByNest(db *sql.DB, nestID string, limit int) ([]TaskRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(`SELECT id, nest_id, egg_id, description, timeout, status,
		result_output, result_error, created_at, sent_at, completed_at
		FROM invasion_tasks WHERE nest_id=? ORDER BY created_at DESC LIMIT ?`, nestID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

// GetTaskByID retrieves a single task by ID.
func GetTaskByID(db *sql.DB, taskID string) (*TaskRecord, error) {
	row := db.QueryRow(`SELECT id, nest_id, egg_id, description, timeout, status,
		result_output, result_error, created_at, sent_at, completed_at
		FROM invasion_tasks WHERE id=?`, taskID)
	var t TaskRecord
	var sentAt, completedAt sql.NullString
	err := row.Scan(&t.ID, &t.NestID, &t.EggID, &t.Description, &t.Timeout, &t.Status,
		&t.ResultOutput, &t.ResultError, &t.CreatedAt, &sentAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	t.SentAt = nullStr(sentAt)
	t.CompletedAt = nullStr(completedAt)
	return &t, nil
}

// CleanupOldTasks removes completed/failed tasks older than the given duration.
func CleanupOldTasks(db *sql.DB, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	res, err := db.Exec(`DELETE FROM invasion_tasks WHERE status IN ('completed','failed','timeout') AND completed_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old tasks: %w", err)
	}
	return res.RowsAffected()
}

func scanTasks(rows *sql.Rows) ([]TaskRecord, error) {
	var tasks []TaskRecord
	for rows.Next() {
		var t TaskRecord
		var sentAt, completedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.NestID, &t.EggID, &t.Description, &t.Timeout, &t.Status,
			&t.ResultOutput, &t.ResultError, &t.CreatedAt, &sentAt, &completedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		t.SentAt = nullStr(sentAt)
		t.CompletedAt = nullStr(completedAt)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ── Deployment history ──────────────────────────────────────────────────────

// DeploymentRecord tracks a single deployment attempt for audit and rollback.
type DeploymentRecord struct {
	ID           string `json:"id"`
	NestID       string `json:"nest_id"`
	EggID        string `json:"egg_id"`
	Status       string `json:"status"` // started, deployed, verified, failed, rolled_back
	BinaryHash   string `json:"binary_hash"`
	ConfigHash   string `json:"config_hash"`
	DeployMethod string `json:"deploy_method"`
	CreatedAt    string `json:"created_at"`
	DeployedAt   string `json:"deployed_at"`
	VerifiedAt   string `json:"verified_at"`
	RolledBackAt string `json:"rolled_back_at"`
}

// CreateDeployment inserts a new deployment history record with status "started".
func CreateDeployment(db *sql.DB, nestID, eggID, deployMethod, binaryHash, configHash string) (string, error) {
	id := uid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO deployment_history (id, nest_id, egg_id, status, binary_hash, config_hash, deploy_method, created_at)
		VALUES (?, ?, ?, 'started', ?, ?, ?, ?)`, id, nestID, eggID, binaryHash, configHash, deployMethod, now)
	if err != nil {
		return "", fmt.Errorf("failed to create deployment record: %w", err)
	}
	return id, nil
}

// UpdateDeploymentStatus transitions a deployment to a new status.
func UpdateDeploymentStatus(db *sql.DB, deployID, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var query string
	var args []interface{}
	switch status {
	case "deployed":
		query = `UPDATE deployment_history SET status=?, deployed_at=? WHERE id=?`
		args = []interface{}{status, now, deployID}
	case "verified":
		query = `UPDATE deployment_history SET status=?, verified_at=? WHERE id=?`
		args = []interface{}{status, now, deployID}
	case "rolled_back":
		query = `UPDATE deployment_history SET status=?, rolled_back_at=? WHERE id=?`
		args = []interface{}{status, now, deployID}
	default: // failed, etc.
		query = `UPDATE deployment_history SET status=? WHERE id=?`
		args = []interface{}{status, deployID}
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update deployment status: %w", err)
	}
	return nil
}

// GetDeploymentHistory returns deployment history for a nest (newest first).
func GetDeploymentHistory(db *sql.DB, nestID string, limit int) ([]DeploymentRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`SELECT id, nest_id, egg_id, status, binary_hash, config_hash, deploy_method,
		created_at, deployed_at, verified_at, rolled_back_at
		FROM deployment_history WHERE nest_id=? ORDER BY created_at DESC LIMIT ?`, nestID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query deployment history: %w", err)
	}
	defer rows.Close()
	return scanDeployments(rows)
}

// GetDeployment retrieves a single deployment record by ID.
func GetDeployment(db *sql.DB, deployID string) (*DeploymentRecord, error) {
	row := db.QueryRow(`SELECT id, nest_id, egg_id, status, binary_hash, config_hash, deploy_method,
		created_at, deployed_at, verified_at, rolled_back_at
		FROM deployment_history WHERE id=?`, deployID)
	d, err := scanDeploymentRow(row)
	if err != nil {
		return nil, fmt.Errorf("deployment not found: %w", err)
	}
	return &d, nil
}

// GetLastSuccessfulDeployment returns the most recent verified or deployed deployment for a nest.
func GetLastSuccessfulDeployment(db *sql.DB, nestID string) (*DeploymentRecord, error) {
	row := db.QueryRow(`SELECT id, nest_id, egg_id, status, binary_hash, config_hash, deploy_method,
		created_at, deployed_at, verified_at, rolled_back_at
		FROM deployment_history WHERE nest_id=? AND status IN ('verified','deployed') ORDER BY created_at DESC LIMIT 1`, nestID)
	d, err := scanDeploymentRow(row)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// CleanupOldDeployments removes deployment records older than the given duration.
func CleanupOldDeployments(db *sql.DB, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	res, err := db.Exec(`DELETE FROM deployment_history WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old deployments: %w", err)
	}
	return res.RowsAffected()
}

func scanDeploymentRow(scanner interface{ Scan(...interface{}) error }) (DeploymentRecord, error) {
	var d DeploymentRecord
	var deployedAt, verifiedAt, rolledBackAt sql.NullString
	err := scanner.Scan(&d.ID, &d.NestID, &d.EggID, &d.Status, &d.BinaryHash, &d.ConfigHash, &d.DeployMethod,
		&d.CreatedAt, &deployedAt, &verifiedAt, &rolledBackAt)
	if err != nil {
		return DeploymentRecord{}, err
	}
	d.DeployedAt = nullStr(deployedAt)
	d.VerifiedAt = nullStr(verifiedAt)
	d.RolledBackAt = nullStr(rolledBackAt)
	return d, nil
}

func scanDeployments(rows *sql.Rows) ([]DeploymentRecord, error) {
	var deployments []DeploymentRecord
	for rows.Next() {
		d, err := scanDeploymentRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, rows.Err()
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
