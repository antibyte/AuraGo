package storymaker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestStudioCatalogSearchMissAndVoice(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	proj, _, _, err := ps.Create(ctx, "Katalog", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	notInstalled := false
	// The active provider comes last here, so the answer's order proves the sort.
	providers := []VoiceProvider{
		{ID: "elevenlabs", Ready: false, Languages: []string{"*"}, Voices: []VoiceOption{{ID: "offline"}}},
		{ID: "piper", Ready: true, Languages: []string{"de", "en"}, Voices: []VoiceOption{
			{ID: "de_DE-thorsten-high", Language: "de", Gender: "male"},
			{ID: "en_US-amy", Language: "en", Installed: &notInstalled},
		}},
		{ID: "openai", Active: true, Ready: true, FreeText: true, Languages: []string{"*"}, DefaultVoice: "alloy"},
	}
	studio, err := NewStudio(StudioDeps{
		Projects: ps,
		Catalog:  loadReal(t),
		Limits:   DefaultLimits(),
		Voices:   func(context.Context) []VoiceProvider { return providers },
		Compile: func(context.Context, string) (CompileSummary, error) {
			return CompileSummary{Available: false, Reason: "no browser in tests"}, nil
		},
		ReadOnly: func() bool { return false },
	}, StudioJob{JobID: "job-1", ProjectID: proj.ID, Kind: "create", Language: "de", TargetSeconds: 60, VoiceAvailable: true})
	if err != nil {
		t.Fatal(err)
	}
	studio.SetPhase(PhaseCast, "")
	search := func(args map[string]any) map[string]any {
		t.Helper()
		res := studio.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "search", Args: args})
		var answer map[string]any
		if err := json.Unmarshal(res.JSON, &answer); err != nil {
			t.Fatalf("answer %s: %v", res.JSON, err)
		}
		return answer
	}

	found := search(map[string]any{"query": "lock", "kind": "prop"})
	results, _ := found["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("search answer = %v", found)
	}
	first := results[0].(map[string]any)
	example, _ := first["usage_example"].(map[string]any)
	if first["id"] != "prop.lock" || example["item"] != "prop.lock" {
		t.Fatalf("first result = %v", first)
	}

	miss := search(map[string]any{"query": "zebra", "kind": "prop"})
	empty, isList := miss["results"].([]any)
	alternatives, _ := miss["alternatives"].([]any)
	if !isList || len(empty) != 0 || len(alternatives) == 0 || !strings.Contains(miss["next_action"].(string), "SVG paths with g.path") {
		t.Fatalf("miss answer = %v", miss)
	}
	for _, alternative := range alternatives {
		if !strings.HasPrefix(alternative.(string), "prop.") {
			t.Fatalf("alternative %v is not a prop", alternative)
		}
	}
	for _, kind := range []string{"character", "environment", "pose", "gesture", "expression", "transition", "sfx", ""} {
		answer := search(map[string]any{"query": "zebra", "kind": kind})
		if strings.Contains(answer["next_action"].(string), "g.path") {
			t.Fatalf("non-prop miss must not suggest replacing %q with SVG: %v", kind, answer)
		}
	}
	described := studio.Call(ctx, StudioCall{Tool: ToolCatalog, Operation: "describe", Args: map[string]any{"id": "prop.zebra"}})
	if !strings.Contains(string(described.JSON), "SVG paths with g.path") {
		t.Fatalf("a missing prop described without kind must receive the SVG workflow: %s", described.JSON)
	}

	voices := search(map[string]any{"kind": "voice"})
	rows, _ := voices["results"].([]any)
	if len(rows) != 2 {
		t.Fatalf("voice answer = %v, want the two ready voices", voices)
	}
	active, listed := rows[0].(map[string]any), rows[1].(map[string]any)
	if active["provider"] != "openai" || active["voice"] != "alloy" {
		t.Fatalf("first voice = %v, want the active provider", active)
	}
	languages, _ := listed["languages"].([]any)
	if listed["provider"] != "piper" || listed["voice"] != "de_DE-thorsten-high" || listed["gender"] != "male" || len(languages) != 1 || languages[0] != "de" {
		t.Fatalf("second voice = %v", listed)
	}

	// kind=voice with an id answers with that one voice; an unknown id gets a hint.
	one := search(map[string]any{"kind": "voice", "id": "de_DE-thorsten-high"})
	if entry, _ := one["entry"].(map[string]any); entry["provider"] != "piper" || entry["voice"] != "de_DE-thorsten-high" || one["results"] != nil {
		t.Fatalf("voice by id = %v", one)
	}
	missing := search(map[string]any{"kind": "voice", "id": "no-such-voice"})
	if list, isList := missing["results"].([]any); !isList || len(list) != 0 || missing["next_action"] == nil {
		t.Fatalf("unknown voice id = %v", missing)
	}
}
