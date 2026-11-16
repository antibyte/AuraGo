package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestModernWebGuidanceLoadsOfflineAndPreservesDisable(t *testing.T) {
	manager, root := setupAgentSkillManagerOnDisk(t)
	root = filepath.Join(filepath.Dir(root), "agent_workspace", "agent_skills")
	manager.agentSkillsDir = root
	manager.workspaceDir = filepath.Join(filepath.Dir(root), "workdir")
	if err := os.MkdirAll(manager.workspaceDir, 0o750); err != nil {
		t.Fatal(err)
	}
	entry, err := manager.InstallModernWebGuidance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Enabled || entry.SecurityStatus != SecurityClean || entry.Origin != OriginSystem {
		t.Fatalf("bundle not ready by default: %+v", entry)
	}
	pkg, err := manager.LoadCurrentAgentSkillPackage(entry, "test")
	if err != nil || len(pkg.Scripts) != 0 || len(pkg.Resources) != 212 {
		t.Fatalf("incomplete or executable bundle: %+v, %v", pkg, err)
	}
	for _, path := range []string{"references/INDEX.md", "references/guides/ui-components/dialog.md", "references/LICENSE.txt"} {
		content, err := manager.ReadAgentSkillFile(entry.ID, path)
		if err != nil || len(content) == 0 {
			t.Fatalf("cannot read %s: %v", path, err)
		}
	}
	read := ExecuteFileReaderAdvanced("read_lines", "../agent_skills/modern-web-guidance/references/guides/ui-components/dialog.md", "", 1, 30, 0, manager.workspaceDir)
	var result FileReaderResult
	if err := json.Unmarshal([]byte(read), &result); err != nil || result.Status != "success" || !strings.Contains(read, "showModal()") {
		t.Fatalf("native reader cannot follow the skill workflow: %s, %v", read, err)
	}
	if err := manager.EnableAgentSkill(entry.ID, false, "user"); err != nil {
		t.Fatal(err)
	}
	manager = NewAgentSkillManager(manager.db, root, manager.workspaceDir, nil)
	again, err := manager.InstallModernWebGuidance(context.Background())
	if err != nil || again.ID != entry.ID || again.Enabled || again.PackageHash != entry.PackageHash {
		t.Fatalf("restart lost saved disablement: %+v, %v", again, err)
	}
	if err := manager.SyncFromDisk(context.Background(), nil, false); err != nil {
		t.Fatal(err)
	}
	active, err := manager.ListAgentSkills(true, "modern-web-guidance")
	if err != nil || len(active) != 0 {
		t.Fatalf("background discovery re-enabled bundle: %+v, %v", active, err)
	}
}

func embeddedSkillFixture(text string) fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md":            {Data: []byte(validAgentSkillMarkdown("embedded-fixture"))},
		"references/guide.md": {Data: []byte(text)},
		"references/a.md":     {Data: []byte("Sorting before a directory.")},
		"references/a/one.md": {Data: []byte("Nested reference.")},
	}
}

func TestEmbeddedAgentSkillUpgradeAndRollback(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			manager, root := setupAgentSkillManagerOnDisk(t)
			ctx := context.Background()
			entry, err := manager.installEmbeddedAgentSkill(ctx, "embedded-fixture", embeddedSkillFixture("First revision"))
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.EnableAgentSkill(entry.ID, enabled, "user"); err != nil {
				t.Fatal(err)
			}
			updated := embeddedSkillFixture("Second revision")
			delete(updated, "references/a.md")
			updated["references/new.md"] = &fstest.MapFile{Data: []byte("New reference")}
			again, err := manager.installEmbeddedAgentSkill(ctx, "embedded-fixture", updated)
			if err != nil || again.Enabled != enabled || again.ID != entry.ID || again.PackageHash == entry.PackageHash {
				t.Fatalf("upgrade: %+v, %v", again, err)
			}
			if _, err := os.Stat(filepath.Join(again.Directory, "references/a.md")); !os.IsNotExist(err) {
				t.Fatalf("obsolete file survived: %v", err)
			}
			if _, err := manager.db.Exec(`CREATE TRIGGER refuse_bundle_update BEFORE UPDATE ON agent_skills_registry
				BEGIN SELECT RAISE(ABORT, 'test write failure'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.installEmbeddedAgentSkill(ctx, "embedded-fixture", embeddedSkillFixture("Third revision")); err == nil {
				t.Fatal("database failure was hidden")
			}
			if _, err := manager.LoadCurrentAgentSkillPackage(again, "test"); err != nil {
				t.Fatalf("failed upgrade lost the verified package: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(again.Directory, "references/guide.md"))
			if err != nil || string(data) != "Second revision" {
				t.Fatalf("rollback lost original content: %q, %v", data, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "embedded-fixture" {
				t.Fatalf("staging was not cleaned: %+v, %v", entries, err)
			}
		})
	}
}

func TestEmbeddedAgentSkillRejectsUntrustedChanges(t *testing.T) {
	for _, change := range []string{"markdown", "reference", "reviewed-edit", "extra", "oversized", "script", "empty-dir", "blocked", "foreign", "symlink"} {
		t.Run(change, func(t *testing.T) {
			manager, _ := setupAgentSkillManagerOnDisk(t)
			ctx := context.Background()
			bundle := embeddedSkillFixture("Original reference")
			entry, err := manager.installEmbeddedAgentSkill(ctx, "embedded-fixture", bundle)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "markdown":
				writeAgentSkillFile(t, entry.Directory, "SKILL.md", validAgentSkillMarkdown(entry.Name)+"\nLocal change.\n")
			case "reference":
				writeAgentSkillFile(t, entry.Directory, "references/guide.md", "Local change")
			case "reviewed-edit":
				writeAgentSkillFile(t, entry.Directory, "references/guide.md", "Reviewed local change")
				pkg, err := ParseAgentSkillPackage(entry.Directory)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := manager.upsertAgentSkillPackage(pkg, "user", nil, SecurityClean, true, false); err != nil {
					t.Fatal(err)
				}
			case "extra":
				writeAgentSkillFile(t, entry.Directory, "EXTRA.txt", "Not part of the package")
			case "oversized":
				writeAgentSkillFile(t, entry.Directory, "EXTRA.txt", strings.Repeat("x", maxAgentSkillPackageBytes+1))
			case "script":
				writeAgentSkillFile(t, entry.Directory, "scripts/extra.py", "print('untrusted')")
			case "empty-dir":
				if err := os.Mkdir(filepath.Join(entry.Directory, "extra"), 0o750); err != nil {
					t.Fatal(err)
				}
			case "blocked", "foreign":
				query := "UPDATE agent_skills_registry SET enabled = 0, security_status = 'dangerous'"
				if change == "foreign" {
					query = "UPDATE agent_skills_registry SET enabled = 0, origin = 'user'"
				}
				if _, err := manager.db.Exec(query); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(entry.Directory, "references/guide.md"), filepath.Join(entry.Directory, "references/link.md")); err != nil {
					t.Skipf("symlink creation unavailable: %v", err)
				}
			}
			if _, err := manager.installEmbeddedAgentSkill(ctx, entry.Name, bundle); err == nil {
				t.Fatal("untrusted package inherited bundle trust")
			}
			after, err := manager.GetAgentSkill(entry.ID)
			if err != nil || after.Enabled {
				t.Fatalf("rejected bundle is enabled: %+v, %v", after, err)
			}
			if change == "reference" {
				data, _ := os.ReadFile(filepath.Join(entry.Directory, "references/guide.md"))
				if string(data) != "Local change" {
					t.Fatal("local content was overwritten")
				}
			}
		})
	}
}

func TestEmbeddedAgentSkillCancellationAndNameCollision(t *testing.T) {
	manager, root := setupAgentSkillManagerOnDisk(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.InstallModernWebGuidance(ctx); err != context.Canceled {
		t.Fatalf("canceled install: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("canceled install wrote files: %v", err)
	}
	writeAgentSkillFile(t, root, "embedded-fixture/SKILL.md", validAgentSkillMarkdown("embedded-fixture")+"User content")
	if _, err := manager.installEmbeddedAgentSkill(context.Background(), "embedded-fixture", embeddedSkillFixture("reference")); err == nil {
		t.Fatal("unregistered user package was overwritten")
	}
	data, err := os.ReadFile(filepath.Join(root, "embedded-fixture/SKILL.md"))
	if err != nil || !strings.Contains(string(data), "User content") {
		t.Fatalf("name collision lost user content: %q, %v", data, err)
	}
}
