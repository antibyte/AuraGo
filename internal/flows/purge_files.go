package flows

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/fileutil"
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
	PurgeBackupLabel = "aurago/easydrag/purge-backup/v1"
	backupTimeLayout = "20060102T150405.000Z"
	// purgeWorkPrefix names the 0700 directories that hold every plaintext temporary file.
	purgeWorkPrefix = security.PlaintextWorkDirPrefix
	// The old-style temporary names, only ever swept.
	purgeTempPrefix     = PurgeBackupPrefix + "tmp-"
	preRestoreTempExt   = ".pre-restore.tmp"
	restoreTempExt      = ".restore.tmp"
	purgeRecoveryMarker = "RECOVERY_REQUIRED.json"
)

type purgeRecoveryNotice struct {
	Backup string `json:"encrypted_backup"`
}

// newPurgeWorkDir creates a fresh directory (mode 0700) inside dir for plaintext temporary files.
func newPurgeWorkDir(dir string) (string, error) {
	work, err := os.MkdirTemp(dir, purgeWorkPrefix)
	if err != nil {
		return "", fmt.Errorf("create the work directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(work); err != nil {
		_ = os.RemoveAll(work)
		return "", fmt.Errorf("protect the work directory: %w", err)
	}
	return work, nil
}

// isPlaintextLeftoverDir and isPlaintextLeftover name what a crashed purge or restore may leave in clear.
func isPlaintextLeftoverDir(name string) bool { return strings.HasPrefix(name, purgeWorkPrefix) }

func isPlaintextLeftover(name string) bool {
	if strings.HasSuffix(name, preRestoreTempExt) || strings.HasSuffix(name, restoreTempExt) ||
		(strings.HasPrefix(name, ".enc-") && strings.HasSuffix(name, ".part")) {
		return true
	}
	return (strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix)) && !strings.HasSuffix(name, PurgeBackupSuffix)
}

func isEncryptedBackupName(name string) bool {
	return strings.HasSuffix(name, PurgeBackupSuffix) &&
		(strings.HasPrefix(name, PurgeBackupPrefix) || strings.HasPrefix(name, PreRestoreBackupPrefix))
}

// SweepPurgeBackups deletes the encrypted purge and pre-restore backups in dir that are older than
// PurgeBackupMaxAge, by modification time. It returns the deleted names.
// Protected recovery directories and their encrypted safety copies never expire.
// Other plaintext leftovers are removed. Only call it while no purge runs.
func SweepPurgeBackups(dir string, now time.Time) ([]string, error) {
	return sweepPurgeDir(dir, now, true)
}

func sweepPurgeDir(dir string, now time.Time, leftovers bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var deleted []string
	protectedWork, protectedBackups := map[string]bool{}, map[string]bool{}
	keepEncrypted := false
	for _, entry := range entries {
		if !entry.IsDir() || !isPlaintextLeftoverDir(entry.Name()) {
			continue
		}
		work := filepath.Join(dir, entry.Name())
		data, markerErr := os.ReadFile(filepath.Join(work, purgeRecoveryMarker))
		if markerErr == nil {
			protectedWork[entry.Name()] = true
			var notice purgeRecoveryNotice
			if json.Unmarshal(data, &notice) != nil || notice.Backup == "" || filepath.Base(notice.Backup) != notice.Backup {
				keepEncrypted = true
			} else {
				protectedBackups[notice.Backup] = true
			}
		} else if !os.IsNotExist(markerErr) {
			return nil, fmt.Errorf("inspect recovery marker: %w", markerErr)
		} else {
			// Older failed restores have no marker; their only original may be here.
			for _, suffix := range []string{"", "-wal", "-shm"} {
				if _, err := os.Lstat(filepath.Join(work, "old.db"+suffix)); !os.IsNotExist(err) {
					protectedWork[entry.Name()], keepEncrypted = true, true
				}
			}
		}
	}
	for _, entry := range entries {
		if protectedWork[entry.Name()] || protectedBackups[entry.Name()] {
			continue
		}
		if leftovers {
			if entry.IsDir() && isPlaintextLeftoverDir(entry.Name()) || entry.Type().IsRegular() && isPlaintextLeftover(entry.Name()) {
				if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
					return deleted, err
				}
				deleted = append(deleted, entry.Name())
				continue
			}
		}
		if keepEncrypted || !entry.Type().IsRegular() || !isEncryptedBackupName(entry.Name()) {
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

// renameFile is os.Rename; a test replaces it to fail the swap.
var renameFile = os.Rename

// RestorePurgeBackup replaces the flows database at dbPath by the decrypted purge backup. It is
// meant for the --restore-flows-purge-backup flag and must run while AuraGo is stopped. It first
// keeps the current database as an encrypted .flows-pre-restore file, verifies the decrypted backup
// opens and passes integrity_check, then moves the old database, WAL and SHM aside, renames the
// restored file into place and deletes the aside files. If the rename fails the old files go back,
// retaining marked private recovery files if rollback also fails. Other paths clean staging.
func RestorePurgeBackup(dbPath, backupFile, masterKeyHex string, now time.Time) (err error) {
	if info, statErr := os.Lstat(backupFile); statErr != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("the backup file %q cannot be read", backupFile)
	}
	work, err := newPurgeWorkDir(filepath.Dir(dbPath))
	if err != nil {
		return err
	}
	keepWork := false
	recoveryBackup := backupFile
	defer func() {
		if !keepWork {
			_ = os.RemoveAll(work)
		}
	}()
	if _, statErr := os.Stat(dbPath); statErr == nil {
		plain := filepath.Join(work, "pre-restore.db")
		current, openErr := sql.Open("sqlite", dbPath)
		if openErr != nil {
			return fmt.Errorf("open the current database: %w", openErr)
		}
		_, execErr := current.Exec(`VACUUM INTO ?`, plain)
		_ = current.Close()
		if execErr != nil {
			return fmt.Errorf("copy the current database: %w", execErr)
		}
		pre := filepath.Join(filepath.Dir(dbPath), PreRestoreBackupPrefix+now.UTC().Format(backupTimeLayout)+PurgeBackupSuffix)
		if encErr := security.EncryptFileWithMasterKey(plain, pre, PurgeBackupLabel, masterKeyHex); encErr != nil {
			return fmt.Errorf("keep the current database: %w", encErr)
		}
		recoveryBackup = pre
		_ = os.Remove(plain)
	}
	restored := filepath.Join(work, "restored.db")
	if decErr := security.DecryptFileWithMasterKey(backupFile, restored, PurgeBackupLabel, masterKeyHex); decErr != nil {
		return decErr
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
	if err := fileutil.RestrictToOwner(restored); err != nil {
		return err
	}
	notice, err := json.Marshal(purgeRecoveryNotice{Backup: filepath.Base(recoveryBackup)})
	if err != nil {
		return err
	}
	// Publish the recovery marker before any original file can leave its live path.
	if err := fileutil.WriteFileContext(context.Background(), filepath.Join(work, purgeRecoveryMarker), notice, 0600); err != nil {
		return err
	}
	// Move the old files aside; a stale WAL must never replay over the restored database.
	type move struct{ from, to string }
	var moved []move
	putBack := func() error {
		var firstErr error
		for i := len(moved) - 1; i >= 0; i-- {
			if e := renameFile(moved[i].to, moved[i].from); e != nil && firstErr == nil {
				firstErr = e
			}
		}
		return firstErr
	}
	fail := func(cause error) error {
		if backErr := putBack(); backErr != nil {
			keepWork = true // the old files are only in the work directory now
			return fmt.Errorf("%w; the old database could not be put back; recovery files are protected in %s; encrypted backup: %s: %v", cause, work, recoveryBackup, backErr)
		}
		return cause
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, statErr := os.Lstat(dbPath + suffix); os.IsNotExist(statErr) {
			continue
		}
		aside := filepath.Join(work, "old.db"+suffix)
		if e := renameFile(dbPath+suffix, aside); e != nil {
			return fail(e)
		}
		moved = append(moved, move{dbPath + suffix, aside})
	}
	if e := renameFile(restored, dbPath); e != nil {
		return fail(e)
	}
	return nil
}
