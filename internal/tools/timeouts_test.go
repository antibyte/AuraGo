package tools

import (
	"testing"
	"time"
)

func TestConfigureTimeouts_SetsValues(t *testing.T) {
	// Save originals
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	origBackground := GetBackgroundTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
		SetBackgroundTimeout(origBackground)
	}()

	ConfigureTimeouts(60, 300, 1800)

	if GetForegroundTimeout() != 60*time.Second {
		t.Errorf("expected ForegroundTimeout=60s, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 300*time.Second {
		t.Errorf("expected SkillTimeout=300s, got %v", GetSkillTimeout())
	}
	if GetBackgroundTimeout() != 1800*time.Second {
		t.Errorf("expected BackgroundTimeout=1800s, got %v", GetBackgroundTimeout())
	}
}

func TestConfigureTimeouts_ZeroKeepsDefaults(t *testing.T) {
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	origBackground := GetBackgroundTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
		SetBackgroundTimeout(origBackground)
	}()

	SetForegroundTimeout(30 * time.Second)
	SetSkillTimeout(120 * time.Second)
	SetBackgroundTimeout(time.Hour)

	ConfigureTimeouts(0, 0, 0)

	if GetForegroundTimeout() != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged at 30s, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged at 120s, got %v", GetSkillTimeout())
	}
	if GetBackgroundTimeout() != time.Hour {
		t.Errorf("expected BackgroundTimeout unchanged at 1h, got %v", GetBackgroundTimeout())
	}
}

func TestGetTimeoutConfig_ReturnsAllCategories(t *testing.T) {
	cfg := GetTimeoutConfig()
	if cfg.Foreground != GetForegroundTimeout() {
		t.Errorf("Foreground mismatch: got %v, want %v", cfg.Foreground, GetForegroundTimeout())
	}
	if cfg.Skills != GetSkillTimeout() {
		t.Errorf("Skills mismatch: got %v, want %v", cfg.Skills, GetSkillTimeout())
	}
	if cfg.Background != GetBackgroundTimeout() {
		t.Errorf("Background mismatch: got %v, want %v", cfg.Background, GetBackgroundTimeout())
	}
	if cfg.Sandbox != GetSandboxTimeout() {
		t.Errorf("Sandbox mismatch: got %v, want %v", cfg.Sandbox, GetSandboxTimeout())
	}
	if cfg.Network != GetNetworkTimeout() {
		t.Errorf("Network mismatch: got %v, want %v", cfg.Network, GetNetworkTimeout())
	}
}

func TestConfigureAllTimeouts_SetsAllValues(t *testing.T) {
	origFg := GetForegroundTimeout()
	origSk := GetSkillTimeout()
	origBg := GetBackgroundTimeout()
	origSb := GetSandboxTimeout()
	origNw := GetNetworkTimeout()
	defer func() {
		SetForegroundTimeout(origFg)
		SetSkillTimeout(origSk)
		SetBackgroundTimeout(origBg)
		SetSandboxTimeout(origSb)
		SetNetworkTimeout(origNw)
	}()

	cfg := TimeoutConfig{
		Foreground: 45 * time.Second,
		Skills:     200 * time.Second,
		Background: 2 * time.Hour,
		Sandbox:    60 * time.Second,
		Network:    90 * time.Second,
	}
	ConfigureAllTimeouts(cfg)

	if GetForegroundTimeout() != 45*time.Second {
		t.Errorf("Foreground: got %v, want 45s", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 200*time.Second {
		t.Errorf("Skills: got %v, want 200s", GetSkillTimeout())
	}
	if GetBackgroundTimeout() != 2*time.Hour {
		t.Errorf("Background: got %v, want 2h", GetBackgroundTimeout())
	}
	if GetSandboxTimeout() != 60*time.Second {
		t.Errorf("Sandbox: got %v, want 60s", GetSandboxTimeout())
	}
	if GetNetworkTimeout() != 90*time.Second {
		t.Errorf("Network: got %v, want 90s", GetNetworkTimeout())
	}
}

func TestConfigureAllTimeouts_ZeroPreservesDefaults(t *testing.T) {
	origFg := GetForegroundTimeout()
	origSk := GetSkillTimeout()
	origBg := GetBackgroundTimeout()
	origSb := GetSandboxTimeout()
	origNw := GetNetworkTimeout()
	defer func() {
		SetForegroundTimeout(origFg)
		SetSkillTimeout(origSk)
		SetBackgroundTimeout(origBg)
		SetSandboxTimeout(origSb)
		SetNetworkTimeout(origNw)
	}()

	// Set to known values
	SetForegroundTimeout(45 * time.Second)
	SetSkillTimeout(200 * time.Second)
	SetBackgroundTimeout(2 * time.Hour)
	SetSandboxTimeout(60 * time.Second)
	SetNetworkTimeout(90 * time.Second)

	// Apply zero config - should preserve all values
	ConfigureAllTimeouts(TimeoutConfig{})

	if GetForegroundTimeout() != 45*time.Second {
		t.Errorf("Foreground should be preserved: got %v, want 45s", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 200*time.Second {
		t.Errorf("Skills should be preserved: got %v, want 200s", GetSkillTimeout())
	}
	if GetBackgroundTimeout() != 2*time.Hour {
		t.Errorf("Background should be preserved: got %v, want 2h", GetBackgroundTimeout())
	}
	if GetSandboxTimeout() != 60*time.Second {
		t.Errorf("Sandbox should be preserved: got %v, want 60s", GetSandboxTimeout())
	}
	if GetNetworkTimeout() != 90*time.Second {
		t.Errorf("Network should be preserved: got %v, want 90s", GetNetworkTimeout())
	}
}

func TestDefaultTimeoutValues(t *testing.T) {
	if DefaultForegroundTimeout != 30*time.Second {
		t.Errorf("DefaultForegroundTimeout: got %v, want 30s", DefaultForegroundTimeout)
	}
	if DefaultSkillTimeout != 120*time.Second {
		t.Errorf("DefaultSkillTimeout: got %v, want 120s", DefaultSkillTimeout)
	}
	if DefaultBackgroundTimeout != 1*time.Hour {
		t.Errorf("DefaultBackgroundTimeout: got %v, want 1h", DefaultBackgroundTimeout)
	}
	if DefaultSandboxTimeout != 30*time.Second {
		t.Errorf("DefaultSandboxTimeout: got %v, want 30s", DefaultSandboxTimeout)
	}
	if DefaultNetworkTimeout != 60*time.Second {
		t.Errorf("DefaultNetworkTimeout: got %v, want 60s", DefaultNetworkTimeout)
	}
}

func TestConfigureTimeouts_NegativeKeepsDefaults(t *testing.T) {
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	origBackground := GetBackgroundTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
		SetBackgroundTimeout(origBackground)
	}()

	SetForegroundTimeout(30 * time.Second)
	SetSkillTimeout(120 * time.Second)
	SetBackgroundTimeout(time.Hour)

	ConfigureTimeouts(-5, -10, -60)

	if GetForegroundTimeout() != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged, got %v", GetSkillTimeout())
	}
	if GetBackgroundTimeout() != time.Hour {
		t.Errorf("expected BackgroundTimeout unchanged, got %v", GetBackgroundTimeout())
	}
}
