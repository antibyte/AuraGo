package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGameMakerFreeSchemasAreStableAndStrict(t *testing.T) {
	var reference string
	for _, stage := range []string{"planning", "building", "repair"} {
		for _, dimension := range []string{"2d", "3d"} {
			for _, variant := range []string{"", "voxel"} {
				definitions := GameMakerPhaseToolSchemas(stage, dimension, variant)
				encoded, _ := json.Marshal(definitions)
				if reference == "" {
					reference = string(encoded)
				}
				if string(encoded) != reference {
					t.Fatal("tool catalog depends on a retired phase or framework")
				}
				strict := newNativeToolSchemaSnapshot(definitions).StrictSchemas()
				if len(strict) != 4 {
					t.Fatalf("tools = %d", len(strict))
				}
				for _, tool := range strict {
					params := tool.Function.Parameters.(map[string]interface{})
					var violations []string
					collectStrictOpenAISchemaViolations(tool.Function.Name, params, &violations)
					if len(violations) > 0 {
						t.Fatal(strings.Join(violations, "\n"))
					}
					props := params["properties"].(map[string]interface{})
					for _, removed := range []string{"job_id", "design", "plan", "scenarios"} {
						if _, exists := props[removed]; exists {
							t.Fatalf("%s exposes %s", tool.Function.Name, removed)
						}
					}
				}
			}
		}
	}
	for _, required := range []string{"game_maker_preview", `"build"`, `"finish"`, `"use_template"`, `"reference"`, `"content"`} {
		if !strings.Contains(reference, required) {
			t.Fatalf("catalog lacks %s", required)
		}
	}
	if strings.Contains(reference, "game_maker_validate") {
		t.Fatal("legacy validator remains advertised")
	}
}
