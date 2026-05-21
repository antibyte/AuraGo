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
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"aurago/internal/desktop"

	_ "modernc.org/sqlite"
)

const storeSource = "desktop-store"

var storeAppIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)

// Config describes the desktop store service dependencies.
type Config struct {
	DBPath        string
	DockerHost    string
	DataDir       string
	Catalog       []CatalogEntry
	Docker        DockerAdapter
	Desktop       DesktopAdapter
	Launchpad     LaunchpadAdapter
	PortAllocator PortAllocator
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
	catalog := cfg.Catalog
	if len(catalog) == 0 {
		catalog = DefaultCatalog()
	}
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
		if entry.PrimaryPort.ContainerPort <= 0 {
			return nil, fmt.Errorf("catalog app %s container port is required", entry.ID)
		}
		catalogByID[entry.ID] = entry
	}
	allocator := cfg.PortAllocator
	if allocator == nil {
		allocator = DefaultPortAllocator
	}
	return &Service{
		cfg:           cfg,
		catalog:       append([]CatalogEntry(nil), catalog...),
		catalogByID:   catalogByID,
		portAllocator: allocator,
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
			volumes_json TEXT NOT NULL DEFAULT '[]',
			env_json TEXT NOT NULL DEFAULT '[]',
			extra_hosts_json TEXT NOT NULL DEFAULT '[]',
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
		tailscale_port, logo_path, volumes_json, env_json, extra_hosts_json, created_at, updated_at,
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
		tailscale_port, logo_path, volumes_json, env_json, extra_hosts_json, created_at, updated_at,
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
func (s *Service) OpenURL(ctx context.Context, appID, requestHost string, fromTailscale bool, tailscaleDNS string) (string, InstalledApp, error) {
	app, ok, err := s.GetInstalled(ctx, appID)
	if err != nil {
		return "", InstalledApp{}, err
	}
	if !ok {
		return "", InstalledApp{}, fmt.Errorf("store app %q is not installed", appID)
	}
	if fromTailscale && app.TailscaleEnabled && app.TailscaleStatus == TailscaleStatusActive && strings.TrimSpace(tailscaleDNS) != "" {
		return fmt.Sprintf("https://%s:%d/", strings.Trim(strings.TrimSpace(tailscaleDNS), "."), app.TailscalePort), app, nil
	}
	host := "127.0.0.1"
	if app.BindMode == BindModeLAN {
		if reqHost := hostWithoutPort(requestHost); reqHost != "" && !isLoopbackHost(reqHost) {
			host = reqHost
		}
	}
	return fmt.Sprintf("http://%s:%d/", host, app.HostPort), app, nil
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

func (s *Service) install(ctx context.Context, op Operation, req InstallRequest) error {
	entry, ok := s.catalogByID[normalizeAppID(req.AppID)]
	if !ok {
		return fmt.Errorf("store app %q is not in the allowlist", req.AppID)
	}
	if _, exists, err := s.GetInstalled(ctx, entry.ID); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("store app %q is already installed", entry.ID)
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
	if err := s.saveInstalled(ctx, record); err != nil {
		return err
	}
	if err := s.requireDocker().PullImage(ctx, entry.Image); err != nil {
		return s.failInstall(ctx, record, err)
	}
	spec := containerSpecFromRecord(record)
	containerID, err := s.requireDocker().CreateContainer(ctx, spec)
	if err != nil {
		return s.failInstall(ctx, record, err)
	}
	record.ContainerID = containerID
	if err := s.requireDocker().StartContainer(ctx, record.ContainerName); err != nil {
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
	record.Status = AppStatusUpdating
	record.Error = ""
	record.LastOperationID = op.ID
	record.LastOperationType = op.Type
	record.LastOperationState = OperationRunning
	if err := s.saveInstalled(ctx, record); err != nil {
		return err
	}
	restorePrevious := func(runErr error) error {
		previous.LastOperationID = op.ID
		previous.LastOperationType = op.Type
		previous.LastOperationState = OperationFailed
		_ = s.saveInstalled(ctx, previous)
		return runErr
	}
	rollbackPrevious := func(runErr error) error {
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
	_ = s.requireDocker().StopContainer(ctx, record.ContainerName)
	if err := s.requireDocker().RemoveContainer(ctx, record.ContainerName, true); err != nil {
		return restorePrevious(fmt.Errorf("remove old container: %w", err))
	}
	containerID, err := s.requireDocker().CreateContainer(ctx, containerSpecFromRecord(record))
	if err != nil {
		return rollbackPrevious(fmt.Errorf("create updated container: %w", err))
	}
	record.ContainerID = containerID
	if previousWasRunning {
		if err := s.requireDocker().StartContainer(ctx, record.ContainerName); err != nil {
			_ = s.requireDocker().RemoveContainer(ctx, record.ContainerName, true)
			return rollbackPrevious(fmt.Errorf("start updated container: %w", err))
		}
	}
	record.Status = previous.Status
	record.Error = ""
	record.LastOperationState = OperationSucceeded
	return s.saveInstalled(ctx, record)
}

func (s *Service) start(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusRunning, func(app InstalledApp) error {
		return s.requireDocker().StartContainer(ctx, app.ContainerName)
	})
}

func (s *Service) stop(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusStopped, func(app InstalledApp) error {
		return s.requireDocker().StopContainer(ctx, app.ContainerName)
	})
}

func (s *Service) restart(ctx context.Context, op Operation) error {
	return s.action(ctx, op, AppStatusRunning, func(app InstalledApp) error {
		return s.requireDocker().RestartContainer(ctx, app.ContainerName)
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
	_ = s.requireDocker().StopContainer(ctx, app.ContainerName)
	if err := s.requireDocker().RemoveContainer(ctx, app.ContainerName, true); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	if s.cfg.Desktop != nil {
		if err := s.cfg.Desktop.DeleteApp(ctx, app.DesktopAppID, storeSource); err != nil {
			return fmt.Errorf("delete desktop app: %w", err)
		}
	}
	if s.cfg.Launchpad != nil && app.LaunchpadLinkID != "" {
		if err := s.cfg.Launchpad.DeleteStoreLink(ctx, app.LaunchpadLinkID); err != nil {
			return fmt.Errorf("delete launchpad link: %w", err)
		}
	}
	if deleteData {
		for _, volume := range app.Volumes {
			if err := s.requireDocker().RemoveVolume(ctx, volume.Name, true); err != nil {
				return fmt.Errorf("remove volume %s: %w", volume.Name, err)
			}
		}
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM desktop_store_apps WHERE app_id = ?`, app.AppID)
	if err != nil {
		return fmt.Errorf("delete desktop store app record: %w", err)
	}
	return nil
}

func (s *Service) buildInstallRecord(entry CatalogEntry, op Operation, bindMode, hostIP string, hostPort int, tailscaleEnabled bool) InstalledApp {
	now := time.Now().UTC()
	env := append([]string(nil), entry.Env...)
	if entry.ID == "homarr" {
		env = append(env, "SECRET_ENCRYPTION_KEY="+randomHex(32))
	}
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
		Volumes:            resolveVolumes(entry),
		Env:                env,
		ExtraHosts:         append([]string(nil), entry.ExtraHosts...),
		CreatedAt:          now,
		UpdatedAt:          now,
		LastOperationID:    op.ID,
		LastOperationType:  op.Type,
		LastOperationState: OperationRunning,
	}
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
	files := map[string]string{
		"index.html": `<!doctype html><meta charset="utf-8"><title>` + entry.Name + `</title><p>` + entry.Name + ` is managed by AuraGo Software Store.</p>`,
	}
	if err := s.cfg.Desktop.InstallApp(ctx, manifest, files, storeSource); err != nil {
		return fmt.Errorf("install desktop app: %w", err)
	}
	dockVisible := false
	startVisible := true
	if err := s.cfg.Desktop.SetAppVisibility(ctx, app.DesktopAppID, &dockVisible, &startVisible, storeSource); err != nil {
		return fmt.Errorf("set desktop app visibility: %w", err)
	}
	if err := s.cfg.Desktop.AddDesktopAppShortcut(ctx, app.DesktopAppID, storeSource); err != nil {
		return fmt.Errorf("add desktop shortcut: %w", err)
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
	volumesJSON, _ := json.Marshal(app.Volumes)
	envJSON, _ := json.Marshal(app.Env)
	extraHostsJSON, _ := json.Marshal(app.ExtraHosts)
	_, err := s.db.ExecContext(ctx, `INSERT INTO desktop_store_apps(app_id, desktop_app_id, launchpad_link_id,
		container_name, container_id, image, status, error, bind_mode, host_ip, host_port, container_port, protocol,
		tailscale_enabled, tailscale_status, tailscale_port, logo_path, volumes_json, env_json, extra_hosts_json,
		created_at, updated_at, last_operation_id, last_operation_type, last_operation_state)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			volumes_json = excluded.volumes_json,
			env_json = excluded.env_json,
			extra_hosts_json = excluded.extra_hosts_json,
			updated_at = excluded.updated_at,
			last_operation_id = excluded.last_operation_id,
			last_operation_type = excluded.last_operation_type,
			last_operation_state = excluded.last_operation_state`,
		app.AppID, app.DesktopAppID, app.LaunchpadLinkID, app.ContainerName, app.ContainerID, app.Image, app.Status, app.Error,
		app.BindMode, app.HostIP, app.HostPort, app.ContainerPort, app.Protocol, boolToInt(app.TailscaleEnabled), app.TailscaleStatus,
		app.TailscalePort, app.LogoPath, string(volumesJSON), string(envJSON), string(extraHostsJSON), formatTime(app.CreatedAt),
		formatTime(app.UpdatedAt), app.LastOperationID, app.LastOperationType, app.LastOperationState)
	if err != nil {
		return fmt.Errorf("save desktop store app: %w", err)
	}
	return nil
}

func (s *Service) failInstall(ctx context.Context, app InstalledApp, runErr error) error {
	_ = s.requireDocker().StopContainer(ctx, app.ContainerName)
	_ = s.requireDocker().RemoveContainer(ctx, app.ContainerName, true)
	for _, volume := range app.Volumes {
		_ = s.requireDocker().RemoveVolume(ctx, volume.Name, true)
	}
	if s.cfg.Desktop != nil {
		_ = s.cfg.Desktop.DeleteApp(ctx, app.DesktopAppID, storeSource)
	}
	if s.cfg.Launchpad != nil && app.LaunchpadLinkID != "" {
		_ = s.cfg.Launchpad.DeleteStoreLink(ctx, app.LaunchpadLinkID)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM desktop_store_apps WHERE app_id = ?`, app.AppID)
	return runErr
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
	var volumesJSON, envJSON, extraHostsJSON string
	err := scanner.Scan(&app.AppID, &app.DesktopAppID, &app.LaunchpadLinkID, &app.ContainerName, &app.ContainerID,
		&app.Image, &app.Status, &app.Error, &app.BindMode, &app.HostIP, &app.HostPort, &app.ContainerPort, &app.Protocol,
		&tailscaleEnabled, &app.TailscaleStatus, &app.TailscalePort, &app.LogoPath, &volumesJSON, &envJSON, &extraHostsJSON,
		&createdAt, &updatedAt, &app.LastOperationID, &app.LastOperationType, &app.LastOperationState)
	if err != nil {
		return InstalledApp{}, err
	}
	app.TailscaleEnabled = tailscaleEnabled != 0
	app.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	app.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	_ = json.Unmarshal([]byte(volumesJSON), &app.Volumes)
	_ = json.Unmarshal([]byte(envJSON), &app.Env)
	_ = json.Unmarshal([]byte(extraHostsJSON), &app.ExtraHosts)
	return app, nil
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
	return ContainerSpec{
		Name:         app.ContainerName,
		Image:        app.Image,
		Env:          append([]string(nil), app.Env...),
		PortBindings: []PortBinding{{ContainerPort: app.ContainerPort, Protocol: app.Protocol, HostIP: app.HostIP, HostPort: app.HostPort}},
		Volumes:      append([]VolumeBinding(nil), app.Volumes...),
		ExtraHosts:   append([]string(nil), app.ExtraHosts...),
		Restart:      "unless-stopped",
		Labels: map[string]string{
			"aurago.desktop_store":        "true",
			"aurago.desktop_store.app_id": app.AppID,
		},
	}
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
func (missingDockerAdapter) InspectContainer(context.Context, string) (ContainerState, error) {
	return ContainerState{}, fmt.Errorf("Docker adapter is not configured")
}
