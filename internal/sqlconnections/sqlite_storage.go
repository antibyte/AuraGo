package sqlconnections

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"aurago/internal/fileutil"
	"aurago/internal/uid"
)

const SQLiteImportRequired = "sqlite_import_required"
const maxSQLiteImportBytes = 64 << 20

var managedSQLiteID = regexp.MustCompile(`^managed:[a-zA-Z0-9-]{8,64}$`)

func sqliteStorageDir(db *sql.DB) (string, error) {
	var seq int
	var name, path string
	if err := db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil || path == "" {
		return "", fmt.Errorf("managed SQLite storage requires persistent metadata")
	}
	return filepath.Join(filepath.Dir(path), "sql_databases"), nil
}

func sqliteDSN(path string, readOnly, immutable bool) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "rw")
	q.Add("_pragma", "trusted_schema(0)")
	if readOnly {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	}
	if immutable {
		q.Set("immutable", "1")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (p *ConnectionPool) managedSQLiteDSN(rec ConnectionRecord, readOnly bool) (string, error) {
	if !managedSQLiteID.MatchString(rec.DatabaseName) {
		return "", fmt.Errorf("%s: import this database through the administrator UI", SQLiteImportRequired)
	}
	dir, err := sqliteStorageDir(p.metaDB)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, strings.TrimPrefix(rec.DatabaseName, "managed:")+".db")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("managed SQLite database is unavailable")
	}
	return sqliteDSN(path, readOnly, false), nil
}

// ImportSQLite accepts a consistent standalone SQLite backup, never a host path.
// An old managed generation remains available for administrator recovery.
func (p *ConnectionPool) ImportSQLite(ctx context.Context, id string, source io.Reader) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, err := GetByID(p.metaDB, id)
	if err != nil {
		return err
	}
	if rec.Driver != "sqlite" {
		return fmt.Errorf("connection is not SQLite")
	}
	dir, err := sqliteStorageDir(p.metaDB)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create SQLite storage: %w", err)
	}
	f, err := os.CreateTemp(dir, ".import-*.db")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, err := io.Copy(f, io.LimitReader(source, maxSQLiteImportBytes+1))
	if err == nil && n > maxSQLiteImportBytes {
		err = fmt.Errorf("SQLite import exceeds 64 MiB")
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	header, err := os.Open(tmp)
	if err != nil {
		return err
	}
	var signature [16]byte
	_, err = io.ReadFull(header, signature[:])
	header.Close()
	if err != nil || string(signature[:]) != "SQLite format 3\x00" {
		return fmt.Errorf("invalid standalone SQLite backup")
	}
	db, err := sql.Open("sqlite", sqliteDSN(tmp, true, true))
	if err != nil {
		return fmt.Errorf("open SQLite import: %w", err)
	}
	var verdict string
	err = db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&verdict)
	closeErr = db.Close()
	if err != nil || verdict != "ok" {
		return fmt.Errorf("invalid standalone SQLite backup")
	}
	if closeErr != nil {
		return closeErr
	}
	backup, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	backupFile, err := os.CreateTemp(dir, ".metadata-backup-*.json")
	if err != nil {
		return err
	}
	_, err = backupFile.Write(backup)
	if err == nil {
		err = backupFile.Sync()
	}
	closeErr = backupFile.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	key := uid.New()
	path := filepath.Join(dir, key+".db")
	if err = fileutil.RenameContext(ctx, tmp, path); err != nil {
		return err
	}
	if entry := p.conns[id]; entry != nil {
		entry.db.Close()
		delete(p.conns, id)
	}
	_, err = p.metaDB.ExecContext(ctx, "UPDATE sql_connections SET database_name = ? WHERE id = ?", "managed:"+key, id)
	if err != nil {
		os.Remove(path)
		return fmt.Errorf("publish SQLite import metadata: %w", err)
	}
	return nil
}
