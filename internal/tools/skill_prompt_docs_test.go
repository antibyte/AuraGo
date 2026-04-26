package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillPromptDocsExposeManifestAndRuntimeDetails(t *testing.T) {
	root := filepath.Join("..", "..")
	checks := map[string][]string{
		filepath.Join(root, "prompts", "tools_manuals", "skill_manifest_spec.md"): {
			"Skill Manifest Schema",
			`"executable"`,
			`"parameters"`,
			`"vault_keys"`,
			`"internal_tools"`,
			`"daemon"`,
			`"wake_rate_limit_seconds"`,
			`"trigger_mission_id"`,
			`"cheatsheet_id"`,
			"AURAGO_SECRET_<KEY>",
			"AURAGO_CRED_<NAME>_<FIELD>",
			"cred:<credential-id>",
			"credential_ids",
		},
		filepath.Join(root, "prompts", "tools_manuals", "skills_engine.md"): {
			"skill_manifest_spec.md",
			"minimal_skill",
			"AURAGO_SECRET_<KEY>",
			"AURAGO_CRED_<NAME>_<FIELD>",
			"credential_ids",
			"Testing",
		},
		filepath.Join(root, "prompts", "tools_manuals", "skill_templates.md"): {
			"After Creation: Editing the Manifest",
			"Testing Your Skill",
			"skill_manifest_spec.md",
			"AURAGO_SECRET_<KEY>",
			"AuraGoTools.is_available()",
			"AuraGoToolError",
		},
		filepath.Join(root, "prompts", "tools_manuals", "execute_skill.md"): {
			"credential_ids",
			"AURAGO_SECRET_<KEY>",
			"AURAGO_CRED_<NAME>_<FIELD>",
			"AuraGoTools.is_available()",
			"AuraGoToolError",
		},
		filepath.Join(root, "prompts", "tools_manuals", "manage_daemon.md"): {
			"Daemon Manifest Settings",
			"wake_rate_limit_seconds",
			"max_runtime_hours",
			"trigger_mission_id",
			"cheatsheet_id",
			"health_check_interval_seconds",
		},
		filepath.Join(root, "prompts", "rules.md"): {
			"daemon_mission",
			"Advanced Daemon Configuration",
			"wake_rate_limit_seconds",
			"max_runtime_hours",
			"trigger_mission_id",
			"cheatsheet_id",
			"env",
		},
		filepath.Join(root, "agent_workspace", "skills", "aurago_tools.py"): {
			"def is_available",
			"AuraGoToolError",
		},
	}

	for path, markers := range checks {
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			content := string(data)
			for _, marker := range markers {
				if !strings.Contains(content, marker) {
					t.Fatalf("%s is missing marker %q", path, marker)
				}
			}
		})
	}
}
