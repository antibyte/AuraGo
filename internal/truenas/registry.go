package truenas

import (
	"aurago/internal/dbutil"
	"database/sql"
	"fmt"
	"time"
)

// InitRegistryDB initializes the TrueNAS registry database schema.
func InitRegistryDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open truenas registry db: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	schema := `
-- TrueNAS Server Registry
CREATE TABLE IF NOT EXISTS truenas_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 443,
    use_https BOOLEAN DEFAULT 1,
    version TEXT,      -- SCALE or Core version
    status TEXT DEFAULT 'unknown', -- online, offline, error
    last_check DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Pool tracking
CREATE TABLE IF NOT EXISTS truenas_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    pool_id INTEGER,  -- TrueNAS internal ID
    name TEXT NOT NULL,
    guid TEXT,
    status TEXT,  -- ONLINE, DEGRADED, FAULTED
    size_bytes INTEGER,
    allocated_bytes INTEGER,
    free_bytes INTEGER,
    scan_status TEXT,
    last_scrub DATETIME,
    last_check DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, name)
);

-- Dataset tracking
CREATE TABLE IF NOT EXISTS truenas_datasets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    pool_name TEXT NOT NULL,
    dataset_id TEXT,
    name TEXT NOT NULL,
    type TEXT,  -- FILESYSTEM, VOLUME
    mountpoint TEXT,
    size_bytes INTEGER,
    used_bytes INTEGER,
    available_bytes INTEGER,
    compression TEXT,
    quota_bytes INTEGER,
    readonly BOOLEAN DEFAULT 0,
    share_type TEXT,  -- GENERIC, SMB
    last_check DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, name)
);

-- Snapshot tracking
CREATE TABLE IF NOT EXISTS truenas_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    dataset_name TEXT NOT NULL,
    snapshot_id TEXT,
    name TEXT NOT NULL,
    snapshot_type TEXT DEFAULT 'manual',  -- manual, scheduled, replication
    size_bytes INTEGER,
    created_at DATETIME,
    replicated BOOLEAN DEFAULT 0,
    retention_days INTEGER,
    expires_at DATETIME,
    last_check DATETIME,
    UNIQUE(server_name, name)
);

-- Share tracking
CREATE TABLE IF NOT EXISTS truenas_shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    share_id INTEGER,
    name TEXT NOT NULL,
    share_type TEXT,  -- SMB, NFS
    dataset_name TEXT,
    path TEXT,
    enabled BOOLEAN DEFAULT 1,
    guest_ok BOOLEAN DEFAULT 0,
    timemachine BOOLEAN DEFAULT 0,
    last_check DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, name, share_type)
);

-- Alert tracking
CREATE TABLE IF NOT EXISTS truenas_alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    alert_id TEXT NOT NULL,
    level TEXT,  -- INFO, WARNING, ERROR, CRITICAL
    title TEXT,
    message TEXT,
    dismissed BOOLEAN DEFAULT 0,
    dismissed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_name, alert_id)
);

-- Sync log for tracking synchronization operations
CREATE TABLE IF NOT EXISTS truenas_sync_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT NOT NULL,
    operation TEXT NOT NULL,
    status TEXT,  -- success, error, partial
    details TEXT,
    duration_ms INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_pools_server ON truenas_pools(server_name);
CREATE INDEX IF NOT EXISTS idx_datasets_server ON truenas_datasets(server_name);
CREATE INDEX IF NOT EXISTS idx_datasets_pool ON truenas_datasets(pool_name);
CREATE INDEX IF NOT EXISTS idx_snapshots_server ON truenas_snapshots(server_name);
CREATE INDEX IF NOT EXISTS idx_snapshots_dataset ON truenas_snapshots(dataset_name);
CREATE INDEX IF NOT EXISTS idx_shares_server ON truenas_shares(server_name);
CREATE INDEX IF NOT EXISTS idx_alerts_server ON truenas_alerts(server_name);
CREATE INDEX IF NOT EXISTS idx_alerts_level ON truenas_alerts(level);
`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create truenas registry tables: %w", err)
	}

	return nil
}

// ServerRecord represents a registered TrueNAS server.
type ServerRecord struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	UseHTTPS  bool      `json:"use_https"`
	Version   string    `json:"version"`
	Status    string    `json:"status"`
	LastCheck time.Time `json:"last_check"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SaveServer registers or updates a TrueNAS server.
func SaveServer(db *sql.DB, record *ServerRecord) error {
	_, err := db.Exec(`
		INSERT INTO truenas_servers (name, host, port, use_https, version, status, last_check)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			host = excluded.host,
			port = excluded.port,
			use_https = excluded.use_https,
			version = excluded.version,
			status = excluded.status,
			last_check = excluded.last_check,
			updated_at = CURRENT_TIMESTAMP
	`, record.Name, record.Host, record.Port, record.UseHTTPS, record.Version, record.Status, record.LastCheck)

	return err
}

// GetServer retrieves a server by name.
func GetServer(db *sql.DB, name string) (*ServerRecord, error) {
	var s ServerRecord
	var lastCheck sql.NullTime

	err := db.QueryRow(`
		SELECT id, name, host, port, use_https, version, status, last_check, created_at, updated_at
		FROM truenas_servers WHERE name = ?
	`, name).Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.UseHTTPS, &s.Version, &s.Status, &lastCheck, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if lastCheck.Valid {
		s.LastCheck = lastCheck.Time
	}

	return &s, nil
}

// SavePool saves pool information.
func SavePool(db *sql.DB, serverName string, pool *PoolRecord) error {
	_, err := db.Exec(`
		INSERT INTO truenas_pools (server_name, pool_id, name, guid, status, size_bytes, allocated_bytes, free_bytes, scan_status, last_scrub, last_check)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_name, name) DO UPDATE SET
			pool_id = excluded.pool_id,
			status = excluded.status,
			size_bytes = excluded.size_bytes,
			allocated_bytes = excluded.allocated_bytes,
			free_bytes = excluded.free_bytes,
			scan_status = excluded.scan_status,
			last_scrub = excluded.last_scrub,
			last_check = excluded.last_check,
			updated_at = CURRENT_TIMESTAMP
	`, serverName, pool.PoolID, pool.Name, pool.GUID, pool.Status, pool.SizeBytes, pool.AllocatedBytes, pool.FreeBytes, pool.ScanStatus, pool.LastScrub, pool.LastCheck)

	return err
}

// PoolRecord represents a tracked pool.
type PoolRecord struct {
	PoolID         int64     `json:"pool_id"`
	Name           string    `json:"name"`
	GUID           string    `json:"guid"`
	Status         string    `json:"status"`
	SizeBytes      int64     `json:"size_bytes"`
	AllocatedBytes int64     `json:"allocated_bytes"`
	FreeBytes      int64     `json:"free_bytes"`
	ScanStatus     string    `json:"scan_status"`
	LastScrub      time.Time `json:"last_scrub"`
	LastCheck      time.Time `json:"last_check"`
}

// GetPoolsByServer returns all pools for a server.
func GetPoolsByServer(db *sql.DB, serverName string) ([]PoolRecord, error) {
	rows, err := db.Query(`
		SELECT pool_id, name, guid, status, size_bytes, allocated_bytes, free_bytes, scan_status, last_scrub, last_check
		FROM truenas_pools WHERE server_name = ?
	`, serverName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []PoolRecord
	for rows.Next() {
		var p PoolRecord
		var lastScrub, lastCheck sql.NullTime

		err := rows.Scan(&p.PoolID, &p.Name, &p.GUID, &p.Status, &p.SizeBytes, &p.AllocatedBytes, &p.FreeBytes, &p.ScanStatus, &lastScrub, &lastCheck)
		if err != nil {
			continue
		}

		if lastScrub.Valid {
			p.LastScrub = lastScrub.Time
		}
		if lastCheck.Valid {
			p.LastCheck = lastCheck.Time
		}

		pools = append(pools, p)
	}

	return pools, rows.Err()
}

// SaveDataset saves dataset information.
func SaveDataset(db *sql.DB, serverName string, ds *DatasetRecord) error {
	_, err := db.Exec(`
		INSERT INTO truenas_datasets (server_name, pool_name, dataset_id, name, type, mountpoint, size_bytes, used_bytes, available_bytes, compression, quota_bytes, readonly, share_type, last_check)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_name, name) DO UPDATE SET
			pool_name = excluded.pool_name,
			type = excluded.type,
			mountpoint = excluded.mountpoint,
			size_bytes = excluded.size_bytes,
			used_bytes = excluded.used_bytes,
			available_bytes = excluded.available_bytes,
			compression = excluded.compression,
			quota_bytes = excluded.quota_bytes,
			readonly = excluded.readonly,
			share_type = excluded.share_type,
			last_check = excluded.last_check,
			updated_at = CURRENT_TIMESTAMP
	`, serverName, ds.PoolName, ds.DatasetID, ds.Name, ds.Type, ds.Mountpoint, ds.SizeBytes, ds.UsedBytes, ds.AvailableBytes, ds.Compression, ds.QuotaBytes, ds.ReadOnly, ds.ShareType, ds.LastCheck)

	return err
}

// DatasetRecord represents a tracked dataset.
type DatasetRecord struct {
	DatasetID      string    `json:"dataset_id"`
	PoolName       string    `json:"pool_name"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	Mountpoint     string    `json:"mountpoint"`
	SizeBytes      int64     `json:"size_bytes"`
	UsedBytes      int64     `json:"used_bytes"`
	AvailableBytes int64     `json:"available_bytes"`
	Compression    string    `json:"compression"`
	QuotaBytes     int64     `json:"quota_bytes"`
	ReadOnly       bool      `json:"readonly"`
	ShareType      string    `json:"share_type"`
	LastCheck      time.Time `json:"last_check"`
}

// GetDatasetsByPool returns datasets for a pool.
func GetDatasetsByPool(db *sql.DB, serverName, poolName string) ([]DatasetRecord, error) {
	rows, err := db.Query(`
		SELECT dataset_id, name, type, mountpoint, size_bytes, used_bytes, available_bytes, compression, quota_bytes, readonly, share_type, last_check
		FROM truenas_datasets WHERE server_name = ? AND pool_name = ?
	`, serverName, poolName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var datasets []DatasetRecord
	for rows.Next() {
		var d DatasetRecord
		d.PoolName = poolName

		var lastCheck sql.NullTime
		err := rows.Scan(&d.DatasetID, &d.Name, &d.Type, &d.Mountpoint, &d.SizeBytes, &d.UsedBytes, &d.AvailableBytes, &d.Compression, &d.QuotaBytes, &d.ReadOnly, &d.ShareType, &lastCheck)
		if err != nil {
			continue
		}

		if lastCheck.Valid {
			d.LastCheck = lastCheck.Time
		}

		datasets = append(datasets, d)
	}

	return datasets, rows.Err()
}

// SaveSnapshot saves snapshot information.
func SaveSnapshot(db *sql.DB, serverName string, snap *SnapshotRecord) error {
	_, err := db.Exec(`
		INSERT INTO truenas_snapshots (server_name, dataset_name, snapshot_id, name, snapshot_type, size_bytes, created_at, replicated, retention_days, expires_at, last_check)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_name, name) DO UPDATE SET
			size_bytes = excluded.size_bytes,
			replicated = excluded.replicated,
			retention_days = excluded.retention_days,
			expires_at = excluded.expires_at,
			last_check = excluded.last_check
	`, serverName, snap.DatasetName, snap.SnapshotID, snap.Name, snap.Type, snap.SizeBytes, snap.CreatedAt, snap.Replicated, snap.RetentionDays, snap.ExpiresAt, snap.LastCheck)

	return err
}

// SnapshotRecord represents a tracked snapshot.
type SnapshotRecord struct {
	SnapshotID    string    `json:"snapshot_id"`
	DatasetName   string    `json:"dataset_name"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	SizeBytes     int64     `json:"size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	Replicated    bool      `json:"replicated"`
	RetentionDays int       `json:"retention_days"`
	ExpiresAt     time.Time `json:"expires_at"`
	LastCheck     time.Time `json:"last_check"`
}

// GetSnapshotsByDataset returns snapshots for a dataset.
func GetSnapshotsByDataset(db *sql.DB, serverName, datasetName string) ([]SnapshotRecord, error) {
	rows, err := db.Query(`
		SELECT snapshot_id, name, snapshot_type, size_bytes, created_at, replicated, retention_days, expires_at, last_check
		FROM truenas_snapshots WHERE server_name = ? AND dataset_name = ?
		ORDER BY created_at DESC
	`, serverName, datasetName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []SnapshotRecord
	for rows.Next() {
		var s SnapshotRecord
		s.DatasetName = datasetName

		var createdAt, expiresAt, lastCheck sql.NullTime
		err := rows.Scan(&s.SnapshotID, &s.Name, &s.Type, &s.SizeBytes, &createdAt, &s.Replicated, &s.RetentionDays, &expiresAt, &lastCheck)
		if err != nil {
			continue
		}

		if createdAt.Valid {
			s.CreatedAt = createdAt.Time
		}
		if expiresAt.Valid {
			s.ExpiresAt = expiresAt.Time
		}
		if lastCheck.Valid {
			s.LastCheck = lastCheck.Time
		}

		snapshots = append(snapshots, s)
	}

	return snapshots, rows.Err()
}

// SaveAlert saves an alert.
func SaveAlert(db *sql.DB, serverName string, alert *AlertRecord) error {
	_, err := db.Exec(`
		INSERT INTO truenas_alerts (server_name, alert_id, level, title, message, dismissed, dismissed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_name, alert_id) DO UPDATE SET
			level = excluded.level,
			title = excluded.title,
			message = excluded.message,
			dismissed = excluded.dismissed,
			dismissed_at = excluded.dismissed_at
	`, serverName, alert.AlertID, alert.Level, alert.Title, alert.Message, alert.Dismissed, alert.DismissedAt)

	return err
}

// AlertRecord represents a tracked alert.
type AlertRecord struct {
	AlertID     string    `json:"alert_id"`
	Level       string    `json:"level"`
	Title       string    `json:"title"`
	Message     string    `json:"message"`
	Dismissed   bool      `json:"dismissed"`
	DismissedAt time.Time `json:"dismissed_at"`
}

// GetActiveAlerts returns non-dismissed alerts for a server.
func GetActiveAlerts(db *sql.DB, serverName string) ([]AlertRecord, error) {
	rows, err := db.Query(`
		SELECT alert_id, level, title, message, dismissed, dismissed_at
		FROM truenas_alerts WHERE server_name = ? AND dismissed = 0
		ORDER BY created_at DESC
	`, serverName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []AlertRecord
	for rows.Next() {
		var a AlertRecord
		var dismissedAt sql.NullTime

		err := rows.Scan(&a.AlertID, &a.Level, &a.Title, &a.Message, &a.Dismissed, &dismissedAt)
		if err != nil {
			continue
		}

		if dismissedAt.Valid {
			a.DismissedAt = dismissedAt.Time
		}

		alerts = append(alerts, a)
	}

	return alerts, rows.Err()
}

// LogSyncOperation logs a synchronization operation.
func LogSyncOperation(db *sql.DB, serverName, operation, status, details string, durationMs int) error {
	_, err := db.Exec(`
		INSERT INTO truenas_sync_log (server_name, operation, status, details, duration_ms)
		VALUES (?, ?, ?, ?, ?)
	`, serverName, operation, status, details, durationMs)
	return err
}

// GetLastSyncLog returns the most recent sync log entries.
func GetLastSyncLog(db *sql.DB, serverName string, limit int) ([]SyncLogEntry, error) {
	rows, err := db.Query(`
		SELECT operation, status, details, duration_ms, created_at
		FROM truenas_sync_log WHERE server_name = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, serverName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []SyncLogEntry
	for rows.Next() {
		var e SyncLogEntry
		e.ServerName = serverName

		var createdAt sql.NullTime
		err := rows.Scan(&e.Operation, &e.Status, &e.Details, &e.DurationMs, &createdAt)
		if err != nil {
			continue
		}

		if createdAt.Valid {
			e.CreatedAt = createdAt.Time
		}

		entries = append(entries, e)
	}

	return entries, rows.Err()
}

// SyncLogEntry represents a sync log record.
type SyncLogEntry struct {
	ServerName string    `json:"server_name"`
	Operation  string    `json:"operation"`
	Status     string    `json:"status"`
	Details    string    `json:"details"`
	DurationMs int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// DeleteOldSnapshots removes expired snapshot records from the database.
func DeleteOldSnapshots(db *sql.DB, serverName string, olderThan time.Time) (int64, error) {
	result, err := db.Exec(`
		DELETE FROM truenas_snapshots WHERE server_name = ? AND last_check < ?
	`, serverName, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupOldRecords removes old records from the database.
func CleanupOldRecords(db *sql.DB, serverName string, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Clean up old alerts
	_, err := db.Exec(`DELETE FROM truenas_alerts WHERE server_name = ? AND created_at < ?`, serverName, cutoff)
	if err != nil {
		return err
	}

	// Clean up old sync logs
	_, err = db.Exec(`DELETE FROM truenas_sync_log WHERE server_name = ? AND created_at < ?`, serverName, cutoff)
	if err != nil {
		return err
	}

	return nil
}
