package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAvailableSkillTemplates(t *testing.T) {
	templates := AvailableSkillTemplates()
	if len(templates) != 15 {
		t.Fatalf("expected 15 templates, got %d", len(templates))
	}
	names := map[string]bool{}
	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("template has empty name")
		}
		if tmpl.Description == "" {
			t.Errorf("template %s has empty description", tmpl.Name)
		}
		if tmpl.Code == "" {
			t.Errorf("template %s has empty code", tmpl.Name)
		}
		if len(tmpl.Parameters) == 0 {
			t.Errorf("template %s has no parameters", tmpl.Name)
		}
		names[tmpl.Name] = true
	}
	for _, expected := range []string{"minimal_skill", "api_client", "data_transformer", "notification_sender", "monitor_check", "log_analyzer", "docker_manager", "backup_runner", "database_query", "ssh_executor", "mqtt_publisher", "daemon_monitor", "daemon_watcher", "daemon_listener", "daemon_mission"} {
		if !names[expected] {
			t.Errorf("missing expected template '%s'", expected)
		}
	}
}

func TestToFunctionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"weather_api", "weather_api"},
		{"my-skill", "my_skill"},
		{"123start", "skill_123start"},
		{"hello world", "hello_world"},
		{"", "skill_main"},
		{"valid_name_123", "valid_name_123"},
		{"--dashes--", "dashes"},
		{"a.b.c", "a_b_c"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toFunctionName(tt.input)
			if got != tt.expected {
				t.Errorf("toFunctionName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCreateSkillFromTemplate(t *testing.T) {
	dir := t.TempDir()

	result, err := CreateSkillFromTemplate(dir, "api_client", "test_api", "Test API skill", "https://api.example.com", nil, []string{"API_KEY"})
	if err != nil {
		t.Fatalf("CreateSkillFromTemplate failed: %v", err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// Check manifest was written
	jsonPath := filepath.Join(dir, "test_api.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest SkillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	if manifest.Name != "test_api" {
		t.Errorf("manifest name = %q, want %q", manifest.Name, "test_api")
	}
	if manifest.Executable != "test_api.py" {
		t.Errorf("manifest executable = %q, want %q", manifest.Executable, "test_api.py")
	}
	if len(manifest.Dependencies) == 0 {
		t.Error("expected dependencies from template")
	}
	if len(manifest.VaultKeys) != 1 || manifest.VaultKeys[0] != "API_KEY" {
		t.Errorf("vault_keys = %v, want [API_KEY]", manifest.VaultKeys)
	}

	// Check Python file was written
	pyPath := filepath.Join(dir, "test_api.py")
	pyData, err := os.ReadFile(pyPath)
	if err != nil {
		t.Fatalf("failed to read Python file: %v", err)
	}
	pyCode := string(pyData)
	if !contains(pyCode, "def test_api(") {
		t.Error("Python code does not contain expected function name")
	}
	if !contains(pyCode, "import requests") {
		t.Error("Python code does not contain 'import requests'")
	}
	if !contains(pyCode, "https://api.example.com") {
		t.Error("Python code does not contain base URL")
	}
}

func TestCreateSkillFromTemplate_AllTemplates(t *testing.T) {
	for _, tmpl := range AvailableSkillTemplates() {
		t.Run(tmpl.Name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := CreateSkillFromTemplate(dir, tmpl.Name, "test_"+tmpl.Name, "Test skill", "https://example.com", nil, nil)
			if err != nil {
				t.Fatalf("CreateSkillFromTemplate(%s) failed: %v", tmpl.Name, err)
			}
			// Verify both files exist
			if _, err := os.Stat(filepath.Join(dir, "test_"+tmpl.Name+".json")); err != nil {
				t.Errorf("manifest not created for %s", tmpl.Name)
			}
			if _, err := os.Stat(filepath.Join(dir, "test_"+tmpl.Name+".py")); err != nil {
				t.Errorf("script not created for %s", tmpl.Name)
			}
			pyData, err := os.ReadFile(filepath.Join(dir, "test_"+tmpl.Name+".py"))
			if err != nil {
				t.Fatalf("failed to read script for %s: %v", tmpl.Name, err)
			}
			if got := strings.Count(string(pyData), `if __name__ == "__main__":`); got != 1 {
				t.Fatalf("expected exactly one main block in %s, got %d", tmpl.Name, got)
			}
		})
	}
}

func TestCreateSkillFromTemplate_LogAnalyzerHasOperations(t *testing.T) {
	dir := t.TempDir()
	_, err := CreateSkillFromTemplate(dir, "log_analyzer", "workspace_log_analyzer", "", "", nil, nil)
	if err != nil {
		t.Fatalf("CreateSkillFromTemplate failed: %v", err)
	}

	pyData, err := os.ReadFile(filepath.Join(dir, "workspace_log_analyzer.py"))
	if err != nil {
		t.Fatalf("failed to read Python file: %v", err)
	}
	pyCode := string(pyData)
	if !contains(pyCode, "def workspace_log_analyzer(") {
		t.Fatal("generated log analyzer is missing function definition")
	}
	if !contains(pyCode, `"summary"`) {
		t.Fatal("generated log analyzer is missing summary operation")
	}
	if !contains(pyCode, `"errors"`) {
		t.Fatal("generated log analyzer is missing errors operation")
	}
	if !contains(pyCode, `"search"`) {
		t.Fatal("generated log analyzer is missing search operation")
	}
}

func TestCreateSkillFromTemplate_MinimalSkill(t *testing.T) {
	dir := t.TempDir()
	_, err := CreateSkillFromTemplate(dir, "minimal_skill", "tiny_helper", "", "", nil, nil)
	if err != nil {
		t.Fatalf("CreateSkillFromTemplate failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "tiny_helper.json"))
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}
	var manifest SkillManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	if manifest.Description == "" {
		t.Fatal("minimal skill manifest should use the template description when none is provided")
	}
	if len(manifest.Dependencies) != 0 {
		t.Fatalf("minimal skill should not add default dependencies, got %v", manifest.Dependencies)
	}
	if _, ok := manifest.Parameters["text"]; !ok {
		t.Fatalf("minimal skill should expose a simple optional text parameter, got %v", manifest.Parameters)
	}

	pyData, err := os.ReadFile(filepath.Join(dir, "tiny_helper.py"))
	if err != nil {
		t.Fatalf("failed to read Python file: %v", err)
	}
	pyCode := string(pyData)
	for _, marker := range []string{
		"def tiny_helper(text=\"\"):",
		`"status": "success"`,
		`"result": text if text else "ok"`,
	} {
		if !contains(pyCode, marker) {
			t.Fatalf("minimal skill code is missing marker %q", marker)
		}
	}
}

func TestCreateSkillFromTemplate_Conflict(t *testing.T) {
	dir := t.TempDir()

	// Create first
	_, err := CreateSkillFromTemplate(dir, "log_analyzer", "my_skill", "", "", nil, nil)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Try duplicate
	_, err = CreateSkillFromTemplate(dir, "log_analyzer", "my_skill", "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error on duplicate skill name, got nil")
	}
}

func TestCreateSkillFromTemplate_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	badNames := []string{
		"../etc/passwd",
		"foo/bar",
		`foo\bar`,
		"skill..name",
	}
	for _, name := range badNames {
		_, err := CreateSkillFromTemplate(dir, "api_client", name, "", "", nil, nil)
		if err == nil {
			t.Errorf("expected error for malicious name %q, got nil", name)
		}
	}
}

func TestCreateSkillFromTemplate_UnknownTemplate(t *testing.T) {
	dir := t.TempDir()
	_, err := CreateSkillFromTemplate(dir, "nonexistent", "test", "", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestCreateSkillFromTemplate_MergesDependencies(t *testing.T) {
	dir := t.TempDir()
	_, err := CreateSkillFromTemplate(dir, "api_client", "merged_deps", "test", "", []string{"pandas", "numpy"}, nil)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "merged_deps.json"))
	var manifest SkillManifest
	json.Unmarshal(data, &manifest)

	// Should have template deps (requests) + user deps (pandas, numpy)
	depSet := map[string]bool{}
	for _, d := range manifest.Dependencies {
		depSet[d] = true
	}
	if !depSet["requests"] {
		t.Error("missing template dep 'requests'")
	}
	if !depSet["pandas"] {
		t.Error("missing user dep 'pandas'")
	}
	if !depSet["numpy"] {
		t.Error("missing user dep 'numpy'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────────────────────
// Auto-dependency detection tests
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractImports(t *testing.T) {
	dir := t.TempDir()
	pyFile := filepath.Join(dir, "test.py")
	code := `import sys
import json
import os
import requests
from bs4 import BeautifulSoup
from PIL import Image
import yaml
from sklearn.ensemble import RandomForestClassifier
# import commented_out
    import indented_ignored
`
	os.WriteFile(pyFile, []byte(code), 0644)

	imports, err := extractImports(pyFile)
	if err != nil {
		t.Fatalf("extractImports failed: %v", err)
	}

	expected := map[string]bool{
		"sys":      true,
		"json":     true,
		"os":       true,
		"requests": true,
		"bs4":      true,
		"PIL":      true,
		"yaml":     true,
		"sklearn":  true,
	}
	for mod := range expected {
		if !imports[mod] {
			t.Errorf("expected import %q not found", mod)
		}
	}
	// Should NOT include commented or indented imports
	if imports["commented_out"] {
		t.Error("should not detect commented import")
	}
}

func TestImportToPyPIMapping(t *testing.T) {
	mappings := map[string]string{
		"PIL":     "Pillow",
		"cv2":     "opencv-python",
		"bs4":     "beautifulsoup4",
		"yaml":    "pyyaml",
		"sklearn": "scikit-learn",
	}
	for imp, expectedPkg := range mappings {
		pkg, ok := importToPyPI[imp]
		if !ok {
			t.Errorf("missing mapping for import %q", imp)
			continue
		}
		if pkg != expectedPkg {
			t.Errorf("importToPyPI[%q] = %q, want %q", imp, pkg, expectedPkg)
		}
	}
}

func TestPythonStdlibCompleteness(t *testing.T) {
	// Verify common stdlib modules are in the set
	essentials := []string{"os", "sys", "json", "re", "io", "csv", "pathlib", "collections",
		"datetime", "math", "hashlib", "subprocess", "threading", "typing", "unittest"}
	for _, mod := range essentials {
		if !pythonStdlib[mod] {
			t.Errorf("stdlib module %q missing from pythonStdlib set", mod)
		}
	}
}
