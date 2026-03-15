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
