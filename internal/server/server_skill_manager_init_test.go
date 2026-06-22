package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestInitSkillManagersInitializesAgentSkillsWhenClassicSkillManagerDisabled(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.SQLite.SkillsPath = filepath.Join(tmp, "skills.db")
	cfg.Directories.SkillsDir = filepath.Join(tmp, "skills")
	cfg.Directories.AgentSkillsDir = filepath.Join(tmp, "agent_skills")
	cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	cfg.Tools.SkillManager.Enabled = false

	oldSkill := tools.DefaultSkillManager()
	oldAgent := tools.DefaultAgentSkillManager()
	t.Cleanup(func() {
		tools.SetDefaultSkillManager(oldSkill)
		tools.SetDefaultAgentSkillManager(oldAgent)
	})

	s := &Server{Cfg: cfg, Logger: slog.Default()}
	s.initSkillManagers(context.Background(), tmp)

	if s.SkillManager != nil {
		t.Fatal("expected SkillManager nil when classic manager disabled")
	}
	if s.AgentSkillManager == nil {
		t.Fatal("expected AgentSkillManager initialized")
	}
	if s.SkillsDB == nil {
		t.Fatal("expected SkillsDB initialized")
	}
	if err := s.SkillsDB.Close(); err != nil {
		t.Fatalf("close skills db: %v", err)
	}
}

func TestInitSkillManagersInitializesBothManagersWhenEnabled(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.SQLite.SkillsPath = filepath.Join(tmp, "skills.db")
	cfg.Directories.SkillsDir = filepath.Join(tmp, "skills")
	cfg.Directories.AgentSkillsDir = filepath.Join(tmp, "agent_skills")
	cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	cfg.Tools.SkillManager.Enabled = true

	oldSkill := tools.DefaultSkillManager()
	oldAgent := tools.DefaultAgentSkillManager()
	t.Cleanup(func() {
		tools.SetDefaultSkillManager(oldSkill)
		tools.SetDefaultAgentSkillManager(oldAgent)
	})

	s := &Server{Cfg: cfg, Logger: slog.Default()}
	s.initSkillManagers(context.Background(), tmp)

	if s.SkillManager == nil {
		t.Fatal("expected SkillManager initialized")
	}
	if s.AgentSkillManager == nil {
		t.Fatal("expected AgentSkillManager initialized")
	}
	if s.SkillsDB == nil {
		t.Fatal("expected SkillsDB initialized")
	}
	if err := s.SkillsDB.Close(); err != nil {
		t.Fatalf("close skills db: %v", err)
	}
}