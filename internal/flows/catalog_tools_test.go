package flows

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestParseToolOutput(t *testing.T) {
	cases := []struct {
		raw  string
		want map[string]any
		code string
	}{
		{`Tool Output: {"status":"success","a":1}`, map[string]any{"status": "success", "a": 1.0}, ""},
		{"[Tool Output]\nTool Output: [1,2]", map[string]any{"items": []any{1.0, 2.0}}, ""},
		{"plain text", map[string]any{"text": "plain text"}, ""},
		{`"quoted"`, map[string]any{"text": "quoted"}, ""},
		{"42", map[string]any{"value": 42.0}, ""},
		{"   ", map[string]any{}, ""},
		{`Tool Output: {"status":"error","message":"nope"}`, map[string]any{"status": "error", "message": "nope"}, "FLOW_TOOL_ERROR"},
		{`Tool Output: {"status":"policy_denied","message":"blocked"}`, map[string]any{"status": "policy_denied", "message": "blocked"}, "FLOW_TOOL_DENIED"},
	}
	for _, tc := range cases {
		got, ne := ParseToolOutput(tc.raw)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseToolOutput(%q) = %#v, want %#v", tc.raw, got, tc.want)
		}
		code := ""
		if ne != nil {
			code = ne.Code
		}
		if code != tc.code {
			t.Errorf("ParseToolOutput(%q) code = %q, want %q", tc.raw, code, tc.code)
		}
	}
	if _, ne := ParseToolOutput(`{"status":"error","error":"bad"}`); ne == nil || ne.Message != "bad" {
		t.Fatalf("error message from the error field = %+v", ne)
	}
}

func TestParseToolOutputUnwrapsExternalData(t *testing.T) {
	wrapped := "[Tool Output]\n<external_data>\nTool Output: {&#34;status&#34;:&#34;success&#34;,&#34;t&#34;:&#34;a&lt;b&#34;}\n</external_data>"
	got, ne := ParseToolOutput(wrapped)
	if ne != nil || !reflect.DeepEqual(got, map[string]any{"status": "success", "t": "a<b"}) {
		t.Fatalf("outer wrapper = %#v, %v", got, ne)
	}
	inner := `{"status":"success","results":[{"title":"<external_data>\nFoo &amp; Bar\n</external_data>","link":"https://x"}]}`
	got, _ = ParseToolOutput(inner)
	first := got["results"].([]any)[0].(map[string]any)
	if first["title"] != "Foo & Bar" || first["link"] != "https://x" {
		t.Fatalf("inner wrapper = %#v", first)
	}
}

func TestCallTool(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"success","n":2}`)}
	in := ExecInput{Node: &Node{ID: testNodeID(1)}, Run: RunInfo{ID: "run_x", FlowID: "flow_x", Mode: ModeLive}, Services: &Services{Tools: tools}}
	out, err := callTool(context.Background(), in, "ddg_search", map[string]any{"query": "x"})
	if err != nil || out["n"] != 2.0 {
		t.Fatalf("callTool = %#v, %v", out, err)
	}
	req := tools.last(t)
	if req.Tool != "ddg_search" || !reflect.DeepEqual(req.AllowedTools, []string{"ddg_search"}) || req.RunID != "run_x" ||
		req.FlowID != "flow_x" || req.NodeID != testNodeID(1) || req.Mode != ModeLive || req.Args["query"] != "x" {
		t.Fatalf("request = %+v", req)
	}

	tools.respond = func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: `Tool Output: {"message":"no"}`, IsError: true, Status: "denied"}, nil
	}
	if _, err := callTool(context.Background(), in, "x", nil); asNodeError(err).Code != "FLOW_TOOL_DENIED" {
		t.Fatalf("denied = %v", err)
	}
	tools.respond = func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: "boom", IsError: true, Status: "failed"}, nil
	}
	if _, err := callTool(context.Background(), in, "x", nil); asNodeError(err).Code != "FLOW_TOOL_ERROR" || asNodeError(err).Message != "boom" {
		t.Fatalf("failed = %v", err)
	}
	tools.respond = func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: "x", IsError: true, Status: "needs_setup"}, nil
	}
	if _, err := callTool(context.Background(), in, "x", nil); asNodeError(err).Code != "FLOW_NODE_UNAVAILABLE" {
		t.Fatalf("needs setup = %v", err)
	}
	tools.respond = func(ToolRequest) (ToolResponse, error) { return ToolResponse{}, errors.New("transport") }
	if _, err := callTool(context.Background(), in, "x", nil); asNodeError(err).Code != "FLOW_NODE_FAILED" {
		t.Fatalf("transport error = %v", err)
	}
	in.Services = &Services{}
	if _, err := callTool(context.Background(), in, "x", nil); asNodeError(err).Code != "FLOW_TOOLS_UNAVAILABLE" {
		t.Fatalf("no tools = %v", err)
	}
}

func TestCatalogEnvHelpers(t *testing.T) {
	env := StaticEnv{"ddg_search": {}, "skill__brave_search": {State: BlockedState, Reason: "readonly"}}
	if env.ToolAvailability("ddg_search").State != AvailableState || env.ToolAvailability("x").State != NeedsSetupState {
		t.Fatal("StaticEnv mismatch")
	}
	if a := anyAvailable(env, "skill__brave_search", "ddg_search")(); a.State != AvailableState {
		t.Fatalf("anyAvailable = %+v", a)
	}
	if a := anyAvailable(env, "skill__brave_search", "missing")(); a.State != BlockedState || a.Reason != "readonly" {
		t.Fatalf("anyAvailable must report the first tool's state, got %+v", a)
	}
	if availabilityOf(nil, "x")().State != NeedsSetupState {
		t.Fatal("a nil env means needs setup")
	}
	args := map[string]any{}
	setArg(args, "a", "")
	setArg(args, "b", nil)
	setArg(args, "c", "x")
	setArg(args, "d", false)
	setArg(args, "e", 0.0)
	if !reflect.DeepEqual(args, map[string]any{"c": "x", "d": false, "e": 0.0}) {
		t.Fatalf("setArg = %#v", args)
	}
}
