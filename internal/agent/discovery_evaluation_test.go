package agent

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestDiscoveryTaskEvaluationV1(t *testing.T) {
	type task struct {
		Query    string
		Expected []string
	}
	var fixture struct {
		Version   int
		Tasks     []task
		Negative  []string
		Ambiguous []task
	}
	data, err := os.ReadFile("testdata/discovery_tasks_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil || fixture.Version != 1 {
		t.Fatal(err)
	}
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	catalog := BuildToolCatalog(schemas, nil, "")
	found := 0
	for _, task := range fixture.Tasks {
		matches := catalog.Search(task.Query)
		hit := false
		for _, entry := range matches[:min(5, len(matches))] {
			for _, want := range task.Expected {
				if entry.Name == want {
					hit = true
				}
			}
		}
		if hit {
			found++
		} else {
			t.Logf("miss: %q; results=%v", task.Query, matches)
		}
	}
	recall := float64(found) / float64(len(fixture.Tasks))
	t.Logf("fixture=v1 Recall@5=%.3f (%d/%d), schema_tokens=%d", recall, found, len(fixture.Tasks), estimateToolSchemaListTokens(schemas))
	if recall < 0.95 {
		t.Fatal("task recall below acceptance threshold")
	}
	for _, schema := range schemas {
		name := schema.Function.Name
		matches := catalog.Search(name)
		if len(matches) == 0 || matches[0].Name != name {
			t.Fatalf("exact ID lost: %s", name)
		}
	}
	for _, query := range fixture.Negative {
		if len(catalog.Search(query)) != 0 {
			t.Fatalf("false match: %s", query)
		}
	}
	for _, name := range []string{"skill__fixture_shared", "tool__fixture_shared"} {
		catalog.add(catalogEntryFromSchema(testToolSchema(name, "shared fixture"), false, ""))
	}
	for _, task := range fixture.Ambiguous {
		matches := catalog.Search(task.Query)
		for _, want := range task.Expected {
			seen := false
			for _, entry := range matches[:min(5, len(matches))] {
				seen = seen || entry.Name == want
			}
			if !seen {
				t.Errorf("ambiguity hidden: %s", want)
			}
		}
	}
}

// TestDiscoveryTaskEvaluationV2 measures how well the all-features catalog
// finds the tool for 100 task phrasings (50 en, 50 de), and how many of them
// would find that tool already in the first-turn tool list of the agent loop.
//
// Recall@k is the share of tasks whose expected tool is among the first k
// results of catalog.Search. The first-turn approximation runs the adaptive
// selection of agent_loop_init.go (buildAdaptiveToolPriority, the hard/soft
// always-include lists, the caps, the additive tools) on the all-features
// schema set without usage history, memory or a semantic guide index, and
// counts a hit when an expected tool is among the selected schemas.
func TestDiscoveryTaskEvaluationV2(t *testing.T) {
	type task struct {
		ID       string   `json:"id"`
		Lang     string   `json:"lang"`
		Style    string   `json:"style"`
		Query    string   `json:"query"`
		Expected []string `json:"expected_any_of"`
	}
	data, err := os.ReadFile("testdata/discovery_tasks_v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []task
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatal(err)
	}

	ff := allBuiltinToolFeatureFlags()
	schemas := BuildNativeToolSchemas("", nil, ff, nil)
	catalog := BuildToolCatalog(schemas, nil, "")
	schemaNames := make(map[string]bool, len(schemas))
	for _, schema := range schemas {
		schemaNames[schema.Function.Name] = true
	}
	cfg := &config.Config{}

	// firstTurn mirrors the adaptive first selection in agent_loop_init.go.
	firstTurn := func(query string) []string {
		alwaysInclude := []string{"filesystem", "query_memory", "manage_memory", "execute_shell"}
		if ff.MusicGenerationEnabled {
			alwaysInclude = append(alwaysInclude, "generate_music")
		}
		if ff.VideoGenerationEnabled {
			alwaysInclude = append(alwaysInclude, "generate_video")
		}
		if ff.ImageGenerationEnabled {
			alwaysInclude = append(alwaysInclude, "generate_image")
		}
		alwaysInclude = channelAdaptiveAlwaysInclude(RunConfig{}, alwaysInclude, ff)
		alwaysInclude = cacheAwareAdaptiveAlwaysInclude(query, alwaysInclude, schemas)
		alwaysInclude = outputRefAdaptiveAlwaysInclude(nil, alwaysInclude)
		alwaysInclude = expandAdaptiveAlwaysInclude(cfg, alwaysInclude)
		result := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{
			PreferredTools:        buildAdaptiveToolPriority(schemas, nil, query, nil, nil),
			HardAlwaysTools:       channelAdaptiveAlwaysInclude(RunConfig{}, adaptiveHardAlwaysInclude(cfg), ff),
			SoftAlwaysTools:       alwaysInclude,
			MaxAdaptiveTools:      10,
			MaxTotalTools:         20,
			MaxSchemaTokens:       6500,
			AdditiveTools:         adaptiveAdditiveToolsForQuery(query),
			AdaptiveExcludedTools: adaptiveIntentOnlyTools,
			AdditiveSwaps:         adaptiveSwapsForQuery(query),
		}, nil)
		return toolSchemaNames(result.Tools)
	}
	anyOf := func(names, want []string) bool {
		return slices.ContainsFunc(names, func(name string) bool { return slices.Contains(want, name) })
	}

	type counts struct{ total, at1, at5, at10 int }
	byLang := map[string]*counts{"all": {}, "en": {}, "de": {}}
	firstTurnHits := 0
	for _, task := range tasks {
		if !slices.ContainsFunc(task.Expected, func(name string) bool { return schemaNames[name] }) {
			t.Fatalf("%s: none of the expected tools %v is a native schema", task.ID, task.Expected)
		}
		matches := catalog.Search(task.Query)
		names := make([]string, 0, len(matches))
		for _, entry := range matches {
			names = append(names, entry.Name)
		}
		for _, key := range []string{"all", task.Lang} {
			c, ok := byLang[key]
			if !ok {
				t.Fatalf("%s: unexpected language %q", task.ID, task.Lang)
			}
			c.total++
			if anyOf(names[:min(1, len(names))], task.Expected) {
				c.at1++
			}
			if anyOf(names[:min(5, len(names))], task.Expected) {
				c.at5++
			}
			if anyOf(names[:min(10, len(names))], task.Expected) {
				c.at10++
			}
		}
		if anyOf(firstTurn(task.Query), task.Expected) {
			firstTurnHits++
		}
	}
	for _, key := range []string{"all", "en", "de"} {
		c := byLang[key]
		rate := func(n int) float64 { return float64(n) / float64(c.total) }
		t.Logf("fixture=v2 %-3s n=%d Recall@1=%d (%.3f) Recall@5=%d (%.3f) Recall@10=%d (%.3f)",
			key, c.total, c.at1, rate(c.at1), c.at5, rate(c.at5), c.at10, rate(c.at10))
	}
	t.Logf("fixture=v2 first-turn selection hits=%d/%d (%.3f)", firstTurnHits, len(tasks), float64(firstTurnHits)/float64(len(tasks)))

	// ratchet: floors may only go up
	const (
		floorAllAt1    = 43
		floorAllAt5    = 58
		floorAllAt10   = 65
		floorEnAt1     = 28
		floorEnAt5     = 38
		floorEnAt10    = 41
		floorDeAt1     = 15
		floorDeAt5     = 20
		floorDeAt10    = 24
		floorFirstTurn = 54
	)
	for _, floor := range []struct {
		name      string
		got, want int
	}{
		{"all Recall@1", byLang["all"].at1, floorAllAt1},
		{"all Recall@5", byLang["all"].at5, floorAllAt5},
		{"all Recall@10", byLang["all"].at10, floorAllAt10},
		{"en Recall@1", byLang["en"].at1, floorEnAt1},
		{"en Recall@5", byLang["en"].at5, floorEnAt5},
		{"en Recall@10", byLang["en"].at10, floorEnAt10},
		{"de Recall@1", byLang["de"].at1, floorDeAt1},
		{"de Recall@5", byLang["de"].at5, floorDeAt5},
		{"de Recall@10", byLang["de"].at10, floorDeAt10},
		{"first-turn hits", firstTurnHits, floorFirstTurn},
	} {
		if floor.got < floor.want {
			t.Errorf("%s fell to %d, floor %d", floor.name, floor.got, floor.want)
		}
	}
}
