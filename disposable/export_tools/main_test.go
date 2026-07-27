package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/agent"
)

func testOperationTool() ToolExport {
	parameters := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []interface{}{"operation"},
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"list", "update", "delete", "delete_item"},
			},
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Required for update/delete.",
			},
			"item_id": map[string]interface{}{
				"type":        "string",
				"description": "Required for delete_item.",
			},
		},
	}
	properties, required := propsAndRequired(parameters)
	return ToolExport{
		Name:       "test_tool",
		Parameters: parameters,
		Properties: properties,
		Required:   required,
	}
}

func TestOperationContractsUseExactOperationNames(t *testing.T) {
	tool := testOperationTool()
	deleteArgs, err := fixtureFromSchema(tool, "operation", "delete")
	if err != nil {
		t.Fatal(err)
	}
	fields := operationRequiredFields(
		tool,
		OperationRef{Selector: "operation", Value: "delete"},
		deleteArgs,
	)
	if !reflect.DeepEqual(fields, []string{"id", "operation"}) {
		t.Fatalf("delete fields = %v, want id and operation", fields)
	}

	deleteItemArgs, err := fixtureFromSchema(tool, "operation", "delete_item")
	if err != nil {
		t.Fatal(err)
	}
	fields = operationRequiredFields(
		tool,
		OperationRef{Selector: "operation", Value: "delete_item"},
		deleteItemArgs,
	)
	if !reflect.DeepEqual(fields, []string{"item_id", "operation"}) {
		t.Fatalf("delete_item fields = %v, want item_id and operation", fields)
	}
}

func TestManualRequiredTableAddsOperationSpecificFields(t *testing.T) {
	root := t.TempDir()
	manualDir := filepath.Join(root, "prompts", "tools_manuals")
	if err := os.MkdirAll(manualDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manual := "| Parameter | Type | Required | Description |\n" +
		"|---|---|---|---|\n" +
		"| `operation` | string | yes | operation |\n" +
		"| `id` | string | for `update`, `delete` | resource ID |\n" +
		"| `item_id` | string | for item operations | item ID |\n" +
		"\n### `list`\n\n- **id** (required): test fixture\n"
	path := filepath.Join(manualDir, "test_tool.md")
	if err := os.WriteFile(path, []byte(manual), 0o600); err != nil {
		t.Fatal(err)
	}
	tool := testOperationTool()
	tool.ManualPath = "prompts/tools_manuals/test_tool.md"
	requirements, err := manualOperationRequirements(root, tool)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(requirements["update"], []string{"id", "operation"}) {
		t.Fatalf("update manual requirements = %v", requirements["update"])
	}
	if !reflect.DeepEqual(requirements["list"], []string{"id", "operation"}) {
		t.Fatalf("list bullet requirements = %v", requirements["list"])
	}
	if !reflect.DeepEqual(requirements["delete_item"], []string{"item_id", "operation"}) {
		t.Fatalf("delete_item manual requirements = %v", requirements["delete_item"])
	}
}

func TestFixtureFromSchemaHonorsEnumsAndStrictSchema(t *testing.T) {
	tool := testOperationTool()
	fixture, err := fixtureFromSchema(tool, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := fixture["operation"]; got != "list" {
		t.Fatalf("operation = %v, want first schema enum value list", got)
	}
	if err := validateArguments(tool, fixture); err != nil {
		t.Fatalf("generated fixture is not schema valid: %v", err)
	}
	fixture["unexpected"] = true
	if err := validateArguments(tool, fixture); err == nil {
		t.Fatal("strict schema accepted an unexpected property")
	}
}

func TestOperationContractRejectsMissingAndExcludedFields(t *testing.T) {
	tool := testOperationTool()
	fixture := OperationFixture{
		Selector:       "operation",
		Value:          "delete",
		Arguments:      map[string]interface{}{"operation": "delete"},
		RequiredFields: []string{"id", "operation"},
		ExcludedFields: []string{},
	}
	if err := validateOperationFields(tool, fixture); err == nil {
		t.Fatal("contract accepted a missing required operation field")
	}
	fixture.Arguments["id"] = "example-id"
	fixture.ExcludedFields = []string{"item_id"}
	if err := validateOperationFields(tool, fixture); err != nil {
		t.Fatalf("valid operation contract rejected: %v", err)
	}
	fixture.Arguments["item_id"] = "example-id"
	if err := validateOperationFields(tool, fixture); err == nil {
		t.Fatal("contract accepted an excluded operation field")
	}
}

func TestNativeTaggedRoundTripPreservesCalls(t *testing.T) {
	id := "v2-test"
	call := newToolCall(id, 0, "test_tool", map[string]interface{}{"operation": "list"})
	native := []Scenario{{
		ID:            id,
		SchemaVersion: datasetSchemaVersion,
		Messages: []Message{
			{Role: "user", Content: "list items"},
			{Role: "assistant", ToolCalls: []ToolCall{call}},
			{Role: "tool", ToolCallID: call.ID, Name: "test_tool", Content: `{"status":"ok"}`},
		},
	}}
	tagged, err := buildTaggedScenarios(native)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateTaggedRoundTrip(native, tagged); err != nil {
		t.Fatalf("native/tagged roundtrip failed: %v", err)
	}
	parsed := agent.ParseToolCall(tagged[0].Messages[1].Content)
	if !parsed.IsTool || parsed.Action != "test_tool" {
		t.Fatalf("AuraGo parser returned IsTool=%v action=%q", parsed.IsTool, parsed.Action)
	}
	if parsed.Params["operation"] != "list" {
		t.Fatalf("AuraGo parser arguments = %#v", parsed.Params)
	}
}

func TestFamilySplitIsDeterministic(t *testing.T) {
	first := splitForFamily("operation:test:list")
	for i := 0; i < 10; i++ {
		if got := splitForFamily("operation:test:list"); got != first {
			t.Fatalf("split changed from %s to %s", first, got)
		}
	}
}

func TestReadJSONReportsSourceLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(path, []byte("{\n  \"version\": 1,\n  broken\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var target map[string]interface{}
	err := readJSON(path, &target)
	if err == nil || !strings.Contains(err.Error(), "broken.json:3") {
		t.Fatalf("readJSON error = %v, want file and line 3", err)
	}
}
