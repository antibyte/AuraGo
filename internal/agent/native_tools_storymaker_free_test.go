package agent

import (
	"slices"
	"testing"

	"aurago/internal/storymaker"
)

// storymakerEnumValues returns the string values of an enum without its null. A story tool that can patch a scene lets a
// null remove the field, so makeSchemaNullable turns the []string enum of such a field into []interface{} with a nil.
func storymakerEnumValues(node map[string]interface{}) []string {
	if values, ok := node["enum"].([]string); ok {
		return values
	}
	out := []string{}
	values, _ := node["enum"].([]interface{})
	for _, value := range values {
		if text, ok := value.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func TestStorymakerFreeSchemasOfferNoCatalogSearchNoCastAndCodeScenesOnly(t *testing.T) {
	names := StorymakerNames{}
	if got := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, storymaker.PhaseCast, false, names); len(got) != 0 {
		t.Fatalf("a free project has no cast phase, got %d tools", len(got))
	}
	for _, phase := range []string{storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise} {
		schemas := StorymakerPhaseToolSchemasFor(storymaker.ModeFree, phase, false, names)
		catalog := storymakerToolParams(schemas, storymaker.ToolCatalog)
		operations, _ := storymakerSchemaAt(catalog, "operation")["enum"].([]string)
		if !slices.Equal(operations, []string{"reference"}) {
			t.Errorf("%s catalog operations = %v, want [reference]", phase, operations)
		}
		if storymakerSchemaAt(catalog, "query") != nil || storymakerSchemaAt(catalog, "kind") != nil {
			t.Errorf("%s: the free catalog tool must not offer query or kind", phase)
		}
		if phase == storymaker.PhaseCheck {
			continue
		}
		story := storymakerToolParams(schemas, storymaker.ToolStory)
		for _, banned := range []string{"actors", "props", "beats", "environment", "speaker", "camera"} {
			if storymakerSchemaAt(story, "data", banned) != nil {
				t.Errorf("%s: the free story tool offers %s", phase, banned)
			}
		}
		kinds := storymakerEnumValues(storymakerSchemaAt(story, "data", "kind"))
		if !slices.Equal(kinds, []string{"code"}) {
			t.Errorf("%s: scene kinds = %v, want [code]", phase, kinds)
		}
		if storymakerSchemaAt(story, "data", "source") == nil {
			t.Errorf("%s: the free story tool must offer source for put_code", phase)
		}
	}
	// The catalog mode keeps its schemas.
	if got, want := len(StorymakerPhaseToolSchemasFor(storymaker.ModeCatalog, storymaker.PhaseFill, false, names)), len(StorymakerPhaseToolSchemas(storymaker.PhaseFill, false, names)); got != want {
		t.Errorf("catalog schemas differ: %d vs %d", got, want)
	}
}
