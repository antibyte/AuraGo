package tools

import (
	"testing"
	"time"
)

func TestConfigureTimeouts_SetsValues(t *testing.T) {
	// Save originals
	origPython := ForegroundTimeout
	origSkill := SkillTimeout
	defer func() {
		ForegroundTimeout = origPython
		SkillTimeout = origSkill
	}()

	ConfigureTimeouts(60, 300)

	if ForegroundTimeout != 60*time.Second {
		t.Errorf("expected ForegroundTimeout=60s, got %v", ForegroundTimeout)
	}
	if SkillTimeout != 300*time.Second {
		t.Errorf("expected SkillTimeout=300s, got %v", SkillTimeout)
	}
}

func TestConfigureTimeouts_ZeroKeepsDefaults(t *testing.T) {
	origPython := ForegroundTimeout
	origSkill := SkillTimeout
	defer func() {
		ForegroundTimeout = origPython
		SkillTimeout = origSkill
	}()

	ForegroundTimeout = 30 * time.Second
	SkillTimeout = 120 * time.Second

	ConfigureTimeouts(0, 0)

	if ForegroundTimeout != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged at 30s, got %v", ForegroundTimeout)
	}
	if SkillTimeout != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged at 120s, got %v", SkillTimeout)
	}
}

func TestConfigureTimeouts_NegativeKeepsDefaults(t *testing.T) {
	origPython := ForegroundTimeout
	origSkill := SkillTimeout
	defer func() {
		ForegroundTimeout = origPython
		SkillTimeout = origSkill
	}()

	ForegroundTimeout = 30 * time.Second
	SkillTimeout = 120 * time.Second

	ConfigureTimeouts(-5, -10)

	if ForegroundTimeout != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged, got %v", ForegroundTimeout)
	}
	if SkillTimeout != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged, got %v", SkillTimeout)
	}
}
