package flows

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	// PurgeBackupPrefix names the encrypted safety backup the purge takes first, and
	// PreRestoreBackupPrefix the encrypted copy the restore flag takes of the current database.
	PurgeBackupPrefix      = ".flows-purge-backup-"
	PreRestoreBackupPrefix = ".flows-pre-restore-"
	PurgeBackupSuffix      = ".enc"
	// PurgeBackupMaxAge is how long an encrypted backup lives (owner decision 2).
	PurgeBackupMaxAge = 7 * 24 * time.Hour
	// PurgeBackupLabel is the HKDF label of both kinds of backup.
	PurgeBackupLabel  = "aurago/easydrag/purge-backup/v1"
	backupTimeLayout  = "20060102T150405.000Z"
	purgeTempPrefix   = PurgeBackupPrefix + "tmp-"
	preRestoreTempExt = ".pre-restore.tmp"
)

func isEncryptedBackupName(name string) bool {
	return strings.HasSuffix(name, PurgeBackupSuffix) &&
		(strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix))
}

// SweepPurgeBackups deletes the encrypted purge and pre-restore backups in dir that are older than
// PurgeBackupMaxAge, by modification time. It returns the deleted names.
func SweepPurgeBackups(dir string, now time.Time) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var deleted []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !isEncryptedBackupName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= PurgeBackupMaxAge {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return deleted, err
		}
		deleted = append(deleted, entry.Name())
	}
	return deleted, nil
}

// dbPath is the file of the open database ("" for an in-memory one).
func (s *Store) dbPath() (string, error) {
	var path string
	if err := s.db.QueryRow(`SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		return "", err
	}
	return path, nil
}

// dir is the directory of flows.db, where the snapshots and the purge backups live ("" in memory).
func (s *Store) dir() (string, error) {
	path, err := s.dbPath()
	if err != nil || path == "" {
		return "", err
	}
	return filepath.Dir(path), nil
}

// RestorePurgeBackup replaces the flows database at dbPath by the decrypted purge backup. It is
// meant for the --restore-flows-purge-backup flag and must run while AuraGo is stopped. It first
// keeps the current database as an encrypted .flows-pre-restore file, verifies the decrypted backup
// opens and passes integrity_check, and removes the stale WAL and SHM before the swap.
func RestorePurgeBackup(dbPath, backupFile, masterKeyHex string, now time.Time) error {
	if info, err := os.Lstat(backupFile); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("the backup file %q cannot be read", backupFile)
	}
	if _, err := os.Stat(dbPath); err == nil {
		plain := dbPath + preRestoreTempExt
		_ = os.Remove(plain)
		current, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return fmt.Errorf("open the current database: %w", err)
		}
		_, err = current.Exec(`VACUUM INTO ?`, plain)
		_ = current.Close()
		if err != nil {
			return fmt.Errorf("copy the current database: %w", err)
		}
		defer os.Remove(plain)
		pre := filepath.Join(filepath.Dir(dbPath), PreRestoreBackupPrefix+now.UTC().Format(backupTimeLayout)+PurgeBackupSuffix)
		if err := security.EncryptFileWithMasterKey(plain, pre, PurgeBackupLabel, masterKeyHex); err != nil {
			return fmt.Errorf("keep the current database: %w", err)
		}
	}
	restored := dbPath + ".restore.tmp"
	defer func() {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(restored + suffix)
		}
	}()
	if err := security.DecryptFileWithMasterKey(backupFile, restored, PurgeBackupLabel, masterKeyHex); err != nil {
		return err
	}
	check, err := sql.Open("sqlite", restored)
	if err != nil {
		return err
	}
	var integrity, version string
	err = check.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
	if err == nil {
		err = check.QueryRow(`SELECT value FROM flows_meta WHERE key='schema_version'`).Scan(&version)
	}
	_ = check.Close()
	if err != nil || integrity != "ok" {
		return fmt.Errorf("the decrypted backup is not a usable flows database (%q, %v)", integrity, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Rename(restored, dbPath)
}
