package catalogsync

import "testing"

// compactFixture is a hand-encoded catalog in the pi-catalog 19.x compact
// format: shared provider constants, interned nodes, boolean bitmasks, an
// explicitly absent value ([-1]) and a compat delta.
const compactFixture = `{
  "version": 1,
  "keys": ["api","baseUrl","provider","reasoning","contextWindow","cost","input","maxTokens","name","output","supportsTools"],
  "booleanShapes": [[], [3], [3, 10]],
  "nodes": [
    [0, [0, "openai-completions", 1, "https://acme.test/v1", 2, "acme"], 0, []],
    [1, ["text", "image"]],
    [0, [6, 1.5, 9, 6], 0, []],
    [0, [4, 200000, 5, [2], 6, [1], 7, 8192, 8, "Acme Large"], 1, [1]],
    [0, [1, [-1], 5, [2], 6, [5], 8, "Acme Small"], 2, [0]],
    [1, ["text"]],
    [0, [], 0, []]
  ],
  "compat": [["openai-completions", [6]]],
  "providers": [
    ["acme", [0], [
      ["acme-large", [3], [[], [6]]],
      ["acme-small", [4]]
    ]]
  ]
}`

func TestParseModelsJSONDecodesCompactCatalog(t *testing.T) {
	models, err := ParseModelsJSON([]byte(compactFixture))
	if err != nil {
		t.Fatalf("ParseModelsJSON: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	byID := map[string]int{}
	for i, m := range models {
		byID[m.ID] = i
	}
	large := models[byID["acme-large"]]
	if large.Provider != "acme" || large.API != "openai-completions" || large.BaseURL != "https://acme.test/v1" {
		t.Fatalf("large constants not applied: %+v", large)
	}
	if large.Name != "Acme Large" || large.ContextWindow != 200000 || large.MaxTokens != 8192 {
		t.Fatalf("large fields wrong: %+v", large)
	}
	if !large.Reasoning || !large.SupportsTools || !large.Multimodal {
		t.Fatalf("large flags wrong: %+v", large)
	}
	if large.Cost.Input != 1.5 || large.Cost.Output != 6 {
		t.Fatalf("large cost wrong: %+v", large.Cost)
	}
	small := models[byID["acme-small"]]
	if small.BaseURL != "" {
		t.Fatalf("absent value should remove the provider constant, got baseUrl %q", small.BaseURL)
	}
	if small.Reasoning || small.SupportsTools || small.Multimodal {
		t.Fatalf("small flags wrong: %+v", small)
	}
	if len(small.Input) != 1 || small.Input[0] != "text" || small.Cost.Output != 6 {
		t.Fatalf("small shared nodes wrong: %+v", small)
	}
}

func TestParseModelsJSONRejectsUnknownCompactVersion(t *testing.T) {
	if _, err := ParseModelsJSON([]byte(`{"version":2,"keys":[],"booleanShapes":[],"nodes":[],"compat":[],"providers":[]}`)); err == nil {
		t.Fatal("expected an error for compact version 2")
	}
}
