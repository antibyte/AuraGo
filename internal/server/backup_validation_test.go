package server

import "testing"

func TestAgoIsAllowedRestorePathCoversBackupPrefixes(t *testing.T) {
	t.Parallel()
	for _, p := range []string{
		"config.yaml", "data/mailer/mailer.db", "data/documents/a.md",
		"agent_workspace/agent_skills", "agent_workspace/agent_skills/demo/SKILL.md",
		"agent_workspace/skills/x", "agent_workspace/tools/x",
	} {
		if !agoIsAllowedRestorePath(p) {
			t.Errorf("%q should be restorable", p)
		}
	}
	for _, p := range []string{"agent_workspace/agent_skills_evil/x", "agent_workspace/other/x", "../data/x", "vault.bin"} {
		if agoIsAllowedRestorePath(p) {
			t.Errorf("%q must not be restorable", p)
		}
	}
}
