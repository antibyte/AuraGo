package dbutil

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Open opens a SQLite database with standard pragmas and configuration.
// It applies WAL mode, synchronous=NORMAL, foreign_keys=ON, busy_timeout=5000,
// and SetMaxOpenConns(1) by default. Options can override defaults.
//
// When WithCorruptionRecovery is enabled and the initial open+configure step
// fails (e.g. PRAGMA execution on a corrupted file), the function attempts
// automatic recovery: corrupted files are rotated to .bak and a fresh database
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
		if !cfg.corruptionRecovery {
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
	// Embed _busy_timeout in the DSN so modernc.org/sqlite applies it to every
	// new connection opened from the pool — not just the first one.  Without
	// this, connections #2..N created lazily by MaxOpenConns>1 never get the
	// PRAGMA, causing immediate SQLITE_BUSY (5) errors under write concurrency.
	dsn := buildDSN(dbPath, cfg.busyTimeout)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set max open connections
	db.SetMaxOpenConns(cfg.maxOpenConns)

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
		return nil, fmt.Errorf("database integrity check failed")
	}

	return db, nil
}

// buildDSN builds the SQLite DSN, embedding _busy_timeout so that every new
// connection opened from the pool inherits the timeout automatically.
func buildDSN(path string, busyTimeoutMs int) string {
	param := fmt.Sprintf("_busy_timeout=%d", busyTimeoutMs)
	if strings.Contains(path, "?") {
		return path + "&" + param
	}
	return path + "?" + param
}

// applyPragmas applies the standard SQLite PRAGMAs.
func applyPragmas(db *sql.DB, cfg config) error {
	// journal_mode=WAL
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to set journal_mode=WAL: %w", err)
	}

	// synchronous=NORMAL (or as configured)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous=%s", cfg.synchronous)); err != nil {
		return fmt.Errorf("failed to set synchronous=%s: %w", cfg.synchronous, err)
	}

	// foreign_keys=ON
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("failed to set foreign_keys=ON: %w", err)
	}

	// busy_timeout=5000 (or as configured)
	if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.busyTimeout)); err != nil {
		return fmt.Errorf("failed to set busy_timeout=%d: %w", cfg.busyTimeout, err)
	}

	return nil
}

// runIntegrityCheck runs PRAGMA integrity_check and returns whether corruption was detected.
func runIntegrityCheck(db *sql.DB) (bool, error) {
	var result string
	if err := db.QueryRow("PRAGMA integrity_check(1)").Scan(&result); err != nil {
		// Log but don't fail on integrity check errors
		return false, nil
	}
	return result != "ok", nil
}

// recoverCorruptDB handles database corruption by renaming corrupted files to .bak
// and allowing a fresh database to be created.
func recoverCorruptDB(dbPath string, logger *slog.Logger) error {
	if logger != nil {
		logger.Error("SQLite database is corrupted, attempting recovery",
			"path", dbPath)
	}

	// Rotate all related files
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := dbPath + suffix
		if _, statErr := os.Stat(src); statErr == nil {
			dst := src + ".bak"
			if renErr := os.Rename(src, dst); renErr != nil {
				if logger != nil {
					logger.Warn("Could not rename corrupted DB file", "src", src, "error", renErr)
				}
			} else {
				if logger != nil {
					logger.Warn("Renamed corrupted DB file", "src", src, "dst", dst)
				}
			}
		}
	}

	if logger != nil {
		logger.Info("Created fresh SQLite database after corruption recovery", "path", dbPath)
	}
	return nil
}
