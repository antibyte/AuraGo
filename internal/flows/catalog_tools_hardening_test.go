package flows

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func toolInput(tools ToolInvoker) ExecInput {
	return ExecInput{
		Node:     &Node{ID: testNodeID(1)},
		Run:      RunInfo{ID: "run_h", FlowID: "flow_h", Mode: ModeTest},
		Services: &Services{Tools: tools},
	}
}

// errorTools returns a fixed error from the invoker.
type errorTools struct{ err error }

func (e errorTools) InvokeTool(context.Context, ToolRequest) (ToolResponse, error) {
	return ToolResponse{}, e.err
}

func assertBoundedMessage(t *testing.T, msg string) {
	t.Helper()
	if !utf8.ValidString(msg) {
		t.Fatalf("message is not valid UTF-8 (cut inside a rune): %q", truncateRunes(strings.ToValidUTF8(msg, "?"), 20))
	}
	// The cut adds one ellipsis rune.
	if n := utf8.RuneCountInString(msg); n > maxToolMessageRunes+1 {
		t.Fatalf("message has %d runes, want at most %d", n, maxToolMessageRunes+1)
	}
}

func TestParseToolOutputBoundsErrorMessage(t *testing.T) {
	for _, key := range []string{"message", "error", "detail", "text"} {
		raw := fmt.Sprintf(`Tool Output: {"status":"error",%q:%q}`, key, strings.Repeat("é", 100000))
		_, ne := ParseToolOutput(raw)
		if ne == nil || ne.Code != "FLOW_TOOL_ERROR" {
			t.Fatalf("%s: ne = %+v", key, ne)
		}
		assertBoundedMessage(t, ne.Message)
		if !strings.HasPrefix(ne.Message, "éééé") || !strings.HasSuffix(ne.Message, "…") {
			t.Fatalf("%s: message should keep the start and end with an ellipsis, got %q", key, ne.Message)
		}
	}
	_, ne := ParseToolOutput(`{"status":"policy_denied","message":"` + strings.Repeat("x", 5000) + `"}`)
	if ne == nil || ne.Code != "FLOW_TOOL_DENIED" {
		t.Fatalf("denied = %+v", ne)
	}
	assertBoundedMessage(t, ne.Message)
}

func TestParseToolOutputBoundsStructuredMessage(t *testing.T) {
	// A message that is an object is echoed as compact JSON, cut as well.
	big := strings.Repeat("a", 5000)
	_, ne := ParseToolOutput(`{"status":"failed","message":{"detail":"` + big + `"}}`)
	if ne == nil {
		t.Fatal("want an error")
	}
	assertBoundedMessage(t, ne.Message)
	if !strings.HasPrefix(ne.Message, `{"detail":"aaa`) {
		t.Fatalf("message = %q", truncateRunes(ne.Message, 30))
	}
	// A short message stays untouched.
	_, ne = ParseToolOutput(`{"status":"error","message":"  short and sweet  "}`)
	if ne == nil || ne.Message != "short and sweet" {
		t.Fatalf("short message = %+v", ne)
	}
	// A blank message falls back to the generic text instead of echoing blanks.
	_, ne = ParseToolOutput(`{"status":"error","message":"   ","error":""}`)
	if ne == nil || ne.Message != "the tool reported an error" {
		t.Fatalf("blank message = %+v", ne)
	}
}

func TestCallToolBoundsFailureMessage(t *testing.T) {
	huge := strings.Repeat("ü", 1<<20)
	for name, resp := range map[string]ToolResponse{
		"plain text":      {Output: huge, IsError: true, Status: "failed"},
		"denied":          {Output: huge, IsError: true, Status: "denied"},
		"needs setup":     {Output: huge, IsError: true, Status: "needs_setup"},
		"json text field": {Output: `{"text":"` + huge + `"}`, IsError: true},
		"wrapped":         {Output: "<external_data>\n" + huge + "\n</external_data>", IsError: true},
	} {
		resp := resp
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return resp, nil }}
		_, err := callTool(context.Background(), toolInput(tools), "x", nil)
		ne := asNodeError(err)
		if ne == nil || ne.Code == "" {
			t.Fatalf("%s: err = %v", name, err)
		}
		assertBoundedMessage(t, ne.Message)
		if utf8.RuneCountInString(ne.Message) < maxToolMessageRunes {
			t.Fatalf("%s: message was cut too short: %d runes", name, utf8.RuneCountInString(ne.Message))
		}
	}
}

func TestCallToolBoundsTransportError(t *testing.T) {
	long := errors.New(strings.Repeat("socket closed ", 20000))
	_, err := callTool(context.Background(), toolInput(errorTools{long}), "x", nil)
	ne := asNodeError(err)
	if ne.Code != "FLOW_NODE_FAILED" || !strings.HasPrefix(ne.Message, "socket closed socket closed") {
		t.Fatalf("transport error = %+v", ne)
	}
	assertBoundedMessage(t, ne.Message)
	if !strings.HasSuffix(ne.Message, "…") {
		t.Fatalf("the cut must be visible: %q", truncateRunes(ne.Message, 30))
	}
}

func TestCallToolKeepsInvokerNodeError(t *testing.T) {
	own := &NodeError{Code: "TOOL_BUSY", Message: strings.Repeat("b", 4000)}
	wrapped := fmt.Errorf("outer: %w", own)
	_, err := callTool(context.Background(), toolInput(errorTools{wrapped}), "x", nil)
	ne := asNodeError(err)
	if ne.Code != "TOOL_BUSY" {
		t.Fatalf("a NodeError from the invoker keeps its code, got %q", ne.Code)
	}
	assertBoundedMessage(t, ne.Message)
	if len(own.Message) != 4000 {
		t.Fatalf("the invoker's own error was modified: %d bytes", len(own.Message))
	}
	if ne == own {
		t.Fatal("callTool must return a copy of the invoker's error")
	}
}

func TestCallToolTypedNilErrorFromInvoker(t *testing.T) {
	var typedNil *NodeError
	_, err := callTool(context.Background(), toolInput(errorTools{typedNil}), "x", nil)
	if err == nil {
		t.Fatal("a typed-nil error from the invoker is still a failure")
	}
	ne := asNodeError(err)
	if ne == nil || ne.Code != "FLOW_NODE_FAILED" || ne.Message != "the node returned a nil error value" {
		t.Fatalf("typed nil = %+v", ne)
	}
	// err itself must be safe to use: Error() on the returned value must not panic.
	if err.Error() == "" {
		t.Fatal("empty error text")
	}
}

func TestCallToolSuccessReturnsLiteralNil(t *testing.T) {
	for name, output := range map[string]string{
		"object":    `Tool Output: {"status":"success"}`,
		"no status": `{"a":1}`,
		"array":     `[1,2]`,
		"text":      `done`,
		"empty":     ``,
		"null":      `null`,
		"wrapped":   "<external_data>\nok\n</external_data>",
	} {
		tools := &fakeTools{respond: toolReply(output)}
		out, err := callTool(context.Background(), toolInput(tools), "x", nil)
		// err == nil on the interface: a nil *NodeError stored in it would be non-nil.
		if err != nil {
			t.Fatalf("%s: err = %#v, want a literal nil", name, err)
		}
		if out == nil {
			t.Fatalf("%s: output must not be nil", name)
		}
		if _, ne := ParseToolOutput(output); ne != nil {
			t.Fatalf("%s: ParseToolOutput error = %+v", name, ne)
		}
	}
}

func TestCallToolFailureOutputAccompaniesError(t *testing.T) {
	tools := &fakeTools{respond: toolReply(`Tool Output: {"status":"error","message":"nope","code":7}`)}
	out, err := callTool(context.Background(), toolInput(tools), "x", nil)
	if asNodeError(err).Code != "FLOW_TOOL_ERROR" || out["code"] != 7.0 {
		t.Fatalf("out = %#v, err = %v", out, err)
	}
}

// ctxProbeKey carries a marker through the context so an invoker can prove it
// received the very context callTool was given.
type ctxProbeKey struct{}

// ctxProbeTools records the context it is invoked with and cancels it mid-call.
type ctxProbeTools struct {
	cancel context.CancelFunc
	marker any
	done   bool
}

func (p *ctxProbeTools) InvokeTool(ctx context.Context, _ ToolRequest) (ToolResponse, error) {
	p.marker = ctx.Value(ctxProbeKey{})
	p.cancel()
	p.done = ctx.Err() != nil // the cancel above is visible through the forwarded context
	return ToolResponse{}, ctx.Err()
}

func TestCallToolForwardsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxProbeKey{}, "marker"))
	defer cancel()
	probe := &ctxProbeTools{cancel: cancel}
	_, err := callTool(ctx, toolInput(probe), "x", nil)
	if probe.marker != "marker" {
		t.Fatalf("the invoker did not receive the caller's context: marker = %v", probe.marker)
	}
	if !probe.done {
		t.Fatal("cancelling the caller's context must be visible inside the invoker")
	}
	if ne := asNodeError(err); ne.Code != "FLOW_NODE_FAILED" || !strings.Contains(ne.Message, "canceled") {
		t.Fatalf("a cancel during the call comes back as a failure, got %v", err)
	}
}

func TestCallToolFailsOnDoneContext(t *testing.T) {
	tools := &fakeTools{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := callTool(ctx, toolInput(tools), "x", nil)
	if ne := asNodeError(err); ne.Code != "FLOW_NODE_FAILED" || !strings.Contains(ne.Message, "canceled") {
		t.Fatalf("cancelled = %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()
	_, err = callTool(ctx, toolInput(tools), "x", nil)
	if ne := asNodeError(err); ne.Code != "FLOW_NODE_TIMEOUT" {
		t.Fatalf("an expired deadline maps to a timeout, got %v", err)
	}
	// An invoker that ignores its context must not be reached at all.
	if tools.count() != 0 {
		t.Fatalf("a finished context must not start a tool call, %d calls made", tools.count())
	}
}

func TestCallToolWithoutNode(t *testing.T) {
	tools := &fakeTools{}
	in := ExecInput{Run: RunInfo{ID: "run_h", FlowID: "flow_h", Mode: ModeLive}, Services: &Services{Tools: tools}}
	if _, err := callTool(context.Background(), in, "x", map[string]any{"a": 1}); err != nil {
		t.Fatalf("callTool without a node: %v", err)
	}
	if req := tools.last(t); req.NodeID != "" || req.RunID != "run_h" || !reflect.DeepEqual(req.Args, map[string]any{"a": 1}) {
		t.Fatalf("request = %+v", req)
	}
	if _, err := callTool(context.Background(), ExecInput{}, "x", nil); asNodeError(err).Code != "FLOW_TOOLS_UNAVAILABLE" {
		t.Fatalf("no services = %v", err)
	}
}

func TestCallToolDoesNotTouchArgs(t *testing.T) {
	tools := &fakeTools{}
	args := map[string]any{"list": []any{"a", "b"}, "n": 1.0}
	if _, err := callTool(context.Background(), toolInput(tools), "x", args); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, map[string]any{"list": []any{"a", "b"}, "n": 1.0}) {
		t.Fatalf("args were modified: %#v", args)
	}
	// Each call gets its own allow-list.
	first := tools.last(t).AllowedTools
	first[0] = "tampered"
	if _, err := callTool(context.Background(), toolInput(tools), "x", nil); err != nil {
		t.Fatal(err)
	}
	if got := tools.last(t).AllowedTools; got[0] != "x" {
		t.Fatalf("allow-list leaked between calls: %v", got)
	}
}

func TestParseToolOutputScalarsAndNull(t *testing.T) {
	cases := map[string]map[string]any{
		`null`:                                   {},
		`Tool Output: null`:                      {},
		`true`:                                   {"value": true},
		`"<external_data>\nx\n</external_data>"`: {"text": "x"},
	}
	for raw, want := range cases {
		got, ne := ParseToolOutput(raw)
		if ne != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("ParseToolOutput(%q) = %#v, %v; want %#v", raw, got, ne, want)
		}
	}
}

func TestParseToolOutputDeepNesting(t *testing.T) {
	// json.Unmarshal caps nesting, and the unwrap walk must cope with what it lets through.
	deep := strings.Repeat("[", 9000) + strings.Repeat("]", 9000)
	got, ne := ParseToolOutput(deep)
	if ne != nil || got["items"] == nil {
		t.Fatalf("deep array = %v, %v", got, ne)
	}
	tooDeep := strings.Repeat("[", 20000) + strings.Repeat("]", 20000)
	got, ne = ParseToolOutput(tooDeep)
	if ne != nil || got["text"] != tooDeep {
		t.Fatalf("an array beyond the nesting limit is plain text, got ne=%v", ne)
	}
}
