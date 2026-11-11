package dbutil

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aurago/internal/fileutil"
)

var sqliteBackupPublication sync.Mutex
var sqliteBackupRename = fileutil.RenameContext
var sqliteBackupSync = (*os.File).Sync

// BackupSQLite publishes a synced, owner-only main-database snapshot, including
// committed WAL content. Existing destinations are retained under their names;
// a collision gets a unique sibling name, returned to the caller.
func BackupSQLite(ctx context.Context, db *sql.DB, destination string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if db == nil {
		return "", fmt.Errorf("backup database is unavailable")
	}
	staging, err := os.MkdirTemp(filepath.Dir(destination), ".sqlite-backup-*")
	if err != nil {
		return "", fmt.Errorf("stage SQLite backup: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := fileutil.RestrictToOwner(staging); err != nil {
		return "", fmt.Errorf("protect SQLite backup directory: %w", err)
	}
	temporary := filepath.Join(staging, "backup.sqlite")
	f, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := fileutil.RestrictToOwner(temporary); err != nil {
		return "", fmt.Errorf("protect SQLite backup: %w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM main INTO ?`, temporary); err != nil {
		return "", fmt.Errorf("copy SQLite backup: %w", err)
	}
	f, err = os.OpenFile(temporary, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	err = sqliteBackupSync(f)
	closeErr := f.Close()
	if err != nil {
		return "", fmt.Errorf("sync SQLite backup: %w", err)
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	sqliteBackupPublication.Lock()
	defer sqliteBackupPublication.Unlock()
	requested := destination
	for {
		if _, err := os.Lstat(destination); os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", err
		}
		ext := filepath.Ext(requested)
		destination = strings.TrimSuffix(requested, ext) + "." + rand.Text() + ext
	}
	if err := sqliteBackupRename(ctx, temporary, destination); err != nil {
		return "", fmt.Errorf("publish SQLite backup: %w", err)
	}
	return destination, nil
}
