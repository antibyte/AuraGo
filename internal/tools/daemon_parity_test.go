package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func allowHostSkillExecutionForTest(t *testing.T) {
	t.Helper()
	ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
}

func setDefaultSkillManagerForTest(t *testing.T, mgr *SkillManager) {
	t.Helper()
	previous := DefaultSkillManager()
	SetDefaultSkillManager(mgr)
	t.Cleanup(func() { SetDefaultSkillManager(previous) })
}

func TestResolveSkillExecutableRejectsTraversalAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if _, err := resolveSkillExecutable(dir, SkillManifest{Name: "escape", Executable: "../escape.sh"}); err == nil || !strings.Contains(err.Error(), "invalid executable path") {
		t.Fatalf("traversal error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "outside.py")
	if err := os.WriteFile(target, []byte("pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "linked.py")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := resolveSkillExecutable(dir, SkillManifest{Name: "linked", Executable: "linked.py"}); err == nil || !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Fatalf("symlink error = %v", err)
	}
}

func TestDaemonRunnerRequiresExecutableRegistryEntry(t *testing.T) {
	allowHostSkillExecutionForTest(t)
	mgr, skillsDir := setupTestSkillManager(t)
	created, err := mgr.CreateSkillEntry("watch_feed", "daemon parity", `def run(): return "ok"`, SkillTypeUser, "test", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}
	setDefaultSkillManagerForTest(t, mgr)
	newRunner := func() *DaemonRunner {
		return NewDaemonRunner(DaemonRunnerConfig{
			SkillID: "watch_feed", SkillName: "watch_feed",
			Manifest:  SkillManifest{Name: "watch_feed", Executable: "watch_feed.py"},
			SkillsDir: skillsDir, WorkspaceDir: t.TempDir(), Logger: noopLogger(),
		})
	}
	if err := newRunner().Start(); err == nil || !strings.Contains(err.Error(), "is disabled") {
		t.Fatalf("disabled registry entry start = %v", err)
	}
	if err := mgr.EnableSkill(created.ID, true, "test"); err != nil {
		t.Fatalf("EnableSkill: %v", err)
	}
	if _, err := mgr.db.Exec("UPDATE skills_registry SET security_status = ? WHERE id = ?", string(SecurityPending), created.ID); err != nil {
		t.Fatalf("set pending: %v", err)
	}
	if err := newRunner().Start(); err == nil || !strings.Contains(err.Error(), "pending cannot execute") {
		t.Fatalf("pending registry entry start = %v", err)
	}
	if _, err := mgr.db.Exec("UPDATE skills_registry SET security_status = ? WHERE id = ?", string(SecurityClean), created.ID); err != nil {
		t.Fatalf("set clean: %v", err)
	}
	// Every gate passes now; the start can only fail because the test workspace has no Python venv.
	if err := newRunner().Start(); err == nil || strings.Contains(err.Error(), "daemon execution denied") {
		t.Fatalf("clean registry entry start = %v, want a process start error", err)
	}
}

func TestDaemonRunnerDeniedWhenSandboxRequired(t *testing.T) {
	allowHostSkillExecutionForTest(t)
	skillsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillsDir, "watcher.py"), []byte("pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID: "watcher", SkillName: "watcher",
		Manifest:       SkillManifest{Name: "watcher", Executable: "watcher.py"},
		SkillsDir:      skillsDir,
		WorkspaceDir:   t.TempDir(),
		Logger:         noopLogger(),
		RequireSandbox: func() bool { return true },
	})
	if err := runner.Start(); err == nil || !strings.Contains(err.Error(), "require_sandbox") {
		t.Fatalf("start with require_sandbox = %v", err)
	}
	if runner.Status() != DaemonStopped {
		t.Fatalf("status = %s, want stopped", runner.Status())
	}
}

func TestDaemonStartOnDemandRejectsTraversalWithoutPersisting(t *testing.T) {
	allowHostSkillExecutionForTest(t)
	skillsDir := t.TempDir()
	manifestPath := filepath.Join(skillsDir, "escape.json")
	if err := os.WriteFile(manifestPath, []byte(`{"name":"escape","executable":"../escape.sh","daemon":{"enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sv := NewDaemonSupervisor(DaemonSupervisorConfig{Enabled: true, SkillsDir: skillsDir, LogDir: t.TempDir()}, nil, nil, nil, nil, noopLogger())
	if err := sv.StartDaemon("escape"); err == nil || !strings.Contains(err.Error(), "invalid executable path") {
		t.Fatalf("StartDaemon traversal = %v", err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored SkillManifest
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Daemon == nil || stored.Daemon.Enabled {
		t.Fatalf("denied daemon was persisted as enabled: %s", data)
	}
}

func TestDaemonSupervisorRefreshStopsDaemonWhenSandboxRequired(t *testing.T) {
	allowHostSkillExecutionForTest(t)
	skillsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(skillsDir, "watcher.py"), []byte("pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sv := NewDaemonSupervisor(DaemonSupervisorConfig{Enabled: true, SkillsDir: skillsDir}, nil, nil, nil, nil, noopLogger())
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID: "watcher", SkillName: "watcher",
		Manifest:       SkillManifest{Name: "watcher", Executable: "watcher.py"},
		SkillsDir:      skillsDir,
		Logger:         noopLogger(),
		RequireSandbox: sv.requireSandbox.Load,
	})
	canceled := false
	runner.status = DaemonRunning
	runner.cancel = func() { canceled = true }
	sv.runners[runner.skillID] = runner
	sv.RefreshRuntimePermissions()
	if canceled || runner.Status() != DaemonRunning {
		t.Fatalf("permitted daemon was stopped: canceled=%v status=%s", canceled, runner.Status())
	}
	sv.SetRequireSandbox(true)
	sv.RefreshRuntimePermissions()
	if !canceled || runner.Status() != DaemonStopped {
		t.Fatalf("daemon kept running under require_sandbox: canceled=%v status=%s", canceled, runner.Status())
	}
}
