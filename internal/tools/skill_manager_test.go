package tools

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// setupTestSkillManager creates a temporary SkillManager for testing.
func setupTestSkillManager(t *testing.T) (*SkillManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "skills_test.db")
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0755)

	db, err := InitSkillsDB(dbPath)
	if err != nil {
		t.Fatalf("failed to init skills DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	mgr := NewSkillManager(db, skillsDir, logger)
	return mgr, skillsDir
}

func TestInitSkillsDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "skills.db")

	db, err := InitSkillsDB(dbPath)
	if err != nil {
		t.Fatalf("InitSkillsDB failed: %v", err)
	}
	defer db.Close()

	// Verify tables exist
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='skills_registry'").Scan(&tableName)
	if err != nil {
		t.Fatalf("skills_registry table not found: %v", err)
	}
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='skills_scan_history'").Scan(&tableName)
	if err != nil {
		t.Fatalf("skills_scan_history table not found: %v", err)
	}
}

func TestGetStats_Empty(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	total, agent, user, pending, err := mgr.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if total != 0 || agent != 0 || user != 0 || pending != 0 {
		t.Errorf("expected all zeros, got total=%d agent=%d user=%d pending=%d", total, agent, user, pending)
	}
}

func TestCreateSkillEntry(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	code := `import json

def run(data="{}"):
    """Process input data."""
    return json.loads(data)
`
	entry, err := mgr.CreateSkillEntry("test_skill", "A test skill", code, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry failed: %v", err)
	}

	if entry.Name != "test_skill" {
		t.Errorf("expected name 'test_skill', got '%s'", entry.Name)
	}
	if entry.Type != SkillTypeUser {
		t.Errorf("expected type 'user', got '%s'", entry.Type)
	}
	if entry.SecurityStatus != SecurityPending {
		t.Errorf("expected security status 'pending', got '%s'", entry.SecurityStatus)
	}

	// Verify files were created
	pyFile := filepath.Join(skillsDir, "test_skill.py")
	if _, err := os.Stat(pyFile); os.IsNotExist(err) {
		t.Error("Python file was not created")
	}
	jsonFile := filepath.Join(skillsDir, "test_skill.json")
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Error("Manifest JSON file was not created")
	}
}

func TestCreateSkillEntry_DuplicateName(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	code := `def run(): return "ok"`
	_, err := mgr.CreateSkillEntry("dup_skill", "First", code, SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = mgr.CreateSkillEntry("dup_skill", "Second", code, SkillTypeAgent, "agent", "", nil)
	if err == nil {
		t.Error("expected error for duplicate skill name")
	}
}

func TestCreateSkillEntry_InvalidName(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	_, err := mgr.CreateSkillEntry("../escaped", "Bad name", `def run(): pass`, SkillTypeUser, "user", "", nil)
	if err == nil {
		t.Error("expected error for path traversal in name")
	}
}

func TestGetSkill(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	created, err := mgr.CreateSkillEntry("get_test", "Get test skill", `def run(): return "hello"`, SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	got, err := mgr.GetSkill(created.ID)
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if got.Name != "get_test" {
		t.Errorf("expected name 'get_test', got '%s'", got.Name)
	}
}

func TestGetSkill_NotFound(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	_, err := mgr.GetSkill("nonexistent_id")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestEnableDisableSkill(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	created, err := mgr.CreateSkillEntry("toggle_test", "Toggle test", `def run(): return True`, SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Initially enabled (default from create)
	skill, _ := mgr.GetSkill(created.ID)
	initialEnabled := skill.Enabled

	// Toggle to opposite
	err = mgr.EnableSkill(created.ID, !initialEnabled, "test")
	if err != nil {
		t.Fatalf("EnableSkill failed: %v", err)
	}

	skill, _ = mgr.GetSkill(created.ID)
	if skill.Enabled == initialEnabled {
		t.Error("expected enabled state to change")
	}
}

func TestDeleteSkill(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	created, err := mgr.CreateSkillEntry("del_test", "Delete test", `def run(): return None`, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Delete without removing files
	err = mgr.DeleteSkill(created.ID, false, "test")
	if err != nil {
		t.Fatalf("DeleteSkill failed: %v", err)
	}

	// Verify DB entry is gone
	_, err = mgr.GetSkill(created.ID)
	if err == nil {
		t.Error("expected skill to be gone from DB after delete")
	}

	// Verify file still exists (deleteFiles=false)
	pyFile := filepath.Join(skillsDir, "del_test.py")
	if _, err := os.Stat(pyFile); os.IsNotExist(err) {
		t.Error("Python file should still exist when deleteFiles=false")
	}
}

func TestDeleteSkill_WithFiles(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	created, err := mgr.CreateSkillEntry("del_files", "Delete with files", `def run(): pass`, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	err = mgr.DeleteSkill(created.ID, true, "test")
	if err != nil {
		t.Fatalf("DeleteSkill with files failed: %v", err)
	}

	pyFile := filepath.Join(skillsDir, "del_files.py")
	if _, err := os.Stat(pyFile); !os.IsNotExist(err) {
		t.Error("Python file should be deleted when deleteFiles=true")
	}

	jsonFile := filepath.Join(skillsDir, "del_files.json")
	if _, err := os.Stat(jsonFile); !os.IsNotExist(err) {
		t.Error("Manifest JSON file should be deleted when deleteFiles=true")
	}
}

func TestListSkillsFiltered(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	// Create a few skills
	mgr.CreateSkillEntry("agent_skill_1", "Agent skill 1", `def run(): return 1`, SkillTypeAgent, "agent", "", nil)
	mgr.CreateSkillEntry("agent_skill_2", "Agent skill 2", `def run(): return 2`, SkillTypeAgent, "agent", "", nil)
	mgr.CreateSkillEntry("user_skill_1", "User skill 1", `def run(): return 3`, SkillTypeUser, "user", "", nil)

	// List all
	all, err := mgr.ListSkillsFiltered("", "", "", nil)
	if err != nil {
		t.Fatalf("ListSkillsFiltered all failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 skills, got %d", len(all))
	}

	// Filter by type
	agents, err := mgr.ListSkillsFiltered("agent", "", "", nil)
	if err != nil {
		t.Fatalf("ListSkillsFiltered by type failed: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agent skills, got %d", len(agents))
	}

	// Search by name
	results, err := mgr.ListSkillsFiltered("", "", "user", nil)
	if err != nil {
		t.Fatalf("ListSkillsFiltered search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for search 'user', got %d", len(results))
	}
}

func TestGetStats_WithSkills(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	mgr.CreateSkillEntry("stat_agent", "Agent", `def run(): pass`, SkillTypeAgent, "agent", "", nil)
	mgr.CreateSkillEntry("stat_user", "User", `def run(): pass`, SkillTypeUser, "user", "", nil)

	total, agent, user, pending, err := mgr.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2, got %d", total)
	}
	if agent != 1 {
		t.Errorf("expected agent=1, got %d", agent)
	}
	if user != 1 {
		t.Errorf("expected user=1, got %d", user)
	}
	if pending != 2 {
		t.Errorf("expected pending=2, got %d", pending)
	}
}

func TestUpdateSkillSecurity(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	created, err := mgr.CreateSkillEntry("sec_test", "Security test", `def run(): pass`, SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	report := &SecurityReport{
		StaticAnalysis: []Finding{},
		OverallStatus:  "clean",
		OverallScore:   0.0,
	}

	err = mgr.UpdateSkillSecurity(created.ID, SecurityClean, report)
	if err != nil {
		t.Fatalf("UpdateSkillSecurity failed: %v", err)
	}

	skill, err := mgr.GetSkill(created.ID)
	if err != nil {
		t.Fatalf("GetSkill after update failed: %v", err)
	}
	if skill.SecurityStatus != SecurityClean {
		t.Errorf("expected SecurityClean, got %s", skill.SecurityStatus)
	}
}

func TestGetSkillCode(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	code := `def run():
    return "test code"
`
	created, err := mgr.CreateSkillEntry("code_test", "Code test", code, SkillTypeAgent, "agent", "", nil)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	retrieved, err := mgr.GetSkillCode(created.ID)
	if err != nil {
		t.Fatalf("GetSkillCode failed: %v", err)
	}
	if retrieved != code {
		t.Errorf("expected code to match, got '%s'", retrieved)
	}
}

func TestCreateSkillEntry_WithCategoryAndTags(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	entry, err := mgr.CreateSkillEntry("tagged_skill", "Tagged", `def run(): return "ok"`, SkillTypeUser, "user", "automation", []string{"net", "api"})
	if err != nil {
		t.Fatalf("CreateSkillEntry failed: %v", err)
	}
	got, err := mgr.GetSkill(entry.ID)
	if err != nil {
		t.Fatalf("GetSkill failed: %v", err)
	}
	if got.Category != "automation" {
		t.Fatalf("expected category automation, got %q", got.Category)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got.Tags))
	}
}

func TestUpdateSkillCode_CreatesVersion(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	entry, err := mgr.CreateSkillEntry("versioned_skill", "Versioned", `def run(): return "v1"`, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry failed: %v", err)
	}
	if err := mgr.UpdateSkillCode(entry.ID, `def run(): return "v2"`, "tester"); err != nil {
		t.Fatalf("UpdateSkillCode failed: %v", err)
	}
	versions, err := mgr.ListSkillVersions(entry.ID)
	if err != nil {
		t.Fatalf("ListSkillVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].CreatedBy != "tester" {
		t.Fatalf("expected top version to be created by tester, got %q", versions[0].CreatedBy)
	}
}

func TestSkillAuditLogRecordsLifecycle(t *testing.T) {
	mgr, _ := setupTestSkillManager(t)

	entry, err := mgr.CreateSkillEntry("audited_skill", "Audited", `def run(): return "ok"`, SkillTypeUser, "user", "", nil)
	if err != nil {
		t.Fatalf("CreateSkillEntry failed: %v", err)
	}
	if err := mgr.EnableSkill(entry.ID, true, "tester"); err != nil {
		t.Fatalf("EnableSkill failed: %v", err)
	}
	audit, err := mgr.ListSkillAudit(entry.ID, 10)
	if err != nil {
		t.Fatalf("ListSkillAudit failed: %v", err)
	}
	if len(audit) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(audit))
	}
}

func TestSyncFromDisk(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	// Write a skill file + manifest directly to disk
	pyContent := `def run():
    return "synced"
`
	manifest := `{
    "name": "disk_skill",
    "description": "Created on disk",
    "executable": "disk_skill.py",
    "parameters": {}
}`
	os.WriteFile(filepath.Join(skillsDir, "disk_skill.py"), []byte(pyContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "disk_skill.json"), []byte(manifest), 0644)

	err := mgr.SyncFromDisk()
	if err != nil {
		t.Fatalf("SyncFromDisk failed: %v", err)
	}

	// Verify it was imported
	total, _, _, _, err := mgr.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if total < 1 {
		t.Error("expected at least 1 skill after sync")
	}
}

func TestExtractImportsFromCode(t *testing.T) {
	code := `import requests
import json
from bs4 import BeautifulSoup
import os
import sys

def run():
    pass
`
	deps := extractImportsFromCode(code)
	// requests and beautifulsoup4 are third-party, json/os/sys are stdlib
	found := map[string]bool{}
	for _, d := range deps {
		found[d] = true
	}
	if !found["requests"] {
		t.Error("expected 'requests' in dependencies")
	}
	// json, os, sys should be filtered as stdlib
	if found["json"] || found["os"] || found["sys"] {
		t.Errorf("stdlib modules should not be in deps, got: %v", deps)
	}
}

func TestSyncFromDisk_SkipsInvalidExecutablePath(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	manifest := `{
		"name": "bad_path_skill",
		"description": "Bad path",
		"executable": "../escape.py"
	}`
	os.WriteFile(filepath.Join(skillsDir, "bad_path_skill.json"), []byte(manifest), 0644)
	os.WriteFile(filepath.Join(skillsDir, "../escape.py"), []byte("pass\n"), 0644)

	err := mgr.SyncFromDisk()
	if err != nil {
		t.Fatalf("SyncFromDisk failed: %v", err)
	}

	_, err = mgr.GetSkill("bad_path_skill")
	if err == nil {
		t.Error("expected skill with invalid executable path to be skipped")
	}
}

func TestSyncFromDisk_SetsSecurityPendingAndScans(t *testing.T) {
	mgr, skillsDir := setupTestSkillManager(t)

	pyContent := `import os
os.system("ls")
`
	manifest := `{
		"name": "scan_test",
		"description": "Scan test",
		"executable": "scan_test.py"
	}`
	os.WriteFile(filepath.Join(skillsDir, "scan_test.py"), []byte(pyContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "scan_test.json"), []byte(manifest), 0644)

	err := mgr.SyncFromDisk()
	if err != nil {
		t.Fatalf("SyncFromDisk failed: %v", err)
	}

	skills, _ := mgr.ListSkillsFiltered("", "", "scan_test", nil)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].SecurityStatus != SecurityWarning && skills[0].SecurityStatus != SecurityClean {
		t.Errorf("expected security status warning or clean after scan, got %s", skills[0].SecurityStatus)
	}
}

