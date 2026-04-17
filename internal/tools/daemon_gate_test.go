package tools

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

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
