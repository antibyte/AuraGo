package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
)

func composioManualTestContext(t *testing.T) (*DispatchContext, string, string) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Composio.Enabled = true
	cfg.Composio.APIKey = "configured-test-value"
	cfg.Composio.Toolkits = []config.ComposioToolkitConfig{{Slug: "gmail", Enabled: true}}
	cfg.Agent.ToolOutputLimit = 1800
	cfg.Directories.PromptsDir = t.TempDir()
	path := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals", "composio_call.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	guide := "# composio_call\n" + strings.Repeat("Read selected service guidance: Äpfel 日本語.\n", 25)
	if err := os.WriteFile(path, []byte(guide), 0600); err != nil {
		t.Fatal(err)
	}
	dc := &DispatchContext{Cfg: cfg, SessionID: t.Name(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	all := dispatchCatalogSchemas(dc)
	setRunDiscoverToolsState(dc, all, nil)
	t.Cleanup(func() { ClearDiscoverToolsState(dc.SessionID) })
	return dc, path, guide
}

func TestComposioServiceManualExactNamesAndPolicy(t *testing.T) {
	dc, _, _ := composioManualTestContext(t)
	read := func(name string) DiscoverToolsResponse {
		var result DiscoverToolsResponse
		out := handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_manual", "tool_name": name}}, dc.Cfg, dc.Logger, dc.SessionID, dc)
		decodeToolOutputJSON(t, out, &result)
		return result
	}
	canonical := read("composio_call")
	for _, name := range []string{"composio:gmail", "gmail", " Gmail ", " COMPOSIO:GMAIL ", "google mail", "googlemail", "g mail"} {
		result := read(name)
		if result.Status != "success" || result.Manual != canonical.Manual || result.Revision != canonical.Revision {
			t.Fatalf("%q: %+v", name, result)
		}
	}
	for _, name := range []string{"composio:slack", "slack", "gmai", "please use gmail", "gmail inbox", "composio:googlemail", "composio:gmail-inbox", ""} {
		if result := read(name); result.Error != "tool_not_found" || result.Manual != "" {
			t.Fatalf("non-exact %q resolved: %+v", name, result)
		}
	}
	dc.Cfg.Composio.Toolkits = append(dc.Cfg.Composio.Toolkits, config.ComposioToolkitConfig{Slug: "googlemail", Enabled: true})
	if result := read("googlemail"); result.Error != "tool_not_found" {
		t.Fatalf("ambiguous alias resolved: %+v", result)
	}
	if result := read("composio:googlemail"); result.Status != "success" {
		t.Fatalf("exact ID lost to alias collision: %+v", result)
	}

	for _, test := range []struct {
		name   string
		change func(*DispatchContext)
	}{
		{"integration_disabled", func(dc *DispatchContext) { dc.Cfg.Composio.Enabled = false }},
		{"not_configured", func(dc *DispatchContext) { dc.Cfg.Composio.APIKey = "" }},
		{"unselected", func(dc *DispatchContext) { dc.Cfg.Composio.Toolkits = nil }},
		{"service_disabled", func(dc *DispatchContext) { dc.Cfg.Composio.Toolkits[0].Enabled = false }},
		{"parent_disabled", func(dc *DispatchContext) { setRunDiscoverToolsState(dc, nil, nil) }},
		{"scope_blocked", func(dc *DispatchContext) {
			dc.ToolScopeRestricted = true
			dc.AllowedTools = map[string]struct{}{"discover_tools": {}}
			setRunDiscoverToolsState(dc, dispatchCatalogSchemas(dc), nil)
		}},
		{"scope_blocked_after_snapshot", func(dc *DispatchContext) {
			dc.ToolScopeRestricted = true
			dc.AllowedTools = map[string]struct{}{"discover_tools": {}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dc, _, _ := composioManualTestContext(t)
			test.change(dc)
			for _, name := range []string{"composio:gmail", "gmail", "google mail"} {
				var result DiscoverToolsResponse
				out := handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_manual", "tool_name": name}}, dc.Cfg, dc.Logger, dc.SessionID, dc)
				decodeToolOutputJSON(t, out, &result)
				if result.Error != "tool_not_found" || result.Manual != "" {
					t.Fatalf("blocked %q resolved: %+v", name, result)
				}
			}
		})
	}
}

func TestComposioServiceManualPaginationAndRevision(t *testing.T) {
	dc, path, guide := composioManualTestContext(t)
	read := func(name, cursor string) DiscoverToolsResponse {
		var result DiscoverToolsResponse
		out := handleDiscoverToolsContext(context.Background(), ToolCall{Params: map[string]interface{}{"operation": "get_manual", "tool_name": name, "cursor": cursor}}, dc.Cfg, dc.Logger, dc.SessionID, dc)
		if len(out) > dc.Cfg.Agent.ToolOutputLimit || !utf8.ValidString(out) {
			t.Fatalf("invalid or oversized page: %d bytes", len(out))
		}
		decodeToolOutputJSON(t, out, &result)
		return result
	}
	first := read("composio:gmail", "")
	if first.Status != "success" || first.NextCursor == "" {
		t.Fatalf("expected paginated manual: %+v", first)
	}
	joined, cursor := first.Manual, first.NextCursor
	for pages := 0; cursor != "" && pages < 100; pages++ {
		part := read("google mail", cursor) // aliases share the canonical service cursor
		if part.Status != "success" || part.Revision != first.Revision || part.NextCursor == cursor {
			t.Fatalf("invalid continuation: %+v", part)
		}
		joined += part.Manual
		cursor = part.NextCursor
	}
	if cursor != "" || joined != guide {
		t.Fatal("manual was incomplete or changed during pagination")
	}
	dc.Cfg.Composio.Toolkits = append(dc.Cfg.Composio.Toolkits, config.ComposioToolkitConfig{Slug: "slack", Enabled: true})
	if result := read("composio:slack", first.NextCursor); result.Error != "manual_cursor_invalid_or_changed" {
		t.Fatalf("cursor crossed service identity: %+v", result)
	}
	if err := os.WriteFile(path, []byte(guide+"\nUpdated guidance.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if result := read("composio:gmail", first.NextCursor); result.Error != "manual_cursor_invalid_or_changed" {
		t.Fatalf("stale content revision accepted: %+v", result)
	}
}

func TestComposioServiceManualRevocationThroughDispatch(t *testing.T) {
	dc, _, _ := composioManualTestContext(t)
	current := *dc.Cfg
	initial := dc.Cfg
	initial.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return initial, &current }
	for _, allowed := range []bool{true, false} {
		current.Composio.Enabled = allowed
		for _, wrapped := range []bool{false, true} {
			args := map[string]interface{}{"operation": "get_manual", "tool_name": "composio:gmail"}
			call := ToolCall{Action: "discover_tools", Params: args}
			if wrapped {
				call = ToolCall{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "discover_tools", "arguments": args}}
			}
			result := DispatchToolCallResult(context.Background(), &call, dc, "read service manual")
			if allowed && result.Status != ToolResultSuccess || !allowed && !strings.Contains(result.Output, "tool_not_found") {
				t.Fatalf("allowed=%v wrapped=%v: %+v", allowed, wrapped, result)
			}
		}
	}
}
