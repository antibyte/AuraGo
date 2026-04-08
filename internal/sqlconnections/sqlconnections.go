package sqlconnections

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// ConnectionRecord stores metadata about a user-configured database connection.
// Actual credentials (username/password) are stored in the vault, referenced by VaultSecretID.
type ConnectionRecord struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Driver        string `json:"driver"`        // "postgres", "mysql", "sqlite"
	Host          string `json:"host"`          // hostname or IP; empty for sqlite
	Port          int    `json:"port"`          // 0 = driver default
	DatabaseName  string `json:"database_name"` // database/schema name or file path for sqlite
	Description   string `json:"description"`   // human-readable purpose
	AllowRead     bool   `json:"allow_read"`    // SELECT
	AllowWrite    bool   `json:"allow_write"`   // INSERT
	AllowChange   bool   `json:"allow_change"`  // UPDATE
	AllowDelete   bool   `json:"allow_delete"`  // DELETE
	VaultSecretID string `json:"vault_secret_id"`
	SSLMode       string `json:"ssl_mode"`   // "disable", "require", "verify-ca", "verify-full"
	CreatedAt     string `json:"created_at"` // RFC3339
	UpdatedAt     string `json:"updated_at"` // RFC3339
}

// connectionCredentials is stored as JSON in the vault.
type connectionCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// InitDB opens (or creates) the metadata database and ensures the schema exists.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("sql_connections: failed to open database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS sql_connections (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL UNIQUE,
		driver          TEXT NOT NULL,
		host            TEXT NOT NULL DEFAULT '',
		port            INTEGER NOT NULL DEFAULT 0,
		database_name   TEXT NOT NULL DEFAULT '',
		description     TEXT NOT NULL DEFAULT '',
		allow_read      INTEGER NOT NULL DEFAULT 1,
		allow_write     INTEGER NOT NULL DEFAULT 0,
		allow_change    INTEGER NOT NULL DEFAULT 0,
		allow_delete    INTEGER NOT NULL DEFAULT 0,
		vault_secret_id TEXT NOT NULL DEFAULT '',
		ssl_mode        TEXT NOT NULL DEFAULT 'disable',
		created_at      TEXT NOT NULL,
		updated_at      TEXT NOT NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sql_connections: failed to create schema: %w", err)
	}

	return db, nil
}

// Create inserts a new connection record and returns its UUID.
func Create(db *sql.DB, name, driver, host string, port int, databaseName, description string,
	allowRead, allowWrite, allowChange, allowDelete bool,
	vaultSecretID, sslMode string) (string, error) {

	if name == "" {
		return "", fmt.Errorf("connection name is required")
	}
	if driver != "postgres" && driver != "mysql" && driver != "sqlite" {
		return "", fmt.Errorf("unsupported driver: %s (must be postgres, mysql, or sqlite)", driver)
	}

	id := uid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(`INSERT INTO sql_connections
		(id, name, driver, host, port, database_name, description,
		 allow_read, allow_write, allow_change, allow_delete,
		 vault_secret_id, ssl_mode, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, driver, host, port, databaseName, description,
		boolToInt(allowRead), boolToInt(allowWrite), boolToInt(allowChange), boolToInt(allowDelete),
		vaultSecretID, sslMode, now, now)
	if err != nil {
		return "", fmt.Errorf("failed to create connection: %w", err)
	}
	return id, nil
}

// GetByID returns a single connection by its UUID.
func GetByID(db *sql.DB, id string) (ConnectionRecord, error) {
	var c ConnectionRecord
	var ar, aw, ac, ad int
	err := db.QueryRow(`SELECT id, name, driver, host, port, database_name, description,
		allow_read, allow_write, allow_change, allow_delete,
		vault_secret_id, ssl_mode, created_at, updated_at
		FROM sql_connections WHERE id = ?`, id).Scan(
		&c.ID, &c.Name, &c.Driver, &c.Host, &c.Port, &c.DatabaseName, &c.Description,
		&ar, &aw, &ac, &ad,
		&c.VaultSecretID, &c.SSLMode, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return c, fmt.Errorf("connection not found: %s", id)
		}
		return c, fmt.Errorf("failed to get connection: %w", err)
	}
	c.AllowRead = ar == 1
	c.AllowWrite = aw == 1
	c.AllowChange = ac == 1
	c.AllowDelete = ad == 1
	return c, nil
}

// GetByName returns a single connection by its unique name.
func GetByName(db *sql.DB, name string) (ConnectionRecord, error) {
	var c ConnectionRecord
	var ar, aw, ac, ad int
	err := db.QueryRow(`SELECT id, name, driver, host, port, database_name, description,
		allow_read, allow_write, allow_change, allow_delete,
		vault_secret_id, ssl_mode, created_at, updated_at
		FROM sql_connections WHERE name = ?`, name).Scan(
		&c.ID, &c.Name, &c.Driver, &c.Host, &c.Port, &c.DatabaseName, &c.Description,
		&ar, &aw, &ac, &ad,
		&c.VaultSecretID, &c.SSLMode, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return c, fmt.Errorf("connection not found: %s", name)
		}
		return c, fmt.Errorf("failed to get connection: %w", err)
	}
	c.AllowRead = ar == 1
	c.AllowWrite = aw == 1
	c.AllowChange = ac == 1
	c.AllowDelete = ad == 1
	return c, nil
}

// List returns all connection records.
func List(db *sql.DB) ([]ConnectionRecord, error) {
	rows, err := db.Query(`SELECT id, name, driver, host, port, database_name, description,
		allow_read, allow_write, allow_change, allow_delete,
		vault_secret_id, ssl_mode, created_at, updated_at
		FROM sql_connections ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list connections: %w", err)
	}
	defer rows.Close()

	var result []ConnectionRecord
	for rows.Next() {
		var c ConnectionRecord
		var ar, aw, ac, ad int
		if err := rows.Scan(&c.ID, &c.Name, &c.Driver, &c.Host, &c.Port, &c.DatabaseName, &c.Description,
			&ar, &aw, &ac, &ad,
			&c.VaultSecretID, &c.SSLMode, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan connection row: %w", err)
		}
		c.AllowRead = ar == 1
		c.AllowWrite = aw == 1
		c.AllowChange = ac == 1
		c.AllowDelete = ad == 1
		result = append(result, c)
	}
	return result, nil
}

// Update modifies an existing connection record.
func Update(db *sql.DB, id, name, driver, host string, port int, databaseName, description string,
	allowRead, allowWrite, allowChange, allowDelete bool,
	vaultSecretID, sslMode string) error {

	if name == "" {
		return fmt.Errorf("connection name is required")
	}
	if driver != "postgres" && driver != "mysql" && driver != "sqlite" {
		return fmt.Errorf("unsupported driver: %s", driver)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(`UPDATE sql_connections SET
		name=?, driver=?, host=?, port=?, database_name=?, description=?,
		allow_read=?, allow_write=?, allow_change=?, allow_delete=?,
		vault_secret_id=?, ssl_mode=?, updated_at=?
		WHERE id=?`,
		name, driver, host, port, databaseName, description,
		boolToInt(allowRead), boolToInt(allowWrite), boolToInt(allowChange), boolToInt(allowDelete),
		vaultSecretID, sslMode, now, id)
	if err != nil {
		return fmt.Errorf("failed to update connection: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", id)
	}
	return nil
}

// Delete removes a connection record by ID.
func Delete(db *sql.DB, id string) error {
	res, err := db.Exec(`DELETE FROM sql_connections WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("connection not found: %s", id)
	}
	return nil
}

// MarshalCredentials serializes username/password to JSON for vault storage.
func MarshalCredentials(username, password string) (string, error) {
	b, err := json.Marshal(connectionCredentials{Username: username, Password: password})
	if err != nil {
		return "", fmt.Errorf("failed to marshal credentials: %w", err)
	}
	return string(b), nil
}

// UnmarshalCredentials deserializes vault-stored JSON credentials.
func UnmarshalCredentials(data string) (username, password string, err error) {
	var creds connectionCredentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal credentials: %w", err)
	}
	return creds.Username, creds.Password, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	if db != nil {
		return db.Close()
	}
	return nil
}
