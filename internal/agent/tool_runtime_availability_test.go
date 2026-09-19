package agent

import (
	"context"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestMissingDependencyAgreesAcrossDiscoveryAndDispatch(t *testing.T) {
	dc := &DispatchContext{Cfg: &config.Config{}, SessionID: t.Name()}
	schema := testToolSchema("sql_query", "query a SQL connection")
	SetDiscoverToolsState(dc.SessionID, nil, nil, "")
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	catalog := BuildToolCatalog([]openai.Tool{schema}, []openai.Tool{schema}, "")
	applyCatalogScope(catalog, dc)
	entry, _ := catalog.Get("sql_query")
	result := discoverResultFromEntry(entry, dc.SessionID, false)
	if result.CallableNow || result.CallMethod != "needs_setup" || result.ToolStatus != "needs_setup" {
		t.Fatalf("misleading discovery: %+v", result)
	}
	output := dispatchInner(context.Background(), ToolCall{Action: "sql_query"}, dc)
	if classifyLegacyToolResult(output) != ToolResultNeedsSetup {
		t.Fatal(output)
	}
}
