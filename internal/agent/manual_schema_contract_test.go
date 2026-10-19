package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestContextDiffAndProcessManualExamplesMatchNativeSchemas(t *testing.T) {
	schemas := appendMemoryToolSchemas(buildCoreToolSchemas(ToolFeatureFlags{}, nil), ToolFeatureFlags{MemoryEnabled: true})
	byName := map[string]openai.Tool{}
	for _, schema := range schemas {
		if schema.Function != nil {
			byName[schema.Function.Name] = schema
		}
	}
	fence := regexp.MustCompile("(?s)```json" + `\s*\n(.*?)\n` + "```")
	for _, manual := range []string{"context_manager", "text_diff", "process_management"} {
		t.Run(manual, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "prompts", "tools_manuals", manual+".md"))
			if err != nil {
				t.Fatal(err)
			}
			examples := fence.FindAllSubmatch(data, -1)
			if len(examples) == 0 {
				t.Fatal("manual has no runnable JSON examples")
			}
			for _, example := range examples {
				var call map[string]any
				if err := json.Unmarshal(example[1], &call); err != nil {
					t.Fatal(err)
				}
				action, _ := call["action"].(string)
				schema, ok := byName[action]
				if !ok {
					t.Fatalf("unknown native action %q", action)
				}
				params := schema.Function.Parameters.(map[string]interface{})
				properties := params["properties"].(map[string]interface{})
				for key, value := range call {
					if key == "action" {
						continue
					}
					raw, ok := properties[key]
					if !ok {
						t.Fatalf("unknown field %s in %s", key, action)
					}
					property := raw.(map[string]interface{})
					if values, ok := property["enum"].([]string); ok {
						found := false
						for _, allowed := range values {
							if value == allowed {
								found = true
							}
						}
						if !found {
							t.Fatalf("invalid %s=%v for %s", key, value, action)
						}
					}
				}
			}
		})
	}
}
