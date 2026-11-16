package flows

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every curated list names real steps the editor can add after another step: a catalog type that
// is not a trigger, has an input, and appears once; a list is short and never empty.
func TestCuratedSuggestionsAreValid(t *testing.T) {
	reg := catalogRegistry(t, fullEnv())
	for category, list := range curatedSuggestions {
		if len(list) == 0 || len(list) > maxSuggestions {
			t.Errorf("%s: %d suggestions, want 1 to %d", category, len(list), maxSuggestions)
		}
		seen := map[string]bool{}
		for _, typ := range list {
			def, ok := reg.Lookup(typ)
			switch {
			case !ok:
				t.Errorf("%s: %q is not a catalog type", category, typ)
			case def.Trigger:
				t.Errorf("%s: %q is a trigger and cannot follow a step", category, typ)
			case len(def.InputPorts()) == 0:
				t.Errorf("%s: %q has no input", category, typ)
			}
			if seen[typ] {
				t.Errorf("%s: %q is listed twice", category, typ)
			}
			seen[typ] = true
		}
	}
}

// Every curated category and the generic tool group has a list, and the catalog serves it on each
// step that has an output (and on no other), never with the step's own type in it.
func TestEveryStepWithOutputsGetsSuggestions(t *testing.T) {
	for _, category := range append(append([]string{}, CategoryOrder...), "tool") {
		if len(curatedSuggestions[category]) == 0 {
			t.Errorf("category %q has no curated suggestions", category)
		}
	}
	for _, info := range DescribeNodeTypes(catalogRegistry(t, fullEnv()), nil) {
		hasOutputs := len(info.Outputs) > 0 || info.DynamicOutputs != ""
		if hasOutputs && len(info.Suggest) == 0 {
			t.Errorf("%s has outputs and no suggestions", info.Type)
		}
		if !hasOutputs && info.Suggest != nil {
			t.Errorf("%s has no outputs and offers %v", info.Type, info.Suggest)
		}
		for _, typ := range info.Suggest {
			if typ == info.Type {
				t.Errorf("%s suggests itself", info.Type)
			}
		}
	}
}

// A generic tool step ("tool:<name>" category) takes the list of the "tool" group, a type that the
// registry lacks is dropped, and the field reaches the catalog JSON as "suggest" only when it is set.
func TestSuggestionsForToolsRegistryAndJSON(t *testing.T) {
	present := map[string]bool{TypeAIStep: true, TypeIf: true}
	got := suggestionsFor("tool.file_reader", "tool:files", true, present)
	if strings.Join(got, ",") != TypeAIStep+","+TypeIf {
		t.Fatalf("tool suggestions = %v, want the listed types the registry has", got)
	}
	if got := suggestionsFor(TypeStop, "logic", false, present); got != nil {
		t.Fatalf("a step without outputs got %v", got)
	}
	reg := catalogRegistry(t, fullEnv())
	raw, err := json.Marshal(DescribeNodeTypes(reg, nil))
	if err != nil {
		t.Fatal(err)
	}
	var infos []map[string]any
	if err := json.Unmarshal(raw, &infos); err != nil {
		t.Fatal(err)
	}
	for _, info := range infos {
		_, has := info["suggest"]
		if want := info["type"] != TypeStop; has != want {
			t.Errorf("%v: suggest present = %v, want %v", info["type"], has, want)
		}
	}
}

// The block catalog uses a smaller registry: its steps are offered only types that registry holds.
func TestBlockCatalogSuggestionsStayInsideItsRegistry(t *testing.T) {
	reg := blockRegistry()
	for _, info := range DescribeNodeTypes(reg, nil) {
		for _, typ := range info.Suggest {
			if _, ok := reg.Lookup(typ); !ok {
				t.Errorf("block step %s suggests %s, which the block registry lacks", info.Type, typ)
			}
		}
	}
}
