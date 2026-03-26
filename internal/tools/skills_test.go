package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupDummySkill creates a minimal skill in skillsDir so that
// ExecuteSkill reaches the args-size validation path.
func setupDummySkill(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// ListSkills expects .json files directly in skillsDir.
	manifest := SkillManifest{
		Name:       "big_args_test",
		Executable: "run.py",
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "big_args_test.json"), data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "run.py"), []byte("pass\n"), 0644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return dir
}

func TestExecuteSkill_RejectsOversizedArgs(t *testing.T) {
	skillsDir := setupDummySkill(t)
	workspaceDir := t.TempDir()

	bigValue := strings.Repeat("x", maxSkillArgsBytes+1)
	args := map[string]interface{}{"data": bigValue}

	_, err := ExecuteSkill(skillsDir, workspaceDir, "big_args_test", args)
	if err == nil {
		t.Fatal("expected error for oversized args, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteSkillWithSecrets_RejectsOversizedArgs(t *testing.T) {
	skillsDir := setupDummySkill(t)
	workspaceDir := t.TempDir()

	bigValue := strings.Repeat("x", maxSkillArgsBytes+1)
	args := map[string]interface{}{"data": bigValue}

	_, err := ExecuteSkillWithSecrets(skillsDir, workspaceDir, "big_args_test", args, nil, nil)
	if err == nil {
		t.Fatal("expected error for oversized args, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("unexpected error: %v", err)
	}
}
