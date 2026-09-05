package agent

import (
	"aurago/internal/meshcore"
	"aurago/internal/tools"
	"context"
	"encoding/json"
	"github.com/sashabaranov/go-openai"
)

func MeshCoreSearchSchema() openai.Tool {
	return tool("brave_search", "Search public web information. No URLs, files or system actions.", schema(map[string]interface{}{"query": prop("string", "Public web search query")}, "query"))
}

func dispatchMeshCoreSearch(tc ToolCall, dc *DispatchContext) string {
	if dc.Cfg == nil || !dc.Cfg.BraveSearch.Enabled {
		return `Tool Output: {"status":"error","code":"search_disabled"}`
	}
	query, _ := tc.Params["query"].(string)
	if len(query) > 512 {
		return `Tool Output: {"status":"error","code":"query_too_long"}`
	}
	return tools.ExecuteBraveSearch(dc.Cfg.BraveSearch.APIKey, query, 3, dc.Cfg.BraveSearch.Country, dc.Cfg.BraveSearch.Lang)
}
func dispatchMeshCore(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	result := map[string]interface{}{"status": "error", "code": "meshcore_unavailable"}
	m := meshcore.DefaultManager()
	if dc.Cfg == nil || !dc.Cfg.MeshCore.Enabled || m == nil {
		b, _ := json.Marshal(result)
		return "Tool Output: " + string(b)
	}
	var args struct {
		Operation string `json:"operation"`
		Key       string `json:"node_key"`
		Channel   *int   `json:"channel"`
		Text      string `json:"text"`
	}
	b, _ := json.Marshal(tc.Params)
	if err := json.Unmarshal(b, &args); err != nil {
		return `Tool Output: {"status":"error","code":"invalid_arguments"}`
	}
	if args.Operation == "" {
		args.Operation = tc.Operation
	}
	result = map[string]interface{}{"status": "success"}
	switch args.Operation {
	case "status":
		st := m.Status()
		result["data"] = map[string]interface{}{"state": st.State, "identity_key": st.IdentityKey, "name": st.Name, "firmware": st.Firmware, "hardware_verified": false}
	case "contacts":
		result["contacts"] = m.Status().Contacts
	case "channels":
		channels := []map[string]interface{}{}
		for _, ch := range m.Status().Channels {
			channels = append(channels, map[string]interface{}{"index": ch.Index, "name": ch.Name})
		}
		result["channels"] = channels
	case "send_direct", "send_channel":
		kind := "direct"
		index := -1
		if args.Operation == "send_channel" {
			kind = "channel"
			if args.Channel == nil {
				return `Tool Output: {"status":"error","code":"channel_required"}`
			}
			index = *args.Channel
		}
		state, err := m.Send(ctx, kind, args.Key, args.Text, index)
		result["send_state"] = state
		if err != nil {
			result["status"] = "error"
			result["code"] = "meshcore_send_failed"
			result["message"] = err.Error()
		}
	default:
		result = map[string]interface{}{"status": "error", "code": "invalid_operation"}
	}
	b, _ = json.Marshal(result)
	return "Tool Output: " + string(b)
}
