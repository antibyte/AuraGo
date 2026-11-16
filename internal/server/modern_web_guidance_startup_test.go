package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestModernWebGuidanceStartup(t *testing.T) {
	oldSkill, oldAgent := tools.DefaultSkillManager(), tools.DefaultAgentSkillManager()
	t.Cleanup(func() {
		tools.SetDefaultSkillManager(oldSkill)
		tools.SetDefaultAgentSkillManager(oldAgent)
	})
	for _, classicEnabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "classic-disabled", true: "classic-enabled"}[classicEnabled], func(t *testing.T) {
			root := t.TempDir()
			cfg := &config.Config{}
			cfg.SQLite.SkillsPath = filepath.Join(root, "skills.db")
			cfg.Directories.AgentSkillsDir = filepath.Join(root, "agent_skills")
			cfg.Directories.SkillsDir = filepath.Join(root, "skills")
			cfg.Directories.WorkspaceDir = filepath.Join(root, "workdir")
			cfg.Tools.SkillManager.Enabled = classicEnabled
			server := &Server{Cfg: cfg, Logger: slog.Default()}
			server.initSkillManagers(context.Background(), root)
			if server.SkillsDB == nil || server.AgentSkillManager == nil {
				t.Fatal("skill manager startup failed")
			}
			defer server.SkillsDB.Close()
			entry, err := server.AgentSkillManager.GetAgentSkillByName("modern-web-guidance")
			if err != nil || !entry.Enabled || entry.SecurityStatus != tools.SecurityClean || entry.Origin != tools.OriginSystem {
				t.Fatalf("bundled skill unavailable: %+v, %v", entry, err)
			}
			if _, err := server.AgentSkillManager.LoadCurrentAgentSkillPackage(entry, "test"); err != nil {
				t.Fatalf("installed skill cannot be activated: %v", err)
			}
		})
	}
}
