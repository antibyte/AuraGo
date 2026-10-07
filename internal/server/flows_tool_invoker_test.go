package server

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/flows"
)

type capturedDispatch struct {
	tc agent.ToolCall
	dc *agent.DispatchContext
}

func newTestFlowInvoker(names map[string]bool, out agent.ToolDispatchResult) (*flowToolInvoker, *[]capturedDispatch, *config.Config) {
	cfg := &config.Config{}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	calls := &[]capturedDispatch{}
	inv := &flowToolInvoker{s: s,
		names: func(*config.Config) map[string]bool { return names },
		dispatch: func(_ context.Context, tc *agent.ToolCall, dc *agent.DispatchContext) agent.ToolDispatchResult {
			*calls = append(*calls, capturedDispatch{*tc, dc})
			return out
		}}
	return inv, calls, cfg
}

func TestFlowToolInvokerDispatchesWithARestrictedScope(t *testing.T) {
	out := agent.ToolDispatchResult{Output: "[Tool Output]\nTool Output: {\"status\":\"success\"}", Status: agent.ToolResultSuccess}
	inv, calls, _ := newTestFlowInvoker(map[string]bool{"filesystem": true}, out)
	resp, err := inv.InvokeTool(context.Background(), flows.ToolRequest{FlowID: "flow_aaaaaaaaaa", RunID: "run_x", NodeID: "n_aaaaaaaa",
		Tool: "filesystem", Args: map[string]any{"operation": "read_file", "file_path": "notes.txt"}, AllowedTools: []string{"filesystem"}})
	if err != nil || resp.Output != out.Output || resp.IsError || resp.Status != "success" {
		t.Fatalf("response = %+v, %v", resp, err)
	}
	if len(*calls) != 1 {
		t.Fatalf("dispatch calls = %d", len(*calls))
	}
	c := (*calls)[0]
	if c.tc.Action != "filesystem" || !c.tc.IsTool || c.tc.Params["file_path"] != "notes.txt" || c.tc.Params["operation"] != "read_file" {
		t.Fatalf("tool call = %+v", c.tc)
	}
	if _, ok := c.dc.AllowedTools["filesystem"]; !ok || len(c.dc.AllowedTools) != 1 || !c.dc.ToolScopeRestricted ||
		c.dc.SessionID != "flow-flow_aaaaaaaaaa" || c.dc.MessageSource != "flow" || c.dc.LLMGuardian != nil {
		t.Fatalf("dispatch context = %+v", c.dc)
	}
}

func TestFlowToolInvokerRoutesThePDFExtractorThroughExecuteSkill(t *testing.T) {
	inv, calls, cfg := newTestFlowInvoker(map[string]bool{"execute_skill": true}, agent.ToolDispatchResult{Output: "ok", Status: agent.ToolResultSuccess})
	cfg.Tools.PDFExtractor.Enabled = true
	if _, err := inv.InvokeTool(context.Background(), flows.ToolRequest{FlowID: "flow_aaaaaaaaaa", Tool: flows.PDFExtractorTool,
		Args: map[string]any{"filepath": "a.pdf"}, AllowedTools: []string{flows.PDFExtractorTool}}); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.tc.Action != "execute_skill" || c.tc.Skill != flows.PDFExtractorTool || c.tc.SkillArgs["filepath"] != "a.pdf" {
		t.Fatalf("tool call = %+v", c.tc)
	}
	if _, ok := c.dc.AllowedTools["execute_skill"]; !ok || len(c.dc.AllowedTools) != 1 {
		t.Fatalf("allowed tools = %+v", c.dc.AllowedTools)
	}
}

func TestFlowToolInvokerRefusesUnavailableAndUnlistedTools(t *testing.T) {
	inv, calls, _ := newTestFlowInvoker(map[string]bool{"filesystem": true}, agent.ToolDispatchResult{})
	cases := []struct {
		req    flows.ToolRequest
		status string
	}{
		{flows.ToolRequest{Tool: "proxmox", AllowedTools: []string{"proxmox"}}, "needs_setup"},
		{flows.ToolRequest{Tool: "filesystem", AllowedTools: []string{"other"}}, "denied"},
		{flows.ToolRequest{Tool: flows.GotenbergTool, AllowedTools: []string{flows.GotenbergTool}}, "denied"},
		{flows.ToolRequest{Tool: flows.BraveSearchTool, AllowedTools: []string{flows.BraveSearchTool}}, "needs_setup"},
	}
	for _, c := range cases {
		resp, err := inv.InvokeTool(context.Background(), c.req)
		if err != nil || !resp.IsError || resp.Status != c.status || !strings.Contains(resp.Output, `"status":"error"`) {
			t.Errorf("%s = %+v, %v", c.req.Tool, resp, err)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("refused tools must not be dispatched")
	}
}
