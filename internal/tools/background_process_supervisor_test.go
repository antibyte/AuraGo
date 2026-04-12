package tools

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestRegisterManagedBackgroundProcessKillsTimedOutProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("KillProcessTree via taskkill does not reliably kill Go test subprocess on Windows")
	}
	originalTimeout := GetBackgroundTimeout()
	defer SetBackgroundTimeout(originalTimeout)
	SetBackgroundTimeout(50 * time.Millisecond)

	registry := NewProcessRegistry(testBackgroundTaskLogger())
	cmd := exec.Command(os.Args[0], "-test.run=TestBackgroundProcessSupervisorHelper", "--", "sleep")
	cmd.Env = append(os.Environ(), "GO_WANT_BACKGROUND_HELPER=1")

	pid, err := registerManagedBackgroundProcess(cmd, registry, nil)
	if err != nil {
		t.Fatalf("registerManagedBackgroundProcess: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("pid = %d, want > 0", pid)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := registry.Get(pid); !ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected timed-out background process %d to be removed from registry", pid)
}

func TestBackgroundProcessSupervisorHelper(t *testing.T) {
	if os.Getenv("GO_WANT_BACKGROUND_HELPER") != "1" {
		return
	}
	time.Sleep(5 * time.Second)
	os.Exit(0)
}
