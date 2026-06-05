package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeTomlEditorResult(t *testing.T, raw string) TomlEditorResult {
	t.Helper()
	var r TomlEditorResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("failed to decode result: %v — raw: %s", err, raw)
	}
	return r
}

func setupTomlEditorTest(t *testing.T, filename, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "agent_workspace", "workdir")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(wsDir, filename), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return wsDir, filename
}

func TestTomlEditorGetKeysAndValidate(t *testing.T) {
	wsDir, fname := setupTomlEditorTest(t, "config.toml", "name = \"AuraGo\"\n[server]\nport = 8088\nenabled = true\n")

	res := decodeTomlEditorResult(t, ExecuteTomlEditor("get", fname, "server.port", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.Data != float64(8088) {
		t.Fatalf("expected port 8088, got %#v", res.Data)
	}

	res = decodeTomlEditorResult(t, ExecuteTomlEditor("keys", fname, "server", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected keys success, got %s: %s", res.Status, res.Message)
	}
	keys, ok := res.Data.([]interface{})
	if !ok || len(keys) != 2 {
		t.Fatalf("keys = %#v, want two keys", res.Data)
	}

	res = decodeTomlEditorResult(t, ExecuteTomlEditor("validate", fname, "", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected validate success, got %s: %s", res.Status, res.Message)
	}
}

func TestTomlEditorSetAndDelete(t *testing.T) {
	wsDir, fname := setupTomlEditorTest(t, "config.toml", "name = \"AuraGo\"\n[server]\nport = 8088\n")

	res := decodeTomlEditorResult(t, ExecuteTomlEditor("set", fname, "server.enabled", true, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected set success, got %s: %s", res.Status, res.Message)
	}

	res = decodeTomlEditorResult(t, ExecuteTomlEditor("delete", fname, "server.port", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected delete success, got %s: %s", res.Status, res.Message)
	}

	data, err := os.ReadFile(filepath.Join(wsDir, fname))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "enabled = true") {
		t.Fatalf("expected enabled key in file, got:\n%s", text)
	}
	if strings.Contains(text, "port") {
		t.Fatalf("expected port deleted, got:\n%s", text)
	}
}

func TestTomlEditorWriteOperationsRequireFilesystemWritePermission(t *testing.T) {
	wsDir, fname := setupTomlEditorTest(t, "config.toml", "name = \"AuraGo\"\n")
	ClearRuntimePermissionsForTest()
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	res := decodeTomlEditorResult(t, ExecuteTomlEditor("set", fname, "name", "Other", wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s: %s", res.Status, res.Message)
	}
	if !strings.Contains(res.Message, "filesystem write is disabled") {
		t.Fatalf("message = %q, want filesystem write permission denial", res.Message)
	}
}
