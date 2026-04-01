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
