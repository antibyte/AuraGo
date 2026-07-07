package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// DeviceRecord represents a generic network device in the registry.
type DeviceRecord struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	Protocol      string   `json:"protocol"`
	IPAddress     string   `json:"ip_address"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	VaultSecretID string   `json:"vault_secret_id"`
	CredentialID  string   `json:"credential_id"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	MACAddress    string   `json:"mac_address,omitempty"` // Optional – required for Wake-on-LAN
}

const (
	ProtocolSSH  = "ssh"
	ProtocolVNC  = "vnc"
	ProtocolNone = "none"
)

// InitDB initializes the SQLite database and handles schema migrations.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create new devices table
	schema := `
	CREATE TABLE IF NOT EXISTS devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		ip_address TEXT,
		port INTEGER NOT NULL,
		username TEXT,
		vault_secret_id TEXT,
		credential_id TEXT,
		description TEXT,
		tags TEXT,
		mac_address TEXT
	);`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create devices schema: %w", err)
	}

	// Migrate: add mac_address column to existing databases that don't have it yet.
	var hasMACCol bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('devices') WHERE name='mac_address'").Scan(&hasMACCol)
	if !hasMACCol {
		if _, err := db.Exec("ALTER TABLE devices ADD COLUMN mac_address TEXT"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to add mac_address column: %w", err)
		}
	}
	var hasCredentialCol bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('devices') WHERE name='credential_id'").Scan(&hasCredentialCol)
	if !hasCredentialCol {
		if _, err := db.Exec("ALTER TABLE devices ADD COLUMN credential_id TEXT"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to add credential_id column: %w", err)
		}
	}

	// Migrate: add protocol column to existing databases that don't have it yet.
	var hasProtocolCol bool
	_ = db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('devices') WHERE name='protocol'").Scan(&hasProtocolCol)
	if !hasProtocolCol {
		if _, err := db.Exec("ALTER TABLE devices ADD COLUMN protocol TEXT NOT NULL DEFAULT 'ssh'"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to add protocol column: %w", err)
		}
	}
	if err := backfillLegacyDeviceRows(db); err != nil {
		db.Close()
		return nil, err
	}

	// Check and set schema version - only migrate legacy servers table if version is below 1.
	const inventorySchemaVersion = 4
	currentVer, err := dbutil.GetUserVersion(db)
	if err != nil {
		currentVer = 0
	}

	if err := ensureInventoryIndexes(db); err != nil {
		db.Close()
		return nil, err
	}

	if currentVer < 1 {
		// Check for legacy servers table and migrate data
		var hasServers bool
		err = db.QueryRow("SELECT count(*) > 0 FROM sqlite_master WHERE type='table' AND name='servers'").Scan(&hasServers)
		if err == nil && hasServers {
			// Ensure old table has ip_address before copying to prevent errors if it was from an older version
			var hasIP bool
			db.QueryRow("SELECT count(*) > 0 FROM pragma_table_info('servers') WHERE name='ip_address'").Scan(&hasIP)

			if hasIP {
				migrationQuery := `
			INSERT INTO devices (id, name, type, ip_address, port, username, vault_secret_id, description, tags)
			SELECT id, hostname, 'server', ip_address, port, username, vault_secret_id, '', tags FROM servers;
			`
				if _, err := db.Exec(migrationQuery); err != nil {
					return nil, fmt.Errorf("failed to migrate servers to devices: %w", err)
				}
			} else {
				migrationQuery := `
			INSERT INTO devices (id, name, type, ip_address, port, username, vault_secret_id, description, tags)
			SELECT id, hostname, 'server', '', port, username, vault_secret_id, '', tags FROM servers;
			`
				if _, err := db.Exec(migrationQuery); err != nil {
					return nil, fmt.Errorf("failed to migrate servers to devices: %w", err)
				}
			}

			// Drop the old servers table
			if _, err := db.Exec("DROP TABLE servers"); err != nil {
				return nil, fmt.Errorf("failed to drop legacy servers table: %w", err)
			}
		}
	}
	if err := backfillLegacyDeviceRows(db); err != nil {
		db.Close()
		return nil, err
	}

	// Set user_version so backup/restore can detect schema generation.
	if currentVer < inventorySchemaVersion {
		if err := dbutil.SetUserVersion(db, inventorySchemaVersion); err != nil {
			return nil, fmt.Errorf("failed to set inventory schema version: %w", err)
		}
	}

	return db, nil
}

func backfillLegacyDeviceRows(db *sql.DB) error {
	statements := []string{
		`UPDATE devices SET tags = '[]' WHERE tags IS NULL OR trim(tags) = ''`,
		`UPDATE devices SET ip_address = '' WHERE ip_address IS NULL`,
		`UPDATE devices SET username = '' WHERE username IS NULL`,
		`UPDATE devices SET vault_secret_id = '' WHERE vault_secret_id IS NULL`,
		`UPDATE devices SET credential_id = '' WHERE credential_id IS NULL`,
		`UPDATE devices SET description = '' WHERE description IS NULL`,
		`UPDATE devices SET mac_address = '' WHERE mac_address IS NULL`,
		`UPDATE devices SET protocol = 'ssh' WHERE protocol IS NULL OR trim(protocol) = ''`,
		`UPDATE devices SET protocol = 'none' WHERE lower(type) = 'printer' AND (tags LIKE '%3d-printer%' OR description LIKE '%3D printer%')`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("backfill legacy inventory rows: %w", err)
		}
	}
	return nil
}

func ensureInventoryIndexes(db *sql.DB) error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_name_ci ON devices(lower(name))`,
	}
	for _, stmt := range indexes {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create inventory index: %w", err)
		}
	}
	return nil
}

// CreateDevice generates a new UUID and adds a device record to the database.
func CreateDevice(db *sql.DB, name, deviceType, protocol, ipAddress string, port int, username, vaultSecretID, credentialID, description string, tags []string, macAddress string) (string, error) {
	id := uid.New()
	d := DeviceRecord{
		ID:            id,
		Name:          name,
		Type:          deviceType,
		Protocol:      protocol,
		IPAddress:     ipAddress,
		Port:          port,
		Username:      username,
		VaultSecretID: vaultSecretID,
		CredentialID:  credentialID,
		Description:   description,
		Tags:          tags,
		MACAddress:    macAddress,
	}

	if err := AddDevice(db, d); err != nil {
		return "", err
	}

	return id, nil
}

// AddDevice adds a new device record to the database.
func AddDevice(db *sql.DB, d DeviceRecord) error {
	tagsJSON, err := json.Marshal(d.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	protocol := d.Protocol
	if protocol == "" {
		protocol = ProtocolSSH
	}

	query := `INSERT INTO devices (id, name, type, protocol, ip_address, port, username, vault_secret_id, credential_id, description, tags, mac_address) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(query, d.ID, d.Name, d.Type, protocol, d.IPAddress, d.Port, d.Username, d.VaultSecretID, d.CredentialID, d.Description, string(tagsJSON), d.MACAddress)
	if err != nil {
		return fmt.Errorf("failed to add device: %w", err)
	}

	return nil
}

// UpdateDevice updates an existing device record in the database.
func UpdateDevice(db *sql.DB, d DeviceRecord) error {
	tagsJSON, err := json.Marshal(d.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	protocol := d.Protocol
	if protocol == "" {
		protocol = ProtocolSSH
	}

	query := `UPDATE devices SET name=?, type=?, protocol=?, ip_address=?, port=?, username=?, vault_secret_id=?, credential_id=?, description=?, tags=?, mac_address=? WHERE id=?`
	res, err := db.Exec(query, d.Name, d.Type, protocol, d.IPAddress, d.Port, d.Username, d.VaultSecretID, d.CredentialID, d.Description, string(tagsJSON), d.MACAddress, d.ID)
	if err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found: %s", d.ID)
	}
	return nil
}

// UpsertDeviceByName inserts a new device or, if a device with the same name
// already exists (case-insensitive), optionally updates it.
// Returns (created, updated, error). When overwrite is false and the device
// already exists, both booleans are false (the entry is silently skipped).
func UpsertDeviceByName(db *sql.DB, d DeviceRecord, overwrite bool) (created bool, updated bool, err error) {
	var existingID string
	err = db.QueryRow(`SELECT id FROM devices WHERE lower(name) = lower(?)`, d.Name).Scan(&existingID)
	if err == sql.ErrNoRows {
		d.ID = uid.New()
		if insertErr := AddDevice(db, d); insertErr != nil {
			return false, false, insertErr
		}
		return true, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("failed to check existing device: %w", err)
	}
	// Device exists.
	if !overwrite {
		return false, false, nil
	}
	d.ID = existingID
	if updateErr := UpdateDevice(db, d); updateErr != nil {
		return false, false, updateErr
	}
	return false, true, nil
}

// DeleteDevice removes a device record from the database by its ID.
func DeleteDevice(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM devices WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("device not found: %s", id)
	}
	return nil
}

// ListAllDevices returns all device records in the database.
func ListAllDevices(db *sql.DB) ([]DeviceRecord, error) {
	rows, err := db.Query(deviceSelectColumns(`FROM devices`))
	if err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}
	defer rows.Close()
	return scanDevices(rows)
}

// GetDeviceByIDOrName retrieves a device by UUID first; if not found, falls back
// to an exact (case-insensitive) name lookup. This allows callers to pass either
// the registered UUID or the human-readable device name interchangeably.
func GetDeviceByIDOrName(db *sql.DB, idOrName string) (DeviceRecord, error) {
	d, err := GetDeviceByID(db, idOrName)
	if err == nil {
		return d, nil
	}
	devices, qErr := QueryDevices(db, "", "", idOrName)
	if qErr != nil {
		return DeviceRecord{}, err // return original ID-lookup error
	}
	lower := strings.ToLower(idOrName)
	for _, dev := range devices {
		if strings.ToLower(dev.Name) == lower {
			return dev, nil
		}
	}
	return DeviceRecord{}, fmt.Errorf("device not found: %s", idOrName)
}

// GetDeviceByID retrieves a device record by its ID.
func GetDeviceByID(db *sql.DB, id string) (DeviceRecord, error) {
	var d DeviceRecord
	var tagsJSON string

	query := deviceSelectColumns(`FROM devices WHERE id = ?`)
	err := db.QueryRow(query, id).Scan(&d.ID, &d.Name, &d.Type, &d.Protocol, &d.IPAddress, &d.Port, &d.Username, &d.VaultSecretID, &d.CredentialID, &d.Description, &tagsJSON, &d.MACAddress)
	if err != nil {
		if err == sql.ErrNoRows {
			return DeviceRecord{}, fmt.Errorf("device not found: %s", id)
		}
		return DeviceRecord{}, fmt.Errorf("failed to get device: %w", err)
	}
	d.Tags = parseDeviceTags(tagsJSON)

	return d, nil
}

// ListDevicesByTag returns all devices that have the specified tag.
func ListDevicesByTag(db *sql.DB, tag string) ([]DeviceRecord, error) {
	query := `
	SELECT d.id, d.name, d.type, COALESCE(NULLIF(d.protocol,''),'ssh'), COALESCE(d.ip_address,''), COALESCE(d.port,22), COALESCE(d.username,''), COALESCE(d.vault_secret_id,''), COALESCE(d.credential_id,''), COALESCE(d.description,''), COALESCE(NULLIF(d.tags,''),'[]'), COALESCE(d.mac_address,'')
	FROM devices d, json_each(CASE WHEN json_valid(COALESCE(d.tags,'')) THEN d.tags ELSE '[]' END) as t
	WHERE t.value = ?`

	rows, err := db.Query(query, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices by tag: %w", err)
	}
	defer rows.Close()

	return scanDevices(rows)
}

// QueryDevices returns devices matching the optional tag, type, and/or name.
func QueryDevices(db *sql.DB, tag, deviceType, name string) ([]DeviceRecord, error) {
	query := `SELECT d.id, d.name, d.type, COALESCE(NULLIF(d.protocol,''),'ssh'), COALESCE(d.ip_address,''), COALESCE(d.port,22), COALESCE(d.username,''), COALESCE(d.vault_secret_id,''), COALESCE(d.credential_id,''), COALESCE(d.description,''), COALESCE(NULLIF(d.tags,''),'[]'), COALESCE(d.mac_address,'') FROM devices d`
	var conditions []string
	var args []interface{}

	if tag != "" {
		query += ", json_each(CASE WHEN json_valid(COALESCE(d.tags,'')) THEN d.tags ELSE '[]' END) as t"
		conditions = append(conditions, "t.value = ?")
		args = append(args, tag)
	}

	if deviceType != "" {
		conditions = append(conditions, "d.type = ?")
		args = append(args, deviceType)
	}

	if name != "" {
		conditions = append(conditions, "d.name LIKE ? ESCAPE '\\'")
		args = append(args, "%"+dbutil.EscapeLike(name)+"%")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	return scanDevices(rows)
}

func scanDevices(rows *sql.Rows) ([]DeviceRecord, error) {
	var devices []DeviceRecord
	for rows.Next() {
		var d DeviceRecord
		var tagsJSON string
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.Protocol, &d.IPAddress, &d.Port, &d.Username, &d.VaultSecretID, &d.CredentialID, &d.Description, &tagsJSON, &d.MACAddress); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}
		d.Tags = parseDeviceTags(tagsJSON)
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return devices, nil
}

func deviceSelectColumns(fromClause string) string {
	return `SELECT id, name, type, COALESCE(NULLIF(protocol,''),'ssh'), COALESCE(ip_address,''), COALESCE(port,22), COALESCE(username,''), COALESCE(vault_secret_id,''), COALESCE(credential_id,''), COALESCE(description,''), COALESCE(NULLIF(tags,''),'[]'), COALESCE(mac_address,'') ` + fromClause
}

func parseDeviceTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err == nil {
		if tags == nil {
			return []string{}
		}
		return tags
	}
	if strings.Contains(raw, ",") && !strings.HasPrefix(raw, "[") {
		parts := strings.Split(raw, ",")
		tags = make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
		return tags
	}
	return []string{}
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
