package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"aurago/internal/config"
)

// ValidateAgentSkillScriptPolicy applies runtime config gates before an Agent Skill
// script is executed by either the agent dispatcher or the Web/API test endpoint.
func ValidateAgentSkillScriptPolicy(cfg *config.Config, scriptPath string) error {
	if cfg == nil {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(scriptPath))
	lang := ""
	switch ext {
	case ".py":
		lang = "python"
	case ".sh":
		lang = "bash"
	case ".js":
		lang = "javascript"
	default:
		return fmt.Errorf("unsupported script extension: %s", ext)
	}

	allowed := cfg.Tools.SkillManager.AllowedScriptLanguages
	if allowed == nil {
		allowed = []string{"python"}
	}
	langAllowed := false
	for _, configured := range allowed {
		if strings.EqualFold(strings.TrimSpace(configured), lang) {
			langAllowed = true
			break
		}
	}
	if !langAllowed {
		return fmt.Errorf("script language %q is not in tools.skill_manager.allowed_script_languages config. Allowed: %v", ext, allowed)
	}

	switch lang {
	case "python":
		if !cfg.Agent.AllowPython {
			return fmt.Errorf("Python scripts require agent.allow_python to be enabled in Danger Zone settings")
		}
	case "bash":
		if !cfg.Agent.AllowShell {
			return fmt.Errorf("Bash scripts require agent.allow_shell to be enabled in Danger Zone settings")
		}
	case "javascript":
		if !cfg.Agent.AllowShell {
			return fmt.Errorf("JavaScript scripts require agent.allow_shell to be enabled in Danger Zone settings")
		}
	}
	return nil
}
