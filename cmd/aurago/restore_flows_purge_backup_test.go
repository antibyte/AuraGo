package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func TestRestoreFlowsPurgeBackupRefusesWhileAuraGoRuns(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "aurago.lock")
	running := flock.New(lockPath)
	if locked, err := running.TryLock(); err != nil || !locked {
		t.Fatalf("test lock: %v %v", locked, err)
	}
	defer running.Unlock()
	var stdout, stderr bytes.Buffer
	code := runRestoreFlowsPurgeBackup(lockPath, filepath.Join(t.TempDir(), "config.yaml"), "backup.enc", &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "AuraGo is running") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q, want a refusal naming the running instance", code, stdout.String(), stderr.String())
	}
}

func TestRestoreFlowsPurgeBackupRefusesWhenAnotherInstallationHoldsTheDataLock(t *testing.T) {
	root := t.TempDir()
	cfgFile := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("sqlite:\n  game_maker_path: ./data/game_maker.db\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	other := flock.New(dataLockPath(filepath.Join(dataDir, "flows.db"))) // another installation's main
	if locked, err := other.TryLock(); err != nil || !locked {
		t.Fatalf("test lock: %v %v", locked, err)
	}
	defer other.Unlock()
	var stdout, stderr bytes.Buffer
	code := runRestoreFlowsPurgeBackup(filepath.Join(t.TempDir(), "aurago.lock"), cfgFile, "backup.enc", &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "AuraGo is running on this data directory") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q, want a refusal naming the data directory", code, stdout.String(), stderr.String())
	}
}
