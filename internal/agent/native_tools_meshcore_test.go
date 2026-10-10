package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/meshcore"
)

func TestMeshCoreDiscoveryAndDispatch(t *testing.T) {
	resetToolCatalogForTest(t)
	m, err := meshcore.NewManager(context.Background(), t.TempDir(), meshcore.Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	previous := meshcore.DefaultManager()
	meshcore.SetDefaultManager(m)
	t.Cleanup(func() { meshcore.SetDefaultManager(previous); _ = m.Close() })
	cfg := &config.Config{}
	cfg.MeshCore.Enabled = true
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	schemas := BuildNativeToolSchemas(t.TempDir(), nil, buildToolFlagsFromConfig(cfg), logger)
	for _, active := range []bool{false, true} {
		session := "meshcore-hidden"
		if active {
			session = "meshcore-active"
			SetDiscoverToolsState(session, schemas, schemas, "")
		} else {
			SetDiscoverToolsState(session, schemas, nil, "")
		}
		var discovery DiscoverToolsResponse
		decodeToolOutputJSON(t, handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": "meshcore"}}, cfg, logger, session), &discovery)
		if discovery.Tool == nil || !discovery.Tool.CallableNow || !discovery.Tool.SchemaAvailable {
			t.Fatalf("MeshCore must be discoverable and callable: %+v", discovery)
		}
		dc := &DispatchContext{Cfg: cfg, Logger: logger, SessionID: session}
		for _, op := range []string{"status", "contacts", "channels"} {
			args := map[string]interface{}{"operation": op}
			for _, call := range []ToolCall{
				{Action: "meshcore", Params: args},
				{Action: "invoke_tool", Params: map[string]interface{}{"tool_name": "meshcore", "arguments": args}},
			} {
				out := dispatchInner(context.Background(), call, dc)
				if !strings.Contains(out, `"status":"success"`) {
					t.Fatalf("%s %s (active=%v) did not reach MeshCore: %s", call.Action, op, active, out)
				}
			}
		}
	}
	cfg.MeshCore.Enabled = false
	schemas = BuildNativeToolSchemas(t.TempDir(), nil, buildToolFlagsFromConfig(cfg), logger)
	SetDiscoverToolsState("meshcore-disabled", schemas, schemas, "")
	var discovery DiscoverToolsResponse
	decodeToolOutputJSON(t, handleDiscoverTools(ToolCall{Params: map[string]interface{}{"operation": "get_tool_info", "tool_name": "meshcore"}}, cfg, logger, "meshcore-disabled"), &discovery)
	if discovery.Tool == nil || discovery.Tool.CallableNow || discovery.Tool.ToolStatus != "disabled" {
		t.Fatalf("disabled integration must stay unavailable: %+v", discovery)
	}
	out := dispatchInner(context.Background(), ToolCall{Action: "meshcore", Operation: "status"}, &DispatchContext{Cfg: cfg, Logger: logger, SessionID: "meshcore-disabled"})
	if !strings.Contains(out, "meshcore_unavailable") {
		t.Fatalf("direct dispatch bypassed disabled integration: %s", out)
	}
}

func TestMeshCoreContactsAndSendErrorsStayNarrow(t *testing.T) {
	hops := 2
	contacts := meshcore.AgentContacts([]meshcore.Contact{{
		Key: strings.Repeat("ab", 32), Name: "Ada", Type: 1, Flags: 3,
		LastAdvert: 11, LastModified: 12,
		Position: &meshcore.Position{Latitude: 52.5, Longitude: 13.4},
		OutPath:  &meshcore.PathInfo{Hashes: "deadbeef", Hops: &hops},
	}})
	raw, err := json.Marshal(contacts)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"advertised_position", "out_path", "52.5", "deadbeef"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("agent contacts leaked %s: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"key"`) || !strings.Contains(text, `"name":"Ada"`) || !strings.Contains(text, `"last_advert_timestamp":11`) {
		t.Fatalf("projection dropped contact identity: %s", text)
	}
	source, err := os.ReadFile("meshcore_tool.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "meshcore.AgentContacts(") || strings.Contains(string(source), `result["message"] = err.Error()`) {
		t.Fatal("meshcore dispatch must project contacts and hide send error text")
	}
	m, err := meshcore.NewManager(context.Background(), t.TempDir(), meshcore.Hooks{})
	if err != nil {
		t.Fatal(err)
	}
	previous := meshcore.DefaultManager()
	meshcore.SetDefaultManager(m)
	t.Cleanup(func() { meshcore.SetDefaultManager(previous); _ = m.Close() })
	cfg := &config.Config{}
	cfg.MeshCore.Enabled = true
	out := dispatchMeshCore(context.Background(), ToolCall{Action: "meshcore", Params: map[string]interface{}{"operation": "send_direct", "node_key": strings.Repeat("ab", 32), "text": "hi"}}, &DispatchContext{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if !strings.Contains(out, `"code":"meshcore_send_failed"`) || !strings.Contains(out, `"send_state":"not_sent"`) || strings.Contains(out, "meshcore_not_connected") || strings.Contains(out, `"message"`) {
		t.Fatalf("send failure leaked: %s", out)
	}
}
