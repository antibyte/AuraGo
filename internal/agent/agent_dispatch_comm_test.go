package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSynthesizeExecuteSkillArgsPromotesTopLevelFields(t *testing.T) {
	tc := ToolCall{
		Action:   "execute_skill",
		Skill:    "virustotal_scan",
		Resource: "example.com",
		FilePath: "test_virussignatur.txt",
		Mode:     "auto",
	}

	args := synthesizeExecuteSkillArgs(tc)
	if got, _ := args["resource"].(string); got != "example.com" {
		t.Fatalf("resource = %q, want example.com", got)
	}
	if got, _ := args["file_path"].(string); got != "test_virussignatur.txt" {
		t.Fatalf("file_path = %q, want test_virussignatur.txt", got)
	}
	if got, _ := args["mode"].(string); got != "auto" {
		t.Fatalf("mode = %q, want auto", got)
	}
	if _, ok := args["skill"]; ok {
		t.Fatal("did not expect skill metadata in synthesized args")
	}
	if _, ok := args["action"]; ok {
		t.Fatal("did not expect action metadata in synthesized args")
	}
}

func TestFilterExecuteSkillArgsUsesManifestParameters(t *testing.T) {
	skillsDir := t.TempDir()
	manifest := `{
  "name": "virustotal_scan",
  "description": "Scan with VirusTotal",
  "executable": "__builtin__",
  "parameters": {
    "resource": "Resource to scan",
    "file_path": "Local file path",
    "mode": "Scan mode"
  }
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "virustotal_scan.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	filtered := filterExecuteSkillArgs(skillsDir, "virustotal_scan", map[string]interface{}{
		"resource":  "example.com",
		"file_path": "sample.txt",
		"mode":      "auto",
		"title":     "should be removed",
	})

	if len(filtered) != 3 {
		t.Fatalf("filtered arg count = %d, want 3", len(filtered))
	}
	if _, ok := filtered["title"]; ok {
		t.Fatal("did not expect unrelated field 'title' to survive filtering")
	}
}
