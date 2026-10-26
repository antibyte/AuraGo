package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// TestDiscoverySuggestionsAreBoundedForAnHugeName guards the cost of an unknown
// tool name: a model-supplied name of any size gets its suggestions in the time
// of a short one, from the first bytes of the name.
func TestDiscoverySuggestionsAreBoundedForAnHugeName(t *testing.T) {
	sid := t.Name()
	SetDiscoverToolsState(sid, BuildNativeToolSchemas("", nil, allBuiltinToolFeatureFlags(), nil), nil, "")
	t.Cleanup(func() { ClearDiscoverToolsState(sid) })
	name := "dokcer " + strings.Repeat("x", 100<<10)

	start := time.Now()
	var response DiscoverToolsResponse
	decodeToolOutputJSON(t, handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": name}}, &config.Config{}, nil, sid, nil), &response)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("get_tool_info with a %d byte name took %s", len(name), elapsed)
	}
	if response.Error != "unknown_or_ambiguous_tool" || !containsName(response.Suggestions, "docker") {
		t.Errorf("response = %+v, want suggestions that include docker", response)
	}

	start = time.Now()
	discoverySuggestions(GetToolCatalogState(sid), name+strings.Repeat(" word", 20000), 3, nil)
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("discoverySuggestions with a huge query took %s", elapsed)
	}
}
