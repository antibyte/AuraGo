package budget

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

// BudgetStatus is the public snapshot returned by GetStatus().
type BudgetStatus struct {
	Event        string                `json:"event"` // always "budget_update"
	Enabled      bool                  `json:"enabled"`
	DailyLimit   float64               `json:"daily_limit_usd"`
	SpentUSD     float64               `json:"spent_usd"`
	RemainingUSD float64               `json:"remaining_usd"`
	Percentage   float64               `json:"percentage"` // 0.0–1.0
	Enforcement  string                `json:"enforcement"`
	IsWarning    bool                  `json:"is_warning"`
	IsExceeded   bool                  `json:"is_exceeded"`
	IsBlocked    bool                  `json:"is_blocked"` // true when enforcement=full + exceeded
	ResetTime    string                `json:"reset_time"` // RFC3339
	Date         string                `json:"date"`
	Models       map[string]ModelUsage `json:"models"`
}

// ModelUsage tracks per-model token usage and cost.
type ModelUsage struct {
	InputTokens     int     `json:"input_tokens"`
	OutputTokens    int     `json:"output_tokens"`
	CostUSD         float64 `json:"cost_usd"`
	Calls           int     `json:"calls"`
	AvgInputTokens  int     `json:"avg_input_tokens"`
	AvgOutputTokens int     `json:"avg_output_tokens"`
}

// persistedState is the JSON structure saved to disk.
type persistedState struct {
	Date         string             `json:"date"`
	TotalCostUSD float64            `json:"total_cost_usd"`
	CategoryCost map[string]float64 `json:"category_cost,omitempty"`
	InputTokens  map[string]int     `json:"input_tokens"`
	OutputTokens map[string]int     `json:"output_tokens"`
	CallCounts   map[string]int     `json:"call_counts"`
	WarningsSent int                `json:"warnings_sent"`
	Exceeded     bool               `json:"exceeded"`
}

// Tracker is the central budget tracking singleton.
type Tracker struct {
	mu sync.RWMutex

	cfg    *config.Config
	logger *slog.Logger

	// Daily counters
	date         string // "2006-01-02"
	totalCostUSD float64
	categoryCost map[string]float64
	inputTokens  map[string]int
	outputTokens map[string]int
	callCounts   map[string]int
	warningsSent int
	exceeded     bool

	persistPath string

	// Optional mission trigger callback: eventType is "budget_warning" or "budget_exceeded"
	missionCallback func(eventType string, spentUSD, limitUSD, percentage float64)

	// Debounced persistence: timer fires actual disk write at most every 3 s
	persistTimer *time.Timer
}

// NewTracker creates a budget tracker. If budget is disabled in config, returns nil.
func NewTracker(cfg *config.Config, logger *slog.Logger, dataDir string) *Tracker {
	if !cfg.Budget.Enabled {
		return nil
	}

	t := &Tracker{
		cfg:          cfg,
		logger:       logger,
		categoryCost: make(map[string]float64),
		inputTokens:  make(map[string]int),
		outputTokens: make(map[string]int),
		callCounts:   make(map[string]int),
		persistPath:  filepath.Join(dataDir, "budget.json"),
	}

	t.load()

	// Check if we need to reset for a new day
	today := t.todayStr()
	if t.date != today {
		t.resetForNewDayLocked(today)
		t.doPersistLocked()
	}

	// Re-evaluate exceeded flag: if the effective limit was raised above current spend,
	// clear the exceeded state so the new limit takes effect immediately.
	if t.exceeded && cfg.Budget.DailyLimitUSD > 0 && t.totalCostUSD < cfg.Budget.DailyLimitUSD {
		t.exceeded = false
		t.doPersistLocked()
		logger.Info("[Budget] Exceeded flag cleared (limit raised above current spend)",
			"spent", t.totalCostUSD, "new_limit", cfg.Budget.DailyLimitUSD)
	}

	logger.Info("[Budget] Tracker initialized",
		"daily_limit", cfg.Budget.DailyLimitUSD,
		"enforcement", cfg.Budget.Enforcement,
		"spent_today", t.totalCostUSD,
	)

	return t
}

// SetMissionCallback sets the callback for budget threshold mission triggers.
func (t *Tracker) SetMissionCallback(cb func(eventType string, spentUSD, limitUSD, percentage float64)) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.missionCallback = cb
}

// Record logs token usage for a model after an LLM call.
// Returns true if a warning threshold was just crossed.
func (t *Tracker) Record(model string, inputTokens, outputTokens int) bool {
	return t.RecordForCategory("chat", model, inputTokens, outputTokens)
}

// RecordForCategory logs token usage for a specific execution category such as
// "chat" or "coagent". Category costs are persisted and can be used for quota checks.
func (t *Tracker) RecordForCategory(category, model string, inputTokens, outputTokens int) bool {
	if t == nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Auto-reset on day boundary
	today := t.todayStr()
	if t.date != today {
		t.logger.Info("[Budget] Day rolled over, resetting counters", "old_date", t.date, "new_date", today)
		t.resetForNewDayLocked(today)
	}

	t.inputTokens[model] += inputTokens
	t.outputTokens[model] += outputTokens
	t.callCounts[model]++

	cost := t.calcCostLocked(model, inputTokens, outputTokens)
	t.totalCostUSD += cost
	if category != "" {
		t.categoryCost[strings.ToLower(category)] += cost
	}

	limit := t.cfg.Budget.DailyLimitUSD
	crossedWarning := false
	crossedExceeded := false

	if limit > 0 {
		pct := t.totalCostUSD / limit
		threshold := t.cfg.Budget.WarningThreshold

		// Check warning threshold
		if pct >= threshold && t.warningsSent == 0 {
			t.warningsSent = 1
			crossedWarning = true
			t.logger.Warn("[Budget] Warning threshold crossed",
				"spent", t.totalCostUSD, "limit", limit, "pct", pct)
		}

		// Check exceeded
		if pct >= 1.0 && !t.exceeded {
			t.exceeded = true
			crossedExceeded = true
			t.logger.Warn("[Budget] Daily budget EXCEEDED",
				"spent", t.totalCostUSD, "limit", limit, "enforcement", t.cfg.Budget.Enforcement)
		}
	}

	cb := t.missionCallback
	spent := t.totalCostUSD
	t.persistLocked()

	// Fire mission callbacks outside the hot path (non-blocking)
	if cb != nil {
		if crossedWarning {
			go cb("budget_warning", spent, limit, spent/limit)
		}
		if crossedExceeded {
			go cb("budget_exceeded", spent, limit, spent/limit)
		}
	}

	return crossedWarning
}

// RecordCost adds a direct cost to the daily budget (e.g. image generation).
// Use RecordCostForCategory to also track the cost under a named category.
func (t *Tracker) RecordCost(costUSD float64) {
	t.RecordCostForCategory("", costUSD)
}

// RecordCostForCategory adds a direct cost and attributes it to a named category
// (e.g. "image_generation"). It also fires mission callbacks for warning/exceeded thresholds.
func (t *Tracker) RecordCostForCategory(category string, costUSD float64) {
	if t == nil || costUSD <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	today := t.todayStr()
	if t.date != today {
		t.resetForNewDayLocked(today)
	}

	t.totalCostUSD += costUSD
	if category != "" {
		t.categoryCost[strings.ToLower(category)] += costUSD
	}

	limit := t.cfg.Budget.DailyLimitUSD
	crossedWarning := false
	crossedExceeded := false

	if limit > 0 {
		pct := t.totalCostUSD / limit
		threshold := t.cfg.Budget.WarningThreshold

		if pct >= threshold && t.warningsSent == 0 {
			t.warningsSent = 1
			crossedWarning = true
			t.logger.Warn("[Budget] Warning threshold crossed",
				"spent", t.totalCostUSD, "limit", limit, "pct", pct)
		}

		if pct >= 1.0 && !t.exceeded {
			t.exceeded = true
			crossedExceeded = true
			t.logger.Warn("[Budget] Daily budget EXCEEDED",
				"spent", t.totalCostUSD, "limit", limit, "category", category)
		}
	}

	cb := t.missionCallback
	spent := t.totalCostUSD
	// Use debounced persist for the LLM-cost hot path; immediate on threshold crossings.
	if crossedWarning || crossedExceeded {
		t.persistLocked()
	} else {
		t.schedulePersistLocked()
	}

	if cb != nil {
		if crossedWarning {
			go cb("budget_warning", spent, limit, spent/limit)
		}
		if crossedExceeded {
			go cb("budget_exceeded", spent, limit, spent/limit)
		}
	}
}

// CategorySpendUSD returns the tracked spend for one execution category on the current day.
func (t *Tracker) CategorySpendUSD(category string) float64 {
	if t == nil {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.categoryCost[strings.ToLower(category)]
}

// IsCategoryQuotaBlocked reports whether a category has exhausted its reserved share
// of the daily budget. A quotaPercent <= 0 disables the quota.
func (t *Tracker) IsCategoryQuotaBlocked(category string, quotaPercent int) bool {
	if t == nil || quotaPercent <= 0 {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.cfg.Budget.DailyLimitUSD <= 0 {
		return false
	}
	limit := t.cfg.Budget.DailyLimitUSD * (float64(quotaPercent) / 100.0)
	if limit <= 0 {
		return false
	}
	return t.categoryCost[strings.ToLower(category)] >= limit
}

// IsBlocked returns true if the given category is blocked by budget enforcement.
// category: "chat", "coagent", "vision", "stt", "image_generation"
func (t *Tracker) IsBlocked(category string) bool {
	if t == nil {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if !t.exceeded {
		return false
	}

	enforcement := strings.ToLower(t.cfg.Budget.Enforcement)
	switch enforcement {
	case "full":
		return true // everything blocked
	case "partial":
		// block co-agents, vision, stt — but not main chat
		return category != "chat"
	default: // "warn"
		return false
	}
}

// IsExceeded returns whether the daily budget has been exceeded.
func (t *Tracker) IsExceeded() bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.exceeded
}

// GetPromptHint returns a budget warning string to inject into the system prompt,
// or "" if no warning is needed.
func (t *Tracker) GetPromptHint() string {
	if t == nil {
		return ""
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	limit := t.cfg.Budget.DailyLimitUSD
	if limit <= 0 {
		return ""
	}

	pct := t.totalCostUSD / limit
	threshold := t.cfg.Budget.WarningThreshold

	if pct < threshold {
		return ""
	}

	return fmt.Sprintf(
		"[BUDGET WARNING: %.0f%% of daily budget ($%.4f/$%.2f) used. Be token-efficient. Prefer concise answers. Avoid unnecessary tool calls.]",
		pct*100, t.totalCostUSD, limit,
	)
}

// GetStatus returns a snapshot of the current budget state.
func (t *Tracker) GetStatus() BudgetStatus {
	if t == nil {
		return BudgetStatus{Event: "budget_update", Enabled: false}
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	limit := t.cfg.Budget.DailyLimitUSD
	remaining := limit - t.totalCostUSD
	if remaining < 0 {
		remaining = 0
	}

	var pct float64
	if limit > 0 {
		pct = t.totalCostUSD / limit
		if pct > 1 {
			pct = 1
		}
	}

	enforcement := strings.ToLower(t.cfg.Budget.Enforcement)
	isBlocked := t.exceeded && enforcement == "full"

	models := make(map[string]ModelUsage)
	allModels := make(map[string]bool)
	for m := range t.inputTokens {
		allModels[m] = true
	}
	for m := range t.outputTokens {
		allModels[m] = true
	}
	for m := range allModels {
		in := t.inputTokens[m]
		out := t.outputTokens[m]
		calls := t.callCounts[m]
		avgIn, avgOut := 0, 0
		if calls > 0 {
			avgIn = in / calls
			avgOut = out / calls
		}
		models[m] = ModelUsage{
			InputTokens:     in,
			OutputTokens:    out,
			CostUSD:         t.calcCostLocked(m, in, out),
			Calls:           calls,
			AvgInputTokens:  avgIn,
			AvgOutputTokens: avgOut,
		}
	}

	return BudgetStatus{
		Event:        "budget_update",
		Enabled:      true,
		DailyLimit:   limit,
		SpentUSD:     t.totalCostUSD,
		RemainingUSD: remaining,
		Percentage:   pct,
		Enforcement:  enforcement,
		IsWarning:    pct >= t.cfg.Budget.WarningThreshold,
		IsExceeded:   t.exceeded,
		IsBlocked:    isBlocked,
		ResetTime:    t.nextResetTime().Format(time.RFC3339),
		Date:         t.date,
		Models:       models,
	}
}

// GetStatusJSON returns the status as a JSON string ready for SSE.
func (t *Tracker) GetStatusJSON() string {
	bs := t.GetStatus()
	data, err := json.Marshal(bs)
	if err != nil {
		t.logger.Error("[Budget] Failed to marshal status JSON", "error", err)
		return `{"event":"budget_update","enabled":true,"error":"marshal_failed"}`
	}
	return string(data)
}

// FormatStatusText returns a human-readable budget summary for /budget command.
func (t *Tracker) FormatStatusText(lang string) string {
	if t == nil {
		if strings.Contains(strings.ToLower(lang), "de") || lang == "German" {
			return "💰 Budget-System ist deaktiviert."
		}
		return "💰 Budget system is disabled."
	}

	bs := t.GetStatus()
	if !bs.Enabled {
		if strings.Contains(strings.ToLower(lang), "de") || lang == "German" {
			return "💰 Budget-System ist deaktiviert."
		}
		return "💰 Budget system is disabled."
	}

	isDE := strings.Contains(strings.ToLower(lang), "de") || lang == "German"

	var sb strings.Builder
	pctInt := int(bs.Percentage * 100)

	if isDE {
		sb.WriteString(fmt.Sprintf("💰 **Budget:** $%.4f / $%.2f (%d%%)\n", bs.SpentUSD, bs.DailyLimit, pctInt))
	} else {
		sb.WriteString(fmt.Sprintf("💰 **Budget:** $%.4f / $%.2f (%d%%)\n", bs.SpentUSD, bs.DailyLimit, pctInt))
	}

	// Per-model breakdown
	for model, usage := range bs.Models {
		inK := float64(usage.InputTokens) / 1000
		outK := float64(usage.OutputTokens) / 1000
		sb.WriteString(fmt.Sprintf("├─ %s: %.1fK in / %.1fK out ($%.4f)\n", model, inK, outK, usage.CostUSD))
	}

	if isDE {
		sb.WriteString(fmt.Sprintf("├─ Modus: **%s**", bs.Enforcement))
		switch bs.Enforcement {
		case "warn":
			sb.WriteString(" (nur Warnung)")
		case "partial":
			sb.WriteString(" (Co-Agents + Vision/STT gesperrt bei Überschreitung)")
		case "full":
			sb.WriteString(" (alles gesperrt bei Überschreitung)")
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("└─ Reset: %s", bs.ResetTime))
	} else {
		sb.WriteString(fmt.Sprintf("├─ Mode: **%s**", bs.Enforcement))
		switch bs.Enforcement {
		case "warn":
			sb.WriteString(" (warning only)")
		case "partial":
			sb.WriteString(" (co-agents + vision/STT blocked when exceeded)")
		case "full":
			sb.WriteString(" (all blocked when exceeded)")
		}
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("└─ Reset: %s", bs.ResetTime))
	}

	if bs.IsExceeded {
		if isDE {
			sb.WriteString("\n\n⛔ **Budget überschritten!**")
		} else {
			sb.WriteString("\n\n⛔ **Budget exceeded!**")
		}
	} else if bs.IsWarning {
		if isDE {
			sb.WriteString("\n\n⚠️ **Budget-Warnung!**")
		} else {
			sb.WriteString("\n\n⚠️ **Budget warning!**")
		}
	}

	return sb.String()
}

// --- Internal helpers ---

func (t *Tracker) calcCostLocked(model string, inputTokens, outputTokens int) float64 {
	rates := t.findRatesLocked(model)
	return (float64(inputTokens)/1_000_000)*rates.InputPerMillion +
		(float64(outputTokens)/1_000_000)*rates.OutputPerMillion
}

func (t *Tracker) findRatesLocked(model string) config.ModelCostRates {
	lowerModel := strings.ToLower(model)

	// 1) Search per-provider model costs
	for _, p := range t.cfg.Providers {
		for _, m := range p.Models {
			if strings.ToLower(m.Name) == lowerModel {
				return config.ModelCostRates{
					InputPerMillion:  m.InputPerMillion,
					OutputPerMillion: m.OutputPerMillion,
				}
			}
		}
	}

	// 2) Legacy fallback: budget.models
	for _, m := range t.cfg.Budget.Models {
		if strings.ToLower(m.Name) == lowerModel {
			return config.ModelCostRates{
				InputPerMillion:  m.InputPerMillion,
				OutputPerMillion: m.OutputPerMillion,
			}
		}
	}

	// 3) Static pricing tables for known models of direct providers.
	// Iterate configured providers to determine the provider type for this request.
	for _, p := range t.cfg.Providers {
		if pricing, ok := llm.StaticPricingForModel(p.Type, model); ok &&
			(pricing.InputPerMillion > 0 || pricing.OutputPerMillion > 0) {
			return config.ModelCostRates{
				InputPerMillion:  pricing.InputPerMillion,
				OutputPerMillion: pricing.OutputPerMillion,
			}
		}
	}

	// 4) Global default
	return t.cfg.Budget.DefaultCost
}

func (t *Tracker) todayStr() string {
	return time.Now().Format("2006-01-02")
}

func (t *Tracker) nextResetTime() time.Time {
	now := time.Now()
	hour := t.cfg.Budget.ResetHour
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// resetForNewDayLocked zeroes all daily counters. Must be called with t.mu held.
func (t *Tracker) resetForNewDayLocked(today string) {
	t.date = today
	t.totalCostUSD = 0
	t.categoryCost = make(map[string]float64)
	t.inputTokens = make(map[string]int)
	t.outputTokens = make(map[string]int)
	t.callCounts = make(map[string]int)
	t.warningsSent = 0
	t.exceeded = false
}

// persistLocked writes the budget state to disk immediately.
// Must be called with t.mu held (write lock).
func (t *Tracker) persistLocked() {
	if t.persistTimer != nil {
		t.persistTimer.Stop()
		t.persistTimer = nil
	}
	t.doPersistLocked()
}

// schedulePersistLocked schedules a debounced disk write (at most every 3 s).
// Prefer this on the hot recording path to avoid per-call I/O.
// Must be called with t.mu held (write lock).
func (t *Tracker) schedulePersistLocked() {
	if t.persistTimer != nil {
		return // write already scheduled
	}
	t.persistTimer = time.AfterFunc(3*time.Second, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.persistTimer = nil
		t.doPersistLocked()
	})
}

// Flush forces an immediate disk write, bypassing the debounce timer.
// Call this during graceful shutdown to avoid losing the last few recordings.
func (t *Tracker) Flush() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.persistTimer != nil {
		t.persistTimer.Stop()
		t.persistTimer = nil
	}
	t.doPersistLocked()
}

// doPersistLocked writes the budget state to disk. Must be called with t.mu held.
func (t *Tracker) doPersistLocked() {
	state := persistedState{
		Date:         t.date,
		TotalCostUSD: t.totalCostUSD,
		CategoryCost: t.categoryCost,
		InputTokens:  t.inputTokens,
		OutputTokens: t.outputTokens,
		CallCounts:   t.callCounts,
		WarningsSent: t.warningsSent,
		Exceeded:     t.exceeded,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.logger.Error("[Budget] Failed to marshal state", "error", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(t.persistPath), 0755); err != nil {
		t.logger.Error("[Budget] Failed to create data dir", "error", err)
		return
	}

	if err := os.WriteFile(t.persistPath, data, 0644); err != nil {
		t.logger.Error("[Budget] Failed to persist state", "error", err)
	}
}

func (t *Tracker) load() {
	data, err := os.ReadFile(t.persistPath)
	if err != nil {
		// No saved state — start fresh
		t.date = t.todayStr()
		return
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		t.logger.Warn("[Budget] Failed to parse saved state, starting fresh", "error", err)
		t.date = t.todayStr()
		return
	}

	t.date = state.Date
	t.totalCostUSD = state.TotalCostUSD
	t.warningsSent = state.WarningsSent
	t.exceeded = state.Exceeded

	if state.InputTokens != nil {
		t.inputTokens = state.InputTokens
	}
	if state.CategoryCost != nil {
		t.categoryCost = state.CategoryCost
	} else {
		t.categoryCost = make(map[string]float64)
	}
	if state.OutputTokens != nil {
		t.outputTokens = state.OutputTokens
	}
	if state.CallCounts != nil {
		t.callCounts = state.CallCounts
	} else {
		t.callCounts = make(map[string]int)
	}
}
