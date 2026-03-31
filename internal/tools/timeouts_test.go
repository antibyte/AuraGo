package tools

import (
	"testing"
	"time"
)

func TestConfigureTimeouts_SetsValues(t *testing.T) {
	// Save originals
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
	}()

	ConfigureTimeouts(60, 300)

	if GetForegroundTimeout() != 60*time.Second {
		t.Errorf("expected ForegroundTimeout=60s, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 300*time.Second {
		t.Errorf("expected SkillTimeout=300s, got %v", GetSkillTimeout())
	}
}

func TestConfigureTimeouts_ZeroKeepsDefaults(t *testing.T) {
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
	}()

	SetForegroundTimeout(30 * time.Second)
	SetSkillTimeout(120 * time.Second)

	ConfigureTimeouts(0, 0)

	if GetForegroundTimeout() != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged at 30s, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged at 120s, got %v", GetSkillTimeout())
	}
}

func TestConfigureTimeouts_NegativeKeepsDefaults(t *testing.T) {
	origPython := GetForegroundTimeout()
	origSkill := GetSkillTimeout()
	defer func() {
		SetForegroundTimeout(origPython)
		SetSkillTimeout(origSkill)
	}()

	SetForegroundTimeout(30 * time.Second)
	SetSkillTimeout(120 * time.Second)

	ConfigureTimeouts(-5, -10)

	if GetForegroundTimeout() != 30*time.Second {
		t.Errorf("expected ForegroundTimeout unchanged, got %v", GetForegroundTimeout())
	}
	if GetSkillTimeout() != 120*time.Second {
		t.Errorf("expected SkillTimeout unchanged, got %v", GetSkillTimeout())
	}
}
