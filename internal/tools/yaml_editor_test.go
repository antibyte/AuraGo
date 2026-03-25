package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeYamlEditorResult(t *testing.T, raw string) YamlEditorResult {
	t.Helper()
	var r YamlEditorResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("failed to decode result: %v — raw: %s", err, raw)
	}
	return r
}

func setupYamlEditorTest(t *testing.T, filename, content string) (string, string) {
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

func TestYamlEditorGet(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "name: Alice\nage: 30\nnested:\n  key: value\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("get", fname, "name", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.Data != "Alice" {
		t.Fatalf("expected 'Alice', got: %v", res.Data)
	}

	res = decodeYamlEditorResult(t, ExecuteYamlEditor("get", fname, "nested.key", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.Data != "value" {
		t.Fatalf("expected 'value', got: %v", res.Data)
	}
}

func TestYamlEditorGetNotFound(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "name: Alice\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("get", fname, "nonexistent", nil, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}

func TestYamlEditorSet(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "name: Alice\nage: 30\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("set", fname, "name", "Bob", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.Contains(string(data), "Bob") {
		t.Fatalf("expected Bob in file, got: %s", data)
	}
}

func TestYamlEditorSetNestedNew(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "name: Alice\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("set", fname, "config.debug", true, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	text := string(data)
	if !strings.Contains(text, "config") || !strings.Contains(text, "debug") {
		t.Fatalf("expected nested config.debug, got: %s", text)
	}
}

func TestYamlEditorDelete(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "name: Alice\nage: 30\ncity: NYC\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("delete", fname, "age", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	text := string(data)
	if strings.Contains(text, "age") {
		t.Fatalf("expected age deleted, got: %s", text)
	}
	if !strings.Contains(text, "name") {
		t.Fatalf("expected name preserved, got: %s", text)
	}
}

func TestYamlEditorKeys(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "a: 1\nb: 2\nc: 3\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("keys", fname, "", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	keys, ok := res.Data.([]interface{})
	if !ok {
		t.Fatalf("expected array, got: %T", res.Data)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestYamlEditorValidate(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "valid: true\n")

	res := decodeYamlEditorResult(t, ExecuteYamlEditor("validate", fname, "", nil, wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
}

func TestYamlEditorUnknownOp(t *testing.T) {
	wsDir, fname := setupYamlEditorTest(t, "test.yaml", "a: 1\n")
	res := decodeYamlEditorResult(t, ExecuteYamlEditor("invalid", fname, "", nil, wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}
