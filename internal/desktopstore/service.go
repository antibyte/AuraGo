package desktopstore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"

	_ "modernc.org/sqlite"
)

const storeSource = "desktop-store"

const oliveTinDesktopNote = `OliveTin configuration

The editable OliveTin configuration file is available in the virtual desktop workspace at:

Shared/OliveTin/config.yaml

Open Files, then Shared, then OliveTin to edit config.yaml.
Inside the OliveTin container this file is mounted as /config/config.yaml.

After changing the configuration, restart OliveTin from the Software Store if it does not reload automatically.
`

var storeAppIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// ErrOperationInProgress is returned when an app already has an active
// lifecycle operation. Callers should surface this as a conflict, not bad input.
var ErrOperationInProgress = errors.New("desktop store operation already in progress")

const operationStaleAfter = 35 * time.Minute
const appReadinessTimeout = 60 * time.Second
const appReadinessPollInterval = 500 * time.Millisecond

// Config describes the desktop store service dependencies.
type Config struct {
	DBPath        string
	DockerHost    string
	DataDir       string
	WorkspaceDir  string
	Catalog       []CatalogEntry
	Docker        DockerAdapter
	Desktop       DesktopAdapter
	Launchpad     LaunchpadAdapter
	Secrets       SecretStore
	PortAllocator PortAllocator
	PortProbe     PortProbe
}

// Service owns the software store catalog, persistent install records and
// lifecycle operations.
type Service struct {
	mu            sync.Mutex
	cfg           Config
	db            *sql.DB
	catalog       []CatalogEntry
	catalogByID   map[string]CatalogEntry
	portAllocator PortAllocator
	portProbe     PortProbe
	closed        bool
}

// NewService creates a desktop store service. Call Init before use.
func NewService(cfg Config) (*Service, error) {
	if strings.TrimSpace(cfg.DBPath) == "" {
		return nil, fmt.Errorf("desktop store database path is required")
	}
	dbPath, err := filepath.Abs(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("resolve desktop store database path: %w", err)
	}
	cfg.DBPath = filepath.Clean(dbPath)
	if strings.TrimSpace(cfg.WorkspaceDir) != "" {
		workspaceDir, err := filepath.Abs(cfg.WorkspaceDir)
		if err != nil {
			return nil, fmt.Errorf("resolve desktop store workspace directory: %w", err)
		}
		cfg.WorkspaceDir = filepath.Clean(workspaceDir)
	}
	catalog := cfg.Catalog
	if len(catalog) == 0 {
		catalog = DefaultCatalog()
	}
	normalizedCatalog := make([]CatalogEntry, 0, len(catalog))
	catalogByID := make(map[string]CatalogEntry, len(catalog))
	for _, entry := range catalog {
		entry.ID = normalizeAppID(entry.ID)
		if !storeAppIDPattern.MatchString(entry.ID) {
			return nil, fmt.Errorf("invalid catalog app id %q", entry.ID)
		}
		if entry.Image == "" {
			return nil, fmt.Errorf("catalog app %s image is required", entry.ID)
		}
		if entry.PrimaryPort.Protocol == "" {
			entry.PrimaryPort.Protocol = "tcp"
		}
		if entry.PrimaryPort.ID == "" {
			entry.PrimaryPort.ID = "main"
		}
		if entry.PrimaryPort.Name == "" {
			entry.PrimaryPort.Name = "Web UI"
		}
		if entry.PrimaryPort.ContainerPort <= 0 {
			return nil, fmt.Errorf("catalog app %s container port is required", entry.ID)
		}
		for i := range entry.ExtraPorts {
			if entry.ExtraPorts[i].ID == "" {
				entry.ExtraPorts[i].ID = fmt.Sprintf("port-%d", i+2)
			}
			if entry.ExtraPorts[i].Name == "" {
				entry.ExtraPorts[i].Name = entry.ExtraPorts[i].ID
			}
			if entry.ExtraPorts[i].Protocol == "" {
				entry.ExtraPorts[i].Protocol = "tcp"
			}
			if entry.ExtraPorts[i].ContainerPort <= 0 {
				return nil, fmt.Errorf("catalog app %s extra port %s is invalid", entry.ID, entry.ExtraPorts[i].ID)
			}
		}
		for _, bind := range entry.HostBinds {
			if strings.TrimSpace(bind.HostPath) == "" || strings.TrimSpace(bind.ContainerPath) == "" {
				return nil, fmt.Errorf("catalog app %s host bind paths are required", entry.ID)
			}
		}
		for _, bind := range entry.WorkspaceBinds {
			if strings.TrimSpace(bind.WorkspacePath) == "" || strings.TrimSpace(bind.ContainerPath) == "" {
				return nil, fmt.Errorf("catalog app %s workspace bind paths are required", entry.ID)
			}
		}
		for _, secret := range entry.GeneratedSecrets {
			if strings.TrimSpace(secret.Key) == "" {
				return nil, fmt.Errorf("catalog app %s generated secret key is required", entry.ID)
			}
		}
		for _, companion := range entry.Companions {
			if !storeAppIDPattern.MatchString(normalizeAppID(companion.ID)) {
				return nil, fmt.Errorf("catalog app %s companion id %q is invalid", entry.ID, companion.ID)
			}
			if strings.TrimSpace(companion.Image) == "" {
				return nil, fmt.Errorf("catalog app %s companion %s image is required", entry.ID, companion.ID)
			}
		}
		catalogByID[entry.ID] = entry
		normalizedCatalog = append(normalizedCatalog, entry)
	}
	allocator := cfg.PortAllocator
	if allocator == nil {
		allocator = DefaultPortAllocator
	}
	portProbe := cfg.PortProbe
	if portProbe == nil {
		portProbe = storePortAccepts
	}
	return &Service{
		cfg:           cfg,
		catalog:       normalizedCatalog,
		catalogByID:   catalogByID,
		portAllocator: allocator,
		portProbe:     portProbe,
	}, nil
}

// Init opens and migrates the store database.
func (s *Service) Init(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("desktop store service is closed")
	}
	if s.db != nil {
		return nil
	}
	if err := ensureDir(filepath.Dir(s.cfg.DBPath)); err != nil {
		return fmt.Errorf("create desktop store database directory: %w", err)
	}
	db, err := sql.Open("sqlite", s.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open desktop store database: %w", err)
	}
	s.db = db
	if err := s.migrateLocked(ctx); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	if err := s.recoverInterruptedOperationsLocked(ctx); err != nil {
		db.Close()
		s.db = nil
		return err
	}
	return nil
}

// Close closes the store database.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// Catalog returns the fixed allowlist.
func (s *Service) Catalog() []CatalogEntry {
	return append([]CatalogEntry(nil), s.catalog...)
}

func (s *Service) migrateLocked(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS desktop_store_apps (
			app_id TEXT PRIMARY KEY,
			desktop_app_id TEXT NOT NULL,
			launchpad_link_id TEXT,
			container_name TEXT NOT NULL,
			container_id TEXT,
			image TEXT NOT NULL,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			bind_mode TEXT NOT NULL,
			host_ip TEXT NOT NULL,
			host_port INTEGER NOT NULL,
			container_port INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			tailscale_enabled INTEGER NOT NULL DEFAULT 0,
			tailscale_status TEXT NOT NULL DEFAULT 'disabled',
			tailscale_port INTEGER NOT NULL DEFAULT 0,
			logo_path TEXT NOT NULL DEFAULT '',
			ports_json TEXT NOT NULL DEFAULT '[]',
			volumes_json TEXT NOT NULL DEFAULT '[]',
			host_binds_json TEXT NOT NULL DEFAULT '[]',
			env_json TEXT NOT NULL DEFAULT '[]',
			extra_hosts_json TEXT NOT NULL DEFAULT '[]',
			secret_refs_json TEXT NOT NULL DEFAULT '[]',
			companions_json TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_operation_id TEXT NOT NULL DEFAULT '',
			last_operation_type TEXT NOT NULL DEFAULT '',
			last_operation_state TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS desktop_store_operations (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			app_id TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			request_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			completed_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_desktop_store_operations_app ON desktop_store_operations(app_id, created_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate desktop store database: %w", err)
		}
	}
	for _, column := range []struct {
		name string
		def  string
	}{
		{"ports_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"host_binds_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"secret_refs_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"companions_json", "TEXT NOT NULL DEFAULT '[]'"},
	} {
		if err := s.ensureColumn(ctx, "desktop_store_apps", column.name, column.def); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureColumn(ctx context.Context, table, name, def string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect desktop store table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan desktop store table %s column: %w", table, err)
		}
		if strings.EqualFold(colName, name) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read desktop store table %s columns: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, name, def)); err != nil {
		return fmt.Errorf("add desktop store column %s.%s: %w", table, name, err)
	}
	return nil
}

func (s *Service) recoverInterruptedOperationsLocked(ctx context.Context) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `UPDATE desktop_store_operations
		SET status = ?, message = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE status IN (?, ?)`,
		OperationFailed, "operation interrupted", "operation did not complete before the store service restarted",
		formatTime(now), formatTime(now), OperationPending, OperationRunning)
	if err != nil {
		return fmt.Errorf("recover interrupted desktop store operations: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT app_id, desktop_app_id, launchpad_link_id, container_name, container_id, image,
		status, error, bind_mode, host_ip, host_port, container_port, protocol, tailscale_enabled, tailscale_status,
		tailscale_port, logo_path, ports_json, volumes_json, host_binds_json, env_json, extra_hosts_json,
		secret_refs_json, companions_json, created_at, updated_at,
		last_operation_id, last_operation_type, last_operation_state
		FROM desktop_store_apps WHERE status IN (?, ?)`, AppStatusInstalling, AppStatusUpdating)
	if err != nil {
		return fmt.Errorf("load interrupted desktop store apps: %w", err)
	}
	defer rows.Close()
	var apps []InstalledApp
	for rows.Next() {
		app, err := scanInstalledApp(rows)
		if err != nil {
			return err
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read interrupted desktop store apps: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE desktop_store_apps
		SET last_operation_state = ?, updated_at = ?
		WHERE last_operation_state IN (?, ?)
			AND status NOT IN (?, ?)`,
		OperationFailed, formatTime(now), OperationPending, OperationRunning, AppStatusInstalling, AppStatusUpdating); err != nil {
		return fmt.Errorf("clear stale desktop store operation markers: %w", err)
	}
	for _, app := range apps {
		switch app.Status {
		case AppStatusInstalling:
			if err := s.cleanupInstallArtifacts(ctx, app); err != nil {
				return fmt.Errorf("recover interrupted install for %s: %w", app.AppID, err)
			}
		case AppStatusUpdating:
			app.Status = AppStatusError
			app.Error = "Store operation was interrupted before completion."
			app.LastOperationState = OperationFailed
			if err := s.saveInstalled(ctx, app); err != nil {
				return fmt.Errorf("recover interrupted update for %s: %w", app.AppID, err)
			}
		}
	}
	return nil
}

// StartInstall persists a pending install operation. The caller may run it in a
// goroutine via RunOperation.
func (s *Service) StartInstall(ctx context.Context, req InstallRequest) (Operation, error) {
	req.AppID = normalizeAppID(req.AppID)
	if _, ok := s.catalogByID[req.AppID]; !ok {
		return Operation{}, fmt.Errorf("store app %q is not in the allowlist", req.AppID)
	}
	req.BindMode = normalizeBindMode(req.BindMode)
	if req.BindMode == "" {
		return Operation{}, fmt.Errorf("unsupported bind mode")
	}
	return s.createOperation(ctx, OperationInstall, req.AppID, req)
}

// StartAppOperation persists a pending lifecycle operation for an installed app.
func (s *Service) StartAppOperation(ctx context.Context, appID, opType string, req OperationRequest) (Operation, error) {
	appID = normalizeAppID(appID)
	switch opType {
	case OperationUpdate, OperationStart, OperationStop, OperationRestart, OperationUninstall:
	default:
		return Operation{}, fmt.Errorf("unsupported store operation %q", opType)
	}
	if _, ok, err := s.GetInstalled(ctx, appID); err != nil {
		return Operation{}, err
	} else if !ok {
		return Operation{}, fmt.Errorf("store app %q is not installed", appID)
	}
	return s.createOperation(ctx, opType, appID, req)
}

func (s *Service) createOperation(ctx context.Context, opType, appID string, request any) (Operation, error) {
	if err := s.ensureReady(ctx); err != nil {
		return Operation{}, err
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return Operation{}, fmt.Errorf("marshal operation request: %w", err)
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rejectActiveOperationLocked(ctx, appID, now); err != nil {
		return Operation{}, err
	}

	op := Operation{
		ID:          newID("op"),
		Type:        opType,
		AppID:       appID,
		Status:      OperationPending,
		RequestJSON: string(requestJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO desktop_store_operations(id, type, app_id, status, request_json, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		op.ID, op.Type, op.AppID, op.Status, op.RequestJSON, formatTime(now), formatTime(now))
	if err != nil {
		return Operation{}, fmt.Errorf("create desktop store operation: %w", err)
	}
	return op, nil
}

func (s *Service) rejectActiveOperationLocked(ctx context.Context, appID string, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id, status, updated_at FROM desktop_store_operations
		WHERE app_id = ? AND status IN (?, ?)
		ORDER BY created_at ASC`,
		appID, OperationPending, OperationRunning)
	if err != nil {
		return fmt.Errorf("check active desktop store operations: %w", err)
	}
	var staleIDs []string
	var activeErr error
	for rows.Next() {
		var id, status, updatedAtText string
		if err := rows.Scan(&id, &status, &updatedAtText); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan active desktop store operation: %w", err)
		}
		updatedAt, parseErr := time.Parse(time.RFC3339Nano, updatedAtText)
		if parseErr == nil && now.Sub(updatedAt) > operationStaleAfter {
			staleIDs = append(staleIDs, id)
			continue
		}
		if activeErr == nil {
			activeErr = fmt.Errorf("%w: app %s has %s operation %s", ErrOperationInProgress, appID, status, id)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("read active desktop store operations: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close active desktop store operations: %w", err)
	}
	for _, id := range staleIDs {
		if _, err := s.db.ExecContext(ctx, `UPDATE desktop_store_operations
			SET status = ?, message = ?, error = ?, updated_at = ?, completed_at = ?
			WHERE id = ?`,
			OperationFailed, "stale operation reset", "operation timed out before completion",
			formatTime(now), formatTime(now), id); err != nil {
			return fmt.Errorf("reset stale desktop store operation: %w", err)
		}
	}
	if activeErr != nil {
		return activeErr
	}
	return nil
}

// RunOperation executes a pending operation and stores its terminal state.
func (s *Service) RunOperation(ctx context.Context, operationID string) error {
	op, err := s.Operation(ctx, operationID)
	if err != nil {
		return err
	}
	if op.Status != OperationPending {
		return fmt.Errorf("operation %s is not pending", op.ID)
	}
	if err := s.updateOperation(ctx, op.ID, OperationRunning, "running", ""); err != nil {
		return err
	}
	var runErr error
	switch op.Type {
	case OperationInstall:
		var req InstallRequest
		if err := json.Unmarshal([]byte(op.RequestJSON), &req); err != nil {
			runErr = fmt.Errorf("parse install request: %w", err)
		} else {
			runErr = s.install(ctx, op, req)
		}
	case OperationUpdate:
		runErr = s.update(ctx, op)
	case OperationStart:
		runErr = s.start(ctx, op)
	case OperationStop:
		runErr = s.stop(ctx, op)
	case OperationRestart:
		runErr = s.restart(ctx, op)
	case OperationUninstall:
		var req OperationRequest
		if err := json.Unmarshal([]byte(op.RequestJSON), &req); err != nil {
			runErr = fmt.Errorf("parse uninstall request: %w", err)
		} else {
			runErr = s.uninstall(ctx, op, req.DeleteData)
		}
	default:
		runErr = fmt.Errorf("unsupported operation %q", op.Type)
	}
	if runErr != nil {
		_ = s.updateOperation(ctx, op.ID, OperationFailed, "", runErr.Error())
		return runErr
	}
	return s.updateOperation(ctx, op.ID, OperationSucceeded, "completed", "")
}

// Operation returns a persisted operation by ID.
func (s *Service) Operation(ctx context.Context, operationID string) (Operation, error) {
	if err := s.ensureReady(ctx); err != nil {
		return Operation{}, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT id, type, app_id, status, message, error, request_json, created_at, updated_at, completed_at
		FROM desktop_store_operations WHERE id = ?`, strings.TrimSpace(operationID))
	return scanOperation(row)
}

func (s *Service) updateOperation(ctx context.Context, id, status, message, errText string) error {
	now := time.Now().UTC()
	completed := sql.NullString{}
	if status == OperationSucceeded || status == OperationFailed {
		completed.Valid = true
		completed.String = formatTime(now)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE desktop_store_operations
		SET status = ?, message = ?, error = ?, updated_at = ?, completed_at = ?
		WHERE id = ?`,
		status, message, errText, formatTime(now), completed, id)
	if err != nil {
		return fmt.Errorf("update desktop store operation: %w", err)
	}
	return nil
}

// ListApps returns all installed store apps.
func (s *Service) ListApps(ctx context.Context) ([]InstalledApp, error) {
	if err := s.ensureReady(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT app_id, desktop_app_id, launchpad_link_id, container_name, container_id, image,
		status, error, bind_mode, host_ip, host_port, container_port, protocol, tailscale_enabled, tailscale_status,
		tailscale_port, logo_path, ports_json, volumes_json, host_binds_json, env_json, extra_hosts_json,
		secret_refs_json, companions_json, created_at, updated_at,
		last_operation_id, last_operation_type, last_operation_state
		FROM desktop_store_apps ORDER BY app_id`)
	if err != nil {
		return nil, fmt.Errorf("list desktop store apps: %w", err)
	}
	defer rows.Close()
	var apps []InstalledApp
	for rows.Next() {
		app, err := scanInstalledApp(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

// GetInstalled returns one installed store app.
func (s *Service) GetInstalled(ctx context.Context, appID string) (InstalledApp, bool, error) {
	if err := s.ensureReady(ctx); err != nil {
		return InstalledApp{}, false, err
	}
	row := s.db.QueryRowContext(ctx, `SELECT app_id, desktop_app_id, launchpad_link_id, container_name, container_id, image,
		status, error, bind_mode, host_ip, host_port, container_port, protocol, tailscale_enabled, tailscale_status,
		tailscale_port, logo_path, ports_json, volumes_json, host_binds_json, env_json, extra_hosts_json,
		secret_refs_json, companions_json, created_at, updated_at,
		last_operation_id, last_operation_type, last_operation_state
		FROM desktop_store_apps WHERE app_id = ?`, normalizeAppID(appID))
	app, err := scanInstalledApp(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InstalledApp{}, false, nil
		}
		return InstalledApp{}, false, err
	}
	return app, true, nil
}

// OpenURL computes the best URL for an installed app in the current request
// context. Tailscale is used only when the caller marks the request as coming
// from a Tailnet surface and the proxy is active.
func (s *Service) OpenURL(ctx context.Context, appID, requestHost string, fromTailscale bool, tailscaleDNS string, portID ...string) (string, InstalledApp, error) {
	app, ok, err := s.GetInstalled(ctx, appID)
	if err != nil {
		return "", InstalledApp{}, err
	}
	if !ok {
		return "", InstalledApp{}, fmt.Errorf("store app %q is not installed", appID)
	}
	if app.Status != AppStatusRunning {
		return "", InstalledApp{}, fmt.Errorf("store app %q is not running (status: %s)", app.AppID, app.Status)
	}
	port, err := selectAppPort(app, firstString(portID))
	if err != nil {
		return "", InstalledApp{}, err
	}
	if fromTailscale && app.TailscaleEnabled && app.TailscaleStatus == TailscaleStatusActive && strings.TrimSpace(tailscaleDNS) != "" {
		tailnetPort := port.HostPort
		if firstString(portID) == "" && app.TailscalePort > 0 {
			tailnetPort = app.TailscalePort
		}
		return fmt.Sprintf("https://%s:%d/", strings.Trim(strings.TrimSpace(tailscaleDNS), "."), tailnetPort), app, nil
	}
	host := "127.0.0.1"
	if app.BindMode == BindModeLAN {
		if reqHost := hostWithoutPort(requestHost); reqHost != "" && !isLoopbackHost(reqHost) {
			host = reqHost
		}
	}
	return fmt.Sprintf("http://%s:%d/", host, port.HostPort), app, nil
}

// SetTailscaleStatus updates the activation state for a stored app proxy.
func (s *Service) SetTailscaleStatus(ctx context.Context, appID, status string) error {
	if err := s.ensureReady(ctx); err != nil {
		return err
	}
	switch status {
	case TailscaleStatusDisabled, TailscaleStatusPending, TailscaleStatusActive:
	default:
		return fmt.Errorf("unsupported tailscale status %q", status)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE desktop_store_apps SET tailscale_status = ?, updated_at = ? WHERE app_id = ?`,
		status, formatTime(time.Now().UTC()), normalizeAppID(appID))
	if err != nil {
		return fmt.Errorf("update desktop store tailscale status: %w", err)
	}
	return nil
}

// ExposedCredentials returns admin-visible generated credentials for a Store app.
func (s *Service) ExposedCredentials(ctx context.Context, appID string) ([]ExposedCredential, error) {
	app, ok, err := s.GetInstalled(ctx, appID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("store app %q is not installed", appID)
	}
	if s.cfg.Secrets == nil {
		return nil, fmt.Errorf("desktop store secret vault is not configured")
	}
	credentials := make([]ExposedCredential, 0, len(app.SecretRefs))
	for _, ref := range app.SecretRefs {
		if !ref.Expose || strings.TrimSpace(ref.VaultKey) == "" {
			continue
		}
		value, err := s.cfg.Secrets.ReadSecret(ref.VaultKey)
		if err != nil {
			return nil, fmt.Errorf("read store credential %s: %w", ref.Key, err)
		}
		credentials = append(credentials, ExposedCredential{
			Key:   ref.Key,
			Label: ref.Label,
			Value: value,
		})
	}
	return credentials, nil
}

func selectAppPort(app InstalledApp, portID string) (PortBinding, error) {
	portID = strings.ToLower(strings.TrimSpace(portID))
	ports := app.Ports
	if len(ports) == 0 && app.HostPort > 0 {
		ports = []PortBinding{{ID: "main", Name: "Web UI", ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}}
	}
	if len(ports) == 0 {
		return PortBinding{}, fmt.Errorf("store app %q has no published ports", app.AppID)
	}
	if portID == "" {
		return ports[0], nil
	}
	for _, port := range ports {
		if strings.EqualFold(port.ID, portID) {
			return port, nil
		}
	}
	return PortBinding{}, fmt.Errorf("store app %q does not expose port %q", app.AppID, portID)
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// ConfigureBeszelAgent stores the local Beszel agent credentials and recreates
// the allowlisted host-network companion container.
func (s *Service) ConfigureBeszelAgent(ctx context.Context, key, token string) (InstalledApp, error) {
	if err := s.ensureReady(ctx); err != nil {
		return InstalledApp{}, err
	}
	key = strings.TrimSpace(key)
	token = strings.TrimSpace(token)
	if key == "" || token == "" {
		return InstalledApp{}, fmt.Errorf("Beszel agent key and token are required")
	}
	if s.cfg.Secrets == nil {
		return InstalledApp{}, fmt.Errorf("desktop store secret vault is not configured")
	}
	if err := s.cfg.Secrets.WriteSecret("desktop_store_beszel_agent_key", key); err != nil {
		return InstalledApp{}, fmt.Errorf("write Beszel agent key: %w", err)
	}
	if err := s.cfg.Secrets.WriteSecret("desktop_store_beszel_agent_token", token); err != nil {
		return InstalledApp{}, fmt.Errorf("write Beszel agent token: %w", err)
	}
	app, ok, err := s.GetInstalled(ctx, "beszel")
	if err != nil {
		return InstalledApp{}, err
	}
	if !ok {
		return InstalledApp{}, fmt.Errorf("Beszel hub is not installed")
	}
	entry, ok := s.catalogByID["beszel"]
	if !ok || len(entry.Companions) == 0 {
		return InstalledApp{}, fmt.Errorf("Beszel agent companion is not in the allowlist")
	}
	var template CompanionTemplate
	for _, candidate := range entry.Companions {
		if candidate.ID == "agent" {
			template = candidate
			break
		}
	}
	if template.ID == "" {
		return InstalledApp{}, fmt.Errorf("Beszel agent companion is not in the allowlist")
	}
	companion := CompanionApp{
		ID:            template.ID,
		Name:          template.Name,
		ContainerName: CompanionContainerName(app.AppID, template.ID),
		Image:         template.Image,
		Status:        AppStatusInstalling,
		NetworkMode:   template.NetworkMode,
		Volumes:       resolveCompanionVolumes(entry, template),
		HostBinds:     resolveHostBinds(template.HostBinds),
	}
	_ = s.requireDocker().StopContainer(ctx, companion.ContainerName)
	_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
	secrets := map[string]string{
		"desktop_store_beszel_agent_key":   key,
		"desktop_store_beszel_agent_token": token,
	}
	companion.Env = applyEnvTemplates(template.Env, app, secrets)
	if err := s.requireDocker().PullImage(ctx, companion.Image); err != nil {
		return InstalledApp{}, err
	}
	containerID, err := s.requireDocker().CreateContainer(ctx, companionContainerSpec(app, companion))
	if err != nil {
		return InstalledApp{}, err
	}
	companion.ContainerID = containerID
	if err := s.requireDocker().StartContainer(ctx, companion.ContainerName); err != nil {
		_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
		return InstalledApp{}, err
	}
	companion.Status = AppStatusRunning
	companion.Error = ""
	app.Companions = replaceCompanion(app.Companions, companion)
	if err := s.saveInstalled(ctx, app); err != nil {
		return InstalledApp{}, err
	}
	return app, nil
}

func (s *Service) install(ctx context.Context, op Operation, req InstallRequest) error {
	entry, ok := s.catalogByID[normalizeAppID(req.AppID)]
	if !ok {
		return fmt.Errorf("store app %q is not in the allowlist", req.AppID)
	}
	if existing, exists, err := s.GetInstalled(ctx, entry.ID); err != nil {
		return err
	} else if exists {
		replace, err := s.canReplaceInstallingRecord(ctx, existing)
		if err != nil {
			return err
		}
		if replace {
			if err := s.cleanupInstallArtifacts(ctx, existing); err != nil {
				return fmt.Errorf("clean up stale store app %q: %w", entry.ID, err)
			}
		} else {
			return fmt.Errorf("store app %q is already installed", entry.ID)
		}
	}
	hostPort, err := s.portAllocator(ctx, entry.PrimaryPort.ContainerPort)
	if err != nil {
		return fmt.Errorf("allocate port: %w", err)
	}
	hostIP := "127.0.0.1"
	bindMode := normalizeBindMode(req.BindMode)
	if bindMode == BindModeLAN {
		hostIP = "0.0.0.0"
	}
	record := s.buildInstallRecord(entry, op, bindMode, hostIP, hostPort, req.TailscaleEnabled)
	ports, err := s.allocatePortBindings(ctx, entry, hostIP, hostPort)
	if err != nil {
		return fmt.Errorf("allocate ports: %w", err)
	}
	record.Ports = ports
	hostBinds, err := s.resolveCatalogHostBinds(entry)
	if err != nil {
		return fmt.Errorf("resolve host binds: %w", err)
	}
	record.HostBinds = hostBinds
	if err := s.prepareManagedWorkspaceBinds(record); err != nil {
		return fmt.Errorf("prepare workspace binds: %w", err)
	}
	env, secretRefs, err := s.installEnv(entry, record)
	if err != nil {
		return fmt.Errorf("resolve install environment: %w", err)
	}
	record.Env = env
	record.SecretRefs = secretRefs
	companions, err := s.prepareAutoCompanions(entry, record)
	if err != nil {
		return fmt.Errorf("resolve companion containers: %w", err)
	}
	record.Companions = companions
	if err := s.saveInstalled(ctx, record); err != nil {
		return err
	}
	if networkName := privateStoreNetworkName(entry); networkName != "" {
		if err := s.requireDocker().CreateNetwork(ctx, networkName); err != nil {
			return s.failInstall(ctx, record, err)
		}
	}
	if err := s.requireDocker().PullImage(ctx, entry.Image); err != nil {
		return s.failInstall(ctx, record, err)
	}
	if err := s.createAutoCompanions(ctx, &record); err != nil {
		return s.failInstall(ctx, record, err)
	}
	spec := containerSpecFromRecord(record)
	containerID, err := s.requireDocker().CreateContainer(ctx, spec)
	if err != nil {
		return s.failInstall(ctx, record, err)
	}
	record.ContainerID = containerID
	if err := s.seedContainerFiles(ctx, entry, record); err != nil {
		return s.failInstall(ctx, record, err)
	}
	if err := s.requireDocker().StartContainer(ctx, record.ContainerName); err != nil {
		return s.failInstall(ctx, record, err)
	}
	if err := s.waitContainerReady(ctx, record, appReadinessTimeout); err != nil {
		return s.failInstall(ctx, record, err)
	}
	record.Status = AppStatusRunning
	record.Error = ""
	if err := s.installDesktopApp(ctx, entry, record); err != nil {
		return s.failInstall(ctx, record, err)
	}
	linkID, err := s.upsertLaunchpad(ctx, entry, record)
	if err != nil {
		return s.failInstall(ctx, record, err)
	}
	record.LaunchpadLinkID = linkID
	record.LastOperationState = OperationSucceeded
	return s.saveInstalled(ctx, record)
}

func (s *Service) update(ctx context.Context, op Operation) error {
	record, ok, err := s.GetInstalled(ctx, op.AppID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("store app %q is not installed", op.AppID)
	}
	previous := record
	previousWasRunning := previous.Status == AppStatusRunning
	entry, ok := s.catalogByID[record.AppID]
	if !ok {
		return fmt.Errorf("store app %q is no longer in the allowlist", record.AppID)
	}
	record.Image = entry.Image
	ports, err := s.updatePortBindings(ctx, entry, record)
	if err != nil {
		return fmt.Errorf("update port bindings: %w", err)
	}
	record.Ports = ports
	if len(ports) > 0 {
		record.HostIP = ports[0].HostIP
		record.HostPort = ports[0].HostPort
		record.ContainerPort = ports[0].ContainerPort
		record.Protocol = strings.ToLower(ports[0].Protocol)
	}
	record.Volumes = resolveVolumes(entry)
	hostBinds, err := s.resolveCatalogHostBinds(entry)
	if err != nil {
		return fmt.Errorf("resolve host binds: %w", err)
	}
	record.HostBinds = hostBinds
	if err := s.prepareManagedWorkspaceBinds(record); err != nil {
		return fmt.Errorf("prepare workspace binds: %w", err)
	}
	record.ExtraHosts = append([]string(nil), entry.ExtraHosts...)
	env, secretRefs, err := s.updateEnv(entry, record, previous)
	if err != nil {
		return fmt.Errorf("resolve update environment: %w", err)
	}
	record.Env = env
	record.SecretRefs = secretRefs
	autoCompanions, err := s.prepareAutoCompanions(entry, record)
	if err != nil {
		return fmt.Errorf("resolve companion containers: %w", err)
	}
	if len(autoCompanions) > 0 {
		record.Companions = mergeCompanions(record.Companions, autoCompanions)
	}
	record.Status = AppStatusUpdating
	record.Error = ""
	record.LastOperationID = op.ID
	record.LastOperationType = op.Type
	record.LastOperationState = OperationRunning
	if err := s.saveInstalled(ctx, record); err != nil {
		return err
	}
	companionsTouched := false
	restorePreviousCompanions := func() error {
		if len(record.Companions) == 0 && len(previous.Companions) == 0 {
			return nil
		}
		for _, companion := range record.Companions {
			if strings.TrimSpace(companion.ContainerName) == "" {
				continue
			}
			_ = s.requireDocker().StopContainer(ctx, companion.ContainerName)
			_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
		}
		restored := previous
		for i := range restored.Companions {
			if err := s.createCompanionAt(ctx, &restored, i); err != nil {
				return err
			}
			if !previousWasRunning {
				_ = s.requireDocker().StopContainer(ctx, restored.Companions[i].ContainerName)
				restored.Companions[i].Status = AppStatusStopped
			}
		}
		previous.Companions = restored.Companions
		return nil
	}
	restorePrevious := func(runErr error) error {
		previous.LastOperationID = op.ID
		previous.LastOperationType = op.Type
		previous.LastOperationState = OperationFailed
		_ = s.saveInstalled(ctx, previous)
		return runErr
	}
	restorePreviousWithCompanions := func(runErr error) error {
		if companionsTouched {
			if companionErr := restorePreviousCompanions(); companionErr != nil {
				previous.Status = AppStatusError
				previous.Error = fmt.Sprintf("%v; companion rollback failed: %v", runErr, companionErr)
			}
		}
		return restorePrevious(runErr)
	}
	rollbackPrevious := func(runErr error) error {
		if companionsTouched {
			if companionErr := restorePreviousCompanions(); companionErr != nil {
				previous.Status = AppStatusError
				previous.Error = fmt.Sprintf("%v; companion rollback failed: %v", runErr, companionErr)
				return restorePrevious(runErr)
			}
		}
		rollbackID, rollbackErr := s.requireDocker().CreateContainer(ctx, containerSpecFromRecord(previous))
		if rollbackErr != nil {
			previous.Status = AppStatusError
			previous.Error = fmt.Sprintf("%v; rollback failed: %v", runErr, rollbackErr)
			return restorePrevious(runErr)
		}
		previous.ContainerID = rollbackID
		if previousWasRunning {
			if rollbackErr := s.requireDocker().StartContainer(ctx, previous.ContainerName); rollbackErr != nil {
				previous.Status = AppStatusError
				previous.Error = fmt.Sprintf("%v; rollback start failed: %v", runErr, rollbackErr)
				return restorePrevious(runErr)
			}
		}
		previous.Error = ""
		return restorePrevious(runErr)
	}
	if err := s.requireDocker().PullImage(ctx, entry.Image); err != nil {
		return restorePrevious(err)
	}
	if networkName := privateStoreNetworkName(entry); networkName != "" {
		if err := s.requireDocker().CreateNetwork(ctx, networkName); err != nil {
			return restorePrevious(err)
		}
	}
	companionsTouched = len(autoCompanions) > 0
	if err := s.recreateAutoCompanions(ctx, &record, autoCompanions); err != nil {
		return restorePreviousWithCompanions(err)
	}
	_ = s.requireDocker().StopContainer(ctx, record.ContainerName)
	if err := s.requireDocker().RemoveContainer(ctx, record.ContainerName, true); err != nil {
		return restorePreviousWithCompanions(fmt.Errorf("remove old container: %w", err))
	}
	containerID, err := s.requireDocker().CreateContainer(ctx, containerSpecFromRecord(record))
	if err != nil {
		return rollbackPrevious(fmt.Errorf("create updated container: %w", err))
	}
	record.ContainerID = containerID
	if err := s.seedContainerFiles(ctx, entry, record); err != nil {
		_ = s.requireDocker().RemoveContainer(ctx, record.ContainerName, true)
		return rollbackPrevious(fmt.Errorf("seed updated container files: %w", err))
	}
	if previousWasRunning {
		if err := s.requireDocker().StartContainer(ctx, record.ContainerName); err != nil {
			_ = s.requireDocker().RemoveContainer(ctx, record.ContainerName, true)
			return rollbackPrevious(fmt.Errorf("start updated container: %w", err))
		}
		if err := s.waitContainerReady(ctx, record, appReadinessTimeout); err != nil {
			_ = s.requireDocker().RemoveContainer(ctx, record.ContainerName, true)
			return rollbackPrevious(fmt.Errorf("updated container readiness: %w", err))
		}
	}
	record.Status = previous.Status
	record.Error = ""
	record.LastOperationState = OperationSucceeded
	return s.saveInstalled(ctx, record)
}

func (s *Service) start(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusRunning, func(app InstalledApp) error {
		for _, companion := range app.Companions {
			if err := s.requireDocker().StartContainer(ctx, companion.ContainerName); err != nil {
				return fmt.Errorf("start companion container %s: %w", companion.ContainerName, err)
			}
		}
		if err := s.requireDocker().StartContainer(ctx, app.ContainerName); err != nil {
			return err
		}
		return s.waitContainerReady(ctx, app, appReadinessTimeout)
	})
}

func (s *Service) stop(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusStopped, func(app InstalledApp) error {
		if err := s.requireDocker().StopContainer(ctx, app.ContainerName); err != nil {
			return err
		}
		for _, companion := range app.Companions {
			if err := s.requireDocker().StopContainer(ctx, companion.ContainerName); err != nil {
				return fmt.Errorf("stop companion container %s: %w", companion.ContainerName, err)
			}
		}
		return nil
	})
}

func (s *Service) restart(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusRunning, func(app InstalledApp) error {
		for _, companion := range app.Companions {
			if err := s.requireDocker().RestartContainer(ctx, companion.ContainerName); err != nil {
				return fmt.Errorf("restart companion container %s: %w", companion.ContainerName, err)
			}
		}
		if err := s.requireDocker().RestartContainer(ctx, app.ContainerName); err != nil {
			return err
		}
		return s.waitContainerReady(ctx, app, appReadinessTimeout)
	})
}

func (s *Service) action(ctx context.Context, op Operation, targetStatus string, fn func(InstalledApp) error) error {
	app, ok, err := s.GetInstalled(ctx, op.AppID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("store app %q is not installed", op.AppID)
	}
	if err := fn(app); err != nil {
		app.Status = AppStatusError
		app.Error = err.Error()
		app.LastOperationID = op.ID
		app.LastOperationType = op.Type
		app.LastOperationState = OperationFailed
		_ = s.saveInstalled(ctx, app)
		return err
	}
	app.Status = targetStatus
	app.Error = ""
	app.LastOperationID = op.ID
	app.LastOperationType = op.Type
	app.LastOperationState = OperationSucceeded
	return s.saveInstalled(ctx, app)
}

func (s *Service) uninstall(ctx context.Context, op Operation, deleteData bool) error {
	app, ok, err := s.GetInstalled(ctx, op.AppID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("store app %q is not installed", op.AppID)
	}
	for _, companion := range app.Companions {
		_ = s.requireDocker().StopContainer(ctx, companion.ContainerName)
		if err := s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true); err != nil {
			return fmt.Errorf("remove companion container %s: %w", companion.ContainerName, err)
		}
	}
	_ = s.requireDocker().StopContainer(ctx, app.ContainerName)
	if err := s.requireDocker().RemoveContainer(ctx, app.ContainerName, true); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if err := s.deleteStoreArtifacts(ctx, app); err != nil {
		return err
	}
	if err := s.removePrivateStoreNetworks(ctx, app); err != nil {
		return err
	}
	if deleteData {
		removed := map[string]struct{}{}
		for _, volume := range app.Volumes {
			removed[volume.Name] = struct{}{}
			if err := s.requireDocker().RemoveVolume(ctx, volume.Name, true); err != nil {
				return fmt.Errorf("remove volume %s: %w", volume.Name, err)
			}
		}
		for _, companion := range app.Companions {
			for _, volume := range companion.Volumes {
				if _, ok := removed[volume.Name]; ok {
					continue
				}
				removed[volume.Name] = struct{}{}
				if err := s.requireDocker().RemoveVolume(ctx, volume.Name, true); err != nil {
					return fmt.Errorf("remove volume %s: %w", volume.Name, err)
				}
			}
		}
		if err := s.deleteStoreSecrets(ctx, app); err != nil {
			return err
		}
		if err := s.removeManagedWorkspaceBinds(app); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM desktop_store_apps WHERE app_id = ?`, app.AppID)
	if err != nil {
		return fmt.Errorf("delete desktop store app record: %w", err)
	}
	return nil
}

func (s *Service) canReplaceInstallingRecord(ctx context.Context, app InstalledApp) (bool, error) {
	if app.Status != AppStatusInstalling || app.LastOperationType != OperationInstall {
		return false, nil
	}
	if strings.TrimSpace(app.LastOperationID) == "" {
		return true, nil
	}
	op, err := s.Operation(ctx, app.LastOperationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return false, fmt.Errorf("load previous install operation for %s: %w", app.AppID, err)
	}
	switch op.Status {
	case OperationFailed, OperationSucceeded:
		return true, nil
	case OperationPending, OperationRunning:
		if time.Since(op.UpdatedAt) > operationStaleAfter {
			_ = s.updateOperation(ctx, op.ID, OperationFailed, "stale operation reset", "operation timed out before completion")
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) buildInstallRecord(entry CatalogEntry, op Operation, bindMode, hostIP string, hostPort int, tailscaleEnabled bool) InstalledApp {
	now := time.Now().UTC()
	tailscaleStatus := TailscaleStatusDisabled
	tailscalePort := 0
	if tailscaleEnabled {
		tailscaleStatus = TailscaleStatusPending
		tailscalePort = hostPort
	}
	return InstalledApp{
		AppID:              entry.ID,
		DesktopAppID:       DesktopAppID(entry.ID),
		LaunchpadLinkID:    ManagedLaunchpadLinkID(entry.ID),
		ContainerName:      ContainerName(entry.ID),
		Image:              entry.Image,
		Status:             AppStatusInstalling,
		BindMode:           bindMode,
		HostIP:             hostIP,
		HostPort:           hostPort,
		ContainerPort:      entry.PrimaryPort.ContainerPort,
		Protocol:           strings.ToLower(entry.PrimaryPort.Protocol),
		TailscaleEnabled:   tailscaleEnabled,
		TailscaleStatus:    tailscaleStatus,
		TailscalePort:      tailscalePort,
		LogoPath:           entry.LogoURL,
		Ports:              []PortBinding{{ID: entry.PrimaryPort.ID, Name: entry.PrimaryPort.Name, ContainerPort: entry.PrimaryPort.ContainerPort, Protocol: strings.ToLower(entry.PrimaryPort.Protocol), HostIP: hostIP, HostPort: hostPort}},
		Volumes:            resolveVolumes(entry),
		HostBinds:          resolveHostBinds(entry.HostBinds),
		Env:                append([]string(nil), entry.Env...),
		ExtraHosts:         append([]string(nil), entry.ExtraHosts...),
		CreatedAt:          now,
		UpdatedAt:          now,
		LastOperationID:    op.ID,
		LastOperationType:  op.Type,
		LastOperationState: OperationRunning,
	}
}

func (s *Service) allocatePortBindings(ctx context.Context, entry CatalogEntry, hostIP string, primaryHostPort int) ([]PortBinding, error) {
	ports := catalogPorts(entry)
	out := make([]PortBinding, 0, len(ports))
	for i, port := range ports {
		hostPort := primaryHostPort
		if i > 0 {
			allocated, err := s.portAllocator(ctx, port.ContainerPort)
			if err != nil {
				return nil, fmt.Errorf("allocate port %s: %w", port.ID, err)
			}
			hostPort = allocated
		}
		out = append(out, PortBinding{
			ID:            port.ID,
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      strings.ToLower(port.Protocol),
			HostIP:        hostIP,
			HostPort:      hostPort,
		})
	}
	return out, nil
}

func (s *Service) updatePortBindings(ctx context.Context, entry CatalogEntry, app InstalledApp) ([]PortBinding, error) {
	previous := map[string]PortBinding{}
	for _, port := range app.Ports {
		if port.ID != "" {
			previous[port.ID] = port
		}
	}
	if len(previous) == 0 && app.HostPort > 0 {
		previous[entry.PrimaryPort.ID] = PortBinding{ID: entry.PrimaryPort.ID, Name: entry.PrimaryPort.Name, ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}
	}
	ports := catalogPorts(entry)
	out := make([]PortBinding, 0, len(ports))
	for _, port := range ports {
		binding, ok := previous[port.ID]
		if !ok {
			allocated, err := s.portAllocator(ctx, port.ContainerPort)
			if err != nil {
				return nil, fmt.Errorf("allocate port %s: %w", port.ID, err)
			}
			binding = PortBinding{HostIP: app.HostIP, HostPort: allocated}
		}
		binding.ID = port.ID
		binding.Name = port.Name
		binding.ContainerPort = port.ContainerPort
		binding.Protocol = strings.ToLower(port.Protocol)
		if strings.TrimSpace(binding.HostIP) == "" {
			binding.HostIP = app.HostIP
		}
		out = append(out, binding)
	}
	return out, nil
}

func catalogPorts(entry CatalogEntry) []PortSpec {
	primary := entry.PrimaryPort
	if primary.ID == "" {
		primary.ID = "main"
	}
	if primary.Name == "" {
		primary.Name = "Web UI"
	}
	if primary.Protocol == "" {
		primary.Protocol = "tcp"
	}
	ports := []PortSpec{primary}
	for _, extra := range entry.ExtraPorts {
		if extra.Protocol == "" {
			extra.Protocol = "tcp"
		}
		ports = append(ports, extra)
	}
	return ports
}

func resolveHostBinds(templates []HostBindTemplate) []HostBinding {
	binds := make([]HostBinding, 0, len(templates))
	for _, template := range templates {
		hostPath := strings.TrimSpace(template.HostPath)
		containerPath := strings.TrimSpace(template.ContainerPath)
		if hostPath == "" || containerPath == "" {
			continue
		}
		binds = append(binds, HostBinding{
			HostPath:      hostPath,
			ContainerPath: containerPath,
			ReadOnly:      template.ReadOnly,
		})
	}
	return binds
}

func (s *Service) resolveCatalogHostBinds(entry CatalogEntry) ([]HostBinding, error) {
	binds := resolveHostBinds(entry.HostBinds)
	for _, template := range entry.WorkspaceBinds {
		hostPath, workspacePath, err := resolveWorkspaceBindPath(s.cfg.WorkspaceDir, template.WorkspacePath)
		if err != nil {
			return nil, fmt.Errorf("resolve workspace bind %s: %w", template.WorkspacePath, err)
		}
		containerPath := strings.TrimSpace(template.ContainerPath)
		if containerPath == "" {
			continue
		}
		binds = append(binds, HostBinding{
			HostPath:      hostPath,
			ContainerPath: containerPath,
			ReadOnly:      template.ReadOnly,
			Managed:       true,
			WorkspacePath: workspacePath,
		})
	}
	return binds, nil
}

func resolveWorkspaceBindPath(workspaceDir, workspacePath string) (string, string, error) {
	root := strings.TrimSpace(workspaceDir)
	if root == "" {
		return "", "", fmt.Errorf("desktop workspace directory is required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve desktop workspace root: %w", err)
	}
	raw := strings.ReplaceAll(strings.TrimSpace(workspacePath), "\\", "/")
	if raw == "" || raw == "/" || raw == "." {
		return "", "", fmt.Errorf("workspace bind path is required")
	}
	if pathpkg.IsAbs(raw) || filepath.IsAbs(filepath.FromSlash(raw)) {
		return "", "", fmt.Errorf("workspace bind path must be relative")
	}
	cleanSlash := pathpkg.Clean(raw)
	if cleanSlash == "." || cleanSlash == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", "", fmt.Errorf("workspace bind path escapes workspace")
	}
	hostPath := filepath.Join(rootAbs, filepath.FromSlash(cleanSlash))
	hostAbs, err := filepath.Abs(hostPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace bind path: %w", err)
	}
	if !isStorePathWithin(rootAbs, hostAbs) {
		return "", "", fmt.Errorf("workspace bind path escapes workspace")
	}
	if err := validateStorePathComponentsWithinRoot(rootAbs, hostAbs); err != nil {
		return "", "", err
	}
	return filepath.Clean(hostAbs), cleanSlash, nil
}

func isStorePathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func validateStorePathComponentsWithinRoot(rootAbs, candidateAbs string) error {
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return fmt.Errorf("resolve workspace bind relative path: %w", err)
	}
	if rel == "." {
		return nil
	}
	current := rootAbs
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("inspect workspace bind path: %w", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(current)
		if err != nil {
			return fmt.Errorf("read workspace bind symlink: %w", err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		targetAbs, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("resolve workspace bind symlink: %w", err)
		}
		if !isStorePathWithin(rootAbs, targetAbs) {
			return fmt.Errorf("workspace bind follows symlink outside workspace")
		}
		evaluated, err := filepath.EvalSymlinks(targetAbs)
		if err != nil {
			return fmt.Errorf("workspace bind follows invalid symlink: %w", err)
		}
		if !isStorePathWithin(rootAbs, evaluated) {
			return fmt.Errorf("workspace bind follows symlink outside workspace")
		}
		current = evaluated
	}
	return nil
}

func (s *Service) prepareManagedWorkspaceBinds(app InstalledApp) error {
	for _, bind := range app.HostBinds {
		if !bind.Managed {
			continue
		}
		if err := os.MkdirAll(bind.HostPath, 0o755); err != nil {
			return fmt.Errorf("create workspace bind %s: %w", bind.WorkspacePath, err)
		}
	}
	return nil
}

func (s *Service) seedContainerFiles(ctx context.Context, entry CatalogEntry, app InstalledApp) error {
	if len(entry.SeedFiles) == 0 {
		return nil
	}
	grouped := map[string]map[string]string{}
	for _, seed := range entry.SeedFiles {
		cleanPath := pathpkg.Clean("/" + strings.TrimLeft(strings.TrimSpace(seed.Path), "/"))
		if hostPath, ok, err := managedWorkspaceSeedPath(app.HostBinds, cleanPath); err != nil {
			return err
		} else if ok {
			if err := writeSeedFileIfMissing(hostPath, seed.Content); err != nil {
				return fmt.Errorf("seed workspace file %s: %w", seed.Path, err)
			}
			continue
		}
		dir, name := pathpkg.Split(cleanPath)
		dir = strings.TrimRight(dir, "/")
		if dir == "" {
			dir = "/"
		}
		if name == "" || name == "." || strings.Contains(name, "/") {
			return fmt.Errorf("invalid seed file path %q", seed.Path)
		}
		if grouped[dir] == nil {
			grouped[dir] = map[string]string{}
		}
		grouped[dir][name] = seed.Content
	}
	for destDir, files := range grouped {
		if err := s.requireDocker().CopyToContainer(ctx, app.ContainerName, destDir, files); err != nil {
			return fmt.Errorf("seed container files in %s: %w", destDir, err)
		}
	}
	return nil
}

func managedWorkspaceSeedPath(binds []HostBinding, seedPath string) (string, bool, error) {
	for _, bind := range binds {
		if !bind.Managed {
			continue
		}
		containerPath := pathpkg.Clean("/" + strings.TrimLeft(strings.TrimSpace(bind.ContainerPath), "/"))
		if containerPath == "." || containerPath == "" {
			containerPath = "/"
		}
		rel := ""
		switch {
		case seedPath == containerPath:
			return "", false, fmt.Errorf("seed path %q targets a workspace bind directory", seedPath)
		case containerPath == "/":
			rel = strings.TrimLeft(seedPath, "/")
		case strings.HasPrefix(seedPath, containerPath+"/"):
			rel = strings.TrimPrefix(seedPath, containerPath+"/")
		default:
			continue
		}
		rel = pathpkg.Clean(rel)
		if rel == "." || rel == "" || strings.HasPrefix(rel, "../") || strings.Contains(rel, "/../") {
			return "", false, fmt.Errorf("invalid workspace seed path %q", seedPath)
		}
		hostPath := filepath.Join(bind.HostPath, filepath.FromSlash(rel))
		return hostPath, true, nil
	}
	return "", false, nil
}

func writeSeedFileIfMissing(hostPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(hostPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(hostPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(hostPath, []byte(content), 0o644)
}

func (s *Service) installEnv(entry CatalogEntry, app InstalledApp) ([]string, []SecretRef, error) {
	return s.resolveEnv(entry, app, nil, nil)
}

func (s *Service) updateEnv(entry CatalogEntry, app InstalledApp, previous InstalledApp) ([]string, []SecretRef, error) {
	return s.resolveEnv(entry, app, previous.Env, previous.SecretRefs)
}

func (s *Service) resolveEnv(entry CatalogEntry, app InstalledApp, previousEnv []string, previousRefs []SecretRef) ([]string, []SecretRef, error) {
	env := applyEnvTemplates(entry.Env, app, nil)
	refs := make([]SecretRef, 0, len(entry.GeneratedSecrets))
	for _, secret := range entry.GeneratedSecrets {
		secret.Key = strings.ToLower(strings.TrimSpace(secret.Key))
		if secret.Key == "" {
			continue
		}
		vaultKey := storeSecretVaultKey(entry.ID, secret.Key)
		value := ""
		if s.cfg.Secrets != nil {
			if stored, err := s.cfg.Secrets.ReadSecret(vaultKey); err == nil {
				value = stored
			}
		}
		if value == "" && secret.Env != "" {
			value, _ = envValue(previousEnv, secret.Env)
		}
		if value == "" {
			value = randomHex(32)
		}
		if s.cfg.Secrets != nil {
			if err := s.cfg.Secrets.WriteSecret(vaultKey, value); err != nil {
				return nil, nil, fmt.Errorf("write generated secret %s: %w", secret.Key, err)
			}
		}
		if secret.Env != "" {
			env = appendEnvValue(env, secret.Env, value)
		}
		refs = append(refs, SecretRef{
			Key:      secret.Key,
			VaultKey: vaultKey,
			Env:      secret.Env,
			Label:    secret.Label,
			Expose:   secret.Expose,
		})
	}
	for _, ref := range previousRefs {
		if ref.Key == "" || ref.VaultKey == "" {
			continue
		}
		found := false
		for _, next := range refs {
			if next.Key == ref.Key {
				found = true
				break
			}
		}
		if !found && ref.Expose {
			refs = append(refs, ref)
		}
	}
	return env, refs, nil
}

func (s *Service) prepareAutoCompanions(entry CatalogEntry, app InstalledApp) ([]CompanionApp, error) {
	if len(entry.Companions) == 0 {
		return nil, nil
	}
	secretValues := s.secretTemplateValues(app.SecretRefs, app.Env)
	companions := make([]CompanionApp, 0, len(entry.Companions))
	for _, template := range entry.Companions {
		env := applyEnvTemplates(template.Env, app, secretValues)
		if hasUnresolvedSecretTemplate(env) {
			if isPrivateStoreNetwork(template.NetworkMode) {
				return nil, fmt.Errorf("companion %s has unresolved generated secrets", template.ID)
			}
			continue
		}
		companions = append(companions, CompanionApp{
			ID:            template.ID,
			Name:          template.Name,
			ContainerName: CompanionContainerName(app.AppID, template.ID),
			Image:         template.Image,
			Status:        AppStatusInstalling,
			NetworkMode:   template.NetworkMode,
			Volumes:       resolveCompanionVolumes(entry, template),
			HostBinds:     resolveHostBinds(template.HostBinds),
			Env:           env,
		})
	}
	return companions, nil
}

func (s *Service) secretTemplateValues(refs []SecretRef, env []string) map[string]string {
	values := map[string]string{}
	for _, ref := range refs {
		value := ""
		if ref.VaultKey != "" && s.cfg.Secrets != nil {
			if stored, err := s.cfg.Secrets.ReadSecret(ref.VaultKey); err == nil {
				value = stored
			}
		}
		if value == "" && ref.Env != "" {
			value, _ = envValue(env, ref.Env)
		}
		if value != "" && ref.VaultKey != "" {
			values[ref.VaultKey] = value
		}
	}
	return values
}

func hasUnresolvedSecretTemplate(env []string) bool {
	for _, item := range env {
		if strings.Contains(item, "${SECRET:") {
			return true
		}
	}
	return false
}

func (s *Service) createAutoCompanions(ctx context.Context, app *InstalledApp) error {
	for i := range app.Companions {
		if err := s.createCompanionAt(ctx, app, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) recreateAutoCompanions(ctx context.Context, app *InstalledApp, companions []CompanionApp) error {
	for _, companion := range companions {
		index := companionIndex(app.Companions, companion.ID)
		if index < 0 {
			app.Companions = append(app.Companions, companion)
			index = len(app.Companions) - 1
		} else {
			app.Companions[index] = companion
		}
		_ = s.requireDocker().StopContainer(ctx, companion.ContainerName)
		_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
		if err := s.createCompanionAt(ctx, app, index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) createCompanionAt(ctx context.Context, app *InstalledApp, index int) error {
	companion := &app.Companions[index]
	if err := s.requireDocker().PullImage(ctx, companion.Image); err != nil {
		return fmt.Errorf("pull companion image %s: %w", companion.Image, err)
	}
	containerID, err := s.requireDocker().CreateContainer(ctx, companionContainerSpec(*app, *companion))
	if err != nil {
		return fmt.Errorf("create companion container %s: %w", companion.ContainerName, err)
	}
	companion.ContainerID = containerID
	if err := s.requireDocker().StartContainer(ctx, companion.ContainerName); err != nil {
		_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
		return fmt.Errorf("start companion container %s: %w", companion.ContainerName, err)
	}
	companion.Status = AppStatusRunning
	companion.Error = ""
	return nil
}

func applyEnvTemplates(env []string, app InstalledApp, secrets map[string]string) []string {
	out := make([]string, 0, len(env))
	replacements := map[string]string{
		"${HOST_PORT}":         strconv.Itoa(app.HostPort),
		"${PRIMARY_HOST_PORT}": strconv.Itoa(app.HostPort),
		"${APP_URL}":           "http://localhost:" + strconv.Itoa(app.HostPort),
	}
	for _, port := range app.Ports {
		if port.ID == "" {
			continue
		}
		key := strings.ToUpper(strings.ReplaceAll(port.ID, "-", "_"))
		replacements["${PORT_"+key+"_HOST_PORT}"] = strconv.Itoa(port.HostPort)
	}
	for key, value := range secrets {
		replacements["${SECRET:"+key+"}"] = value
	}
	for _, item := range env {
		next := item
		for old, value := range replacements {
			next = strings.ReplaceAll(next, old, value)
		}
		out = append(out, next)
	}
	return out
}

func storeSecretVaultKey(appID, key string) string {
	return "desktop_store_" + normalizeAppID(appID) + "_" + strings.ToLower(strings.TrimSpace(key))
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix), true
		}
	}
	return "", false
}

func appendEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func (s *Service) installDesktopApp(ctx context.Context, entry CatalogEntry, app InstalledApp) error {
	if s.cfg.Desktop == nil {
		return nil
	}
	manifest := desktop.AppManifest{
		ID:          app.DesktopAppID,
		Name:        entry.Name,
		Version:     "1.0.0",
		Icon:        entry.Icon,
		Entry:       "index.html",
		Runtime:     RuntimeContainerWebApp,
		Description: entry.Description,
		Metadata: map[string]string{
			"store_app_id":   entry.ID,
			"logo_path":      app.LogoPath,
			"container_name": app.ContainerName,
		},
	}
	for key, value := range entry.Metadata {
		if strings.TrimSpace(key) == "" {
			continue
		}
		manifest.Metadata[key] = value
	}
	files := map[string]string{
		"index.html": `<!doctype html><meta charset="utf-8"><title>` + entry.Name + `</title><p>` + entry.Name + ` is managed by AuraGo Software Store.</p>`,
	}
	if err := s.cfg.Desktop.InstallApp(ctx, manifest, files, storeSource); err != nil {
		return fmt.Errorf("install desktop app: %w", err)
	}
	dockVisible := true
	startVisible := true
	if err := s.cfg.Desktop.SetAppVisibility(ctx, app.DesktopAppID, &dockVisible, &startVisible, storeSource); err != nil {
		return fmt.Errorf("set desktop app visibility: %w", err)
	}
	if err := s.cfg.Desktop.AddDesktopAppShortcut(ctx, app.DesktopAppID, storeSource); err != nil {
		return fmt.Errorf("add desktop shortcut: %w", err)
	}
	if entry.ID == "olivetin" {
		if err := s.cfg.Desktop.WriteFile(ctx, "Desktop/olivetin.txt", oliveTinDesktopNote, storeSource); err != nil {
			return fmt.Errorf("write OliveTin desktop note: %w", err)
		}
	}
	return nil
}

func (s *Service) upsertLaunchpad(ctx context.Context, entry CatalogEntry, app InstalledApp) (string, error) {
	if s.cfg.Launchpad == nil {
		return "", nil
	}
	return s.cfg.Launchpad.UpsertStoreLink(ctx, LaunchpadLink{
		ID:          ManagedLaunchpadLinkID(entry.ID),
		Title:       entry.Name,
		URL:         "aurago-store://" + entry.ID,
		Description: entry.Description,
		IconPath:    app.LogoPath,
		Category:    "Software Store",
		Tags:        []string{"aurago-store", "container", entry.ID},
		SortOrder:   500,
	})
}

func (s *Service) saveInstalled(ctx context.Context, app InstalledApp) error {
	app.UpdatedAt = time.Now().UTC()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = app.UpdatedAt
	}
	if len(app.Ports) == 0 && app.HostPort > 0 {
		app.Ports = []PortBinding{{ID: "main", Name: "Web UI", ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}}
	}
	portsJSON, _ := json.Marshal(app.Ports)
	volumesJSON, _ := json.Marshal(app.Volumes)
	hostBindsJSON, _ := json.Marshal(app.HostBinds)
	envJSON, _ := json.Marshal(app.Env)
	extraHostsJSON, _ := json.Marshal(app.ExtraHosts)
	secretRefsJSON, _ := json.Marshal(secretRefsForStorage(app.SecretRefs))
	companionsJSON, _ := json.Marshal(app.Companions)
	_, err := s.db.ExecContext(ctx, `INSERT INTO desktop_store_apps(app_id, desktop_app_id, launchpad_link_id,
		container_name, container_id, image, status, error, bind_mode, host_ip, host_port, container_port, protocol,
		tailscale_enabled, tailscale_status, tailscale_port, logo_path, ports_json, volumes_json, host_binds_json,
		env_json, extra_hosts_json, secret_refs_json, companions_json, created_at, updated_at,
		last_operation_id, last_operation_type, last_operation_state)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(app_id) DO UPDATE SET
			desktop_app_id = excluded.desktop_app_id,
			launchpad_link_id = excluded.launchpad_link_id,
			container_name = excluded.container_name,
			container_id = excluded.container_id,
			image = excluded.image,
			status = excluded.status,
			error = excluded.error,
			bind_mode = excluded.bind_mode,
			host_ip = excluded.host_ip,
			host_port = excluded.host_port,
			container_port = excluded.container_port,
			protocol = excluded.protocol,
			tailscale_enabled = excluded.tailscale_enabled,
			tailscale_status = excluded.tailscale_status,
			tailscale_port = excluded.tailscale_port,
			logo_path = excluded.logo_path,
			ports_json = excluded.ports_json,
			volumes_json = excluded.volumes_json,
			host_binds_json = excluded.host_binds_json,
			env_json = excluded.env_json,
			extra_hosts_json = excluded.extra_hosts_json,
			secret_refs_json = excluded.secret_refs_json,
			companions_json = excluded.companions_json,
			updated_at = excluded.updated_at,
			last_operation_id = excluded.last_operation_id,
			last_operation_type = excluded.last_operation_type,
			last_operation_state = excluded.last_operation_state`,
		app.AppID, app.DesktopAppID, app.LaunchpadLinkID, app.ContainerName, app.ContainerID, app.Image, app.Status, app.Error,
		app.BindMode, app.HostIP, app.HostPort, app.ContainerPort, app.Protocol, boolToInt(app.TailscaleEnabled), app.TailscaleStatus,
		app.TailscalePort, app.LogoPath, string(portsJSON), string(volumesJSON), string(hostBindsJSON), string(envJSON),
		string(extraHostsJSON), string(secretRefsJSON), string(companionsJSON), formatTime(app.CreatedAt), formatTime(app.UpdatedAt),
		app.LastOperationID, app.LastOperationType, app.LastOperationState)
	if err != nil {
		return fmt.Errorf("save desktop store app: %w", err)
	}
	return nil
}

func (s *Service) failInstall(ctx context.Context, app InstalledApp, runErr error) error {
	_ = s.cleanupInstallArtifacts(ctx, app)
	return runErr
}

func (s *Service) cleanupInstallArtifacts(ctx context.Context, app InstalledApp) error {
	for _, companion := range app.Companions {
		_ = s.requireDocker().StopContainer(ctx, companion.ContainerName)
		_ = s.requireDocker().RemoveContainer(ctx, companion.ContainerName, true)
	}
	_ = s.requireDocker().StopContainer(ctx, app.ContainerName)
	_ = s.requireDocker().RemoveContainer(ctx, app.ContainerName, true)
	for _, volume := range app.Volumes {
		_ = s.requireDocker().RemoveVolume(ctx, volume.Name, true)
	}
	for _, companion := range app.Companions {
		for _, volume := range companion.Volumes {
			_ = s.requireDocker().RemoveVolume(ctx, volume.Name, true)
		}
	}
	_ = s.removePrivateStoreNetworks(ctx, app)
	_ = s.deleteStoreSecrets(ctx, app)
	_ = s.removeManagedWorkspaceBinds(app)
	if err := s.deleteStoreArtifacts(ctx, app); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM desktop_store_apps WHERE app_id = ?`, app.AppID)
	if err != nil {
		return fmt.Errorf("delete desktop store app record: %w", err)
	}
	return nil
}

func (s *Service) deleteStoreArtifacts(ctx context.Context, app InstalledApp) error {
	if s.cfg.Desktop != nil {
		if err := s.cfg.Desktop.DeleteApp(ctx, app.DesktopAppID, storeSource); err != nil && !isMissingStoreArtifact(err) {
			return fmt.Errorf("delete desktop app: %w", err)
		}
	}
	if s.cfg.Launchpad != nil && app.LaunchpadLinkID != "" {
		if err := s.cfg.Launchpad.DeleteStoreLink(ctx, app.LaunchpadLinkID); err != nil && !isMissingStoreArtifact(err) {
			return fmt.Errorf("delete launchpad link: %w", err)
		}
	}
	return nil
}

func (s *Service) removeManagedWorkspaceBinds(app InstalledApp) error {
	for _, bind := range app.HostBinds {
		if !bind.Managed {
			continue
		}
		hostPath, _, err := resolveWorkspaceBindPath(s.cfg.WorkspaceDir, bind.WorkspacePath)
		if err != nil {
			return fmt.Errorf("resolve managed workspace bind %s: %w", bind.WorkspacePath, err)
		}
		if filepath.Clean(hostPath) != filepath.Clean(bind.HostPath) {
			return fmt.Errorf("managed workspace bind path changed for %s", bind.WorkspacePath)
		}
		if err := os.RemoveAll(hostPath); err != nil {
			return fmt.Errorf("remove workspace bind %s: %w", bind.WorkspacePath, err)
		}
	}
	return nil
}

func (s *Service) deleteStoreSecrets(ctx context.Context, app InstalledApp) error {
	if s.cfg.Secrets == nil {
		return nil
	}
	for _, ref := range app.SecretRefs {
		if strings.TrimSpace(ref.VaultKey) == "" {
			continue
		}
		if err := s.cfg.Secrets.DeleteSecret(ref.VaultKey); err != nil {
			return fmt.Errorf("delete store secret %s: %w", ref.Key, err)
		}
	}
	if app.AppID == "beszel" {
		for _, key := range []string{"desktop_store_beszel_agent_key", "desktop_store_beszel_agent_token"} {
			if err := s.cfg.Secrets.DeleteSecret(key); err != nil {
				return fmt.Errorf("delete store secret %s: %w", key, err)
			}
		}
	}
	_ = ctx
	return nil
}

func (s *Service) removePrivateStoreNetworks(ctx context.Context, app InstalledApp) error {
	removed := map[string]struct{}{}
	for _, companion := range app.Companions {
		networkMode := strings.TrimSpace(companion.NetworkMode)
		if !isPrivateStoreNetwork(networkMode) {
			continue
		}
		if _, ok := removed[networkMode]; ok {
			continue
		}
		removed[networkMode] = struct{}{}
		if err := s.requireDocker().RemoveNetwork(ctx, networkMode); err != nil {
			return fmt.Errorf("remove network %s: %w", networkMode, err)
		}
	}
	return nil
}

func isMissingStoreArtifact(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "not found") || strings.Contains(errText, "no rows")
}

func (s *Service) waitContainerReady(ctx context.Context, app InstalledApp, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = appReadinessTimeout
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		state, err := s.requireDocker().InspectContainer(ctx, app.ContainerName)
		if err != nil {
			lastErr = err
		} else if !state.Running {
			if state.Status != "" {
				lastErr = fmt.Errorf("container status is %s", state.Status)
			} else {
				lastErr = fmt.Errorf("container is not running")
			}
		} else if health := strings.ToLower(strings.TrimSpace(state.Health)); health == "starting" || health == "unhealthy" {
			lastErr = fmt.Errorf("container health is %s", health)
		} else if state.Health != "" && health != "healthy" {
			lastErr = fmt.Errorf("container health is %s", state.Health)
		} else if app.Protocol != "tcp" || s.portProbe(ctx, app.HostIP, app.HostPort) {
			return nil
		} else {
			lastErr = fmt.Errorf("container web port %d is not reachable yet", app.HostPort)
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("wait for container readiness: %w", lastErr)
			}
			return fmt.Errorf("wait for container readiness timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(appReadinessPollInterval):
		}
	}
}

func storePortAccepts(ctx context.Context, hostIP string, hostPort int) bool {
	host := strings.TrimSpace(hostIP)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	dialCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", net.JoinHostPort(host, fmt.Sprintf("%d", hostPort)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (s *Service) requireDocker() DockerAdapter {
	if s.cfg.Docker == nil {
		return missingDockerAdapter{}
	}
	return s.cfg.Docker
}

func (s *Service) ensureReady(ctx context.Context) error {
	s.mu.Lock()
	ready := s.db != nil && !s.closed
	s.mu.Unlock()
	if ready {
		return nil
	}
	return s.Init(ctx)
}

func scanInstalledApp(scanner interface{ Scan(dest ...any) error }) (InstalledApp, error) {
	var app InstalledApp
	var tailscaleEnabled int
	var createdAt, updatedAt string
	var portsJSON, volumesJSON, hostBindsJSON, envJSON, extraHostsJSON, secretRefsJSON, companionsJSON string
	err := scanner.Scan(&app.AppID, &app.DesktopAppID, &app.LaunchpadLinkID, &app.ContainerName, &app.ContainerID,
		&app.Image, &app.Status, &app.Error, &app.BindMode, &app.HostIP, &app.HostPort, &app.ContainerPort, &app.Protocol,
		&tailscaleEnabled, &app.TailscaleStatus, &app.TailscalePort, &app.LogoPath, &portsJSON, &volumesJSON, &hostBindsJSON,
		&envJSON, &extraHostsJSON, &secretRefsJSON, &companionsJSON, &createdAt, &updatedAt, &app.LastOperationID,
		&app.LastOperationType, &app.LastOperationState)
	if err != nil {
		return InstalledApp{}, err
	}
	app.TailscaleEnabled = tailscaleEnabled != 0
	app.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	app.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if err := json.Unmarshal([]byte(portsJSON), &app.Ports); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store ports for %s: %w", app.AppID, err)
	}
	if len(app.Ports) == 0 && app.HostPort > 0 {
		app.Ports = []PortBinding{{ID: "main", Name: "Web UI", ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}}
	}
	if err := json.Unmarshal([]byte(volumesJSON), &app.Volumes); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store volumes for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(hostBindsJSON), &app.HostBinds); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store host binds for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(envJSON), &app.Env); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store env for %s: %w", app.AppID, err)
	}
	if err := json.Unmarshal([]byte(extraHostsJSON), &app.ExtraHosts); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store extra hosts for %s: %w", app.AppID, err)
	}
	secretRefs, err := secretRefsFromStorage(secretRefsJSON)
	if err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store secret refs for %s: %w", app.AppID, err)
	}
	app.SecretRefs = secretRefs
	if err := json.Unmarshal([]byte(companionsJSON), &app.Companions); err != nil {
		return InstalledApp{}, fmt.Errorf("decode desktop store companions for %s: %w", app.AppID, err)
	}
	return app, nil
}

type storedSecretRef struct {
	Key      string `json:"key"`
	VaultKey string `json:"vault_key"`
	Env      string `json:"env,omitempty"`
	Label    string `json:"label,omitempty"`
	Expose   bool   `json:"expose,omitempty"`
}

func secretRefsForStorage(refs []SecretRef) []storedSecretRef {
	stored := make([]storedSecretRef, 0, len(refs))
	for _, ref := range refs {
		stored = append(stored, storedSecretRef{
			Key:      ref.Key,
			VaultKey: ref.VaultKey,
			Env:      ref.Env,
			Label:    ref.Label,
			Expose:   ref.Expose,
		})
	}
	return stored
}

func secretRefsFromStorage(raw string) ([]SecretRef, error) {
	var stored []storedSecretRef
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, err
	}
	refs := make([]SecretRef, 0, len(stored))
	for _, ref := range stored {
		refs = append(refs, SecretRef{
			Key:      ref.Key,
			VaultKey: ref.VaultKey,
			Env:      ref.Env,
			Label:    ref.Label,
			Expose:   ref.Expose,
		})
	}
	return refs, nil
}

func scanOperation(scanner interface{ Scan(dest ...any) error }) (Operation, error) {
	var op Operation
	var createdAt, updatedAt string
	var completed sql.NullString
	err := scanner.Scan(&op.ID, &op.Type, &op.AppID, &op.Status, &op.Message, &op.Error, &op.RequestJSON, &createdAt, &updatedAt, &completed)
	if err != nil {
		return Operation{}, err
	}
	op.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	op.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if completed.Valid {
		if parsed, err := time.Parse(time.RFC3339Nano, completed.String); err == nil {
			op.CompletedAt = &parsed
		}
	}
	return op, nil
}

func containerSpecFromRecord(app InstalledApp) ContainerSpec {
	ports := append([]PortBinding(nil), app.Ports...)
	if len(ports) == 0 && app.HostPort > 0 {
		ports = []PortBinding{{ID: "main", Name: "Web UI", ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}}
	}
	return ContainerSpec{
		Name:         app.ContainerName,
		Image:        app.Image,
		Env:          append([]string(nil), app.Env...),
		PortBindings: ports,
		Volumes:      append([]VolumeBinding(nil), app.Volumes...),
		HostBinds:    append([]HostBinding(nil), app.HostBinds...),
		ExtraHosts:   append([]string(nil), app.ExtraHosts...),
		NetworkMode:  privateNetworkModeFromApp(app),
		Restart:      "unless-stopped",
		Labels: map[string]string{
			"aurago.desktop_store":        "true",
			"aurago.desktop_store.app_id": app.AppID,
		},
	}
}

func companionContainerSpec(app InstalledApp, companion CompanionApp) ContainerSpec {
	return ContainerSpec{
		Name:        companion.ContainerName,
		Image:       companion.Image,
		Env:         append([]string(nil), companion.Env...),
		Volumes:     append([]VolumeBinding(nil), companion.Volumes...),
		HostBinds:   append([]HostBinding(nil), companion.HostBinds...),
		NetworkMode: companion.NetworkMode,
		Restart:     "unless-stopped",
		Labels: map[string]string{
			"aurago.desktop_store":           "true",
			"aurago.desktop_store.app_id":    app.AppID,
			"aurago.desktop_store.companion": companion.ID,
		},
	}
}

func replaceCompanion(companions []CompanionApp, companion CompanionApp) []CompanionApp {
	out := make([]CompanionApp, 0, len(companions)+1)
	replaced := false
	for _, current := range companions {
		if current.ID == companion.ID {
			out = append(out, companion)
			replaced = true
			continue
		}
		out = append(out, current)
	}
	if !replaced {
		out = append(out, companion)
	}
	return out
}

func mergeCompanions(existing []CompanionApp, replacements []CompanionApp) []CompanionApp {
	out := append([]CompanionApp(nil), existing...)
	for _, replacement := range replacements {
		out = replaceCompanion(out, replacement)
	}
	return out
}

func companionIndex(companions []CompanionApp, id string) int {
	for i, companion := range companions {
		if companion.ID == id {
			return i
		}
	}
	return -1
}

func resolveVolumes(entry CatalogEntry) []VolumeBinding {
	volumes := make([]VolumeBinding, 0, len(entry.Volumes))
	for _, template := range entry.Volumes {
		suffix := strings.Trim(strings.ToLower(template.NameSuffix), "-_ ")
		if suffix == "" {
			suffix = "data"
		}
		volumes = append(volumes, VolumeBinding{
			Name:          fmt.Sprintf("aurago_store_%s_%s", strings.ReplaceAll(entry.ID, "-", "_"), suffix),
			ContainerPath: template.ContainerPath,
		})
	}
	return volumes
}

func resolveCompanionVolumes(entry CatalogEntry, companion CompanionTemplate) []VolumeBinding {
	volumes := make([]VolumeBinding, 0, len(companion.Volumes))
	for _, template := range companion.Volumes {
		suffix := strings.Trim(strings.ToLower(template.NameSuffix), "-_ ")
		if suffix == "" {
			suffix = companion.ID + "-data"
		}
		volumes = append(volumes, VolumeBinding{
			Name:          fmt.Sprintf("aurago_store_%s_%s", strings.ReplaceAll(entry.ID, "-", "_"), strings.ReplaceAll(suffix, "-", "_")),
			ContainerPath: template.ContainerPath,
		})
	}
	return volumes
}

func privateStoreNetworkName(entry CatalogEntry) string {
	for _, companion := range entry.Companions {
		networkMode := strings.TrimSpace(companion.NetworkMode)
		if isPrivateStoreNetwork(networkMode) {
			return networkMode
		}
	}
	return ""
}

func privateNetworkModeFromApp(app InstalledApp) string {
	for _, companion := range app.Companions {
		networkMode := strings.TrimSpace(companion.NetworkMode)
		if isPrivateStoreNetwork(networkMode) {
			return networkMode
		}
	}
	return ""
}

func isPrivateStoreNetwork(networkMode string) bool {
	networkMode = strings.ToLower(strings.TrimSpace(networkMode))
	if networkMode == "" {
		return false
	}
	switch networkMode {
	case "host", "bridge", "none", "default":
		return false
	}
	return !strings.HasPrefix(networkMode, "container:")
}

func normalizeAppID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func normalizeBindMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", BindModeLocal:
		return BindModeLocal
	case BindModeLAN, "public", "all":
		return BindModeLAN
	default:
		return ""
	}
}

// DesktopAppID returns the managed generated desktop app ID for a store app.
func DesktopAppID(appID string) string {
	return "store-" + normalizeAppID(appID)
}

// ContainerName returns the managed Docker container name for a store app.
func ContainerName(appID string) string {
	return "aurago-store-" + normalizeAppID(appID)
}

// CompanionContainerName returns the managed Docker container name for a Store
// app companion container.
func CompanionContainerName(appID, companionID string) string {
	return ContainerName(appID) + "-" + normalizeAppID(companionID)
}

// ManagedLaunchpadLinkID returns the stable Launchpad link ID for a store app.
func ManagedLaunchpadLinkID(appID string) string {
	return "store-" + normalizeAppID(appID)
}

func DefaultPortAllocator(ctx context.Context, preferred int) (int, error) {
	if preferred >= 1024 && portAvailable(ctx, preferred) {
		return preferred, nil
	}
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp4", "0.0.0.0:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func portAvailable(ctx context.Context, port int) bool {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			return parsed
		}
	}
	return host
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}

func randomHex(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureDir(dir string) error {
	return os.MkdirAll(filepath.Clean(dir), 0o755)
}

type missingDockerAdapter struct{}

func (missingDockerAdapter) PullImage(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CreateContainer(context.Context, ContainerSpec) (string, error) {
	return "", fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CopyToContainer(context.Context, string, string, map[string]string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) StartContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) StopContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RestartContainer(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveContainer(context.Context, string, bool) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveVolume(context.Context, string, bool) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) CreateNetwork(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) RemoveNetwork(context.Context, string) error {
	return fmt.Errorf("Docker adapter is not configured")
}
func (missingDockerAdapter) InspectContainer(context.Context, string) (ContainerState, error) {
	return ContainerState{}, fmt.Errorf("Docker adapter is not configured")
}
