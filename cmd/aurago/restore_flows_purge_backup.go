package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"

	"aurago/internal/config"
	"aurago/internal/flows"
)

// instanceLockPath is the lock file main takes at startup ("Robust File Locking"): the binary's directory.
func instanceLockPath(installDir string) string {
	path := "aurago.lock"
	if installDir != "" {
		path = filepath.Join(installDir, "aurago.lock")
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// dataLockPath is the lock file of a data directory, next to flows.db. main and the restore flag take it, so
// a second installation on the same data directory is refused too.
func dataLockPath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "aurago-data.lock")
}

// runRestoreFlowsPurgeBackup implements --restore-flows-purge-backup and returns the exit code. It takes the
// instance lock first, so it refuses while AuraGo runs on this installation; it keeps the current flows.db
// as an encrypted .flows-pre-restore file, puts the decrypted backup in its place and exits.
func runRestoreFlowsPurgeBackup(lockPath, configFile, backupFile string, stdout, stderr io.Writer) int {
	lock := flock.New(lockPath)
	if locked, err := lock.TryLock(); err != nil || !locked {
		fmt.Fprintln(stderr, "AuraGo is running on this installation. Stop it first, then run the restore again.")
		return 1
	}
	defer lock.Unlock()
	cfg, err := config.Load(configFile)
	if err != nil {
		fmt.Fprintf(stderr, "CONFIG ERROR: %v\n", err)
		return 1
	}
	dbPath := config.FlowsDBPath(cfg)
	if dbPath == "" {
		fmt.Fprintln(stderr, "The flows database path is not configured (sqlite.game_maker_path).")
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fmt.Fprintf(stderr, "RESTORE ERROR: %v\n", err)
		return 1
	}
	dataLock := flock.New(dataLockPath(dbPath))
	if locked, err := dataLock.TryLock(); err != nil || !locked {
		fmt.Fprintln(stderr, "AuraGo is running on this data directory. Stop it first, then run the restore again.")
		return 1
	}
	defer dataLock.Unlock()
	key := strings.TrimSpace(cfg.Server.MasterKey)
	if key == "" {
		fmt.Fprintln(stderr, "The master key is not available: set AURAGO_MASTER_KEY or AURAGO_MASTER_KEY_FILE.")
		return 1
	}
	if err := flows.RestorePurgeBackup(dbPath, backupFile, key, time.Now()); err != nil {
		fmt.Fprintf(stderr, "RESTORE ERROR: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "flows.db restored from %s. The database it replaced was kept as an encrypted .flows-pre-restore file and is deleted after 7 days.\n", filepath.Base(backupFile))
	return 0
}
