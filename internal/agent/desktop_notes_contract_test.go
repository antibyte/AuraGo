package agent

import "testing"

func TestDesktopNotesSchemaAndChannelContract(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		schemas := builtinToolSchemas(ToolFeatureFlags{VirtualDesktopEnabled: enabled})
		names := toolNames(schemas)
		if containsName(names, "desktop_notes") != enabled {
			t.Fatalf("notes schema gating: %v", enabled)
		}
		channel := channelAdaptiveAlwaysInclude(RunConfig{MessageSource: "virtual_desktop_chat"}, nil, ToolFeatureFlags{VirtualDesktopEnabled: enabled})
		if containsName(channel, "desktop_notes") != enabled {
			t.Fatalf("desktop notes access: %v", enabled)
		}
		for _, schema := range schemas {
			if schema.Function == nil || schema.Function.Name != "desktop_notes" {
				continue
			}
			properties := schema.Function.Parameters.(map[string]interface{})["properties"].(map[string]interface{})
			operations := properties["operation"].(map[string]interface{})["enum"].([]string)
			if len(operations) != 4 {
				t.Fatal(operations)
			}
			for _, name := range operations {
				if name != "list" && name != "search" && name != "read" && name != "create" {
					t.Fatal("mutation advertised", name)
				}
			}
		}
	}
}
