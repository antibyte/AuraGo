package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/prompts"
	openai "github.com/sashabaranov/go-openai"
)

func TestLiveDiscoveryRunSurvivesOtherSessionPruning(t *testing.T) {
	id := acquireDiscoveryRun()
	t.Cleanup(func() { releaseDiscoveryRun(id) })
	schemas := []openai.Tool{testToolSchema("fixture", "read")}
	SetDiscoverToolsState(id, schemas, schemas, "")
	discoverToolsState.mu.Lock()
	snapshot := discoverToolsState.snapshots[id]
	snapshot.updatedAt = time.Now().Add(-24 * time.Hour)
	discoverToolsState.snapshots[id] = snapshot
	discoverToolsState.mu.Unlock()
	SetDiscoverToolsState(t.Name(), schemas, nil, "")
	t.Cleanup(func() { ClearDiscoverToolsState(t.Name()) })
	if GetToolCatalogState(id) == nil {
		t.Fatal("live run lost its catalog")
	}
	releaseDiscoveryRun(id)
	if GetToolCatalogState(id) != nil {
		t.Fatal("finished run leaked its catalog")
	}
}

func TestBoundedIsolatedJSONPreservesStatusAndErrorCode(t *testing.T) {
	raw := `{"status":"error","code":"fixture_failure","message":"` + strings.Repeat("long message ", 300) + `"}`
	output := boundedToolResult("<external_data>\n"+raw+"\n</external_data>", 280, ToolResultFailed)
	payload, isolated := toolResultPayload(output)
	if len(output) > 280 || !isolated || !json.Valid([]byte(payload)) || !strings.Contains(payload, "fixture_failure") || !strings.Contains(payload, "failed") {
		t.Fatal(output)
	}
}

func TestCatalogNamespacedCollisionDoesNotChooseAliasWinner(t *testing.T) {
	catalog := BuildToolCatalog([]openai.Tool{testToolSchema("skill__same", "skill"), testToolSchema("tool__same", "custom")}, nil, "")
	if _, ok := catalog.Get("same"); ok {
		t.Fatal("ambiguous alias resolved")
	}
	for _, name := range []string{"skill__same", "tool__same"} {
		if entry, ok := catalog.Get(name); !ok || entry.Name != name {
			t.Fatalf("lost %s", name)
		}
	}
}

func TestProgressiveDiscoveryPagesStayBoundedAndComplete(t *testing.T) {
	sid := t.Name()
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	var schemas []openai.Tool
	for i := 0; i < 37; i++ {
		schemas = append(schemas, testToolSchema(fmt.Sprintf("page_fixture_%02d", i), strings.Repeat("description ", 1000)))
	}
	SetDiscoverToolsState(sid, schemas, nil, "")
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 1800
	seen := map[string]bool{}
	cursor := ""
	for pages := 0; pages < 100; pages++ {
		out := handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "search", "query": "page_fixture", "cursor": cursor}}, cfg, nil, sid)
		if len(out) > 1800 || !json.Valid([]byte(strings.TrimPrefix(out, "Tool Output: "))) {
			t.Fatalf("invalid bounded JSON: %s", out)
		}
		var response DiscoverToolsResponse
		decodeToolOutputJSON(t, out, &response)
		if response.Status != "success" || len(response.Results) == 0 {
			t.Fatal(out)
		}
		for _, result := range response.Results {
			if seen[result.Name] || result.Parameters != nil {
				t.Fatalf("duplicate/schema in summary: %+v", result)
			}
			seen[result.Name] = true
		}
		if response.NextCursor == "" {
			break
		}
		if response.NextCursor == cursor {
			t.Fatal("cursor did not advance")
		}
		cursor = response.NextCursor
	}
	if len(seen) != 37 {
		t.Fatalf("received %d of 37", len(seen))
	}
	out := handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "list_categories"}}, cfg, nil, sid)
	var overview DiscoverToolsResponse
	decodeToolOutputJSON(t, out, &overview)
	if len(overview.Results) != 0 || len(overview.Categories) == 0 {
		t.Fatal("overview includes full tool entries")
	}
	SetDiscoverToolsState(sid, schemas[:1], nil, "")
	out = handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "search", "query": "page_fixture", "cursor": cursor}}, cfg, nil, sid)
	if !strings.Contains(out, "catalog_changed") {
		t.Fatal(out)
	}
}

func TestDiscoverySearchOperationsAndGermanTasks(t *testing.T) {
	schemas := BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil)
	catalog := BuildToolCatalog(schemas, nil, "")
	for query, want := range map[string]string{"disk_io": "system_metrics", "Notiz speichern": "manage_notes", "Licht einschalten": "home_assistant", "Kalender Termin": "manage_appointments"} {
		matches := catalog.Search(query)
		found := false
		for _, entry := range matches[:min(5, len(matches))] {
			if entry.Name == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q did not find %s in top five", query, want)
		}
	}
}

func TestPreferredSchemaWinsOverIrrelevantCheapSchema(t *testing.T) {
	schemas := []openai.Tool{testToolSchema("unrelated", "tiny"), testToolSchema("requested", strings.Repeat("description ", 100))}
	filtered := filterToolSchemasWithReport(schemas, toolSchemaFilterOptions{PreferredTools: []string{"requested"}, MaxAdaptiveTools: 1, MaxTotalTools: 1, MaxSchemaTokens: 10000}, nil)
	if len(filtered.Tools) != 1 || filtered.Tools[0].Function.Name != "requested" {
		t.Fatal(toolSchemaNames(filtered.Tools))
	}
}

func TestBuiltinManualBindingsAreValidOrExplicitlyExplained(t *testing.T) {
	for _, schema := range BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil) {
		name := schema.Function.Name
		if prompts.ToolManualAbsenceReason(name) != "" {
			continue
		}
		if _, ok := prompts.ReadToolGuideFull(manualPathFor(filepath.Join("..", "..", "prompts"), name)); !ok {
			t.Errorf("missing manual binding: %s", name)
		}
	}
}
