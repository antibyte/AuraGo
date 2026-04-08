package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ParseDaemonMessage tests
// ---------------------------------------------------------------------------

func TestParseDaemonMessage_WakeAgent(t *testing.T) {
	line := `{"type":"wake_agent","message":"Disk / at 95%","severity":"warning","data":{"disk":"/","percent":95}}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgWakeAgent {
		t.Errorf("expected type %q, got %q", DaemonMsgWakeAgent, msg.Type)
	}
	if msg.Message != "Disk / at 95%" {
		t.Errorf("unexpected message: %q", msg.Message)
	}
	if msg.Severity != "warning" {
		t.Errorf("expected severity warning, got %q", msg.Severity)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_WakeAgentDefaultSeverity(t *testing.T) {
	line := `{"type":"wake_agent","message":"event happened"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Severity != "info" {
		t.Errorf("expected default severity 'info', got %q", msg.Severity)
	}
}

func TestParseDaemonMessage_Log(t *testing.T) {
	line := `{"type":"log","level":"warn","message":"something happened"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type %q, got %q", DaemonMsgLog, msg.Type)
	}
	if msg.Level != "warn" {
		t.Errorf("expected level warn, got %q", msg.Level)
	}
}

func TestParseDaemonMessage_LogDefaultLevel(t *testing.T) {
	line := `{"type":"log","message":"hello"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Level != "info" {
		t.Errorf("expected default level 'info', got %q", msg.Level)
	}
}

func TestParseDaemonMessage_Heartbeat(t *testing.T) {
	line := `{"type":"heartbeat","timestamp":1712419200}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected type %q, got %q", DaemonMsgHeartbeat, msg.Type)
	}
	if msg.Timestamp != 1712419200 {
		t.Errorf("expected timestamp 1712419200, got %d", msg.Timestamp)
	}
}

func TestParseDaemonMessage_Error(t *testing.T) {
	line := `{"type":"error","message":"disk read failed","fatal":true}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgError {
		t.Errorf("expected type %q, got %q", DaemonMsgError, msg.Type)
	}
	if !msg.Fatal {
		t.Error("expected fatal=true")
	}
}

func TestParseDaemonMessage_Shutdown(t *testing.T) {
	line := `{"type":"shutdown","reason":"monitoring window complete"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgShutdown {
		t.Errorf("expected type %q, got %q", DaemonMsgShutdown, msg.Type)
	}
	if msg.Reason != "monitoring window complete" {
		t.Errorf("unexpected reason: %q", msg.Reason)
	}
}

func TestParseDaemonMessage_EmptyLine(t *testing.T) {
	_, err := ParseDaemonMessage("")
	if err == nil {
		t.Error("expected error for empty line")
	}
}

func TestParseDaemonMessage_InvalidJSON(t *testing.T) {
	_, err := ParseDaemonMessage("this is not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseDaemonMessage_MissingType(t *testing.T) {
	_, err := ParseDaemonMessage(`{"message":"no type field"}`)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestParseDaemonMessage_UnknownType(t *testing.T) {
	_, err := ParseDaemonMessage(`{"type":"foobar"}`)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestParseDaemonMessage_WhitespaceHandling(t *testing.T) {
	line := "  \t" + `{"type":"heartbeat"}` + "  \n"
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected heartbeat, got %q", msg.Type)
	}
}

// ---------------------------------------------------------------------------
// EncodeDaemonCommand tests
// ---------------------------------------------------------------------------

func TestEncodeDaemonCommand_Stop(t *testing.T) {
	cmd := NewStopCommand("user_requested", 30)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode command: %v", err)
	}
	if decoded.Type != DaemonCmdStop {
		t.Errorf("expected type %q, got %q", DaemonCmdStop, decoded.Type)
	}
	if decoded.Reason != "user_requested" {
		t.Errorf("unexpected reason: %q", decoded.Reason)
	}
	if decoded.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30, got %d", decoded.TimeoutSeconds)
	}
}

func TestEncodeDaemonCommand_ConfigUpdate(t *testing.T) {
	env := map[string]string{"THRESHOLD": "80", "INTERVAL": "60"}
	cmd := NewConfigUpdateCommand(env)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode command: %v", err)
	}
	if decoded.Type != DaemonCmdConfigUpdate {
		t.Errorf("expected type %q, got %q", DaemonCmdConfigUpdate, decoded.Type)
	}
	if decoded.Env["THRESHOLD"] != "80" {
		t.Errorf("expected THRESHOLD=80, got %q", decoded.Env["THRESHOLD"])
	}
}

func TestEncodeDaemonCommand_MissingType(t *testing.T) {
	_, err := EncodeDaemonCommand(DaemonCommand{})
	if err == nil {
		t.Error("expected error for missing type")
	}
}

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

// noopLogger returns a logger that discards all output.
func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---------------------------------------------------------------------------
// WakeUpGate tests
// ---------------------------------------------------------------------------

func TestWakeUpGate_GlobalDisabled(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       false,
		GlobalRateLimitSecs: 1,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected denial when globally disabled")
	}
	if denial.Layer != "global_toggle" {
		t.Errorf("expected layer global_toggle, got %q", denial.Layer)
	}
}

func TestWakeUpGate_SkillToggleOff(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", false, 1)

	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected denial when skill toggle off")
	}
	if denial.Layer != "skill_toggle" {
		t.Errorf("expected layer skill_toggle, got %q", denial.Layer)
	}
}

func TestWakeUpGate_UnregisteredSkill(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
	}, nil, noopLogger())

	ok, denial := gate.Allow("unknown")
	if ok {
		t.Error("expected denial for unregistered skill")
	}
	if denial.Layer != "skill_not_registered" {
		t.Errorf("expected layer skill_not_registered, got %q", denial.Layer)
	}
}

func TestWakeUpGate_AllowAndRateLimit(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 60) // minimum 60s between wakes

	// First call should be allowed
	ok, denial := gate.Allow("s1")
	if !ok {
		t.Fatalf("expected first wake-up to be allowed, got denial: %v", denial)
	}
	gate.RecordWakeUp("s1", 0.01)

	// Immediate second call should be rate-limited (per-skill)
	ok, denial = gate.Allow("s1")
	if ok {
		t.Error("expected second wake-up to be rate-limited")
	}
	if denial.Layer != "skill_rate_limit" && denial.Layer != "global_rate_limit" {
		t.Errorf("expected rate limit denial, got layer %q", denial.Layer)
	}
}

func TestWakeUpGate_GlobalRateLimit(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 600, // 10 min global
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1) // 1s per-skill, but global is 600s

	ok, _ := gate.Allow("s1")
	if !ok {
		t.Fatal("expected first wake-up to be allowed")
	}
	gate.RecordWakeUp("s1", 0)

	// Register another skill — should still be blocked by global rate limit
	gate.RegisterSkill("s2", true, 1)
	ok, denial := gate.Allow("s2")
	if ok {
		t.Error("expected global rate limit to block second skill")
	}
	if denial.Layer != "global_rate_limit" {
		t.Errorf("expected global_rate_limit layer, got %q", denial.Layer)
	}
}

func TestWakeUpGate_Escalation(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 10) // 10s base

	// Simulate consecutive wake-ups
	gate.RecordWakeUp("s1", 0.01)
	gate.RecordWakeUp("s1", 0.01)
	gate.RecordWakeUp("s1", 0.01)

	gate.mu.RLock()
	skill := gate.skills["s1"]
	factor := skill.escalationFactor
	gate.mu.RUnlock()

	if factor <= 1.0 {
		t.Errorf("expected escalation factor > 1.0, got %f", factor)
	}
}

func TestWakeUpGate_ResetEscalation(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 10)

	gate.RecordWakeUp("s1", 0.01)
	gate.RecordWakeUp("s1", 0.01)

	gate.ResetEscalation("s1")

	gate.mu.RLock()
	factor := gate.skills["s1"].escalationFactor
	gate.mu.RUnlock()

	if factor != 1.0 {
		t.Errorf("expected escalation reset to 1.0, got %f", factor)
	}
}

func TestWakeUpGate_CircuitBreaker(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   3,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	// Record 3 wakes — should trigger circuit breaker
	for i := 0; i < 3; i++ {
		gate.RecordWakeUp("s1", 0)
	}

	if !gate.ShouldAutoDisable("s1") {
		t.Error("expected circuit breaker to trigger after max wakes per hour")
	}
}

func TestWakeUpGate_HourlyBudget(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxBudgetPerHourUSD: 0.10,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	// Record wake-ups that exceed the hourly budget
	gate.RecordWakeUp("s1", 0.05)
	gate.RecordWakeUp("s1", 0.06)

	// Manually reset global + skill timestamps so rate limit doesn't interfere
	gate.mu.Lock()
	gate.lastGlobalWake = time.Time{}
	gate.skills["s1"].lastWake = time.Time{}
	gate.skills["s1"].escalationFactor = 1.0
	gate.mu.Unlock()

	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected hourly budget to block wake-up")
	}
	if denial.Layer != "hourly_budget" {
		t.Errorf("expected hourly_budget layer, got %q", denial.Layer)
	}
}

func TestWakeUpGate_Stats(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	gate.RecordWakeUp("s1", 0)
	gate.RecordWakeUp("s1", 0)
	gate.RecordSuppressed("s1")

	wakes, suppressed := gate.Stats("s1")
	if wakes != 2 {
		t.Errorf("expected 2 wakes, got %d", wakes)
	}
	if suppressed != 1 {
		t.Errorf("expected 1 suppressed, got %d", suppressed)
	}
}

func TestWakeUpGate_SetGlobalEnabled(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       false,
		GlobalRateLimitSecs: 1,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	ok, _ := gate.Allow("s1")
	if ok {
		t.Error("expected denial with global disabled")
	}

	gate.SetGlobalEnabled(true)
	ok, _ = gate.Allow("s1")
	if !ok {
		t.Error("expected allow after enabling globally")
	}
}

// ---------------------------------------------------------------------------
// DaemonSupervisor tests
// ---------------------------------------------------------------------------

// mockBroadcaster implements DaemonEventBroadcaster for testing.
type mockBroadcaster struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	Type    string
	Payload any
}

func (b *mockBroadcaster) BroadcastDaemonEvent(eventType string, payload any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, mockEvent{Type: eventType, Payload: payload})
}

func (b *mockBroadcaster) eventCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}

func TestDaemonSupervisor_NewAndDefaults(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil,
		nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		&mockBroadcaster{},
		noopLogger(),
	)
	if sv == nil {
		t.Fatal("expected non-nil supervisor")
	}
	if sv.config.MaxConcurrentDaemons != 5 {
		t.Errorf("expected default MaxConcurrentDaemons=5, got %d", sv.config.MaxConcurrentDaemons)
	}
	if sv.RunnerCount() != 0 {
		t.Errorf("expected 0 runners initially, got %d", sv.RunnerCount())
	}
}

func TestDaemonSupervisor_DisabledStart(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: false},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	if err := sv.Start(); err != nil {
		t.Fatalf("expected no error when disabled, got: %v", err)
	}
}

func TestDaemonSupervisor_ListDaemonsEmpty(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	states := sv.ListDaemons()
	if len(states) != 0 {
		t.Errorf("expected empty daemon list, got %d", len(states))
	}
}

func TestDaemonSupervisor_StopIdempotent(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	// Multiple stops should not panic
	sv.Stop()
	sv.Stop()
}

func TestDaemonSupervisor_StopDaemonNotFound(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	err := sv.StopDaemon("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent daemon")
	}
}

func TestDaemonSupervisor_GateAccess(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{
			Enabled: true,
			WakeUpGate: WakeUpGateConfig{
				GlobalEnabled:       true,
				GlobalRateLimitSecs: 60,
			},
		},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	gate := sv.Gate()
	if gate == nil {
		t.Fatal("expected non-nil gate")
	}
	gate.RegisterSkill("test-skill", true, 60)
	ok, _ := gate.Allow("test-skill")
	if !ok {
		t.Error("expected gate to allow first wake-up")
	}
}

// ---------------------------------------------------------------------------
// WakeUpGate — additional edge-case tests
// ---------------------------------------------------------------------------

func TestWakeUpGate_UnregisterSkill(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	ok, _ := gate.Allow("s1")
	if !ok {
		t.Fatal("expected allow before unregister")
	}

	gate.UnregisterSkill("s1")

	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected denial after unregister")
	}
	if denial.Layer != "skill_not_registered" {
		t.Errorf("expected layer skill_not_registered, got %q", denial.Layer)
	}
}

func TestWakeUpGate_UnregisterNonexistent(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled: true,
	}, nil, noopLogger())
	// Should not panic
	gate.UnregisterSkill("does-not-exist")
}

func TestWakeUpGate_MultipleSkillsIndependent(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 60)
	gate.RegisterSkill("s2", true, 60)

	// Allow s1
	ok, _ := gate.Allow("s1")
	if !ok {
		t.Fatal("expected s1 first wake to be allowed")
	}
	gate.RecordWakeUp("s1", 0)

	// s1 should be rate-limited now
	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected s1 to be rate-limited")
	}
	if denial.Layer != "skill_rate_limit" && denial.Layer != "global_rate_limit" {
		t.Errorf("expected rate limit, got %q", denial.Layer)
	}

	// If global rate limit is only 1s, wait briefly so s2 passes global check
	// but the per-skill rate limit for s2 should still be fresh
	gate.mu.Lock()
	gate.lastGlobalWake = time.Time{} // reset global for this test
	gate.mu.Unlock()

	ok, _ = gate.Allow("s2")
	if !ok {
		t.Error("expected s2 to be independently allowed")
	}
}

func TestWakeUpGate_EscalationCap(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 10)

	// Record many consecutive wakes to exceed the 16x cap
	for i := 0; i < 30; i++ {
		gate.RecordWakeUp("s1", 0)
	}

	gate.mu.RLock()
	factor := gate.skills["s1"].escalationFactor
	gate.mu.RUnlock()

	if factor > maxEscalationMultiplier {
		t.Errorf("escalation factor %f exceeded cap %f", factor, maxEscalationMultiplier)
	}
	if factor < maxEscalationMultiplier {
		t.Errorf("expected escalation factor to reach cap %f, got %f", maxEscalationMultiplier, factor)
	}
}

func TestWakeUpGate_EscalationAutoReset(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 10)

	// Build up escalation
	gate.RecordWakeUp("s1", 0)
	gate.RecordWakeUp("s1", 0)
	gate.RecordWakeUp("s1", 0)

	gate.mu.RLock()
	factorBefore := gate.skills["s1"].escalationFactor
	gate.mu.RUnlock()
	if factorBefore <= 1.0 {
		t.Fatalf("expected escalation factor > 1.0, got %f", factorBefore)
	}

	// Simulate time passing beyond the escalation reset window
	gate.mu.Lock()
	gate.skills["s1"].lastEscalationReset = time.Now().Add(-2 * time.Hour)
	gate.skills["s1"].lastWake = time.Now().Add(-2 * time.Hour)
	gate.lastGlobalWake = time.Time{}
	gate.mu.Unlock()

	// RecordWakeUp should detect the reset window has passed and reset
	gate.RecordWakeUp("s1", 0)

	gate.mu.RLock()
	factorAfter := gate.skills["s1"].escalationFactor
	gate.mu.RUnlock()
	// After reset, escalation should restart from a low factor
	if factorAfter >= factorBefore {
		t.Errorf("expected escalation to reset, before=%f after=%f", factorBefore, factorAfter)
	}
}

func TestWakeUpGate_StatsUnregisteredSkill(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled: true,
	}, nil, noopLogger())

	wakes, suppressed := gate.Stats("nonexistent")
	if wakes != 0 || suppressed != 0 {
		t.Errorf("expected 0/0 for unregistered skill, got %d/%d", wakes, suppressed)
	}
}

func TestWakeUpGate_RecordWakeUpUnregistered(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled: true,
	}, nil, noopLogger())
	// Should not panic
	gate.RecordWakeUp("nonexistent", 0.05)
}

func TestWakeUpGate_RecordSuppressedUnregistered(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled: true,
	}, nil, noopLogger())
	// Should not panic
	gate.RecordSuppressed("nonexistent")
}

func TestWakeUpGate_ShouldAutoDisableUnregistered(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:     true,
		MaxWakeUpsPerHour: 3,
	}, nil, noopLogger())
	// Unregistered skill should not trigger auto-disable
	if gate.ShouldAutoDisable("nonexistent") {
		t.Error("expected ShouldAutoDisable to return false for unregistered skill")
	}
}

func TestWakeUpGate_ResetEscalationUnregistered(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled: true,
	}, nil, noopLogger())
	// Should not panic
	gate.ResetEscalation("nonexistent")
}

func TestWakeUpGate_ConcurrentAccess(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   1000,
		MaxBudgetPerHourUSD: 100,
	}, nil, noopLogger())
	for i := 0; i < 10; i++ {
		gate.RegisterSkill(fmt.Sprintf("s%d", i), true, 1)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			skillID := fmt.Sprintf("s%d", idx)
			for j := 0; j < 50; j++ {
				gate.Allow(skillID)
				gate.RecordWakeUp(skillID, 0.001)
				gate.RecordSuppressed(skillID)
				gate.Stats(skillID)
				gate.ShouldAutoDisable(skillID)
			}
		}(i)
	}
	wg.Wait()
	// If we get here without a race panic or deadlock, the test passes
}

func TestWakeUpGate_CircuitBreakerBelowThreshold(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   10,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 1)

	// Record fewer wakes than the threshold
	gate.RecordWakeUp("s1", 0)
	gate.RecordWakeUp("s1", 0)

	if gate.ShouldAutoDisable("s1") {
		t.Error("expected circuit breaker NOT to trigger below threshold")
	}
}

func TestWakeUpGate_RegisterSkillOverwrite(t *testing.T) {
	gate := NewWakeUpGate(WakeUpGateConfig{
		GlobalEnabled:       true,
		GlobalRateLimitSecs: 1,
		MaxWakeUpsPerHour:   100,
	}, nil, noopLogger())
	gate.RegisterSkill("s1", true, 60)
	gate.RecordWakeUp("s1", 0.01)

	// Re-register with different settings
	gate.RegisterSkill("s1", false, 120)

	ok, denial := gate.Allow("s1")
	if ok {
		t.Error("expected denial after re-registering with wakeEnabled=false")
	}
	if denial.Layer != "skill_toggle" {
		t.Errorf("expected skill_toggle layer, got %q", denial.Layer)
	}
}

// ---------------------------------------------------------------------------
// ParseDaemonMessage — IPC security / edge-case tests
// ---------------------------------------------------------------------------

func TestParseDaemonMessage_OversizedJSON(t *testing.T) {
	// Build a message with a very large "message" field
	bigMsg := strings.Repeat("A", 1024*1024) // 1 MB
	line := fmt.Sprintf(`{"type":"log","message":"%s"}`, bigMsg)
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		// It's acceptable to reject oversized messages
		return
	}
	// If accepted, type should still be correct
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type log, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_DeeplyNestedJSON(t *testing.T) {
	// Build deeply nested JSON in "data" field
	nested := `{"a":`
	for i := 0; i < 100; i++ {
		nested += `{"b":`
	}
	nested += `"leaf"`
	for i := 0; i < 100; i++ {
		nested += `}`
	}
	nested += `}`
	line := fmt.Sprintf(`{"type":"wake_agent","message":"test","data":%s}`, nested)
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		// It's acceptable to reject deeply nested content
		return
	}
	if msg.Type != DaemonMsgWakeAgent {
		t.Errorf("expected type wake_agent, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_NullBytes(t *testing.T) {
	line := "{\"type\":\"log\",\"message\":\"hello\\u0000world\"}"
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		return // acceptable to reject
	}
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type log, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_ExtraUnknownFields(t *testing.T) {
	line := `{"type":"heartbeat","unknown_field":"should be ignored","another":42}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("extra fields should be silently ignored, got error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected heartbeat, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_TypeAsNumber(t *testing.T) {
	line := `{"type":42,"message":"type confusion"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for numeric type field")
	}
}

func TestParseDaemonMessage_TypeAsNull(t *testing.T) {
	line := `{"type":null,"message":"null type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for null type field")
	}
}

func TestParseDaemonMessage_TypeAsArray(t *testing.T) {
	line := `{"type":["wake_agent"],"message":"array type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for array type field")
	}
}

func TestParseDaemonMessage_EmptyType(t *testing.T) {
	line := `{"type":"","message":"empty type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for empty type string")
	}
}

func TestParseDaemonMessage_BinaryGarbage(t *testing.T) {
	line := "\x80\x81\x82\x83\xff\xfe"
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for binary garbage")
	}
}

func TestParseDaemonMessage_UnicodeMessage(t *testing.T) {
	line := `{"type":"log","message":"日本語テスト 🚀 Ümlauts"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error for unicode message: %v", err)
	}
	if !strings.Contains(msg.Message, "日本語") {
		t.Error("expected unicode content to be preserved")
	}
}

func TestParseDaemonMessage_DataAsString(t *testing.T) {
	// data should be json.RawMessage — a string is valid JSON
	line := `{"type":"wake_agent","message":"test","data":"just a string"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_DataAsArray(t *testing.T) {
	line := `{"type":"wake_agent","message":"test","data":[1,2,3]}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_TrailingComma(t *testing.T) {
	line := `{"type":"heartbeat","message":"test",}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for trailing comma (invalid JSON)")
	}
}

// ---------------------------------------------------------------------------
// EncodeDaemonCommand — additional tests
// ---------------------------------------------------------------------------

func TestEncodeDaemonCommand_StopRoundTrip(t *testing.T) {
	original := NewStopCommand("test_reason", 45)
	data, err := EncodeDaemonCommand(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != original.Type {
		t.Errorf("type mismatch: %q vs %q", decoded.Type, original.Type)
	}
	if decoded.Reason != original.Reason {
		t.Errorf("reason mismatch: %q vs %q", decoded.Reason, original.Reason)
	}
	if decoded.TimeoutSeconds != original.TimeoutSeconds {
		t.Errorf("timeout mismatch: %d vs %d", decoded.TimeoutSeconds, original.TimeoutSeconds)
	}
}

func TestEncodeDaemonCommand_ConfigUpdateRoundTrip(t *testing.T) {
	env := map[string]string{"A": "B", "KEY": "VALUE"}
	original := NewConfigUpdateCommand(env)
	data, err := EncodeDaemonCommand(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(decoded.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(decoded.Env))
	}
	if decoded.Env["A"] != "B" {
		t.Errorf("expected A=B, got %q", decoded.Env["A"])
	}
}

func TestEncodeDaemonCommand_ConfigUpdateNilEnv(t *testing.T) {
	cmd := NewConfigUpdateCommand(nil)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("should not fail for nil env: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty encoded output")
	}
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

// ---------------------------------------------------------------------------
// DaemonSupervisor — advanced tests
// ---------------------------------------------------------------------------

func TestDaemonSupervisor_StartDaemonNotFound(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	err := sv.StartDaemon("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent daemon")
	}
}

func TestDaemonSupervisor_ReenableDaemonNotFound(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	err := sv.ReenableDaemon("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent daemon")
	}
}

func TestDaemonSupervisor_GetDaemonStateNotFound(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	_, found := sv.GetDaemonState("nonexistent")
	if found {
		t.Error("expected found=false for nonexistent daemon")
	}
}

func TestDaemonSupervisor_BroadcastStatus(t *testing.T) {
	bc := &mockBroadcaster{}
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		bc,
		noopLogger(),
	)

	// Manually add a runner to test broadcastStatus
	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "s1",
		SkillName: "s1",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "s1", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	sv.mu.Lock()
	sv.runners["s1"] = runner
	sv.mu.Unlock()

	sv.broadcastStatus("s1", runner)

	if bc.eventCount() != 1 {
		t.Errorf("expected 1 broadcast event, got %d", bc.eventCount())
	}
	bc.mu.Lock()
	evt := bc.events[0]
	bc.mu.Unlock()
	if evt.Type != DaemonSSEStatus {
		t.Errorf("expected event type %q, got %q", DaemonSSEStatus, evt.Type)
	}
}

func TestDaemonSupervisor_BroadcastStatusNilBroadcaster(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil, // nil broadcaster
		noopLogger(),
	)

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "s1",
		SkillName: "s1",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "s1", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	// Should not panic
	sv.broadcastStatus("s1", runner)
}

func TestDaemonSupervisor_BuildWakeUpPrompt(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)

	event := daemonWakeEvent{
		SkillID:   "disk-monitor",
		SkillName: "disk-monitor",
		Message: DaemonMessage{
			Type:     DaemonMsgWakeAgent,
			Message:  "Disk / at 95%",
			Severity: "warning",
		},
		Timestamp: time.Now(),
	}

	prompt := sv.buildWakeUpPrompt(event)
	if !strings.Contains(prompt, "DAEMON EVENT") {
		t.Error("expected prompt to contain 'DAEMON EVENT'")
	}
	if !strings.Contains(prompt, "disk-monitor") {
		t.Error("expected prompt to contain skill name")
	}
	if !strings.Contains(prompt, "warning") {
		t.Error("expected prompt to contain severity")
	}
	if !strings.Contains(prompt, "Disk / at 95%") {
		t.Error("expected prompt to contain message")
	}
}

func TestDaemonSupervisor_BuildWakeUpPromptWithData(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)

	event := daemonWakeEvent{
		SkillID:   "test",
		SkillName: "test",
		Message: DaemonMessage{
			Type:     DaemonMsgWakeAgent,
			Message:  "alert",
			Severity: "critical",
			Data:     json.RawMessage(`{"cpu":99}`),
		},
		Timestamp: time.Now(),
	}

	prompt := sv.buildWakeUpPrompt(event)
	if !strings.Contains(prompt, "Additional data") {
		t.Error("expected prompt to contain 'Additional data'")
	}
	if !strings.Contains(prompt, "cpu") {
		t.Error("expected prompt to contain data content")
	}
}

func TestDaemonSupervisor_BuildWakeUpPromptDefaultSeverity(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)

	event := daemonWakeEvent{
		SkillID:   "test",
		SkillName: "test",
		Message: DaemonMessage{
			Type:    DaemonMsgWakeAgent,
			Message: "no severity set",
		},
		Timestamp: time.Now(),
	}

	prompt := sv.buildWakeUpPrompt(event)
	if !strings.Contains(prompt, "info") {
		t.Error("expected default severity 'info' in prompt")
	}
}

func TestDaemonSupervisor_AutoDisableDaemon(t *testing.T) {
	bc := &mockBroadcaster{}
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		bc,
		noopLogger(),
	)

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "noisy",
		SkillName: "noisy-skill",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "noisy-skill", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	sv.mu.Lock()
	sv.runners["noisy"] = runner
	sv.mu.Unlock()

	sv.autoDisableDaemon("noisy", "circuit breaker test")

	if runner.Status() != DaemonDisabled {
		t.Errorf("expected daemon to be disabled, got %s", runner.Status())
	}
	state := runner.State()
	if !state.AutoDisabled {
		t.Error("expected AutoDisabled=true")
	}

	// Check SSE broadcast
	if bc.eventCount() != 1 {
		t.Errorf("expected 1 SSE event from auto-disable, got %d", bc.eventCount())
	}
	bc.mu.Lock()
	evt := bc.events[0]
	bc.mu.Unlock()
	if evt.Type != DaemonSSEAutoDisabled {
		t.Errorf("expected event type %q, got %q", DaemonSSEAutoDisabled, evt.Type)
	}
}

func TestDaemonSupervisor_AutoDisableNonexistent(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: true},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	// Should not panic
	sv.autoDisableDaemon("nonexistent", "test")
}

func TestDaemonSupervisor_HandleWakeUpAllowed(t *testing.T) {
	bc := &mockBroadcaster{}
	taskDir := t.TempDir()
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{
			Enabled: true,
			WakeUpGate: WakeUpGateConfig{
				GlobalEnabled:       true,
				GlobalRateLimitSecs: 1,
				MaxWakeUpsPerHour:   100,
				MaxBudgetPerHourUSD: 10,
			},
		},
		nil, nil,
		NewBackgroundTaskManager(taskDir, noopLogger()),
		bc,
		noopLogger(),
	)

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "s1",
		SkillName: "s1",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "s1", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	sv.mu.Lock()
	sv.runners["s1"] = runner
	sv.mu.Unlock()
	sv.gate.RegisterSkill("s1", true, 1)

	event := daemonWakeEvent{
		SkillID:   "s1",
		SkillName: "s1",
		Message: DaemonMessage{
			Type:     DaemonMsgWakeAgent,
			Message:  "test wake",
			Severity: "info",
		},
		Timestamp: time.Now(),
	}

	sv.handleWakeUp(event)

	state := runner.State()
	if state.WakeUpCount != 1 {
		t.Errorf("expected WakeUpCount=1, got %d", state.WakeUpCount)
	}

	// Check SSE broadcast (should have wakeup event)
	if bc.eventCount() < 1 {
		t.Error("expected at least 1 broadcast event for allowed wake-up")
	}
}

func TestDaemonSupervisor_HandleWakeUpDenied(t *testing.T) {
	bc := &mockBroadcaster{}
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{
			Enabled: true,
			WakeUpGate: WakeUpGateConfig{
				GlobalEnabled:       false, // globally disabled
				GlobalRateLimitSecs: 1,
				MaxWakeUpsPerHour:   100,
			},
		},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		bc,
		noopLogger(),
	)

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "s1",
		SkillName: "s1",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "s1", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	sv.mu.Lock()
	sv.runners["s1"] = runner
	sv.mu.Unlock()
	sv.gate.RegisterSkill("s1", true, 1)

	event := daemonWakeEvent{
		SkillID:   "s1",
		SkillName: "s1",
		Message: DaemonMessage{
			Type:     DaemonMsgWakeAgent,
			Message:  "denied wake",
			Severity: "info",
		},
		Timestamp: time.Now(),
	}

	sv.handleWakeUp(event)

	state := runner.State()
	if state.WakeUpCount != 0 {
		t.Errorf("expected WakeUpCount=0 (denied), got %d", state.WakeUpCount)
	}
	if state.SuppressedCount != 1 {
		t.Errorf("expected SuppressedCount=1, got %d", state.SuppressedCount)
	}
}

func TestDaemonSupervisor_HandleWakeUpCircuitBreaker(t *testing.T) {
	bc := &mockBroadcaster{}
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{
			Enabled: true,
			WakeUpGate: WakeUpGateConfig{
				GlobalEnabled:       true,
				GlobalRateLimitSecs: 1,
				MaxWakeUpsPerHour:   2,
				MaxBudgetPerHourUSD: 10,
			},
		},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		bc,
		noopLogger(),
	)

	runner := NewDaemonRunner(DaemonRunnerConfig{
		SkillID:   "s1",
		SkillName: "s1",
		Config:    DaemonManifest{},
		Manifest:  SkillManifest{Name: "s1", Executable: "test.py"},
		Logger:    noopLogger(),
	})
	sv.mu.Lock()
	sv.runners["s1"] = runner
	sv.mu.Unlock()
	sv.gate.RegisterSkill("s1", true, 1)

	// Record enough wakes to exceed circuit breaker threshold
	for i := 0; i < 3; i++ {
		sv.gate.RecordWakeUp("s1", 0)
	}

	// Now a denied wake-up should trigger auto-disable
	sv.gate.mu.Lock()
	sv.gate.lastGlobalWake = time.Time{} // allow through global
	sv.gate.mu.Unlock()

	event := daemonWakeEvent{
		SkillID:   "s1",
		SkillName: "s1",
		Message: DaemonMessage{
			Type:     DaemonMsgWakeAgent,
			Message:  "storm",
			Severity: "info",
		},
		Timestamp: time.Now(),
	}

	sv.handleWakeUp(event)

	// The gate should deny and circuit breaker should trigger auto-disable
	if runner.Status() != DaemonDisabled {
		// It's possible the circuit breaker fired
		// Check if suppressed count went up
		state := runner.State()
		if state.SuppressedCount == 0 && !state.AutoDisabled {
			t.Error("expected either suppressed count > 0 or auto-disabled")
		}
	}
}

func TestDaemonSupervisor_ConcurrentOperations(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{
			Enabled: true,
			WakeUpGate: WakeUpGateConfig{
				GlobalEnabled:       true,
				GlobalRateLimitSecs: 1,
				MaxWakeUpsPerHour:   1000,
			},
		},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		&mockBroadcaster{},
		noopLogger(),
	)

	// Add several runners manually
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("s%d", i)
		runner := NewDaemonRunner(DaemonRunnerConfig{
			SkillID:   id,
			SkillName: id,
			Config:    DaemonManifest{},
			Manifest:  SkillManifest{Name: id, Executable: "test.py"},
			Logger:    noopLogger(),
		})
		sv.mu.Lock()
		sv.runners[id] = runner
		sv.mu.Unlock()
		sv.gate.RegisterSkill(id, true, 1)
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("s%d", idx)
			for j := 0; j < 20; j++ {
				_ = sv.ListDaemons()
				_, _ = sv.GetDaemonState(id)
				_ = sv.RunnerCount()
			}
		}(i)
	}
	wg.Wait()
	// No race/deadlock = pass
}

func TestDaemonSupervisor_RefreshSkillsDisabled(t *testing.T) {
	sv := NewDaemonSupervisor(
		DaemonSupervisorConfig{Enabled: false},
		nil, nil,
		NewBackgroundTaskManager(t.TempDir(), noopLogger()),
		nil,
		noopLogger(),
	)
	err := sv.RefreshSkills()
	if err != nil {
		t.Errorf("expected no error when disabled, got: %v", err)
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
