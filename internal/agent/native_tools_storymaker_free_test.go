package agent

import (
	"slices"
	"strings"
	"testing"

	openai "github.com/sashabaranov/go-openai"

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
		want := []string{"reference", "search"}
		if phase == storymaker.PhaseCheck {
			want = []string{"reference"}
		}
		if got := slices.Sorted(slices.Values(operations)); !slices.Equal(got, want) {
			t.Errorf("%s catalog operations = %v, want %v", phase, operations, want)
		}
		kindEnum, _ := storymakerSchemaAt(catalog, "kind")["enum"].([]string)
		if searches := slices.Contains(operations, "search"); searches != (storymakerSchemaAt(catalog, "query") != nil) || searches != slices.Equal(kindEnum, []string{"icon"}) {
			t.Errorf("%s: the free catalog tool offers query and kind icon exactly when it searches, kind enum = %v", phase, kindEnum)
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

func storymakerToolDescription(tools []openai.Tool, name string) string {
	for _, definition := range tools {
		if definition.Function.Name == name {
			return definition.Function.Description
		}
	}
	return ""
}

func TestStorymakerFreeStoryToolDescribesCodeScenes(t *testing.T) {
	names := StorymakerNames{}
	const catalogDescription = "Job storyboard via data; positions: spot or x,y 0..100."
	for _, phase := range []string{storymaker.PhaseFill, storymaker.PhaseCheck, storymaker.PhaseRevise} {
		for _, compact := range []bool{false, true} {
			free := storymakerToolDescription(StorymakerPhaseToolSchemasFor(storymaker.ModeFree, phase, compact, names), storymaker.ToolStory)
			catalog := storymakerToolDescription(StorymakerPhaseToolSchemasFor(storymaker.ModeCatalog, phase, compact, names), storymaker.ToolStory)
			if catalog != catalogDescription {
				t.Errorf("%s compact=%v: the catalog story tool description changed: %q", phase, compact, catalog)
			}
			if free == catalog || !strings.Contains(free, "code scene") || !strings.Contains(free, "put_code") {
				t.Errorf("%s compact=%v: free story tool description = %q", phase, compact, free)
			}
			if strings.Contains(free, "storyboard") || strings.Contains(free, "positions") {
				t.Errorf("%s compact=%v: the free description still talks about storyboards and positions: %q", phase, compact, free)
			}
		}
	}
	if got := storymakerToolDescription(StorymakerPhaseToolSchemasFor(storymaker.ModeFree, storymaker.PhaseFill, false, names), storymaker.ToolStory); got !=
		"Job scenes via data: every scene is a code scene (kind, narration, transition, duration); put_code takes data.source." {
		t.Errorf("free story tool description = %q", got)
	}
}
