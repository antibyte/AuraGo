package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func decodeJsonEditorResult(t *testing.T, raw string) JsonEditorResult {
	t.Helper()
	var r JsonEditorResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("failed to decode result: %v — raw: %s", err, raw)
	}
	return r
}

func setupJsonEditorTest(t *testing.T, filename, content string) (string, string) {
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

func TestJsonEditorGet(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"name":"Alice","age":30,"nested":{"key":"value"}}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("get", fname, "name", nil, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.Data != "Alice" {
		t.Fatalf("expected 'Alice', got: %v", res.Data)
	}

	res = decodeJsonEditorResult(t, ExecuteJsonEditor("get", fname, "nested.key", nil, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	if res.Data != "value" {
		t.Fatalf("expected 'value', got: %v", res.Data)
	}
}

func TestJsonEditorGetNotFound(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"name":"Alice"}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("get", fname, "nonexistent", nil, "", wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}

func TestJsonEditorSet(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"name":"Alice","age":30}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("set", fname, "name", "Bob", "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	// Verify the value was set
	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.Contains(string(data), `"Bob"`) {
		t.Fatalf("expected Bob in file, got: %s", data)
	}
}

func TestJsonEditorSetNestedNew(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"name":"Alice"}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("set", fname, "config.debug", true, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.Contains(string(data), `"debug": true`) {
		t.Fatalf("expected nested value, got: %s", data)
	}
}

func TestJsonEditorSetCreatesFile(t *testing.T) {
	wsDir, _ := setupJsonEditorTest(t, "dummy.json", "")
	fname := "new.json"

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("set", fname, "key", "value", "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, err := os.ReadFile(filepath.Join(wsDir, fname))
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if !strings.Contains(string(data), `"key"`) {
		t.Fatalf("expected key in file, got: %s", data)
	}
}

func TestJsonEditorDelete(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"name":"Alice","age":30,"city":"NYC"}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("delete", fname, "age", nil, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if strings.Contains(string(data), "age") {
		t.Fatalf("expected age deleted, got: %s", data)
	}
	if !strings.Contains(string(data), "name") {
		t.Fatalf("expected name preserved, got: %s", data)
	}
}

func TestJsonEditorKeys(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"a":1,"b":2,"c":3}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("keys", fname, "", nil, "", wsDir))
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

func TestJsonEditorValidate(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"valid": true}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("validate", fname, "", nil, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
}

func TestJsonEditorValidateInvalid(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "bad.json", `{not valid json`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("validate", fname, "", nil, "", wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error for invalid JSON, got %s", res.Status)
	}
}

func TestJsonEditorFormat(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{"a":1,"b":2}`)

	res := decodeJsonEditorResult(t, ExecuteJsonEditor("format", fname, "", nil, "", wsDir))
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	data, _ := os.ReadFile(filepath.Join(wsDir, fname))
	if !strings.Contains(string(data), "  ") {
		t.Fatalf("expected indented output, got: %s", data)
	}
}

func TestJsonEditorUnknownOp(t *testing.T) {
	wsDir, fname := setupJsonEditorTest(t, "test.json", `{}`)
	res := decodeJsonEditorResult(t, ExecuteJsonEditor("invalid", fname, "", nil, "", wsDir))
	if res.Status != "error" {
		t.Fatalf("expected error, got %s", res.Status)
	}
}
