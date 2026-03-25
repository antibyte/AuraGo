package inventory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

// DeviceRecord represents a generic network device in the registry.
type DeviceRecord struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	IPAddress     string   `json:"ip_address"`
	Port          int      `json:"port"`
	Username      string   `json:"username"`
	VaultSecretID string   `json:"vault_secret_id"`
	CredentialID  string   `json:"credential_id"`
	Description   string   `json:"description"`
	Tags          []string `json:"tags"`
	MACAddress    string   `json:"mac_address,omitempty"` // Optional – required for Wake-on-LAN
}

// InitDB initializes the SQLite database and handles schema migrations.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
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

	// Set user_version so backup/restore can detect schema generation.
	const inventorySchemaVersion = 2
	var currentVer int
	_ = db.QueryRow("PRAGMA user_version").Scan(&currentVer)
	if currentVer != inventorySchemaVersion {
		db.Exec(fmt.Sprintf("PRAGMA user_version = %d", inventorySchemaVersion))
	}

	return db, nil
}

// CreateDevice generates a new UUID and adds a device record to the database.
func CreateDevice(db *sql.DB, name, deviceType, ipAddress string, port int, username, vaultSecretID, credentialID, description string, tags []string, macAddress string) (string, error) {
	id := uuid.New().String()
	d := DeviceRecord{
		ID:            id,
		Name:          name,
		Type:          deviceType,
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

	query := `INSERT INTO devices (id, name, type, ip_address, port, username, vault_secret_id, credential_id, description, tags, mac_address) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = db.Exec(query, d.ID, d.Name, d.Type, d.IPAddress, d.Port, d.Username, d.VaultSecretID, d.CredentialID, d.Description, string(tagsJSON), d.MACAddress)
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

	query := `UPDATE devices SET name=?, type=?, ip_address=?, port=?, username=?, vault_secret_id=?, credential_id=?, description=?, tags=?, mac_address=? WHERE id=?`
	res, err := db.Exec(query, d.Name, d.Type, d.IPAddress, d.Port, d.Username, d.VaultSecretID, d.CredentialID, d.Description, string(tagsJSON), d.MACAddress, d.ID)
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
		d.ID = uuid.New().String()
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
	rows, err := db.Query(`SELECT id, name, type, ip_address, port, username, vault_secret_id, COALESCE(credential_id,''), description, tags, COALESCE(mac_address,'') FROM devices`)
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
	var ipNull sql.NullString
	var userNull sql.NullString
	var secretNull sql.NullString
	var descNull sql.NullString
	var macNull sql.NullString

	query := `SELECT id, name, type, ip_address, port, username, vault_secret_id, COALESCE(credential_id,''), description, tags, COALESCE(mac_address,'') FROM devices WHERE id = ?`
	err := db.QueryRow(query, id).Scan(&d.ID, &d.Name, &d.Type, &ipNull, &d.Port, &userNull, &secretNull, &d.CredentialID, &descNull, &tagsJSON, &macNull)
	if err != nil {
		if err == sql.ErrNoRows {
			return DeviceRecord{}, fmt.Errorf("device not found: %s", id)
		}
		return DeviceRecord{}, fmt.Errorf("failed to get device: %w", err)
	}
	if ipNull.Valid {
		d.IPAddress = ipNull.String
	}
	if userNull.Valid {
		d.Username = userNull.String
	}
	if secretNull.Valid {
		d.VaultSecretID = secretNull.String
	}
	if descNull.Valid {
		d.Description = descNull.String
	}
	if macNull.Valid {
		d.MACAddress = macNull.String
	}

	if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
		return DeviceRecord{}, fmt.Errorf("failed to unmarshal tags: %w", err)
	}

	return d, nil
}

// ListDevicesByTag returns all devices that have the specified tag.
func ListDevicesByTag(db *sql.DB, tag string) ([]DeviceRecord, error) {
	query := `
	SELECT d.id, d.name, d.type, d.ip_address, d.port, d.username, d.vault_secret_id, COALESCE(d.credential_id,''), d.description, d.tags, COALESCE(d.mac_address,'')
	FROM devices d, json_each(d.tags) as t
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
	query := `SELECT d.id, d.name, d.type, d.ip_address, d.port, d.username, d.vault_secret_id, COALESCE(d.credential_id,''), d.description, d.tags, COALESCE(d.mac_address,'') FROM devices d`
	var conditions []string
	var args []interface{}

	if tag != "" {
		query += ", json_each(d.tags) as t"
		conditions = append(conditions, "t.value = ?")
		args = append(args, tag)
	}

	if deviceType != "" {
		conditions = append(conditions, "d.type = ?")
		args = append(args, deviceType)
	}

	if name != "" {
		conditions = append(conditions, "d.name LIKE ?")
		args = append(args, "%"+name+"%")
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
		var ipNull sql.NullString
		var userNull sql.NullString
		var secretNull sql.NullString
		var descNull sql.NullString
		var macNull sql.NullString
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &ipNull, &d.Port, &userNull, &secretNull, &d.CredentialID, &descNull, &tagsJSON, &macNull); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}
		if ipNull.Valid {
			d.IPAddress = ipNull.String
		}
		if userNull.Valid {
			d.Username = userNull.String
		}
		if secretNull.Valid {
			d.VaultSecretID = secretNull.String
		}
		if descNull.Valid {
			d.Description = descNull.String
		}
		if macNull.Valid {
			d.MACAddress = macNull.String
		}
		if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return devices, nil
}
