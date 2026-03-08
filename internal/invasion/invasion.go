package invasion

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

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
	db, err := sql.Open("sqlite", dbPath)
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
		_, _ = db.Exec(m) // ignore "duplicate column" errors
	}

	// Drop the personality column if it still exists (eggs no longer use personality)
	// SQLite doesn't support DROP COLUMN before 3.35.0, so we just ignore it.

	return db, nil
}

// ── Nests CRUD ──────────────────────────────────────────────────────────────

// CreateNest generates a UUID and inserts a new nest record.
func CreateNest(db *sql.DB, n NestRecord) (string, error) {
	n.ID = uuid.New().String()
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
		boolToInt(n.Active), n.EggID, n.HatchStatus, n.LastHatchAt, n.HatchError, n.Route, n.RouteConfig, n.DeployMethod, n.TargetArch, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert nest: %w", err)
	}
	return nil
}

// GetNest retrieves a single nest by ID.
func GetNest(db *sql.DB, id string) (NestRecord, error) {
	query := `SELECT id, name, notes, access_type, host, port, username, vault_secret_id, active, egg_id,
	          hatch_status, last_hatch_at, hatch_error, route, route_config, deploy_method, target_arch, created_at, updated_at FROM nests WHERE id = ?`
	var n NestRecord
	var active int
	var notesNull, hostNull, userNull, secretNull, eggNull sql.NullString
	var hatchStatusNull, lastHatchNull, hatchErrNull, routeNull, routeCfgNull, deployNull, archNull sql.NullString
	err := db.QueryRow(query, id).Scan(&n.ID, &n.Name, &notesNull, &n.AccessType, &hostNull, &n.Port, &userNull, &secretNull, &active, &eggNull,
		&hatchStatusNull, &lastHatchNull, &hatchErrNull, &routeNull, &routeCfgNull, &deployNull, &archNull, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return NestRecord{}, fmt.Errorf("nest not found: %s", id)
		}
		return NestRecord{}, fmt.Errorf("failed to get nest: %w", err)
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
	res, err := db.Exec(query, n.Name, n.Notes, n.AccessType, n.Host, n.Port, n.Username, n.VaultSecretID, boolToInt(n.Active), n.EggID,
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
	res, err := db.Exec(`UPDATE nests SET active=?, updated_at=? WHERE id=?`, boolToInt(active), now, id)
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
	e.ID = uuid.New().String()
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
		boolToInt(e.Active), boolToInt(e.Permanent), boolToInt(e.IncludeVault), boolToInt(e.InheritLLM), e.EggPort, e.AllowedTools, e.CreatedAt, e.UpdatedAt)
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
	res, err := db.Exec(query, e.Name, e.Description, e.Model, e.Provider, e.BaseURL, e.APIKeyRef, boolToInt(e.Active),
		boolToInt(e.Permanent), boolToInt(e.IncludeVault), boolToInt(e.InheritLLM), e.EggPort, e.AllowedTools, e.UpdatedAt, e.ID)
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
	res, err := db.Exec(`UPDATE eggs SET active=?, updated_at=? WHERE id=?`, boolToInt(active), now, id)
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
	var n NestRecord
	var active int
	var notesNull, hostNull, userNull, secretNull, eggNull sql.NullString
	var hatchStatusNull, lastHatchNull, hatchErrNull, routeNull, routeCfgNull, deployNull, archNull sql.NullString
	err := db.QueryRow(query, name).Scan(&n.ID, &n.Name, &notesNull, &n.AccessType, &hostNull, &n.Port, &userNull, &secretNull, &active, &eggNull,
		&hatchStatusNull, &lastHatchNull, &hatchErrNull, &routeNull, &routeCfgNull, &deployNull, &archNull, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return NestRecord{}, fmt.Errorf("nest not found by name: %s", name)
		}
		return NestRecord{}, fmt.Errorf("failed to get nest by name: %w", err)
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

// ── Helpers ─────────────────────────────────────────────────────────────────

func scanNests(rows *sql.Rows) ([]NestRecord, error) {
	var nests []NestRecord
	for rows.Next() {
		var n NestRecord
		var active int
		var notesNull, hostNull, userNull, secretNull, eggNull sql.NullString
		var hatchStatusNull, lastHatchNull, hatchErrNull, routeNull, routeCfgNull, deployNull, archNull sql.NullString
		if err := rows.Scan(&n.ID, &n.Name, &notesNull, &n.AccessType, &hostNull, &n.Port, &userNull, &secretNull, &active, &eggNull,
			&hatchStatusNull, &lastHatchNull, &hatchErrNull, &routeNull, &routeCfgNull, &deployNull, &archNull, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan nest row: %w", err)
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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
