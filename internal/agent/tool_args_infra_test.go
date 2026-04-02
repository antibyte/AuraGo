package agent

import "testing"

func TestDecodeGitHubArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "github",
		Params: map[string]interface{}{
			"operation":   "create_issue",
			"owner":       "antibyte",
			"name":        "AuraGo",
			"title":       "Bug report",
			"body":        "details",
			"label":       "bug,urgent",
			"id":          "42",
			"query":       "main",
			"limit":       float64(7),
			"description": "tracked repo",
		},
	}

	req := decodeGitHubArgs(tc)
	if req.Operation != "create_issue" {
		t.Fatalf("Operation = %q, want create_issue", req.Operation)
	}
	if req.Owner != "antibyte" {
		t.Fatalf("Owner = %q, want antibyte", req.Owner)
	}
	if req.Repo != "AuraGo" {
		t.Fatalf("Repo = %q, want AuraGo", req.Repo)
	}
	if req.Title != "Bug report" || req.Body != "details" {
		t.Fatalf("unexpected issue payload: %#v", req)
	}
	if req.issueNumber() != 42 {
		t.Fatalf("issueNumber = %d, want 42", req.issueNumber())
	}
	labels := req.labels()
	if len(labels) != 2 || labels[0] != "bug" || labels[1] != "urgent" {
		t.Fatalf("labels = %#v, want [bug urgent]", labels)
	}
	if req.Query != "main" {
		t.Fatalf("Query = %q, want main", req.Query)
	}
	if req.Limit != 7 {
		t.Fatalf("Limit = %d, want 7", req.Limit)
	}
}

func TestDecodeNetlifyArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "netlify",
		Params: map[string]interface{}{
			"operation":     "create_hook",
			"site_id":       "site-123",
			"hook_id":       "hook-123",
			"hook_type":     "url",
			"hook_event":    "deploy_created",
			"url":           "https://example.com/hook",
			"value":         "ops@example.com",
			"site_name":     "aurago-docs",
			"custom_domain": "docs.example.com",
		},
	}

	req := decodeNetlifyArgs(tc)
	if req.Operation != "create_hook" {
		t.Fatalf("Operation = %q, want create_hook", req.Operation)
	}
	if req.SiteID != "site-123" || req.HookID != "hook-123" {
		t.Fatalf("unexpected site/hook ids: %#v", req)
	}
	if req.SiteName != "aurago-docs" || req.CustomDomain != "docs.example.com" {
		t.Fatalf("unexpected site metadata: %#v", req)
	}
	hookData := req.hookData()
	if hookData["url"] != "https://example.com/hook" {
		t.Fatalf("hookData[url] = %#v, want hook URL", hookData["url"])
	}
	if hookData["email"] != "ops@example.com" {
		t.Fatalf("hookData[email] = %#v, want email", hookData["email"])
	}
}

func TestDecodeMCPCallArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "mcp_call",
		Params: map[string]interface{}{
			"operation": "call_tool",
			"server":    "filesystem",
			"tool_name": "read_file",
			"args": map[string]interface{}{
				"path": "/tmp/demo.txt",
			},
		},
	}

	req := decodeMCPCallArgs(tc)
	if req.Operation != "call_tool" {
		t.Fatalf("Operation = %q, want call_tool", req.Operation)
	}
	if req.Server != "filesystem" {
		t.Fatalf("Server = %q, want filesystem", req.Server)
	}
	if req.ToolName != "read_file" {
		t.Fatalf("ToolName = %q, want read_file", req.ToolName)
	}
	if req.Args["path"] != "/tmp/demo.txt" {
		t.Fatalf("Args[path] = %#v, want /tmp/demo.txt", req.Args["path"])
	}
}

func TestDecodeAdGuardArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "adguard",
		Params: map[string]interface{}{
			"operation": "blocked_services_set",
			"query":     "client:test",
			"limit":     float64(25),
			"offset":    float64(5),
			"services":  []interface{}{"tiktok", "youtube"},
			"enabled":   true,
			"rules":     []interface{}{"||ads.example^", "||track.example^"},
			"domain":    "bad.example",
			"answer":    "0.0.0.0",
			"content":   "{\"client\":\"demo\"}",
			"mac":       "00:11:22:33:44:55",
			"ip":        "192.168.1.10",
			"hostname":  "demo-host",
			"url":       "https://filters.example/list.txt",
			"name":      "My filter",
		},
	}

	req := decodeAdGuardArgs(tc)
	if req.Operation != "blocked_services_set" {
		t.Fatalf("Operation = %q, want blocked_services_set", req.Operation)
	}
	if req.Query != "client:test" || req.Limit != 25 || req.Offset != 5 {
		t.Fatalf("unexpected query pagination: %#v", req)
	}
	if len(req.Services) != 2 || req.Services[0] != "tiktok" || req.Services[1] != "youtube" {
		t.Fatalf("Services = %#v, want [tiktok youtube]", req.Services)
	}
	if !req.Enabled {
		t.Fatal("expected Enabled to be true")
	}
	if req.Rules != "||ads.example^\n||track.example^" {
		t.Fatalf("Rules = %q, want newline-joined rules", req.Rules)
	}
	if req.Domain != "bad.example" || req.Answer != "0.0.0.0" {
		t.Fatalf("unexpected rewrite data: %#v", req)
	}
	if req.MAC != "00:11:22:33:44:55" || req.IP != "192.168.1.10" || req.Hostname != "demo-host" {
		t.Fatalf("unexpected lease data: %#v", req)
	}
	if req.URL != "https://filters.example/list.txt" || req.Name != "My filter" {
		t.Fatalf("unexpected URL/name data: %#v", req)
	}
}

func TestDecodeSQLQueryArgsUsesParamsFallback(t *testing.T) {
	tc := ToolCall{
		Action: "sql_query",
		Params: map[string]interface{}{
			"operation":       "describe",
			"connection_name": "analytics",
			"sql_query":       "select 1",
			"table_name":      "events",
		},
	}

	req := decodeSQLQueryArgs(tc)
	if req.Operation != "describe" {
		t.Fatalf("Operation = %q, want describe", req.Operation)
	}
	if req.ConnectionName != "analytics" {
		t.Fatalf("ConnectionName = %q, want analytics", req.ConnectionName)
	}
	if req.SQLQuery != "select 1" {
		t.Fatalf("SQLQuery = %q, want select 1", req.SQLQuery)
	}
	if req.TableName != "events" {
		t.Fatalf("TableName = %q, want events", req.TableName)
	}
}

func TestDecodeMQTTArgsUsesPayloadFallbacks(t *testing.T) {
	tc := ToolCall{
		Action: "mqtt_publish",
		Params: map[string]interface{}{
			"topic":  "home/test",
			"qos":    float64(2),
			"retain": true,
			"limit":  float64(15),
		},
		Message: "hello",
	}

	req := decodeMQTTArgs(tc)
	if req.Topic != "home/test" {
		t.Fatalf("Topic = %q, want home/test", req.Topic)
	}
	if req.Payload != "hello" {
		t.Fatalf("Payload = %q, want hello", req.Payload)
	}
	if req.QoS != 2 {
		t.Fatalf("QoS = %d, want 2", req.QoS)
	}
	if !req.Retain {
		t.Fatal("expected Retain to be true")
	}
	if req.Limit != 15 {
		t.Fatalf("Limit = %d, want 15", req.Limit)
	}
}
