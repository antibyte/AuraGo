package tools

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aurago/internal/budget"
)

// Default WakeUpGate settings. These are overridden by config in production.
const (
	defaultGlobalRateLimitSecs  = 60
	defaultMaxBudgetPerHourUSD  = 0.50
	defaultMaxWakeUpsPerHour    = 6
	defaultEscalationResetHours = 1
	maxEscalationMultiplier     = 16.0
)

// WakeUpGateConfig holds configuration for the WakeUpGate.
type WakeUpGateConfig struct {
	GlobalEnabled       bool
	GlobalRateLimitSecs int
	MaxBudgetPerHourUSD float64
	MaxWakeUpsPerHour   int
}

// WakeUpGate controls whether daemon wake-up events are allowed to reach the agent.
// It implements 7 safety layers to prevent runaway token consumption.
type WakeUpGate struct {
	mu sync.RWMutex

	// Global toggles
	globalEnabled bool

	// Rate limiting
	globalRateLimit time.Duration
	lastGlobalWake  time.Time

	// Per-skill state
	skills map[string]*skillGateState

	// Budget integration
	budgetTracker     *budget.Tracker
	maxBudgetPerHour  float64
	costWindow        []costEntry // sliding window of wake-up costs
	maxWakeUpsPerHour int

	logger *slog.Logger
}

// skillGateState tracks per-skill rate limiting and escalation.
type skillGateState struct {
	wakeEnabled bool
	minInterval time.Duration // base rate limit from manifest
	lastWake    time.Time
	wakeCount   int // total allowed wake-ups
	suppressed  int // total suppressed wake-ups

	// Hourly window for circuit breaker
	hourlyWakes []time.Time

	// Escalation: consecutive rapid wake-ups increase the effective rate limit
	consecutiveWakes    int
	escalationFactor    float64
	lastEscalationReset time.Time
}

// costEntry records a single wake-up cost for the sliding budget window.
type costEntry struct {
	at   time.Time
	cost float64
}

// WakeUpDenial contains the reason a wake-up was denied.
type WakeUpDenial struct {
	Reason string
	Layer  string // which safety layer blocked it
}

func (d WakeUpDenial) Error() string {
	return fmt.Sprintf("%s: %s", d.Layer, d.Reason)
}

// NewWakeUpGate creates a new WakeUpGate with the given configuration.
func NewWakeUpGate(cfg WakeUpGateConfig, budgetTracker *budget.Tracker, logger *slog.Logger) *WakeUpGate {
	globalRateLimit := time.Duration(cfg.GlobalRateLimitSecs) * time.Second
	if globalRateLimit <= 0 {
		globalRateLimit = time.Duration(defaultGlobalRateLimitSecs) * time.Second
	}
	maxBudget := cfg.MaxBudgetPerHourUSD
	if maxBudget <= 0 {
		maxBudget = defaultMaxBudgetPerHourUSD
	}
	maxWakeUps := cfg.MaxWakeUpsPerHour
	if maxWakeUps <= 0 {
		maxWakeUps = defaultMaxWakeUpsPerHour
	}
	return &WakeUpGate{
		globalEnabled:     cfg.GlobalEnabled,
		globalRateLimit:   globalRateLimit,
		skills:            make(map[string]*skillGateState),
		budgetTracker:     budgetTracker,
		maxBudgetPerHour:  maxBudget,
		maxWakeUpsPerHour: maxWakeUps,
		logger:            logger,
	}
}

// RegisterSkill sets up rate limiting state for a daemon skill.
func (g *WakeUpGate) RegisterSkill(skillID string, wakeEnabled bool, rateLimitSeconds int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	interval := time.Duration(rateLimitSeconds) * time.Second
	if interval < g.globalRateLimit {
		interval = g.globalRateLimit
	}
	g.skills[skillID] = &skillGateState{
		wakeEnabled:         wakeEnabled,
		minInterval:         interval,
		escalationFactor:    1.0,
		lastEscalationReset: time.Now(),
	}
}

// UnregisterSkill removes a skill from the gate.
func (g *WakeUpGate) UnregisterSkill(skillID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.skills, skillID)
}

// Allow checks all 7 safety layers and returns whether the wake-up should proceed.
// If denied, returns a WakeUpDenial with the reason and blocking layer.
func (g *WakeUpGate) Allow(skillID string) (bool, *WakeUpDenial) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Layer 1: Global kill switch
	if !g.globalEnabled {
		return false, &WakeUpDenial{Layer: "global_toggle", Reason: "daemon wake-ups globally disabled"}
	}

	// Layer 2: Per-skill toggle
	skill, ok := g.skills[skillID]
	if !ok {
		return false, &WakeUpDenial{Layer: "skill_not_registered", Reason: fmt.Sprintf("skill %q not registered in wake-up gate", skillID)}
	}
	if !skill.wakeEnabled {
		return false, &WakeUpDenial{Layer: "skill_toggle", Reason: "wake_agent disabled for this skill"}
	}

	// Layer 3: Per-skill rate limit (with escalation)
	effectiveInterval := time.Duration(float64(skill.minInterval) * skill.escalationFactor)
	if !skill.lastWake.IsZero() && now.Sub(skill.lastWake) < effectiveInterval {
		remaining := effectiveInterval - now.Sub(skill.lastWake)
		return false, &WakeUpDenial{
			Layer:  "skill_rate_limit",
			Reason: fmt.Sprintf("per-skill rate limit: next allowed in %s (escalation: %.0fx)", remaining.Round(time.Second), skill.escalationFactor),
		}
	}

	// Layer 4: Global rate limit
	if !g.lastGlobalWake.IsZero() && now.Sub(g.lastGlobalWake) < g.globalRateLimit {
		remaining := g.globalRateLimit - now.Sub(g.lastGlobalWake)
		return false, &WakeUpDenial{
			Layer:  "global_rate_limit",
			Reason: fmt.Sprintf("global rate limit: next allowed in %s", remaining.Round(time.Second)),
		}
	}

	// Layer 5: Daily budget check
	if g.budgetTracker != nil {
		status := g.budgetTracker.GetStatus()
		if status.IsBlocked {
			return false, &WakeUpDenial{Layer: "daily_budget", Reason: "daily budget exhausted — all wake-ups blocked"}
		}
	}

	// Layer 6: Hourly wake-up budget (sliding window cost)
	oneHourAgo := now.Add(-1 * time.Hour)
	g.pruneOldCosts(oneHourAgo)
	var hourlyCost float64
	for _, entry := range g.costWindow {
		hourlyCost += entry.cost
	}
	if hourlyCost >= g.maxBudgetPerHour {
		return false, &WakeUpDenial{
			Layer:  "hourly_budget",
			Reason: fmt.Sprintf("hourly wake-up budget exceeded: $%.4f / $%.2f", hourlyCost, g.maxBudgetPerHour),
		}
	}

	// Layer 7: Circuit breaker — per-skill hourly wake-up count
	skill.pruneHourlyWakes(oneHourAgo)
	if len(skill.hourlyWakes) >= g.maxWakeUpsPerHour {
		return false, &WakeUpDenial{
			Layer:  "circuit_breaker",
			Reason: fmt.Sprintf("circuit breaker: skill exceeded %d wake-ups per hour", g.maxWakeUpsPerHour),
		}
	}

	return true, nil
}

// RecordWakeUp records a successful wake-up for tracking and escalation.
func (g *WakeUpGate) RecordWakeUp(skillID string, cost float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	g.lastGlobalWake = now
	g.costWindow = append(g.costWindow, costEntry{at: now, cost: cost})

	skill, ok := g.skills[skillID]
	if !ok {
		return
	}
	skill.lastWake = now
	skill.wakeCount++
	skill.hourlyWakes = append(skill.hourlyWakes, now)

	// Escalation: if waking up again before the base interval has fully elapsed,
	// double the effective rate limit (up to maxEscalationMultiplier)
	if skill.consecutiveWakes > 0 && skill.escalationFactor < maxEscalationMultiplier {
		skill.escalationFactor *= 2
		if skill.escalationFactor > maxEscalationMultiplier {
			skill.escalationFactor = maxEscalationMultiplier
		}
		g.logger.Info("Wake-up escalation increased",
			"skill_id", skillID,
			"factor", skill.escalationFactor,
			"consecutive", skill.consecutiveWakes+1,
		)
	}
	skill.consecutiveWakes++

	// Reset escalation after 1 hour of inactivity
	if now.Sub(skill.lastEscalationReset) > time.Duration(defaultEscalationResetHours)*time.Hour {
		skill.consecutiveWakes = 0
		skill.escalationFactor = 1.0
		skill.lastEscalationReset = now
	}
}

// RecordSuppressed records a suppressed wake-up.
func (g *WakeUpGate) RecordSuppressed(skillID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if skill, ok := g.skills[skillID]; ok {
		skill.suppressed++
	}
}

// ShouldAutoDisable checks if the circuit breaker threshold has been reached
// and the skill should be auto-disabled.
func (g *WakeUpGate) ShouldAutoDisable(skillID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	skill, ok := g.skills[skillID]
	if !ok {
		return false
	}
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	skill.pruneHourlyWakes(oneHourAgo)
	return len(skill.hourlyWakes) >= g.maxWakeUpsPerHour
}

// ResetEscalation resets the escalation multiplier for a skill (e.g., after re-enable).
func (g *WakeUpGate) ResetEscalation(skillID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if skill, ok := g.skills[skillID]; ok {
		skill.consecutiveWakes = 0
		skill.escalationFactor = 1.0
		skill.lastEscalationReset = time.Now()
	}
}

// SetGlobalEnabled toggles the global wake-up switch.
func (g *WakeUpGate) SetGlobalEnabled(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.globalEnabled = enabled
}

// Stats returns wake-up statistics for a skill.
func (g *WakeUpGate) Stats(skillID string) (wakes, suppressed int) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if skill, ok := g.skills[skillID]; ok {
		return skill.wakeCount, skill.suppressed
	}
	return 0, 0
}

// pruneOldCosts removes cost entries older than the cutoff.
func (g *WakeUpGate) pruneOldCosts(cutoff time.Time) {
	n := 0
	for _, entry := range g.costWindow {
		if entry.at.After(cutoff) {
			g.costWindow[n] = entry
			n++
		}
	}
	g.costWindow = g.costWindow[:n]
}

// pruneHourlyWakes removes wake timestamps older than the cutoff.
func (s *skillGateState) pruneHourlyWakes(cutoff time.Time) {
	n := 0
	for _, t := range s.hourlyWakes {
		if t.After(cutoff) {
			s.hourlyWakes[n] = t
			n++
		}
	}
	s.hourlyWakes = s.hourlyWakes[:n]
}
