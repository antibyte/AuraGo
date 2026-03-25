package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXmlEditorGet(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?>
<config>
  <server host="localhost" port="8080">
    <name>Main Server</name>
  </server>
  <database type="postgres">
    <host>db.local</host>
    <port>5432</port>
  </database>
</config>`), 0644)

	// Get single element
	result := ExecuteXmlEditor("get", "test.xml", "//server/name", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data := res.Data.(map[string]interface{})
	if data["text"] != "Main Server" {
		t.Errorf("expected text 'Main Server', got '%v'", data["text"])
	}

	// Get with attributes
	result = ExecuteXmlEditor("get", "test.xml", "//server", nil, dir)
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}
	data = res.Data.(map[string]interface{})
	attrs := data["attributes"].(map[string]interface{})
	if attrs["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got '%v'", attrs["host"])
	}

	// Get non-existent path
	result = ExecuteXmlEditor("get", "test.xml", "//nonexistent", nil, dir)
	json.Unmarshal([]byte(result), &res)
	if res.Status != "error" {
		t.Errorf("expected error for non-existent path, got %s", res.Status)
	}
}

func TestXmlEditorSetText(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?>
<config>
  <database>
    <host>localhost</host>
    <port>5432</port>
  </database>
</config>`), 0644)

	result := ExecuteXmlEditor("set_text", "test.xml", "//database/host", "db.production.local", dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	// Verify
	content, _ := os.ReadFile(fp)
	if !strings.Contains(string(content), "db.production.local") {
		t.Error("file should contain updated host value")
	}
	if strings.Contains(string(content), ">localhost<") {
		t.Error("file should no longer contain old host value")
	}
}

func TestXmlEditorSetAttribute(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?>
<config>
  <server host="localhost" port="8080"/>
</config>`), 0644)

	value := map[string]interface{}{"name": "port", "value": "9090"}
	result := ExecuteXmlEditor("set_attribute", "test.xml", "//server", value, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	content, _ := os.ReadFile(fp)
	if !strings.Contains(string(content), `port="9090"`) {
		t.Error("file should contain updated port attribute")
	}
}

func TestXmlEditorAddElement(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?>
<config>
  <servers>
    <server name="main"/>
  </servers>
</config>`), 0644)

	value := map[string]interface{}{
		"tag": "server",
		"attributes": map[string]interface{}{
			"name": "backup",
			"host": "backup.local",
		},
	}
	result := ExecuteXmlEditor("add_element", "test.xml", "//servers", value, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	content, _ := os.ReadFile(fp)
	if !strings.Contains(string(content), `name="backup"`) {
		t.Error("file should contain new server element")
	}
}

func TestXmlEditorDelete(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?>
<config>
  <database>
    <host>localhost</host>
    <port>5432</port>
    <debug>true</debug>
  </database>
</config>`), 0644)

	result := ExecuteXmlEditor("delete", "test.xml", "//database/debug", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	content, _ := os.ReadFile(fp)
	if strings.Contains(string(content), "<debug>") {
		t.Error("file should not contain debug element after deletion")
	}
	if !strings.Contains(string(content), "<host>") {
		t.Error("file should still contain host element")
	}
}

func TestXmlEditorValidate(t *testing.T) {
	dir := t.TempDir()

	// Valid XML
	fp := filepath.Join(dir, "valid.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?><root><child/></root>`), 0644)
	result := ExecuteXmlEditor("validate", "valid.xml", "", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Errorf("expected success for valid XML, got %s: %s", res.Status, res.Message)
	}

	// Invalid XML
	fp = filepath.Join(dir, "invalid.xml")
	os.WriteFile(fp, []byte(`<root><unclosed>`), 0644)
	result = ExecuteXmlEditor("validate", "invalid.xml", "", nil, dir)
	json.Unmarshal([]byte(result), &res)
	if res.Status != "error" {
		t.Errorf("expected error for invalid XML, got %s", res.Status)
	}
}

func TestXmlEditorFormat(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<?xml version="1.0"?><config><server><host>localhost</host></server></config>`), 0644)

	result := ExecuteXmlEditor("format", "test.xml", "", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "success" {
		t.Fatalf("expected success, got %s: %s", res.Status, res.Message)
	}

	content, _ := os.ReadFile(fp)
	if !strings.Contains(string(content), "\n") {
		t.Error("formatted XML should contain newlines")
	}
}

func TestXmlEditorPathTraversal(t *testing.T) {
	dir := t.TempDir()
	result := ExecuteXmlEditor("get", "../../../etc/passwd", "//root", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "error" {
		t.Error("path traversal should be rejected")
	}
}

func TestXmlEditorUnknownOperation(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.xml")
	os.WriteFile(fp, []byte(`<root/>`), 0644)

	result := ExecuteXmlEditor("unknown", "test.xml", "", nil, dir)
	var res XmlEditorResult
	json.Unmarshal([]byte(result), &res)
	if res.Status != "error" {
		t.Error("unknown operation should return error")
	}
	if !strings.Contains(res.Message, "Unknown xml_editor operation") {
		t.Error("error message should mention unknown operation")
	}
}
