package dbutil

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
)

// Open opens a SQLite database with standard pragmas and configuration.
// It applies WAL mode, synchronous=NORMAL, foreign_keys=ON, busy_timeout=5000,
// and SetMaxOpenConns(4) by default. Options can override defaults.
//
// When WithCorruptionRecovery is enabled and the initial open+configure step
// proves corruption, the function attempts automatic recovery: original files
// are moved to a unique restricted quarantine and a fresh database
// is created. This handles the case where the file header is invalid and
// PRAGMA statements cannot execute at all.
//
// Returns an error if any step fails. On failure, the database handle is closed.
func Open(dbPath string, opts ...Option) (*sql.DB, error) {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := openAndConfigure(dbPath, cfg)
	if err != nil {
		if !cfg.corruptionRecovery || !isCorruption(err) {
			return nil, err
		}
		// PRAGMA or integrity check failure — attempt recovery.
		if cfg.recoveryLogger != nil {
			cfg.recoveryLogger.Error("SQLite open failed, attempting corruption recovery",
				"path", dbPath, "error", err)
		}
		if recErr := recoverCorruptDB(dbPath, cfg.recoveryLogger); recErr != nil {
			return nil, fmt.Errorf("%w (recovery also failed: %v)", err, recErr)
		}
		// Retry after recovery.
		db, err = openAndConfigure(dbPath, cfg)
		if err != nil {
			return nil, err
		}
	}

	return db, nil
}

// openAndConfigure opens the database, applies connection settings and PRAGMAs,
// and runs an integrity check. Returns an error if any step fails.
func openAndConfigure(dbPath string, cfg config) (*sql.DB, error) {
	// Open database.
	// Embed connection-local PRAGMAs in the DSN so modernc.org/sqlite applies
	// them to every connection opened from the pool, including lazy connections.
	dsn := buildDSN(dbPath, cfg)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set max open connections. In-memory databases must keep a single
	// connection even with cache=shared to avoid schema visibility races.
	if dbPath == ":memory:" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(cfg.maxOpenConns)
	}

	// Apply PRAGMAs
	if err := applyPragmas(db, cfg); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply PRAGMAs: %w", err)
	}

	// Run integrity check
	corrupt, err := runIntegrityCheck(db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run integrity check: %w", err)
	}

	if corrupt {
		db.Close()
		return nil, errIntegrityCorrupt
	}

	return db, nil
}

// buildDSN builds the SQLite DSN with connection-local settings configured as
// repeated modernc _pragma parameters. Unrelated caller parameters, including
// unrelated pragmas, are retained while managed settings are replaced.
func buildDSN(path string, cfg config) string {
	base, rawQuery, _ := strings.Cut(path, "?")
	params := make([]string, 0, 8)
	hasCache := false
	for _, param := range strings.Split(rawQuery, "&") {
		if param == "" {
			continue
		}
		rawKey, rawValue, _ := strings.Cut(param, "=")
		key, err := url.QueryUnescape(rawKey)
		if err != nil {
			params = append(params, param)
			continue
		}
		if strings.EqualFold(key, "cache") {
			hasCache = true
		}
		if strings.EqualFold(key, "_busy_timeout") {
			continue
		}
		if strings.EqualFold(key, "_pragma") {
			value, err := url.QueryUnescape(rawValue)
			if err == nil && isManagedPragma(value) {
				continue
			}
		}
		params = append(params, param)
	}

	for _, pragma := range []string{
		"foreign_keys(1)",
		fmt.Sprintf("synchronous(%s)", cfg.synchronous),
		fmt.Sprintf("busy_timeout(%d)", cfg.busyTimeout),
	} {
		params = append(params, "_pragma="+url.QueryEscape(pragma))
	}
	if path == ":memory:" && !hasCache {
		params = append(params, "cache=shared")
	}
	if cfg.immediateTransactions {
		params = append(params, "_txlock=immediate")
	}
	return base + "?" + strings.Join(params, "&")
}

func isManagedPragma(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if separator := strings.IndexAny(value, "(= \t"); separator >= 0 {
		value = value[:separator]
	}
	switch value {
	case "foreign_keys", "synchronous", "busy_timeout", "journal_mode":
		return true
	default:
		return false
	}
}

// applyPragmas applies the persistent SQLite settings that only need to be set
// once. Connection-local settings are embedded in the DSN by buildDSN.
func applyPragmas(db *sql.DB, _ config) error {
	// journal_mode=WAL
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to set journal_mode=WAL: %w", err)
	}

	return nil
}

// runIntegrityCheck runs PRAGMA integrity_check and returns whether corruption was detected.
func runIntegrityCheck(db *sql.DB) (bool, error) {
	var result string
	if err := db.QueryRow("PRAGMA integrity_check(1)").Scan(&result); err != nil {
		return false, fmt.Errorf("integrity check query failed: %w", err)
	}
	return result != "ok", nil
}

// recoverCorruptDB handles proven corruption by quarantining original files
// and allowing a fresh database to be created.
var errIntegrityCorrupt = errors.New("database integrity check failed")

func isCorruption(err error) bool {
	var sqliteErr interface{ Code() int }
	return errors.Is(err, errIntegrityCorrupt) || (errors.As(err, &sqliteErr) && sqliteErr.Code()&255 == 11)
}

func recoverCorruptDB(dbPath string, logger *slog.Logger) error {
	if logger != nil {
		logger.Error("SQLite database is corrupted, attempting recovery",
			"path", dbPath)
	}

	// Recovery is restricted to literal disk paths; never reinterpret a SQLite URI.
	if dbPath == ":memory:" || strings.HasPrefix(dbPath, "file:") || strings.Contains(dbPath, "?") {
		return fmt.Errorf("cannot quarantine a nonliteral database path")
	}
	var sources []string
	for _, suffix := range []string{"-wal", "-shm", "-journal", ""} {
		src := dbPath + suffix
		info, err := os.Lstat(src)
		if errors.Is(err, os.ErrNotExist) && suffix != "" {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect recovery source: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("recovery source is not a regular file: %s", src)
		}
		sources = append(sources, src)
	}
	quarantine, err := os.MkdirTemp(filepath.Dir(dbPath), filepath.Base(dbPath)+".corrupt-")
	if err != nil {
		return fmt.Errorf("create quarantine: %w", err)
	}
	var moved []string
	for _, src := range sources {
		if err := fileutil.Rename(src, filepath.Join(quarantine, filepath.Base(src))); err != nil {
			rollbackErr := error(nil)
			for i := len(moved) - 1; i >= 0; i-- {
				rollbackErr = errors.Join(rollbackErr, fileutil.Rename(filepath.Join(quarantine, filepath.Base(moved[i])), moved[i]))
			}
			return errors.Join(fmt.Errorf("quarantine database: %w", err), rollbackErr)
		}
		moved = append(moved, src)
	}
	file, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create replacement database: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close replacement database: %w", err)
	}

	if logger != nil {
		logger.Info("Quarantined corrupted SQLite database", "path", dbPath, "quarantine", quarantine)
	}
	return nil
}
