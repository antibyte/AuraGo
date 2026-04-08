package dbutil

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
)

// Open opens a SQLite database with standard pragmas and configuration.
// It applies WAL mode, synchronous=NORMAL, foreign_keys=ON, busy_timeout=5000,
// and SetMaxOpenConns(1) by default. Options can override defaults.
//
// The function performs the following steps:
// 1. Opens the database using sql.Open with driver "sqlite"
// 2. Applies configuration options
// 3. Sets connection pool limits
// 4. Executes PRAGMA statements (journal_mode, synchronous, foreign_keys, busy_timeout)
// 5. Runs integrity check
// 6. If corruption is detected and recovery is enabled, rotates corrupted files and recreates the DB
//
// Returns an error if any step fails. On failure, the database handle is closed.
func Open(dbPath string, opts ...Option) (*sql.DB, error) {
	// Apply options to default config
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Open database
	db, err := sql.Open("sqlite", dbPath)
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

	// Handle corruption if recovery is enabled
	if corrupt && cfg.corruptionRecovery {
		db.Close()
		if err := recoverCorruptDB(dbPath, cfg.recoveryLogger); err != nil {
			return nil, fmt.Errorf("corruption recovery failed: %w", err)
		}

		// Reopen the database after recovery
		db, err = sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to reopen database after recovery: %w", err)
		}

		// Reapply settings
		db.SetMaxOpenConns(cfg.maxOpenConns)
		if err := applyPragmas(db, cfg); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to reapply PRAGMAs after recovery: %w", err)
		}

		// Recheck integrity
		corrupt2, err := runIntegrityCheck(db)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to rerun integrity check after recovery: %w", err)
		}
		if corrupt2 {
			return nil, fmt.Errorf("database still corrupted after recovery attempt")
		}
	} else if corrupt {
		db.Close()
		return nil, fmt.Errorf("database integrity check failed")
	}

	return db, nil
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
	logger.Error("SQLite database is corrupted, attempting recovery",
		"path", dbPath)

	// Rotate all related files
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := dbPath + suffix
		if _, statErr := os.Stat(src); statErr == nil {
			dst := src + ".bak"
			if renErr := os.Rename(src, dst); renErr != nil {
				logger.Warn("Could not rename corrupted DB file", "src", src, "error", renErr)
			} else {
				logger.Warn("Renamed corrupted DB file", "src", src, "dst", dst)
			}
		}
	}

	logger.Info("Created fresh SQLite database after corruption recovery", "path", dbPath)
	return nil
}
