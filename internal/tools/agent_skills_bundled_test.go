package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBundledAgentSkillRequiresExactCompletePackage(t *testing.T) {
	for _, change := range []string{"none", "markdown", "script", "unrecognized", "directory", "blocked", "cancelled"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			db, err := InitAgentSkillsDB(filepath.Join(t.TempDir(), "skills.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			manager := NewAgentSkillManager(db, root, t.TempDir(), nil)
			name := "trusted-bundle"
			markdown := validAgentSkillMarkdown(name)
			writeAgentSkillFile(t, root, name+"/SKILL.md", markdown)
			ctx := context.Background()
			switch change {
			case "markdown":
				writeAgentSkillFile(t, root, name+"/SKILL.md", markdown+"Changed instructions")
			case "script":
				writeAgentSkillFile(t, root, name+"/scripts/run.py", "print('extra')")
			case "unrecognized":
				writeAgentSkillFile(t, root, name+"/EXTRA.txt", "not in binary")
			case "directory":
				if err := os.Mkdir(filepath.Join(root, name, "assets"), 0o750); err != nil {
					t.Fatal(err)
				}
			case "blocked":
				if _, err := manager.RegisterBundledAgentSkill(ctx, name, []byte(markdown)); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Exec("UPDATE agent_skills_registry SET enabled = 0, security_status = ?", string(SecurityDangerous)); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			entry, err := manager.RegisterBundledAgentSkill(ctx, name, []byte(markdown))
			if change != "none" {
				if err == nil {
					t.Fatal("invalid bundle inherited binary trust")
				}
				var enabled int
				if err := db.QueryRow("SELECT count(*) FROM agent_skills_registry WHERE enabled = 1").Scan(&enabled); err != nil || enabled != 0 {
					t.Fatalf("failed verification enabled a skill: %d, %v", enabled, err)
				}
				return
			}
			if err != nil || !entry.Enabled || entry.SecurityStatus != SecurityClean || entry.Origin != OriginSystem {
				t.Fatalf("trusted registration: %+v, %v", entry, err)
			}
			pkg, err := manager.LoadCurrentAgentSkillPackage(entry, "test")
			if err != nil || pkg.PackageHash != entry.PackageHash {
				t.Fatalf("registered bundle cannot be loaded: %v", err)
			}
			// The generic discovery path must reuse the exact hash without a rescan.
			if err := manager.SyncFromDisk(ctx, nil, false); err != nil {
				t.Fatal(err)
			}
			again, err := manager.RegisterBundledAgentSkill(ctx, name, []byte(markdown))
			if err != nil || again.ID != entry.ID || again.PackageHash != entry.PackageHash {
				t.Fatalf("non-idempotent verification: %+v, %v", again, err)
			}
		})
	}
}
