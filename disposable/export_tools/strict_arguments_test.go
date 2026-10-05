package main

import (
	"reflect"
	"testing"
)

func TestStrictArgumentsOnlyCompleteOptionalValues(t *testing.T) {
	tool := testOperationTool()
	tool.ArgumentSchema = cloneMap(tool.Parameters)
	tool.Parameters["required"] = []interface{}{"id", "item_id", "operation"}
	for _, field := range []string{"id", "item_id"} {
		tool.Properties[field].(map[string]interface{})["type"] = []interface{}{"string", "null"}
	}
	sparse := map[string]interface{}{"operation": "list"}
	if err := validateArguments(tool, sparse); err != nil {
		t.Fatal(err)
	}
	if len(sparse) != 1 {
		t.Fatal("curated arguments mutated")
	}
	completed := strictArguments(tool, sparse)
	if len(completed) != 3 || completed["id"] != nil || completed["item_id"] != nil {
		t.Fatal(completed)
	}
	if err := validateArguments(tool, map[string]interface{}{}); err == nil {
		t.Fatal("invented required operation")
	}
	fixture := OperationFixture{Selector: "operation", Value: "update", Arguments: map[string]interface{}{"operation": "update", "id": nil}, RequiredFields: []string{"operation", "id"}}
	if err := validateOperationFields(tool, fixture); err == nil {
		t.Fatal("null satisfied a required operation value")
	}
	fixture.Arguments = map[string]interface{}{"operation": "list"}
	fixture.RequiredFields, fixture.ExcludedFields = []string{"operation"}, []string{"id"}
	if err := validateOperationFields(tool, fixture); err != nil {
		t.Fatal(err)
	}
	selector := tool.Properties["operation"].(map[string]interface{})
	selector["enum"] = append(selector["enum"].([]interface{}), nil)
	if len(extractOperations(tool.Parameters)) != 4 {
		t.Fatal("null generated an operation")
	}
}

func TestStrictArgumentsNestedCopiesAndOpaqueData(t *testing.T) {
	child := map[string]interface{}{"type": "object", "required": []interface{}{"id"}, "properties": map[string]interface{}{"id": map[string]interface{}{"type": "integer"}, "optional": map[string]interface{}{"type": "boolean"}}}
	tool := ToolExport{ArgumentSchema: map[string]interface{}{"properties": map[string]interface{}{
		"rows":    map[string]interface{}{"type": "array", "items": child},
		"payload": map[string]interface{}{"type": "string"},
		"free":    map[string]interface{}{"type": "object"},
	}}}
	args := map[string]interface{}{"rows": []interface{}{map[string]interface{}{"id": float64(0)}}, "payload": `{"optional":null}`, "free": map[string]interface{}{"optional": nil}}
	before := cloneMap(args)
	got := strictArguments(tool, args)
	if !reflect.DeepEqual(args, before) {
		t.Fatal("projection changed source fixture")
	}
	row := got["rows"].([]interface{})[0].(map[string]interface{})
	if len(row) != 2 || row["id"] != float64(0) || row["optional"] != nil {
		t.Fatal(got)
	}
	if got["payload"] != args["payload"] || !reflect.DeepEqual(got["free"], args["free"]) {
		t.Fatal("opaque payload changed")
	}
}
