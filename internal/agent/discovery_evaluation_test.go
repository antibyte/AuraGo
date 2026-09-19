package agent

import (
	"encoding/json"
	"os"
	"testing"
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
