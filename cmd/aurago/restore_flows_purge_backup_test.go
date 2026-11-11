package main

import (
	"bytes"
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
