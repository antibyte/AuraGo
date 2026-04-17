package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// DaemonSupervisor tests
// ---------------------------------------------------------------------------

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
