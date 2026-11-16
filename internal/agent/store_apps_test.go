package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
	openai "github.com/sashabaranov/go-openai"
)

func TestStoreAppNativeDispatchAndCapturedPermissions(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Tools.VirtualDesktop.Enabled = true
	cfg.Agent.AllowNetworkRequests = true
	var calls int
	tools.SetStoreApplications(func(ctx context.Context, id string, req tools.StoreAppRequest) (any, error) {
		calls++
		if id != "olivetin" || req.ActionID != "backup" || len(req.Arguments) != 1 || req.Arguments[0].Value != "weekly" {
			t.Fatalf("lost arguments: %+v", req)
		}
		return map[string]any{"status": "accepted"}, nil
	}, func(string) bool { return true })
	t.Cleanup(func() { tools.SetStoreApplications(nil, nil) })
	tc := NativeToolCallToToolCall(openai.ToolCall{Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "olivetin", Arguments: `{"operation":"execute","action_id":"backup","arguments":[{"name":"kind","value":"weekly"}]}`}}, nil)
	dc := &DispatchContext{Cfg: cfg}
	if out := dispatchStoreApplication(t.Context(), tc, dc); !strings.Contains(out, "accepted") {
		t.Fatal(out)
	}
	cfg.VirtualDesktop.ReadOnly = true
	if out := dispatchStoreApplication(t.Context(), tc, dc); !strings.Contains(out, "read-only") {
		t.Fatal(out)
	}
	cfg.VirtualDesktop.ReadOnly = false
	cfg.Agent.AllowNetworkRequests = false
	if out := dispatchStoreApplication(t.Context(), tc, dc); !strings.Contains(out, "disabled") {
		t.Fatal(out)
	}
	if calls != 1 {
		t.Fatal("captured policy bypassed")
	}
}

func TestStoreAppA2AOnlyReads(t *testing.T) {
	for _, tc := range []ToolCall{{Action: "node_red", Operation: "trigger"}, {Action: "olivetin", Operation: "execute"}, {Action: "node_red"}} {
		if a2aRestriction(tc) == "" {
			t.Fatalf("A2A mutation accepted: %+v", tc)
		}
	}
	for _, tc := range []ToolCall{{Action: "node_red", Operation: "list_flows"}, {Action: "olivetin", Operation: "get_execution"}} {
		if a2aRestriction(tc) != "" {
			t.Fatalf("A2A read denied: %+v", tc)
		}
	}
}
