package inventory

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EnsureRDPCertPinsTable creates the trust-on-first-use table for RDP server
// certificates. It is idempotent and cheap, so callers may invoke it lazily.
func EnsureRDPCertPinsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS rdp_cert_pins (
		device_id TEXT PRIMARY KEY,
		sha256 TEXT NOT NULL,
		first_seen TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create rdp_cert_pins: %w", err)
	}
	return nil
}

// GetRDPCertPin returns the pinned SHA-256 (lower-case hex) of the device's RDP
// server leaf certificate. found is false when the device has no pin yet.
func GetRDPCertPin(db *sql.DB, deviceID string) (sha256 string, found bool, err error) {
	if err := EnsureRDPCertPinsTable(db); err != nil {
		return "", false, err
	}
	err = db.QueryRow(`SELECT sha256 FROM rdp_cert_pins WHERE device_id = ?`, strings.TrimSpace(deviceID)).Scan(&sha256)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read rdp cert pin: %w", err)
	}
	return sha256, true, nil
}

// SetRDPCertPin stores the pin for a device unless one already exists.
func SetRDPCertPin(db *sql.DB, deviceID, sha256 string) error {
	if err := EnsureRDPCertPinsTable(db); err != nil {
		return err
	}
	_, err := db.Exec(`INSERT OR IGNORE INTO rdp_cert_pins (device_id, sha256, first_seen) VALUES (?, ?, ?)`,
		strings.TrimSpace(deviceID), strings.ToLower(strings.TrimSpace(sha256)), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store rdp cert pin: %w", err)
	}
	return nil
}

// DeleteRDPCertPin removes the pin of a device (no error when there is none).
func DeleteRDPCertPin(db *sql.DB, deviceID string) error {
	if err := EnsureRDPCertPinsTable(db); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM rdp_cert_pins WHERE device_id = ?`, strings.TrimSpace(deviceID)); err != nil {
		return fmt.Errorf("delete rdp cert pin: %w", err)
	}
	return nil
}
