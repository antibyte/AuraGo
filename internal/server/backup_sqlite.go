package server

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func agoSQLiteStringLiteral(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

func agoCreateSQLiteSnapshot(src, tempDir string) (string, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("sqlite source is not a regular file")
	}
	hash := sha256.Sum256([]byte(src))
	dst := filepath.Join(tempDir, fmt.Sprintf("%x_%s", hash[:6], filepath.Base(src)))
	_ = os.Remove(dst)

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(src)+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return "", fmt.Errorf("open sqlite for snapshot: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return "", fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := db.Exec("VACUUM INTO " + agoSQLiteStringLiteral(filepath.ToSlash(dst))); err != nil {
		return "", fmt.Errorf("create sqlite snapshot: %w", err)
	}
	if err := agoValidateSQLiteDB(dst); err != nil {
		return "", fmt.Errorf("validate sqlite snapshot: %w", err)
	}
	return dst, nil
}
