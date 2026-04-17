package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// DaemonRunner unit tests
// ---------------------------------------------------------------------------

func TestNewDaemonRunner_Defaults(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	if runner.Status() != DaemonStopped {
		t.Errorf("expected initial status stopped, got %s", runner.Status())
	}
	if runner.config.MaxRestartAttempts != 0 {
		t.Errorf("expected MaxRestartAttempts=0 (raw config, ApplyDefaults not called in runner), got %d", runner.config.MaxRestartAttempts)
	}
}

func TestDaemonRunner_State(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	state := runner.State()
	if state.SkillID != "test-id" {
		t.Errorf("expected SkillID=test-id, got %q", state.SkillID)
	}
	if state.Status != DaemonStopped {
		t.Errorf("expected status stopped, got %s", state.Status)
	}
	if state.AutoDisabled {
		t.Error("expected AutoDisabled=false")
	}
}

func TestDaemonRunner_DisableReenable(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})

	runner.Disable("circuit breaker triggered")
	if runner.Status() != DaemonDisabled {
		t.Errorf("expected status disabled, got %s", runner.Status())
	}
	state := runner.State()
	if !state.AutoDisabled {
		t.Error("expected AutoDisabled=true after Disable")
	}

	runner.Reenable()
	if runner.Status() != DaemonStopped {
		t.Errorf("expected status stopped after re-enable, got %s", runner.Status())
	}
	state = runner.State()
	if state.AutoDisabled {
		t.Error("expected AutoDisabled=false after Reenable")
	}
}

func TestDaemonRunner_StartWhenDisabled(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	runner.Disable("test")
	err := runner.Start()
	if err == nil {
		t.Error("expected error when starting disabled daemon")
	}
}

func TestDaemonRunner_StartMissingExecutable(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:      "test-id",
		SkillName:    "test-skill",
		Config:       DaemonManifest{},
		Manifest:     SkillManifest{Name: "test-skill", Executable: "nonexistent_skill.py"},
		SkillsDir:    t.TempDir(),
		WorkspaceDir: t.TempDir(),
		Logger:       noopLogger(),
	})
	err := runner.Start()
	if err == nil {
		t.Error("expected error for missing executable")
	}
}

func TestDaemonRunner_IncrementCounters(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	runner.IncrementWakeUp()
	runner.IncrementWakeUp()
	runner.IncrementSuppressed()

	state := runner.State()
	if state.WakeUpCount != 2 {
		t.Errorf("expected WakeUpCount=2, got %d", state.WakeUpCount)
	}
	if state.SuppressedCount != 1 {
		t.Errorf("expected SuppressedCount=1, got %d", state.SuppressedCount)
	}
	if state.LastWakeUp == nil {
		t.Error("expected LastWakeUp to be set")
	}
}

// ---------------------------------------------------------------------------
// DaemonRunner canRestart tests
// ---------------------------------------------------------------------------

func TestDaemonRunner_CanRestart(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config: DaemonManifest{
			RestartOnCrash:         true,
			MaxRestartAttempts:     2,
			RestartCooldownSeconds: 300,
		},
		Manifest: SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:   noopLogger(),
	})

	// First restart should be allowed
	runner.mu.Lock()
	if !runner.canRestart() {
		t.Error("expected first restart to be allowed")
	}
	runner.restartCount = 1
	if !runner.canRestart() {
		t.Error("expected second restart to be allowed (count=1 < max=2)")
	}
	runner.restartCount = 2
	runner.lastRestartTime = time.Now()
	if runner.canRestart() {
		t.Error("expected restart to be denied (count=max, within cooldown)")
	}
	runner.mu.Unlock()
}

// ---------------------------------------------------------------------------
// DaemonRunner — lifecycle and edge-case tests
// ---------------------------------------------------------------------------

func TestDaemonRunner_DoubleDisable(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	runner.Disable("first")
	runner.Disable("second")
	if runner.Status() != DaemonDisabled {
		t.Errorf("expected disabled, got %s", runner.Status())
	}
	state := runner.State()
	if state.LastError != "second" {
		t.Errorf("expected last error to be 'second', got %q", state.LastError)
	}
}

func TestDaemonRunner_ReenableResetsRestartCount(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{MaxRestartAttempts: 3},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	runner.mu.Lock()
	runner.restartCount = 3
	runner.mu.Unlock()

	runner.Disable("maxed out")
	runner.Reenable()

	runner.mu.Lock()
	count := runner.restartCount
	runner.mu.Unlock()
	if count != 0 {
		t.Errorf("expected restart count reset to 0 after reenable, got %d", count)
	}
}

func TestDaemonRunner_ConcurrentIncrements(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			runner.IncrementWakeUp()
		}()
		go func() {
			defer wg.Done()
			runner.IncrementSuppressed()
		}()
	}
	wg.Wait()

	state := runner.State()
	if state.WakeUpCount != 100 {
		t.Errorf("expected 100 wakeups, got %d", state.WakeUpCount)
	}
	if state.SuppressedCount != 100 {
		t.Errorf("expected 100 suppressed, got %d", state.SuppressedCount)
	}
}

func TestDaemonRunner_ConcurrentStateAndDisable(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_ = runner.State()
		}()
		go func() {
			defer wg.Done()
			_ = runner.Status()
		}()
		go func() {
			defer wg.Done()
			runner.IncrementWakeUp()
		}()
	}
	wg.Wait()
	// No race/deadlock = pass
}

func TestDaemonRunner_CanRestartCooldownReset(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config: DaemonManifest{
			RestartOnCrash:         true,
			MaxRestartAttempts:     2,
			RestartCooldownSeconds: 1, // 1 second cooldown
		},
		Manifest: SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:   noopLogger(),
	})

	runner.mu.Lock()
	runner.restartCount = 2
	runner.lastRestartTime = time.Now().Add(-2 * time.Second) // cooldown expired
	if !runner.canRestart() {
		t.Error("expected canRestart=true after cooldown expired")
	}
	if runner.restartCount != 0 {
		t.Errorf("expected restartCount reset to 0, got %d", runner.restartCount)
	}
	runner.mu.Unlock()
}

func TestDaemonRunner_StartAlreadyRunning(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})

	// Simulate running state
	runner.mu.Lock()
	runner.status = DaemonRunning
	runner.mu.Unlock()

	err := runner.Start()
	if err == nil {
		t.Error("expected error when starting already running daemon")
	}
	if !strings.Contains(err.Error(), "already") {
		t.Errorf("expected 'already' in error, got: %v", err)
	}
}

func TestDaemonRunner_StartStartingState(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "test-id",
		SkillName: "test-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "test-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})

	runner.mu.Lock()
	runner.status = DaemonStarting
	runner.mu.Unlock()

	err := runner.Start()
	if err == nil {
		t.Error("expected error when starting a daemon in starting state")
	}
}

// ---------------------------------------------------------------------------
// DaemonRunner — log rotation tests
// ---------------------------------------------------------------------------

func TestDaemonRunner_AppendDaemonLog(t *testing.T) {
	logDir := t.TempDir()
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "log-test",
		SkillName: "log-test",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "log-test", Executable: "test.py"},
		LogDir:    logDir,
		Logger:    noopLogger(),
	})

	runner.appendDaemonLog("hello world")
	runner.appendDaemonLog("second line")

	logPath := filepath.Join(logDir, "log-test.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "hello world") {
		t.Error("expected 'hello world' in log")
	}
	if !strings.Contains(content, "second line") {
		t.Error("expected 'second line' in log")
	}
}

func TestDaemonRunner_AppendDaemonLogNoDir(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "no-log",
		SkillName: "no-log",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "no-log", Executable: "test.py"},
		LogDir:    "", // empty — should no-op
		Logger:    noopLogger(),
	})
	// Should not panic
	runner.appendDaemonLog("no log dir")
}

func TestDaemonRunner_LogRotation(t *testing.T) {
	logDir := t.TempDir()
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "rotate-test",
		SkillName: "rotate-test",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "rotate-test", Executable: "test.py"},
		LogDir:    logDir,
		Logger:    noopLogger(),
	})

	logPath := filepath.Join(logDir, "rotate-test.log")

	// Write a file just above the daemon log max size
	bigLine := strings.Repeat("X", 1024)
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("failed to create log: %v", err)
	}
	// Write 6MB to exceed the 5MB threshold
	for i := 0; i < 6*1024; i++ {
		fmt.Fprintln(f, bigLine)
	}
	f.Close()

	sizeBefore, _ := os.Stat(logPath)

	// This append should trigger rotation
	runner.appendDaemonLog("after rotation")

	sizeAfter, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file should exist after rotation: %v", err)
	}

	if sizeAfter.Size() >= sizeBefore.Size() {
		t.Errorf("expected log file to shrink after rotation, before=%d after=%d",
			sizeBefore.Size(), sizeAfter.Size())
	}
}
