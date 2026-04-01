package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifest_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir)

	tools, err := m.Load()
	if err != nil {
		t.Fatalf("unexpected error loading empty manifest: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected empty map, got %d entries", len(tools))
	}
}

func TestManifest_RegisterAndLoad(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir)

	if err := m.Register("hello.py", "says hello"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	tools, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if tools["hello.py"] != "says hello" {
		t.Errorf("expected 'says hello', got %q", tools["hello.py"])
	}
}

func TestManifest_SavedFormatHasVersion(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir)

	if err := m.Register("test.py", "test tool"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest file: %v", err)
	}

	var mf manifestFile
	if err := json.Unmarshal(data, &mf); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}
	if mf.Version != currentManifestVersion {
		t.Errorf("expected version %d, got %d", currentManifestVersion, mf.Version)
	}
	if mf.Tools["test.py"] != "test tool" {
		t.Errorf("expected 'test tool', got %q", mf.Tools["test.py"])
	}
}

func TestManifest_LoadLegacyBareMap(t *testing.T) {
	dir := t.TempDir()
	legacy := map[string]string{"old_tool.py": "legacy tool"}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("failed to write legacy manifest: %v", err)
	}

	m := NewManifest(dir)
	tools, err := m.Load()
	if err != nil {
		t.Fatalf("Load failed for legacy manifest: %v", err)
	}
	if tools["old_tool.py"] != "legacy tool" {
		t.Errorf("expected 'legacy tool', got %q", tools["old_tool.py"])
	}
}

func TestManifest_LegacyMigratedOnSave(t *testing.T) {
	dir := t.TempDir()
	// Write legacy bare map
	legacy := map[string]string{"old_tool.py": "legacy"}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("failed to write legacy manifest: %v", err)
	}

	m := NewManifest(dir)
	// Register a new tool — this triggers save in versioned format
	if err := m.Register("new_tool.py", "new"); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify file is now in versioned format
	raw, _ := os.ReadFile(filepath.Join(dir, "manifest.json"))
	var mf manifestFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if mf.Version != currentManifestVersion {
		t.Errorf("expected version %d after migration, got %d", currentManifestVersion, mf.Version)
	}
	if mf.Tools["old_tool.py"] != "legacy" {
		t.Errorf("lost legacy tool after migration")
	}
	if mf.Tools["new_tool.py"] != "new" {
		t.Errorf("new tool not present after migration")
	}
}

func TestManifest_SaveToolPathTraversal(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir)

	tests := []string{"../escape.py", "sub/dir.py", "..\\escape.py", ""}
	for _, name := range tests {
		err := m.SaveTool(dir, name, "bad", "code")
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
	}
}

func TestManifest_RegisterRejectsCorruptManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"version":`), 0o644); err != nil {
		t.Fatalf("failed to write corrupt manifest: %v", err)
	}
	m := NewManifest(dir)

	err := m.Register("new_tool.py", "new tool")
	if err == nil {
		t.Fatal("expected Register to fail on corrupt manifest")
	}
	if got := err.Error(); got == "" || !containsManifestText(got, "failed to parse manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func containsManifestText(value, want string) bool {
	return len(want) > 0 && strings.Contains(value, want)
}
