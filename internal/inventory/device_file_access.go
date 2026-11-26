package inventory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// File access modes for RDP devices.
const (
	FileAccessOff  = "off"
	FileAccessAuto = "auto"
	FileAccessSFTP = "sftp"
	FileAccessSMB  = "smb"

	DefaultFileAccessSFTPPort = 22
	DefaultFileAccessSMBShare = "C$"
)

// FileAccess is the per-device file access setting kept in a side table so the
// device record itself (and its positional constructors) stay unchanged.
type FileAccess struct {
	Mode         string `json:"mode"`
	SFTPPort     int    `json:"sftp_port"`
	SMBShare     string `json:"smb_share"`
	CredentialID string `json:"credential_id"`
}

// DefaultFileAccess returns the setting of a device without a stored row: off.
func DefaultFileAccess() FileAccess {
	return FileAccess{Mode: FileAccessOff, SFTPPort: DefaultFileAccessSFTPPort, SMBShare: DefaultFileAccessSMBShare}
}

// NormalizeFileAccess trims and validates a file access setting. Zero values
// for port and share are replaced by the defaults; an empty mode means off.
func NormalizeFileAccess(fa FileAccess) (FileAccess, error) {
	fa.Mode = strings.ToLower(strings.TrimSpace(fa.Mode))
	if fa.Mode == "" {
		fa.Mode = FileAccessOff
	}
	switch fa.Mode {
	case FileAccessOff, FileAccessAuto, FileAccessSFTP, FileAccessSMB:
	default:
		return FileAccess{}, fmt.Errorf("invalid file access mode %q", fa.Mode)
	}
	if fa.SFTPPort == 0 {
		fa.SFTPPort = DefaultFileAccessSFTPPort
	}
	if fa.SFTPPort < 1 || fa.SFTPPort > 65535 {
		return FileAccess{}, errors.New("sftp_port must be between 1 and 65535")
	}
	fa.SMBShare = strings.TrimSpace(fa.SMBShare)
	if fa.SMBShare == "" {
		fa.SMBShare = DefaultFileAccessSMBShare
	}
	if len(fa.SMBShare) > 80 {
		return FileAccess{}, errors.New("smb_share must be 1 to 80 characters")
	}
	if strings.ContainsAny(fa.SMBShare, "/\\\x00\r\n") {
		return FileAccess{}, errors.New("smb_share contains invalid characters")
	}
	if strings.EqualFold(fa.SMBShare, "IPC$") {
		return FileAccess{}, errors.New("smb_share IPC$ is not allowed")
	}
	fa.CredentialID = strings.TrimSpace(fa.CredentialID)
	return fa, nil
}

// EnsureDeviceFileAccessTable creates the side table. It is idempotent.
func EnsureDeviceFileAccessTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS device_file_access (
		device_id TEXT PRIMARY KEY,
		mode TEXT NOT NULL DEFAULT 'off',
		sftp_port INTEGER NOT NULL DEFAULT 22,
		smb_share TEXT NOT NULL DEFAULT 'C$',
		credential_id TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create device_file_access: %w", err)
	}
	return nil
}

// GetDeviceFileAccess returns the stored setting; a missing row means off.
func GetDeviceFileAccess(db *sql.DB, deviceID string) (FileAccess, error) {
	if err := EnsureDeviceFileAccessTable(db); err != nil {
		return DefaultFileAccess(), err
	}
	var fa FileAccess
	err := db.QueryRow(`SELECT mode, sftp_port, smb_share, credential_id FROM device_file_access WHERE device_id = ?`,
		strings.TrimSpace(deviceID)).Scan(&fa.Mode, &fa.SFTPPort, &fa.SMBShare, &fa.CredentialID)
	if err == sql.ErrNoRows {
		return DefaultFileAccess(), nil
	}
	if err != nil {
		return DefaultFileAccess(), fmt.Errorf("read device file access: %w", err)
	}
	return fa, nil
}

// SetDeviceFileAccess validates and upserts the setting of a device.
func SetDeviceFileAccess(db *sql.DB, deviceID string, fa FileAccess) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("device id is required")
	}
	fa, err := NormalizeFileAccess(fa)
	if err != nil {
		return err
	}
	if err := EnsureDeviceFileAccessTable(db); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO device_file_access (device_id, mode, sftp_port, smb_share, credential_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET mode=excluded.mode, sftp_port=excluded.sftp_port,
			smb_share=excluded.smb_share, credential_id=excluded.credential_id, updated_at=excluded.updated_at`,
		deviceID, fa.Mode, fa.SFTPPort, fa.SMBShare, fa.CredentialID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("store device file access: %w", err)
	}
	return nil
}

// DeleteDeviceFileAccess removes the setting of a device (no error when absent).
func DeleteDeviceFileAccess(db *sql.DB, deviceID string) error {
	if err := EnsureDeviceFileAccessTable(db); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM device_file_access WHERE device_id = ?`, strings.TrimSpace(deviceID)); err != nil {
		return fmt.Errorf("delete device file access: %w", err)
	}
	return nil
}

// ListDeviceFileAccess returns all stored settings keyed by device id.
func ListDeviceFileAccess(db *sql.DB) (map[string]FileAccess, error) {
	if err := EnsureDeviceFileAccessTable(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT device_id, mode, sftp_port, smb_share, credential_id FROM device_file_access`)
	if err != nil {
		return nil, fmt.Errorf("list device file access: %w", err)
	}
	defer rows.Close()
	out := map[string]FileAccess{}
	for rows.Next() {
		var id string
		var fa FileAccess
		if err := rows.Scan(&id, &fa.Mode, &fa.SFTPPort, &fa.SMBShare, &fa.CredentialID); err != nil {
			return nil, fmt.Errorf("scan device file access: %w", err)
		}
		out[id] = fa
	}
	return out, rows.Err()
}
