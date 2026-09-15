package agent

import (
	"encoding/json"
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

func TestGameMakerSettingsSchemaExplainsFreeBaseCorrection(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		for _, tool := range newNativeToolSchemaSnapshot(GameMakerPhaseToolSchemas("planning", dimension)).StrictSchemas() {
			if tool.Function.Name != "game_maker_project" {
				continue
			}
			props := tool.Function.Parameters.(map[string]interface{})["properties"].(map[string]interface{})
			design := props["design"].(map[string]interface{})["properties"].(map[string]interface{})
			settings, exists := design["settings"]
			if dimension == "2d" {
				if exists {
					t.Fatal("2D planning advertises guided settings")
				}
				continue
			}
			encoded, err := json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"fps/exploration/transport/flight/space", "null", "three", "failed draft", "Keep the requested base", "0 disables countdown"} {
				if !strings.Contains(string(encoded), want) {
					t.Errorf("strict provider lost %q: %s", want, encoded)
				}
			}
		}
	}
}
