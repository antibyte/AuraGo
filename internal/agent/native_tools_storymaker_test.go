package agent

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"aurago/internal/storymaker"

	openai "github.com/sashabaranov/go-openai"
)

func storymakerCatalogNames(t *testing.T) StorymakerNames {
	t.Helper()
	cat, err := storymaker.LoadCatalog(os.DirFS("../../ui"), "img/storymaker/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return StorymakerNames{
		Poses: cat.Names("pose"), Gestures: cat.Names("gesture"), Expressions: cat.Names("expression"),
		Environments: cat.Names("environment"), Characters: cat.Names("character"),
	}
}

func storymakerSorted(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}

func storymakerToolParams(tools []openai.Tool, name string) map[string]interface{} {
	for _, definition := range tools {
		if definition.Function.Name == name {
			params, _ := definition.Function.Parameters.(map[string]interface{})
			return params
		}
	}
	return nil
}

// storymakerSchemaAt walks properties by name; "[]" steps into array items. It
// returns nil when the path does not exist.
func storymakerSchemaAt(node map[string]interface{}, path ...string) map[string]interface{} {
	for _, key := range path {
		var next map[string]interface{}
		if key == "[]" {
			next, _ = node["items"].(map[string]interface{})
		} else {
			properties, _ := node["properties"].(map[string]interface{})
			next, _ = properties[key].(map[string]interface{})
		}
		if next == nil {
			return nil
		}
		node = next
	}
	return node
}

func storymakerRequireClosed(t *testing.T, path string, node map[string]interface{}) {
	t.Helper()
	if schemaHasType(node, "object") && node["additionalProperties"] != false {
		t.Errorf("%s is not closed", path)
	}
	properties, _ := node["properties"].(map[string]interface{})
	for key, raw := range properties {
		if key == "project_id" || key == "job_id" {
			t.Errorf("%s exposes %s", path, key)
		}
		if child, ok := raw.(map[string]interface{}); ok {
			storymakerRequireClosed(t, path+"."+key, child)
		}
	}
	if items, ok := node["items"].(map[string]interface{}); ok {
		storymakerRequireClosed(t, path+"[]", items)
	}
}

func TestStorymakerPhaseSchemasFollowThePhaseTable(t *testing.T) {
	names := storymakerCatalogNames(t)
	catalogOps := []string{"search", "describe"}
	checkOps := []string{"validate"}
	table := map[string]map[string][]string{
		storymaker.PhaseOutline: {storymaker.ToolStory: {"outline", "get"}},
		storymaker.PhaseCast:    {storymaker.ToolCatalog: catalogOps, storymaker.ToolStory: {"set_meta", "get"}},
		storymaker.PhaseFill:    {storymaker.ToolCatalog: catalogOps, storymaker.ToolStory: {"put_scene", "patch_scene", "dry_run", "get"}},
		storymaker.PhaseCheck:   {storymaker.ToolCheck: checkOps, storymaker.ToolStory: {"patch_scene", "get"}},
		storymaker.PhaseRevise: {
			storymaker.ToolCatalog: catalogOps,
			storymaker.ToolCheck:   checkOps,
			storymaker.ToolStory:   {"get", "put_scene", "patch_scene", "remove_scene", "move_scene", "set_meta", "dry_run"},
		},
	}
	type fields struct{ present, absent []string }
	sceneFields := fields{present: []string{"environment", "narration", "actors", "props", "beats"}, absent: []string{"title", "template"}}
	dataFields := map[string]fields{
		storymaker.PhaseOutline: {present: []string{"template", "target_seconds", "scenes"}, absent: []string{"title", "beats"}},
		storymaker.PhaseCast:    {present: []string{"title", "cast"}, absent: []string{"template", "beats"}},
		storymaker.PhaseFill:    sceneFields,
		storymaker.PhaseCheck:   sceneFields,
		storymaker.PhaseRevise:  {present: []string{"environment", "beats", "title", "cast"}, absent: []string{"template"}},
	}
	for phase, tools := range table {
		for _, compact := range []bool{false, true} {
			schemas := StorymakerPhaseToolSchemas(phase, compact, names)
			var got []string
			for _, definition := range schemas {
				name := definition.Function.Name
				got = append(got, name)
				params := definition.Function.Parameters.(map[string]interface{})
				storymakerRequireClosed(t, phase+"/"+name, params)
				enum, _ := storymakerSchemaAt(params, "operation")["enum"].([]string)
				if !slices.Equal(storymakerSorted(enum), storymakerSorted(tools[name])) {
					t.Errorf("%s %s operations = %v, want %v", phase, name, enum, tools[name])
				}
			}
			if want := slices.Sorted(maps.Keys(tools)); !slices.Equal(storymakerSorted(got), want) {
				t.Errorf("%s tools = %v, want %v", phase, got, want)
			}
			story := storymakerToolParams(schemas, storymaker.ToolStory)
			for _, field := range dataFields[phase].present {
				if storymakerSchemaAt(story, "data", field) == nil {
					t.Errorf("%s compact=%v data lacks %s", phase, compact, field)
				}
			}
			for _, field := range dataFields[phase].absent {
				if storymakerSchemaAt(story, "data", field) != nil {
					t.Errorf("%s compact=%v data has %s", phase, compact, field)
				}
			}
			if compact {
				for _, definition := range schemas {
					if definition.Function.Name != storymaker.ToolStory {
						continue
					}
					if tokens := estimateSingleToolSchemaTokens(definition); tokens > 900 {
						t.Errorf("%s compact story schema is %d tokens, keep it at most 900", phase, tokens)
					}
				}
			}
		}
	}

	if outline := storymakerToolParams(StorymakerPhaseToolSchemas(storymaker.PhaseOutline, true, names), storymaker.ToolStory); storymakerSchemaAt(outline, "data", "scenes")["minItems"] != 2 || storymakerSchemaAt(outline, "data", "scenes")["maxItems"] != 24 {
		t.Errorf("outline scenes bounds = %v", storymakerSchemaAt(outline, "data", "scenes"))
	}

	for _, compact := range []bool{false, true} {
		fill := StorymakerPhaseToolSchemas(storymaker.PhaseFill, compact, names)
		story := storymakerToolParams(fill, storymaker.ToolStory)
		for _, field := range []string{"count", "id", "sound", "delay", "duration", "ease"} {
			if (storymakerSchemaAt(story, "data", "beats", "[]", field) != nil) == compact {
				t.Errorf("fill compact=%v beat field %s has the wrong presence", compact, field)
			}
		}
		poses, _ := storymakerSchemaAt(story, "data", "actors", "[]", "pose")["enum"].([]string)
		if len(poses) == 0 || !slices.Equal(poses, names.Poses) {
			t.Errorf("actor pose enum = %v, want %v", poses, names.Poses)
		}
		// A null in a native patch_scene call must reach Studio, which removes the field.
		args := map[string]interface{}{"data": map[string]interface{}{"camera": nil, "narration": nil, "transition": nil}}
		stripStrictNulls(args, story)
		if data := args["data"].(map[string]interface{}); len(data) != 3 {
			t.Errorf("compact=%v null removal stripped: %v", compact, data)
		}

		cast := StorymakerPhaseToolSchemas(storymaker.PhaseCast, compact, names)
		meta := storymakerToolParams(cast, storymaker.ToolStory)
		for _, path := range [][]string{{"data", "style"}, {"data", "voice"}, {"data", "cast", "[]", "voice"}} {
			if (storymakerSchemaAt(meta, path...) != nil) == compact {
				t.Errorf("cast compact=%v %s has the wrong presence", compact, strings.Join(path, "."))
			}
		}
		characters, _ := storymakerSchemaAt(meta, "data", "cast", "[]", "character")["enum"].([]string)
		if len(characters) == 0 || !slices.Equal(characters, names.Characters) {
			t.Errorf("character enum = %v, want %v", characters, names.Characters)
		}
		kinds, _ := storymakerSchemaAt(storymakerToolParams(cast, storymaker.ToolCatalog), "kind")["enum"].([]string)
		if slices.Contains(kinds, "voice") == compact {
			t.Errorf("cast compact=%v kinds = %v", compact, kinds)
		}
		revise := StorymakerPhaseToolSchemas(storymaker.PhaseRevise, compact, names)
		kinds, _ = storymakerSchemaAt(storymakerToolParams(revise, storymaker.ToolCatalog), "kind")["enum"].([]string)
		if !slices.Contains(kinds, "voice") {
			t.Errorf("revise compact=%v lost the voice kind: %v", compact, kinds)
		}
	}
}

func TestStorymakerToolsStayOutOfTheNormalCatalog(t *testing.T) {
	for _, name := range builtinToolNames(allBuiltinToolFeatureFlags()) {
		if strings.HasPrefix(name, "storymaker_") {
			t.Fatalf("%s is in the normal tool catalog", name)
		}
	}
}

func TestStorymakerToolsStayOutOfTheStrictProjection(t *testing.T) {
	names := storymakerCatalogNames(t)
	schemas := StorymakerPhaseToolSchemas(storymaker.PhaseRevise, false, names)
	ordinary := tool("ordinary_probe", "probe", schema(map[string]interface{}{"q": prop("string", "query")}))
	schemas = append(slices.Clone(schemas), ordinary)
	snapshot := newNativeToolSchemaSnapshot(schemas)
	projected := strictSchemasForActiveTools(snapshot, schemas)
	found := false
	for _, definition := range projected {
		switch {
		case definition.Function.Name == storymaker.ToolStory:
			found = true
			if definition.Function.Strict {
				t.Errorf("%s was projected strict", storymaker.ToolStory)
			}
			params, _ := definition.Function.Parameters.(map[string]interface{})
			data := storymakerSchemaAt(params, "data")
			if data == nil {
				t.Fatalf("%s has no data schema", storymaker.ToolStory)
			}
			var required []string
			switch list := data["required"].(type) {
			case []string:
				required = list
			case []interface{}:
				for _, item := range list {
					if name, ok := item.(string); ok {
						required = append(required, name)
					}
				}
			}
			if slices.Contains(required, "transition") {
				t.Errorf("data.required lists transition: %v", required)
			}
		case definition.Function.Name == "ordinary_probe":
			if !definition.Function.Strict {
				t.Errorf("the ordinary tool lost its strict projection")
			}
		}
	}
	if !found {
		t.Fatalf("%s missing from the revise schemas", storymaker.ToolStory)
	}
}
