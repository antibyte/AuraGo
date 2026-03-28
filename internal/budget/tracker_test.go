package budget

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func testConfig(limit float64, enforcement string) *config.Config {
	cfg := &config.Config{}
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = limit
	cfg.Budget.Enforcement = enforcement
	cfg.Budget.WarningThreshold = 0.8
	return cfg
}

func TestNewTracker_ClearsExceededOnHigherLimit(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create a tracker, record enough to exceed the limit
	cfg := testConfig(1.0, "warn")
	tr := NewTracker(cfg, logger, dir)
	if tr == nil {
		t.Fatal("tracker should not be nil")
	}

	// Manually set exceeded state
	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.persistLocked()
	tr.mu.Unlock()

	if !tr.IsExceeded() {
		t.Fatal("tracker should be exceeded")
	}

	// Re-create tracker with a higher limit — exceeded should be cleared
	cfg2 := testConfig(5.0, "warn")
	tr2 := NewTracker(cfg2, logger, dir)
	if tr2 == nil {
		t.Fatal("tracker should not be nil")
	}
	if tr2.IsExceeded() {
		t.Error("exceeded flag should be cleared when new limit > spent")
	}
}

func TestNewTracker_KeepsExceededIfStillOver(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(1.0, "warn")
	tr := NewTracker(cfg, logger, dir)

	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.persistLocked()
	tr.mu.Unlock()

	// Re-create with same limit — should stay exceeded
	tr2 := NewTracker(cfg, logger, dir)
	if !tr2.IsExceeded() {
		t.Error("exceeded flag should stay when spend >= limit")
	}
}

func TestIsBlocked_WarnMode(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(1.0, "warn")
	tr := NewTracker(cfg, logger, dir)

	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.mu.Unlock()

	// Warn mode should never block
	if tr.IsBlocked("chat") {
		t.Error("warn mode should not block chat")
	}
	if tr.IsBlocked("coagent") {
		t.Error("warn mode should not block coagent")
	}
}

func TestIsBlocked_FullMode(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(1.0, "full")
	tr := NewTracker(cfg, logger, dir)

	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.mu.Unlock()

	if !tr.IsBlocked("chat") {
		t.Error("full mode should block chat when exceeded")
	}
	if !tr.IsBlocked("coagent") {
		t.Error("full mode should block coagent when exceeded")
	}
}

func TestIsBlocked_PartialMode(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(1.0, "partial")
	tr := NewTracker(cfg, logger, dir)

	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.mu.Unlock()

	if tr.IsBlocked("chat") {
		t.Error("partial mode should not block chat")
	}
	if !tr.IsBlocked("coagent") {
		t.Error("partial mode should block coagent when exceeded")
	}
	if !tr.IsBlocked("vision") {
		t.Error("partial mode should block vision when exceeded")
	}
}

func TestGetStatus_IsBlockedReflectsEnforcement(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Warn mode: IsBlocked should be false even when exceeded
	cfg := testConfig(1.0, "warn")
	tr := NewTracker(cfg, logger, dir)
	tr.mu.Lock()
	tr.totalCostUSD = 1.5
	tr.exceeded = true
	tr.mu.Unlock()

	bs := tr.GetStatus()
	if bs.IsBlocked {
		t.Error("GetStatus().IsBlocked should be false in warn mode")
	}
	if !bs.IsExceeded {
		t.Error("GetStatus().IsExceeded should be true")
	}

	// Full mode: IsBlocked should be true when exceeded
	cfg2 := testConfig(1.0, "full")
	tr2 := NewTracker(cfg2, logger, dir)
	tr2.mu.Lock()
	tr2.totalCostUSD = 1.5
	tr2.exceeded = true
	tr2.mu.Unlock()

	bs2 := tr2.GetStatus()
	if !bs2.IsBlocked {
		t.Error("GetStatus().IsBlocked should be true in full mode when exceeded")
	}
}

func TestPersistAndLoad(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(10.0, "warn")
	tr := NewTracker(cfg, logger, dir)

	tr.mu.Lock()
	tr.totalCostUSD = 5.0
	tr.exceeded = false
	tr.inputTokens["test-model"] = 1000
	tr.outputTokens["test-model"] = 500
	tr.persistLocked()
	tr.mu.Unlock()

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "budget.json")); os.IsNotExist(err) {
		t.Fatal("budget.json should exist")
	}

	// Re-create and verify state loaded
	tr2 := NewTracker(cfg, logger, dir)
	tr2.mu.RLock()
	if tr2.totalCostUSD != 5.0 {
		t.Errorf("totalCostUSD = %f, want 5.0", tr2.totalCostUSD)
	}
	if tr2.inputTokens["test-model"] != 1000 {
		t.Errorf("input tokens = %d, want 1000", tr2.inputTokens["test-model"])
	}
	tr2.mu.RUnlock()
}

func TestRecordForCategoryTracksQuota(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(10.0, "warn")
	cfg.Budget.DefaultCost = config.ModelCostRates{
		InputPerMillion:  1000,
		OutputPerMillion: 1000,
	}
	tr := NewTracker(cfg, logger, dir)
	if tr == nil {
		t.Fatal("tracker should not be nil")
	}

	tr.RecordForCategory("coagent", "test-model", 3000, 0)

	if got := tr.CategorySpendUSD("coagent"); got <= 0 {
		t.Fatalf("CategorySpendUSD(coagent) = %f, want > 0", got)
	}
	if !tr.IsCategoryQuotaBlocked("coagent", 25) {
		t.Fatal("expected coagent quota to be blocked after spending beyond 25%")
	}
	if tr.IsCategoryQuotaBlocked("chat", 25) {
		t.Fatal("did not expect chat quota to be blocked")
	}

	tr2 := NewTracker(cfg, logger, dir)
	if got := tr2.CategorySpendUSD("coagent"); got <= 0 {
		t.Fatalf("persisted CategorySpendUSD(coagent) = %f, want > 0", got)
	}
}

func TestGetEffectiveDailyLimit_AdaptiveCapabilityWeighted(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(5.0, "warn")
	cfg.Budget.AdaptiveLimit.Enabled = true
	cfg.Budget.AdaptiveLimit.MinMultiplier = 1.0
	cfg.Budget.AdaptiveLimit.MaxMultiplier = 3.0
	cfg.Budget.AdaptiveLimit.NativeToolWeight = 0.10
	cfg.Budget.AdaptiveLimit.IntegrationWeight = 0.25
	cfg.Budget.AdaptiveLimit.CoAgentWeight = 0.50
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.Notes.Enabled = true
	cfg.Docker.Enabled = true
	cfg.CoAgents.Enabled = true

	tr := NewTracker(cfg, logger, dir)
	if tr == nil {
		t.Fatal("tracker should not be nil")
	}

	got := tr.GetEffectiveDailyLimit()
	want := 9.75 // 5.0 * (1 + 0.10 + 0.10 + 0.25 + 0.50)
	if got != want {
		t.Fatalf("GetEffectiveDailyLimit() = %v, want %v", got, want)
	}
}

func TestIsCategoryQuotaBlocked_UsesEffectiveLimit(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(10.0, "warn")
	cfg.Budget.DefaultCost = config.ModelCostRates{
		InputPerMillion:  1000,
		OutputPerMillion: 1000,
	}
	cfg.Budget.AdaptiveLimit.Enabled = true
	cfg.Budget.AdaptiveLimit.MinMultiplier = 1.0
	cfg.Budget.AdaptiveLimit.MaxMultiplier = 3.0
	cfg.Budget.AdaptiveLimit.CoAgentWeight = 1.0
	cfg.CoAgents.Enabled = true

	tr := NewTracker(cfg, logger, dir)
	if tr == nil {
		t.Fatal("tracker should not be nil")
	}

	// Costs $3.00. With base limit $10.00 and 25% quota this would be blocked.
	// With the adaptive effective limit of $20.00, the same 25% quota should not block.
	tr.RecordForCategory("coagent", "test-model", 3000, 0)

	if tr.IsCategoryQuotaBlocked("coagent", 25) {
		t.Fatal("expected coagent quota to use adaptive effective limit")
	}
}

func TestGetStatus_ReportsAdaptiveFields(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := testConfig(4.0, "warn")
	cfg.Budget.AdaptiveLimit.Enabled = true
	cfg.Budget.AdaptiveLimit.MinMultiplier = 1.0
	cfg.Budget.AdaptiveLimit.MaxMultiplier = 3.0
	cfg.Budget.AdaptiveLimit.NativeToolWeight = 0.10
	cfg.Tools.Memory.Enabled = true

	tr := NewTracker(cfg, logger, dir)
	if tr == nil {
		t.Fatal("tracker should not be nil")
	}

	status := tr.GetStatus()
	if !status.AdaptiveEnabled {
		t.Fatal("expected AdaptiveEnabled to be true")
	}
	if status.BaseDailyLimitUSD != 4.0 {
		t.Fatalf("BaseDailyLimitUSD = %v, want 4.0", status.BaseDailyLimitUSD)
	}
	if status.EffectiveLimitUSD != 4.4 {
		t.Fatalf("EffectiveLimitUSD = %v, want 4.4", status.EffectiveLimitUSD)
	}
	if status.DailyLimit != status.EffectiveLimitUSD {
		t.Fatalf("DailyLimit = %v, want effective limit %v", status.DailyLimit, status.EffectiveLimitUSD)
	}
	if status.AdaptiveMultiplier != 1.1 {
		t.Fatalf("AdaptiveMultiplier = %v, want 1.1", status.AdaptiveMultiplier)
	}
	if status.AdaptiveBreakdown.NativeCapabilities != 1 {
		t.Fatalf("AdaptiveBreakdown.NativeCapabilities = %d, want 1", status.AdaptiveBreakdown.NativeCapabilities)
	}
}
