package agent

import (
	"context"
	"testing"

	"aurago/internal/config"
)

func TestMissingDependencyAgreesAcrossDiscoveryAndDispatch(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled missing runtime", false: "disabled capability"}[enabled], func(t *testing.T) {
			cfg := &config.Config{}
			cfg.SQLConnections.Enabled = enabled
			dc := &DispatchContext{Cfg: cfg, SessionID: t.Name()}
			// Build the administrative config catalog with real schemas, then
			// apply runtime availability. No database or pool is fabricated.
			schemas := BuildNativeToolSchemas(t.TempDir(), nil, buildToolFlagsFromConfig(cfg), nil)
			SetDiscoverToolsState(dc.SessionID, schemas, schemas, "", dc)
			t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
			catalog := GetToolCatalogState(dc.SessionID)
			entry, ok := catalog.Get("sql_query")
			if !ok {
				t.Fatal("SQL tool missing from discovery catalog")
			}
			result := discoverResultFromEntry(entry, dc.SessionID, false)
			wantStatus := "disabled"
			if enabled {
				wantStatus = "needs_setup"
			}
			if result.CallableNow || result.ToolStatus != wantStatus || enabled && result.CallMethod != "needs_setup" {
				t.Fatalf("misleading discovery: %+v", result)
			}
			// Missing runtime and disabled capability both refuse execution at
			// the native catalog gate, while discovery keeps the cause distinct.
			output := dispatchInner(context.Background(), ToolCall{Action: "sql_query"}, dc)
			assertCanonicalToolDenial(t, output, "tool_disabled")
		})
	}
}
