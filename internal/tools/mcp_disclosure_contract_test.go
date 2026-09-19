package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type disclosureMCPTransport struct {
	mcpNotificationState
	responses []string
	requests  []map[string]interface{}
}

func TestMCPExecutionErrorPreservesStructuredContent(t *testing.T) {
	raw := `{"isError":true,"content":[],"structuredContent":{"code":"not_found"}}`
	conn := &mcpConn{transport: &disclosureMCPTransport{responses: []string{raw}}}
	_, err := conn.callTool(context.Background(), "read", nil)
	var execution *MCPToolExecutionError
	if !errors.As(err, &execution) || string(execution.Result) != raw {
		t.Fatalf("lost typed error: %v", err)
	}
	for _, raw := range []string{"null", "{}"} {
		conn.transport = &disclosureMCPTransport{responses: []string{raw}}
		if _, err := conn.callTool(context.Background(), "read", nil); err == nil {
			t.Fatal("empty result accepted")
		}
	}
}

func (m *disclosureMCPTransport) Send(_ context.Context, _ string, params interface{}) (*jsonRPCResponse, error) {
	if p, ok := params.(map[string]interface{}); ok {
		m.requests = append(m.requests, p)
	}
	raw := m.responses[0]
	m.responses = m.responses[1:]
	return &jsonRPCResponse{Result: json.RawMessage(raw)}, nil
}
func (*disclosureMCPTransport) Notify(context.Context, string, interface{}) error { return nil }
func (*disclosureMCPTransport) Close()                                            {}

func TestMCPCatalogPaginationAtomicRefreshAndNotification(t *testing.T) {
	transport := &disclosureMCPTransport{responses: []string{
		`{"tools":[{"name":"first","inputSchema":{"type":"object"}}],"nextCursor":"page2"}`,
		`{"tools":[{"name":"second","inputSchema":{"type":"object"}}]}`,
	}}
	conn := &mcpConn{name: "fixture", transport: transport}
	if err := conn.discoverTools(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(conn.tools) != 2 || transport.requests[1]["cursor"] != "page2" {
		t.Fatal("pagination lost")
	}
	transport.observeNotification(&jsonRPCResponse{Method: "notifications/tools/list_changed"})
	transport.responses = []string{`{"tools":[{"name":"partial"}],"nextCursor":"loop"}`, `{"tools":[],"nextCursor":"loop"}`}
	if err := conn.refreshToolsIfNeeded(context.Background(), nil); err == nil {
		t.Fatal("cursor cycle accepted")
	}
	if len(conn.tools) != 2 || conn.toolsGeneration == transport.ToolsGeneration() {
		t.Fatal("partial refresh committed or notification lost")
	}
	transport.responses = []string{`{"tools":[{"name":"replacement"}]}`}
	if err := conn.refreshToolsIfNeeded(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(conn.tools) != 1 || conn.tools[0].Name != "replacement" {
		t.Fatal("catalog did not refresh")
	}
}

func TestMCPMixedToolResultPreservesEveryContentKind(t *testing.T) {
	raw := `{"content":[{"type":"text","text":"caption"},{"type":"image","data":"AA==","mimeType":"image/png"},{"type":"resource_link","uri":"https://example.test/item","name":"item"}],"structuredContent":{"count":3}}`
	conn := &mcpConn{transport: &disclosureMCPTransport{responses: []string{raw}}}
	output, err := conn.callTool(context.Background(), "read", nil)
	if err != nil || !json.Valid([]byte(output)) {
		t.Fatalf("invalid mixed result: %s %v", output, err)
	}
	for _, part := range []string{"caption", "AA==", "resource_link", "structuredContent"} {
		if !strings.Contains(output, part) {
			t.Errorf("lost %s", part)
		}
	}
}

func TestMCPCatalogKeepsHealthyResultsWhenAnotherServerIsUnavailable(t *testing.T) {
	mgr := &MCPManager{conns: map[string]*mcpConn{"healthy": {name: "healthy", ready: true, tools: []MCPToolInfo{{Server: "healthy", Name: "read"}}}}, configs: map[string]MCPServerConfig{
		"healthy": {Name: "healthy", Enabled: true, AllowedTools: []string{"read"}},
		"offline": {Name: "offline", Enabled: true},
	}}
	results, err := mgr.ListToolsWithStatus(context.Background(), "")
	if err == nil || len(results) != 1 || results[0].Server != "healthy" {
		t.Fatalf("partial catalog: %+v, %v", results, err)
	}
}
