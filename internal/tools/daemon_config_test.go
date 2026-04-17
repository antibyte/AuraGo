package tools

import (
	"testing"
)

// ---------------------------------------------------------------------------
// DaemonManifest defaults tests
// ---------------------------------------------------------------------------

func TestDaemonManifestDefaults(t *testing.T) {
	d := DaemonManifestDefaults()
	if d.WakeRateLimitSeconds != 60 {
		t.Errorf("expected WakeRateLimitSeconds=60, got %d", d.WakeRateLimitSeconds)
	}
	if d.MaxRestartAttempts != 3 {
		t.Errorf("expected MaxRestartAttempts=3, got %d", d.MaxRestartAttempts)
	}
	if d.RestartCooldownSeconds != 300 {
		t.Errorf("expected RestartCooldownSeconds=300, got %d", d.RestartCooldownSeconds)
	}
	if d.HealthCheckIntervalSeconds != 60 {
		t.Errorf("expected HealthCheckIntervalSeconds=60, got %d", d.HealthCheckIntervalSeconds)
	}
	if !d.RestartOnCrash {
		t.Error("expected RestartOnCrash=true")
	}
}

func TestDaemonManifestApplyDefaults(t *testing.T) {
	d := &DaemonManifest{
		WakeRateLimitSeconds: 500, // explicitly set — should NOT be overwritten
	}
	d.ApplyDefaults()
	if d.WakeRateLimitSeconds != 500 {
		t.Errorf("expected WakeRateLimitSeconds=500 (explicit), got %d", d.WakeRateLimitSeconds)
	}
	if d.MaxRestartAttempts != 3 {
		t.Errorf("expected MaxRestartAttempts=3 (default), got %d", d.MaxRestartAttempts)
	}
	if d.RestartCooldownSeconds != 300 {
		t.Errorf("expected RestartCooldownSeconds=300 (default), got %d", d.RestartCooldownSeconds)
	}
}

// ---------------------------------------------------------------------------
// DaemonState / DaemonStatus tests
// ---------------------------------------------------------------------------

func TestDaemonStatusValues(t *testing.T) {
	statuses := []DaemonStatus{DaemonStopped, DaemonStarting, DaemonRunning, DaemonCrashed, DaemonDisabled}
	expected := []string{"stopped", "starting", "running", "crashed", "disabled"}
	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected status %q, got %q", expected[i], s)
		}
	}
}

// ---------------------------------------------------------------------------
// DaemonManifest — additional tests
// ---------------------------------------------------------------------------

func TestDaemonManifestApplyDefaults_AllZero(t *testing.T) {
	d := &DaemonManifest{}
	d.ApplyDefaults()
	if d.WakeRateLimitSeconds != 60 {
		t.Errorf("expected WakeRateLimitSeconds=60, got %d", d.WakeRateLimitSeconds)
	}
	if d.MaxRestartAttempts != 3 {
		t.Errorf("expected MaxRestartAttempts=3, got %d", d.MaxRestartAttempts)
	}
	if d.RestartCooldownSeconds != 300 {
		t.Errorf("expected RestartCooldownSeconds=300, got %d", d.RestartCooldownSeconds)
	}
	if d.HealthCheckIntervalSeconds != 60 {
		t.Errorf("expected HealthCheckIntervalSeconds=60, got %d", d.HealthCheckIntervalSeconds)
	}
	if !d.RestartOnCrash {
		t.Error("expected RestartOnCrash=true after ApplyDefaults")
	}
}

func TestDaemonManifestApplyDefaults_PartialSet(t *testing.T) {
	d := &DaemonManifest{
		WakeRateLimitSeconds:       100,
		HealthCheckIntervalSeconds: 30,
	}
	d.ApplyDefaults()
	if d.WakeRateLimitSeconds != 100 {
		t.Errorf("expected explicitly set WakeRateLimitSeconds=100, got %d", d.WakeRateLimitSeconds)
	}
	if d.HealthCheckIntervalSeconds != 30 {
		t.Errorf("expected explicitly set HealthCheckIntervalSeconds=30, got %d", d.HealthCheckIntervalSeconds)
	}
	if d.MaxRestartAttempts != 3 {
		t.Errorf("expected MaxRestartAttempts=3 (default), got %d", d.MaxRestartAttempts)
	}
	if !d.RestartOnCrash {
		t.Error("expected RestartOnCrash=true (always applied from defaults)")
	}
}

// ---------------------------------------------------------------------------
// DaemonState tests
// ---------------------------------------------------------------------------

func TestDaemonState_TimestampsNil(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "time-test",
		SkillName: "time-test",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "time-test", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	state := runner.State()
	if state.StartedAt != nil {
		t.Error("expected StartedAt=nil for stopped daemon")
	}
	if state.LastWakeUp != nil {
		t.Error("expected LastWakeUp=nil before any wakes")
	}
	if state.LastActivity != nil {
		t.Error("expected LastActivity=nil before any activity")
	}
}

func TestDaemonState_AfterWakeUp(t *testing.T) {
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "time-test",
		SkillName: "time-test",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "time-test", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	runner.IncrementWakeUp()
	state := runner.State()
	if state.LastWakeUp == nil {
		t.Error("expected LastWakeUp to be set after IncrementWakeUp")
	}
}

// ---------------------------------------------------------------------------
// DaemonSSE constants tests
// ---------------------------------------------------------------------------

func TestDaemonSSEConstants(t *testing.T) {
	if DaemonSSEStatus != "daemon_update" {
		t.Errorf("unexpected DaemonSSEStatus: %q", DaemonSSEStatus)
	}
	if DaemonSSEWakeUp != "daemon_update" {
		t.Errorf("unexpected DaemonSSEWakeUp: %q", DaemonSSEWakeUp)
	}
	if DaemonSSEAutoDisabled != "daemon_update" {
		t.Errorf("unexpected DaemonSSEAutoDisabled: %q", DaemonSSEAutoDisabled)
	}
}
