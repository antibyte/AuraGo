package tools

import (
	"os"
	"path/filepath"
	"testing"

	"log/slog"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestSeedWelcomeMissions(t *testing.T) {
	tmpDir := t.TempDir()
	mm := NewMissionManagerV2(tmpDir, NewCronManager(tmpDir))
	_ = mm.Start()
	defer mm.Stop()

	// Create sample manifest
	assetsDir := filepath.Join(tmpDir, "assets", "mission_samples")
	os.MkdirAll(assetsDir, 0o755)
	manifest := `[
		{"id": "test-mission-1", "name": "Test Mission", "prompt": "Do something", "execution_type": "manual", "priority": "high", "enabled": true}
	]`
	os.WriteFile(filepath.Join(assetsDir, "metadata.json"), []byte(manifest), 0o644)

	// First seed should create the mission
	SeedWelcomeMissions(mm, tmpDir, testLogger())
	m, ok := mm.Get("test-mission-1")
	if !ok {
		t.Fatal("expected mission to be seeded")
	}
	if m.Name != "Test Mission" {
		t.Fatalf("unexpected name: %s", m.Name)
	}

	// Second seed should be idempotent
	SeedWelcomeMissions(mm, tmpDir, testLogger())
	m2, ok := mm.Get("test-mission-1")
	if !ok {
		t.Fatal("expected mission to still exist")
	}
	if m2.CreatedAt != m.CreatedAt {
		t.Fatal("mission was overwritten on second seed")
	}
}

func TestSeedWelcomeCheatsheets(t *testing.T) {
	db, err := InitCheatsheetDB(filepath.Join(t.TempDir(), "cheatsheets.db"))
	if err != nil {
		t.Fatalf("failed to init cheatsheet db: %v", err)
	}
	defer db.Close()

	assetsDir := filepath.Join(t.TempDir(), "assets", "cheatsheet_samples")
	os.MkdirAll(assetsDir, 0o755)
	manifest := `[
		{"id": "test-cs-1", "name": "Test CS", "content": "Hello", "active": true, "created_by": "system"}
	]`
	os.WriteFile(filepath.Join(assetsDir, "metadata.json"), []byte(manifest), 0o644)

	// First seed
	SeedWelcomeCheatsheets(db, filepath.Dir(filepath.Dir(assetsDir)), testLogger())
	cs, err := CheatsheetGet(db, "test-cs-1")
	if err != nil {
		t.Fatalf("expected cheatsheet to be seeded: %v", err)
	}
	if cs.Name != "Test CS" {
		t.Fatalf("unexpected name: %s", cs.Name)
	}

	// Second seed should be idempotent
	SeedWelcomeCheatsheets(db, filepath.Dir(filepath.Dir(assetsDir)), testLogger())
	cs2, err := CheatsheetGet(db, "test-cs-1")
	if err != nil {
		t.Fatalf("expected cheatsheet to still exist: %v", err)
	}
	if cs2.CreatedAt != cs.CreatedAt {
		t.Fatal("cheatsheet was overwritten on second seed")
	}
}

func TestSeedWelcomeSkills(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0o755)

	db, err := InitSkillsDB(filepath.Join(tmpDir, "skills.db"))
	if err != nil {
		t.Fatalf("failed to init skills db: %v", err)
	}
	defer db.Close()

	mgr := NewSkillManager(db, skillsDir, testLogger())

	assetsDir := filepath.Join(tmpDir, "assets", "skill_samples")
	os.MkdirAll(assetsDir, 0o755)
	manifest := `[
		{"name": "Test Skill", "description": "A test skill", "executable": "test_skill.py", "category": "test", "tags": ["test"]}
	]`
	os.WriteFile(filepath.Join(assetsDir, "metadata.json"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(assetsDir, "test_skill.py"), []byte("print('hello')"), 0o644)

	// First seed
	SeedWelcomeSkills(mgr, skillsDir, tmpDir, testLogger())
	if _, err := os.Stat(filepath.Join(skillsDir, "test_skill.py")); os.IsNotExist(err) {
		t.Fatal("expected skill executable to be copied")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "test_skill.json")); os.IsNotExist(err) {
		t.Fatal("expected skill manifest to be written")
	}

	skills, err := mgr.ListSkillsFiltered("", "", "", nil)
	if err != nil {
		t.Fatalf("failed to list skills: %v", err)
	}
	found := false
	for _, s := range skills {
		if s.Name == "Test Skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected seeded skill to appear in registry")
	}

	// Second seed should not overwrite or error
	SeedWelcomeSkills(mgr, skillsDir, tmpDir, testLogger())
}

func TestSeedWelcomeSkillsWithExistingJSON(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0o755)

	db, err := InitSkillsDB(filepath.Join(tmpDir, "skills.db"))
	if err != nil {
		t.Fatalf("failed to init skills db: %v", err)
	}
	defer db.Close()

	mgr := NewSkillManager(db, skillsDir, testLogger())

	assetsDir := filepath.Join(tmpDir, "assets", "skill_samples")
	os.MkdirAll(assetsDir, 0o755)
	manifest := `[
		{"name": "Test Skill 2", "description": "A test skill", "executable": "test_skill2.py", "category": "test"}
	]`
	os.WriteFile(filepath.Join(assetsDir, "metadata.json"), []byte(manifest), 0o644)
	os.WriteFile(filepath.Join(assetsDir, "test_skill2.py"), []byte("print('hello')"), 0o644)
	os.WriteFile(filepath.Join(assetsDir, "test_skill2.json"), []byte(`{"name":"Test Skill 2","executable":"test_skill2.py"}`), 0o644)

	SeedWelcomeSkills(mgr, skillsDir, tmpDir, testLogger())
	if _, err := os.Stat(filepath.Join(skillsDir, "test_skill2.json")); os.IsNotExist(err) {
		t.Fatal("expected skill manifest to be copied")
	}
}

func TestSeedWelcomeSkillsMissingManifest(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	os.MkdirAll(skillsDir, 0o755)

	db, err := InitSkillsDB(filepath.Join(tmpDir, "skills.db"))
	if err != nil {
		t.Fatalf("failed to init skills db: %v", err)
	}
	defer db.Close()

	mgr := NewSkillManager(db, skillsDir, testLogger())
	// No manifest present — should log a warning and return without error
	SeedWelcomeSkills(mgr, skillsDir, tmpDir, testLogger())
}
