package tools

import (
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestValidateAgentSkillScriptPolicyLanguageAndDangerZone(t *testing.T) {
	t.Run("python requires allow_python", func(t *testing.T) {
		cfg := &config.Config{}
		err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.py")
		if err == nil || !strings.Contains(err.Error(), "agent.allow_python") {
			t.Fatalf("error = %v, want agent.allow_python denial", err)
		}
	})

	t.Run("bash does not require allow_python", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowShell = true
		cfg.Tools.SkillManager.AllowedScriptLanguages = []string{"bash"}
		if err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.sh"); err != nil {
			t.Fatalf("ValidateAgentSkillScriptPolicy returned %v, want bash allowed", err)
		}
	})

	t.Run("bash still requires allow_shell", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Tools.SkillManager.AllowedScriptLanguages = []string{"bash"}
		err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.sh")
		if err == nil || !strings.Contains(err.Error(), "agent.allow_shell") {
			t.Fatalf("error = %v, want agent.allow_shell denial", err)
		}
	})

	t.Run("javascript requires allow_shell as executable runtime", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Tools.SkillManager.AllowedScriptLanguages = []string{"javascript"}
		err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.js")
		if err == nil || !strings.Contains(err.Error(), "agent.allow_shell") {
			t.Fatalf("error = %v, want agent.allow_shell denial", err)
		}
		cfg.Agent.AllowShell = true
		if err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.js"); err != nil {
			t.Fatalf("ValidateAgentSkillScriptPolicy returned %v, want javascript allowed", err)
		}
	})

	t.Run("default allowed languages only include python", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Agent.AllowShell = true
		err := ValidateAgentSkillScriptPolicy(cfg, "scripts/run.sh")
		if err == nil || !strings.Contains(err.Error(), "allowed_script_languages") {
			t.Fatalf("error = %v, want allowed_script_languages denial", err)
		}
	})
}
