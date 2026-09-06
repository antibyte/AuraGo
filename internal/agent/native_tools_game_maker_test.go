package agent

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestGameMakerAssetSchemaKeepsGenerationAndPackOperations(t *testing.T) {
	if got := appendGameMakerToolSchemas(nil, ToolFeatureFlags{}); len(got) != 0 {
		t.Fatal("disabled Game Maker advertised tools")
	}
	tools := appendGameMakerToolSchemas(nil, ToolFeatureFlags{GameMakerEnabled: true})
	if len(tools) != 4 {
		t.Fatalf("tool isolation changed: %d", len(tools))
	}
	for _, tool := range tools {
		if tool.Function.Name != "game_maker_asset" {
			continue
		}
		data, err := json.Marshal(tool.Function.Parameters)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Required   []string
			Properties map[string]struct{ Enum []string }
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(schema.Required, []string{"job_id"}) {
			t.Fatalf("default generation compatibility: required %v", schema.Required)
		}
		if !slices.Equal(schema.Properties["operation"].Enum, []string{"generate", "list_packs", "describe_pack", "import_pack"}) {
			t.Fatal("missing pack operations")
		}
		for _, key := range []string{"kind", "prompt", "path", "pack_id"} {
			if _, ok := schema.Properties[key]; !ok {
				t.Fatalf("missing %s", key)
			}
		}
		return
	}
	t.Fatal("missing game_maker_asset")
}
