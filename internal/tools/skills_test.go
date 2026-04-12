package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

	_, err := ExecuteSkill(context.Background(), skillsDir, workspaceDir, "big_args_test", args)
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

	_, err := ExecuteSkillWithSecrets(context.Background(), skillsDir, workspaceDir, "big_args_test", args, nil, nil, "", "", nil)
	if err == nil {
		t.Fatal("expected error for oversized args, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildSandboxSkillExecCodeUsesBase64Args(t *testing.T) {
	skillCode := "def test_skill(**kwargs):\n    return kwargs\n\nif __name__ == \"__main__\":\n    pass\n"
	argsBytes := []byte(`{"name":"x' ); import os; #"}`)
	code, err := buildSandboxSkillExecCode("test_skill", skillCode, argsBytes, "")
	if err != nil {
		t.Fatalf("buildSandboxSkillExecCode failed: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(argsBytes)
	if !strings.Contains(code, "base64.b64decode") {
		t.Fatalf("expected base64 decode in generated code, got %q", code)
	}
	if !strings.Contains(code, encoded) {
		t.Fatalf("expected encoded args in generated code")
	}
	if strings.Contains(code, string(argsBytes)) {
		t.Fatalf("raw args JSON should not be embedded directly in generated code: %q", code)
	}
	if strings.Contains(code, "if __name__") {
		t.Fatalf("main block should be stripped from generated sandbox code")
	}
}

func TestExecuteSkillWrapsListSkillsError(t *testing.T) {
	skillsDir := setupDummySkill(t)
	workspaceDir := t.TempDir()
	args := map[string]interface{}{"bad": func() {}}
	_, err := ExecuteSkill(context.Background(), skillsDir, workspaceDir, "big_args_test", args)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "failed to serialize args JSON") {
		t.Fatalf("unexpected error text: %v", err)
	}
	var marshalErr *json.UnsupportedTypeError
	if !errors.As(err, &marshalErr) {
		t.Fatalf("expected wrapped *json.UnsupportedTypeError, got %T: %v", err, err)
	}
}

func TestExecuteSkillWithSecretsWrapsListSkillsError(t *testing.T) {
	skillsDir := setupDummySkill(t)
	workspaceDir := t.TempDir()
	args := map[string]interface{}{"bad": func() {}}
	_, err := ExecuteSkillWithSecrets(context.Background(), skillsDir, workspaceDir, "big_args_test", args, nil, nil, "", "", nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !strings.Contains(err.Error(), "failed to serialize args JSON") {
		t.Fatalf("unexpected error text: %v", err)
	}
	var marshalErr *json.UnsupportedTypeError
	if !errors.As(err, &marshalErr) {
		t.Fatalf("expected wrapped *json.UnsupportedTypeError, got %T: %v", err, err)
	}
}

func TestExecuteSkillWithSecretsRejectsInvalidExecutablePath(t *testing.T) {
	dir := t.TempDir()
	manifest := SkillManifest{
		Name:       "bad_path_skill",
		Executable: "../escape.py",
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "bad_path_skill.json"), data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := ExecuteSkillWithSecrets(context.Background(), dir, t.TempDir(), "bad_path_skill", nil, nil, nil, "", "", nil)
	if err == nil {
		t.Fatal("expected invalid executable path error")
	}
	if !strings.Contains(err.Error(), "invalid executable path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPythonPackagesForImportsSortsAndDeduplicates(t *testing.T) {
	imports := map[string]bool{
		"requests": true,
		"json":     true,
		"bs4":      true,
		"PIL":      true,
		"yaml":     true,
	}

	got := pythonPackagesForImports(imports)
	want := []string{"Pillow", "beautifulsoup4", "pyyaml", "requests"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("packages = %v, want %v", got, want)
	}
}

func TestParsePipShowInstalledPackagesParsesBatchOutput(t *testing.T) {
	output := []byte("Name: requests\nVersion: 2.32.0\n\nWARNING: Package(s) not found: missing\n\nName: beautifulsoup4\nVersion: 4.12.3\n")
	installed := parsePipShowInstalledPackages(output)
	if !installed["requests"] {
		t.Fatal("expected requests to be detected as installed")
	}
	if !installed["beautifulsoup4"] {
		t.Fatal("expected beautifulsoup4 to be detected as installed")
	}
	if installed["missing"] {
		t.Fatal("did not expect missing package to be marked installed")
	}
}

func TestMissingPythonPackagesUsesCaseInsensitiveComparison(t *testing.T) {
	packages := []string{"Pillow", "beautifulsoup4", "requests"}
	installed := map[string]bool{"pillow": true, "requests": true}
	got := missingPythonPackages(packages, installed)
	want := []string{"beautifulsoup4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing = %v, want %v", got, want)
	}
}

func TestBuildSandboxSkillExecCode_RejectsMissingFunction(t *testing.T) {
	skillCode := "def other_func():\n    pass\n\nif __name__ == '__main__':\n    pass\n"
	_, err := buildSandboxSkillExecCode("my_skill", skillCode, []byte("{}"), "")
	if err == nil {
		t.Fatal("expected error for missing function, got nil")
	}
	if !strings.Contains(err.Error(), "does not define the expected function") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildSandboxSkillExecCode_StripsMainBlockCorrectly(t *testing.T) {
	skillCode := "# if __name__ comment\ndef my_skill(x):\n    return x\n\nif __name__ == '__main__':\n    pass\n"
	code, err := buildSandboxSkillExecCode("my_skill", skillCode, []byte(`{"x":1}`), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(code, "if __name__ == '__main__'") {
		t.Errorf("main block should be stripped, got:\n%s", code)
	}
	if !strings.Contains(code, "def my_skill(x)") {
		t.Error("function definition should be preserved")
	}
}

func TestExecuteSkillInSandbox_RejectPathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := ExecuteSkillInSandbox(dir, "../etc/passwd", nil, nil, nil, 0, nil, "", "", nil)
	if err == nil {
		t.Fatal("expected error for path traversal skill name")
	}
}

func TestExecuteSkillInSandbox_RejectMissingSkill(t *testing.T) {
	dir := t.TempDir()
	_, err := ExecuteSkillInSandbox(dir, "nonexistent", nil, nil, nil, 0, nil, "", "", nil)
	if err == nil {
		t.Fatal("expected error for missing skill")
	}
}

func TestExtractSkillParameterNames_LegacyFlatMap(t *testing.T) {
	params := map[string]interface{}{"stadt": "Name", "einheit": "Einheit"}
	names := ExtractSkillParameterNames(params)
	want := []string{"einheit", "stadt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestExtractSkillParameterNames_JSONSchema(t *testing.T) {
	params := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"stadt": map[string]interface{}{"type": "string"},
			"einheit": map[string]interface{}{"type": "string"},
		},
	}
	names := ExtractSkillParameterNames(params)
	want := []string{"einheit", "stadt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestBuildSkillCommand_PythonAndShell(t *testing.T) {
	manifest := SkillManifest{Executable: "test.py"}
	cmd := buildSkillCommand(context.Background(), "/tmp", manifest, "/tmp/test.py")
	if cmd.Path == "" {
		t.Fatal("expected python command")
	}

	if runtime.GOOS != "windows" {
		manifest.Executable = "test.sh"
		cmd = buildSkillCommand(context.Background(), "/tmp", manifest, "/tmp/test.sh")
		if cmd.Path == "" {
			t.Fatal("expected bash command for .sh")
		}
	} else {
		manifest.Executable = "test.ps1"
		cmd = buildSkillCommand(context.Background(), "/tmp", manifest, "/tmp/test.ps1")
		if cmd.Path == "" {
			t.Fatal("expected powershell command for .ps1")
		}
	}
}
