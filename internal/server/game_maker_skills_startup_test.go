package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/gamemaker"
	"aurago/internal/tools"
)

// TestGameMakerSkillStartupRecoversRegistry simulates a server whose skill
// registry was left in a degraded state (disabled, pending security status)
// by an older binary, then runs the current startup sequence of bundled
// install, disk sync, and verification. The curated Game Maker skills must
// come back ready without manual intervention.
func TestGameMakerSkillStartupRecoversRegistry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	workspace := t.TempDir()
	db, err := tools.InitSkillsDB(filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := tools.MigrateAgentSkillsDB(db); err != nil {
		t.Fatal(err)
	}

	oldSkill := func(name string) string {
		return fmt.Sprintf(`---
name: %s
description: Old bundled variant.
license: MIT
metadata:
  managed_by: aurago
---

# Old

Previous bundled content.
`, name)
	}

	// Seed the previous installation: old content on disk, enabled registry rows.
	for _, name := range gamemaker.CuratedSkillNames() {
		skillDir := filepath.Join(dir, name)
		if err := os.MkdirAll(skillDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(oldSkill(name)), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	manager := tools.NewAgentSkillManager(db, dir, workspace, nil)
	if err := manager.SyncFromDisk(ctx, nil, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range gamemaker.CuratedSkillNames() {
		entry, err := manager.GetAgentSkillByName(name)
		if err != nil {
			t.Fatalf("seed entry %s: %v", name, err)
		}
		if err := manager.EnableAgentSkill(entry.ID, true, "test"); err != nil {
			t.Fatalf("seed enable %s: %v", name, err)
		}
	}

	// Simulate the incident: a stale binary saw drifted files and disabled the
	// registry entries with a pending security status.
	for _, name := range gamemaker.CuratedSkillNames() {
		if err := os.WriteFile(filepath.Join(dir, name, "SKILL.md"), []byte(oldSkill(name)+"drift"), 0o640); err != nil {
			t.Fatal(err)
		}
		entry, err := manager.GetAgentSkillByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.LoadCurrentAgentSkillPackage(entry, "test"); err == nil {
			t.Fatalf("%s: expected hash drift error", name)
		}
		entry, err = manager.GetAgentSkillByName(name)
		if err != nil {
			t.Fatal(err)
		}
		if entry.Enabled {
			t.Fatalf("%s: entry should be disabled after drift", name)
		}
	}

	// Current startup sequence: bundled self-heal install, sync, verify.
	install, err := gamemaker.InstallBundledSkills(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !install.Ready {
		t.Fatalf("install not ready: %+v", install.Skills)
	}
	manager = tools.NewAgentSkillManager(db, dir, workspace, nil)
	if err := manager.SyncFromDisk(ctx, nil, false); err != nil {
		t.Fatal(err)
	}
	skills, ready := verifyGameMakerAgentSkills(manager, install, nil)
	if !ready {
		t.Fatalf("skills not ready after recovery: %+v", skills)
	}
	for _, skill := range skills {
		if skill.Status != "ready" {
			t.Errorf("skill %s status = %q, want ready", skill.Name, skill.Status)
		}
	}
}
