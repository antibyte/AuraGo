package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestRunToolNotFoundOutputIsStructuredAndDoesNotExecuteAlias(t *testing.T) {
	toolsDir := filepath.Join(t.TempDir(), "tools")
	if err := os.MkdirAll(toolsDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	manifest := tools.NewManifest(toolsDir)
	for _, name := range []string{"zeta.py", "alpha.py", "beta.py"} {
		if err := manifest.Register(name, "test custom tool"); err != nil {
			t.Fatalf("Register(%q) error = %v", name, err)
		}
	}

	output := runToolNotFoundOutput(manifest, "save_note")
	if !strings.HasPrefix(output, "Tool Output: ") {
		t.Fatalf("output prefix = %q", output)
	}
	var result runToolNotFoundResult
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.ErrorCode != "CUSTOM_TOOL_NOT_FOUND" || result.Status != "error" {
		t.Fatalf("result = %#v", result)
	}
	if want := []string{"alpha.py", "beta.py", "zeta.py"}; !reflect.DeepEqual(result.AvailableCustomTools, want) {
		t.Fatalf("available tools = %#v, want %#v", result.AvailableCustomTools, want)
	}
	if result.Suggestion == nil || result.Suggestion.ToolName != "manage_notes" {
		t.Fatalf("suggestion = %#v, want manage_notes", result.Suggestion)
	}
	if result.Suggestion.Arguments["operation"] != "add" {
		t.Fatalf("suggestion arguments = %#v", result.Suggestion.Arguments)
	}
}

func TestRunToolNotFoundLegacyTodoSuggestionUsesRealSchema(t *testing.T) {
	output := runToolNotFoundOutput(nil, "list_open_tasks.py")
	var result runToolNotFoundResult
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "Tool Output: ")), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.Suggestion == nil || result.Suggestion.ToolName != "manage_todos" {
		t.Fatalf("suggestion = %#v, want manage_todos", result.Suggestion)
	}
	if result.Suggestion.Arguments["operation"] != "list" || result.Suggestion.Arguments["status"] != "open" {
		t.Fatalf("suggestion arguments = %#v", result.Suggestion.Arguments)
	}
	if len(result.AvailableCustomTools) != 0 {
		t.Fatalf("available tools = %#v, want empty", result.AvailableCustomTools)
	}
}

func TestDispatchRunToolMissingAliasReturnsRecoveryWithoutMutation(t *testing.T) {
	root := t.TempDir()
	toolsDir := filepath.Join(root, "tools")
	skillsDir := filepath.Join(root, "skills")
	for _, dir := range []string{toolsDir, skillsDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}
	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.ToolsDir = toolsDir
	cfg.Directories.SkillsDir = skillsDir
	output := dispatchPython(ToolCall{Action: "run_tool", Name: "list_open_tasks"}, &DispatchContext{
		Cfg:      cfg,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Manifest: tools.NewManifest(toolsDir),
	})
	if !strings.Contains(output, `"error_code":"CUSTOM_TOOL_NOT_FOUND"`) ||
		!strings.Contains(output, `"tool_name":"manage_todos"`) {
		t.Fatalf("dispatch output = %q", output)
	}
	entries, err := os.ReadDir(toolsDir)
	if err != nil {
		t.Fatalf("ReadDir(tools) error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("missing alias dispatch mutated tools directory: %#v", entries)
	}
}

func TestAvailableCustomToolNamesIsBoundedAndSorted(t *testing.T) {
	toolsDir := filepath.Join(t.TempDir(), "tools")
	if err := os.MkdirAll(toolsDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	manifest := tools.NewManifest(toolsDir)
	for _, name := range []string{"f.py", "e.py", "d.py", "c.py", "b.py", "a.py"} {
		if err := manifest.Register(name, "test custom tool"); err != nil {
			t.Fatalf("Register(%q) error = %v", name, err)
		}
	}
	if got, want := availableCustomToolNames(manifest, "", 3), []string{"a.py", "b.py", "c.py"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("availableCustomToolNames() = %#v, want %#v", got, want)
	}
}
