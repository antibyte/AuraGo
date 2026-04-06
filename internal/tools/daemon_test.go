package tools

import (
	"encoding/json"
	"io"
	"log/slog"
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
	if d.WakeRateLimitSeconds != 3000 {
		t.Errorf("expected WakeRateLimitSeconds=3000, got %d", d.WakeRateLimitSeconds)
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
	// Config defaults should be applied
	if runner.config.MaxRestartAttempts != 3 {
		t.Errorf("expected MaxRestartAttempts=3 after defaults, got %d", runner.config.MaxRestartAttempts)
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
