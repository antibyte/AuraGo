package agent

import (
	"strings"
	"testing"
)

func TestGameMakerPhaseSchemasRemainStrictObjects(t *testing.T) {
	for _, stage := range []string{"planning", "building", "repair"} {
		for _, dimension := range []string{"2d", "3d"} {
			definitions := GameMakerPhaseToolSchemas(stage, dimension)
			strict := newNativeToolSchemaSnapshot(definitions).StrictSchemas()
			for _, tool := range strict {
				params := tool.Function.Parameters.(map[string]interface{})
				var violations []string
				collectStrictOpenAISchemaViolations(tool.Function.Name, params, &violations)
				if len(violations) > 0 {
					t.Fatal(strings.Join(violations, "\n"))
				}
				props := params["properties"].(map[string]interface{})
				if stage == "planning" {
					if tool.Function.Name == "game_maker_validate" {
						t.Fatal("validation in planning")
					}
					if tool.Function.Name == "game_maker_project" {
						design := props["design"].(map[string]interface{})
						if _, ok := design["properties"]; !ok {
							t.Fatalf("compact design was converted to string: %+v", design)
						}
						if _, ok := props["plan"]; ok {
							t.Fatal("full plan exposed to isolated planner")
						}
					}
					if tool.Function.Name == "game_maker_file" {
						if _, ok := props["content"]; ok {
							t.Fatal("planning write exposed")
						}
					}
				} else if tool.Function.Name == "game_maker_project" {
					if _, ok := props["design"]; ok {
						t.Fatal("design mutation exposed after acceptance")
					}
				}
			}
		}
	}
	// Building schemas must not mutate a previous planning snapshot.
	for _, tool := range GameMakerPhaseToolSchemas("planning", "3d") {
		if tool.Function.Name == "game_maker_project" && tool.Function.Parameters.(map[string]interface{})["properties"].(map[string]interface{})["design"] == nil {
			t.Fatal("shared mutable schema")
		}
	}
}
