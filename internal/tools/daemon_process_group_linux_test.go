//go:build linux

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDaemonStopKillsProcessGroupAndAppliesDaemonLimits(t *testing.T) {
	allowHostSkillExecutionForTest(t)
	skillsDir := t.TempDir()
	stateDir := t.TempDir()
	readyPath := filepath.Join(stateDir, "ready")
	pidPath := filepath.Join(stateDir, "child.pid")
	// The script waits with builtins only, so its first fork happens after Start applied the limits.
	script := fmt.Sprintf("#!/bin/bash\nuntil [ -e %q ]; do read -r -t 0.05 _ || true; done\nsleep 300 &\necho $! > %q\nwait\n", readyPath, pidPath)
	if err := os.WriteFile(filepath.Join(skillsDir, "sleeper.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID: "sleeper", SkillName: "sleeper",
		Manifest:  SkillManifest{Name: "sleeper", Executable: "sleeper.sh"},
		SkillsDir: skillsDir, WorkspaceDir: t.TempDir(), Logger: noopLogger(),
	})
	if err := runner.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = runner.Stop() })
	leader := runner.State().PID
	if leader <= 0 {
		t.Fatalf("daemon leader PID = %d", leader)
	}
	limits, err := os.ReadFile(fmt.Sprintf("/proc/%d/limits", leader))
	if err != nil {
		t.Fatalf("read daemon limits: %v", err)
	}
	// RLIMIT_NPROC is deliberately not set: Linux counts it per user including
	// every thread of the AuraGo process, so it would block daemon forks.
	for _, want := range []struct{ prefix, value string }{
		{"Max address space", strconv.Itoa(daemonMemoryLimitMB * 1024 * 1024)},
		{"Max cpu time", "unlimited"},
	} {
		if !daemonProcLimitIs(string(limits), want.prefix, want.value) {
			t.Fatalf("%s is not %s:\n%s", want.prefix, want.value, limits)
		}
	}
	if err := os.WriteFile(readyPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	childPID := 0
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && childPID == 0; time.Sleep(50 * time.Millisecond) {
		if data, err := os.ReadFile(pidPath); err == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
	}
	if childPID <= 0 {
		t.Fatal("daemon could not fork its child under the daemon limits")
	}
	if err := runner.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if err := syscall.Kill(childPID, 0); err == syscall.ESRCH {
			return
		}
	}
	t.Fatalf("daemon child %d survived Stop", childPID)
}

func daemonProcLimitIs(limits, prefix, value string) bool {
	for _, line := range strings.Split(limits, "\n") {
		if strings.HasPrefix(line, prefix) {
			fields := strings.Fields(strings.TrimPrefix(line, prefix))
			return len(fields) >= 2 && fields[0] == value && fields[1] == value
		}
	}
	return false
}
