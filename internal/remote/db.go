package remote

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// ── Record types ────────────────────────────────────────────────────────────

// DeviceRecord represents a managed remote device.
type DeviceRecord struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Hostname        string   `json:"hostname"`
	OS              string   `json:"os"`
	Arch            string   `json:"arch"`
	IPAddress       string   `json:"ip_address"`
	Status          string   `json:"status"` // "pending", "approved", "connected", "offline", "revoked"
	ReadOnly        bool     `json:"read_only"`
	AllowedPaths    []string `json:"allowed_paths"`
	SharedKeyHash   string   `json:"-"`
	EnrollmentToken string   `json:"-"` // one-time token hash (null after use)
	LastSeen        string   `json:"last_seen"`
	CreatedAt       string   `json:"created_at"`
	Version         string   `json:"version"`
	Tags            []string `json:"tags"`
}

// EnrollmentRecord tracks one-time enrollment tokens.
type EnrollmentRecord struct {
	ID           string `json:"id"`
	TokenHash    string `json:"-"`
	DeviceName   string `json:"device_name"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
	Used         bool   `json:"used"`
	UsedByDevice string `json:"used_by_device"`
}

// AuditEntry records a remote operation for compliance.
type AuditEntry struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"device_id"`
	Timestamp  string `json:"timestamp"`
	Operation  string `json:"operation"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
}

// ── Database init ───────────────────────────────────────────────────────────

// InitDB initializes the remote control SQLite database.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote control database: %w", err)
	}

	devicesSchema := `
	CREATE TABLE IF NOT EXISTS remote_devices (
		id               TEXT PRIMARY KEY,
		name             TEXT NOT NULL,
		hostname         TEXT DEFAULT '',
		os               TEXT DEFAULT '',
		arch             TEXT DEFAULT '',
		ip_address       TEXT DEFAULT '',
		status           TEXT DEFAULT 'pending',
		read_only        INTEGER DEFAULT 1,
		allowed_paths    TEXT DEFAULT '[]',
		shared_key_hash  TEXT DEFAULT '',
		enrollment_token TEXT DEFAULT '',
		last_seen        TEXT DEFAULT '',
		created_at       TEXT NOT NULL,
		version          TEXT DEFAULT '',
		tags             TEXT DEFAULT '[]'
	);`
	if _, err := db.Exec(devicesSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create remote_devices schema: %w", err)
	}

	auditSchema := `
	CREATE TABLE IF NOT EXISTS remote_audit_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id   TEXT NOT NULL,
		timestamp   TEXT NOT NULL,
		operation   TEXT NOT NULL,
		command     TEXT DEFAULT '',
		status      TEXT DEFAULT '',
		duration_ms INTEGER DEFAULT 0
	);`
	if _, err := db.Exec(auditSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create remote_audit_log schema: %w", err)
	}

	enrollSchema := `
	CREATE TABLE IF NOT EXISTS remote_enrollments (
		id             TEXT PRIMARY KEY,
		token_hash     TEXT NOT NULL,
		device_name    TEXT DEFAULT '',
		created_at     TEXT NOT NULL,
		expires_at     TEXT NOT NULL,
		used           INTEGER DEFAULT 0,
		used_by_device TEXT DEFAULT ''
	);`
	if _, err := db.Exec(enrollSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create remote_enrollments schema: %w", err)
	}

	return db, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func jsonStrings(ss []string) string {
	if ss == nil {
		ss = []string{}
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func parseStrings(s string) []string {
	var out []string
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

// ── Devices CRUD ────────────────────────────────────────────────────────────

// CreateDevice inserts a new device record and returns its UUID.
func CreateDevice(db *sql.DB, d DeviceRecord) (string, error) {
	d.ID = uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO remote_devices
			(id, name, hostname, os, arch, ip_address, status, read_only, allowed_paths,
			 shared_key_hash, enrollment_token, last_seen, created_at, version, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.Hostname, d.OS, d.Arch, d.IPAddress, d.Status,
		boolToInt(d.ReadOnly), jsonStrings(d.AllowedPaths),
		d.SharedKeyHash, d.EnrollmentToken, d.LastSeen, now, d.Version, jsonStrings(d.Tags),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create device: %w", err)
	}
	return d.ID, nil
}

// GetDevice retrieves a device by ID.
func GetDevice(db *sql.DB, id string) (DeviceRecord, error) {
	row := db.QueryRow(`SELECT id, name, hostname, os, arch, ip_address, status,
		read_only, allowed_paths, shared_key_hash, enrollment_token,
		last_seen, created_at, version, tags
		FROM remote_devices WHERE id = ?`, id)
	return scanDevice(row)
}

// GetDeviceByName retrieves a device by name (case-insensitive).
func GetDeviceByName(db *sql.DB, name string) (DeviceRecord, error) {
	row := db.QueryRow(`SELECT id, name, hostname, os, arch, ip_address, status,
		read_only, allowed_paths, shared_key_hash, enrollment_token,
		last_seen, created_at, version, tags
		FROM remote_devices WHERE LOWER(name) = LOWER(?)`, name)
	return scanDevice(row)
}

// ListDevices returns all device records.
func ListDevices(db *sql.DB) ([]DeviceRecord, error) {
	rows, err := db.Query(`SELECT id, name, hostname, os, arch, ip_address, status,
		read_only, allowed_paths, shared_key_hash, enrollment_token,
		last_seen, created_at, version, tags
		FROM remote_devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()
	return scanDevices(rows)
}

// UpdateDevice updates a device record.
func UpdateDevice(db *sql.DB, d DeviceRecord) error {
	_, err := db.Exec(`
		UPDATE remote_devices SET
			name = ?, hostname = ?, os = ?, arch = ?, ip_address = ?,
			status = ?, read_only = ?, allowed_paths = ?,
			shared_key_hash = ?, last_seen = ?, version = ?, tags = ?
		WHERE id = ?`,
		d.Name, d.Hostname, d.OS, d.Arch, d.IPAddress,
		d.Status, boolToInt(d.ReadOnly), jsonStrings(d.AllowedPaths),
		d.SharedKeyHash, d.LastSeen, d.Version, jsonStrings(d.Tags), d.ID,
	)
	return err
}

// UpdateDeviceStatus updates just the status and last_seen fields.
func UpdateDeviceStatus(db *sql.DB, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`UPDATE remote_devices SET status = ?, last_seen = ? WHERE id = ?`,
		status, now, id)
	return err
}

// DeleteDevice removes a device record.
func DeleteDevice(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM remote_devices WHERE id = ?`, id)
	return err
}

func scanDevice(row *sql.Row) (DeviceRecord, error) {
	var d DeviceRecord
	var readOnly int
	var allowedPaths, tags string
	var sharedKeyHash, enrollToken sql.NullString
	err := row.Scan(&d.ID, &d.Name, &d.Hostname, &d.OS, &d.Arch, &d.IPAddress,
		&d.Status, &readOnly, &allowedPaths, &sharedKeyHash, &enrollToken,
		&d.LastSeen, &d.CreatedAt, &d.Version, &tags)
	if err != nil {
		return d, err
	}
	d.ReadOnly = readOnly != 0
	d.AllowedPaths = parseStrings(allowedPaths)
	d.Tags = parseStrings(tags)
	if sharedKeyHash.Valid {
		d.SharedKeyHash = sharedKeyHash.String
	}
	if enrollToken.Valid {
		d.EnrollmentToken = enrollToken.String
	}
	return d, nil
}

func scanDevices(rows *sql.Rows) ([]DeviceRecord, error) {
	var result []DeviceRecord
	for rows.Next() {
		var d DeviceRecord
		var readOnly int
		var allowedPaths, tags string
		var sharedKeyHash, enrollToken sql.NullString
		err := rows.Scan(&d.ID, &d.Name, &d.Hostname, &d.OS, &d.Arch, &d.IPAddress,
			&d.Status, &readOnly, &allowedPaths, &sharedKeyHash, &enrollToken,
			&d.LastSeen, &d.CreatedAt, &d.Version, &tags)
		if err != nil {
			return nil, err
		}
		d.ReadOnly = readOnly != 0
		d.AllowedPaths = parseStrings(allowedPaths)
		d.Tags = parseStrings(tags)
		if sharedKeyHash.Valid {
			d.SharedKeyHash = sharedKeyHash.String
		}
		if enrollToken.Valid {
			d.EnrollmentToken = enrollToken.String
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ── Enrollments CRUD ────────────────────────────────────────────────────────

// CreateEnrollment stores a new enrollment token record.
func CreateEnrollment(db *sql.DB, e EnrollmentRecord) (string, error) {
	e.ID = uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO remote_enrollments (id, token_hash, device_name, created_at, expires_at, used, used_by_device)
		VALUES (?, ?, ?, ?, ?, 0, '')`,
		e.ID, e.TokenHash, e.DeviceName, now, e.ExpiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create enrollment: %w", err)
	}
	return e.ID, nil
}

// GetEnrollmentByTokenHash finds an enrollment by the SHA-256 hash of the raw token.
func GetEnrollmentByTokenHash(db *sql.DB, tokenHash string) (EnrollmentRecord, error) {
	var e EnrollmentRecord
	var used int
	err := db.QueryRow(`SELECT id, token_hash, device_name, created_at, expires_at, used, used_by_device
		FROM remote_enrollments WHERE token_hash = ?`, tokenHash).
		Scan(&e.ID, &e.TokenHash, &e.DeviceName, &e.CreatedAt, &e.ExpiresAt, &used, &e.UsedByDevice)
	if err != nil {
		return e, err
	}
	e.Used = used != 0
	return e, nil
}

// MarkEnrollmentUsed marks an enrollment as consumed by a device.
func MarkEnrollmentUsed(db *sql.DB, enrollmentID, deviceID string) error {
	_, err := db.Exec(`UPDATE remote_enrollments SET used = 1, used_by_device = ? WHERE id = ?`,
		deviceID, enrollmentID)
	return err
}

// CleanExpiredEnrollments removes enrollments that have expired.
func CleanExpiredEnrollments(db *sql.DB) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`DELETE FROM remote_enrollments WHERE expires_at < ? AND used = 0`, now)
	return err
}

// ── Audit log ───────────────────────────────────────────────────────────────

// LogAudit records a remote operation.
func LogAudit(db *sql.DB, deviceID, operation, command, status string, durationMs int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO remote_audit_log (device_id, timestamp, operation, command, status, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?)`,
		deviceID, now, operation, command, status, durationMs)
	return err
}

// ListAuditLog returns the most recent audit entries for a device (or all if deviceID is empty).
func ListAuditLog(db *sql.DB, deviceID string, limit int) ([]AuditEntry, error) {
	var rows *sql.Rows
	var err error
	if deviceID != "" {
		rows, err = db.Query(`SELECT id, device_id, timestamp, operation, command, status, duration_ms
			FROM remote_audit_log WHERE device_id = ? ORDER BY timestamp DESC LIMIT ?`, deviceID, limit)
	} else {
		rows, err = db.Query(`SELECT id, device_id, timestamp, operation, command, status, duration_ms
			FROM remote_audit_log ORDER BY timestamp DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list audit log: %w", err)
	}
	defer rows.Close()

	var result []AuditEntry
	for rows.Next() {
		var a AuditEntry
		if err := rows.Scan(&a.ID, &a.DeviceID, &a.Timestamp, &a.Operation, &a.Command, &a.Status, &a.DurationMs); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

// TrimAuditLog deletes old entries, keeping only the most recent maxRows rows.
// Call this at startup to prevent unbounded table growth.
func TrimAuditLog(db *sql.DB, maxRows int) error {
	_, err := db.Exec(`
		DELETE FROM remote_audit_log
		WHERE id NOT IN (
			SELECT id FROM remote_audit_log ORDER BY id DESC LIMIT ?
		)`, maxRows)
	return err
}
