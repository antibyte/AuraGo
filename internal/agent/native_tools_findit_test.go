package agent

import (
	"reflect"
	"testing"

	"aurago/internal/config"
)

func TestFindItNativeToolSchemaAndTypedFilters(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "findit") {
		t.Fatal("FindIt schema is visible when its feature flag is disabled")
	}
	schemas := builtinToolSchemas(ToolFeatureFlags{FindItEnabled: true})
	props := nativeToolProperties(t, schemas, "findit")
	for _, field := range []string{"operation", "entity", "id", "query", "location_id", "template_id", "category", "package", "available_only", "low_stock", "filters", "limit", "offset"} {
		if _, ok := props[field]; !ok {
			t.Errorf("FindIt schema is missing %q", field)
		}
	}
	operation := props["operation"].(map[string]interface{})
	if got, want := operation["enum"], []string{"search", "get", "availability", "shortages"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FindIt operations = %#v, want %#v", got, want)
	}
	entity := props["entity"].(map[string]interface{})
	if got, want := entity["enum"], []string{"items", "locations", "projects", "templates"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FindIt entities = %#v, want %#v", got, want)
	}
	filters := props["filters"].(map[string]interface{})
	filter := filters["items"].(map[string]interface{})
	filterProps := filter["properties"].(map[string]interface{})
	value := filterProps["value"].(map[string]interface{})
	choices := value["anyOf"].([]interface{})
	if len(choices) != 3 {
		t.Fatalf("FindIt filter value supports %d types, want string, number, and boolean", len(choices))
	}
	gotTypes := make([]string, 0, len(choices))
	for _, choice := range choices {
		gotTypes = append(gotTypes, choice.(map[string]interface{})["type"].(string))
	}
	if want := []string{"string", "number", "boolean"}; !reflect.DeepEqual(gotTypes, want) {
		t.Fatalf("FindIt filter value types = %#v, want %#v", gotTypes, want)
	}
}

func TestFindItReadOperationsAreClassifiedAsReads(t *testing.T) {
	for _, operation := range []string{"search", "get", "availability", "shortages"} {
		if !readActionOperation(operation) {
			t.Errorf("FindIt operation %q is not classified as a read", operation)
		}
	}
}

func TestFindItConfigRequiresExplicitGrantAndDesktopService(t *testing.T) {
	cfg := &config.Config{}
	cfg.FindIt.Enabled = true
	cfg.FindIt.AgentAccess = true
	if buildToolFlagsFromConfig(cfg).FindItEnabled {
		t.Fatal("FindIt agent schema is enabled without the Desktop app service")
	}
	cfg.VirtualDesktop.Enabled = true
	if !buildToolFlagsFromConfig(cfg).FindItEnabled {
		t.Fatal("FindIt agent schema is hidden with its app service and explicit grant enabled")
	}
	cfg.FindIt.AgentAccess = false
	if buildToolFlagsFromConfig(cfg).FindItEnabled {
		t.Fatal("FindIt agent schema is enabled without the agent access grant")
	}
	cfg.FindIt.AgentAccess = true
	cfg.FindIt.Enabled = false
	if buildToolFlagsFromConfig(cfg).FindItEnabled {
		t.Fatal("FindIt agent schema is enabled while FindIt is disabled")
	}
}
