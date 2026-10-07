package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

func TestCallToolRejectsOversizedOutput(t *testing.T) {
	over := strings.Repeat("x", maxToolOutputBytes+1)
	for name, resp := range map[string]ToolResponse{
		"success":     {Output: over, Status: "success"},
		"error":       {Output: over, IsError: true, Status: "failed"},
		"denied":      {Output: over, IsError: true, Status: "denied"},
		"json prefix": {Output: `{"status":"success","pad":"` + over + `"}`},
	} {
		resp := resp
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return resp, nil }}
		out, err := callTool(context.Background(), toolInput(tools), "x", nil)
		ne := asNodeError(err)
		if ne == nil || ne.Code != "FLOW_OUTPUT_TOO_LARGE" || out != nil {
			t.Fatalf("%s: out = %v, err = %v", name, out, err)
		}
		if !strings.Contains(ne.Message, fmt.Sprint(len(resp.Output))) || !strings.Contains(ne.Message, fmt.Sprint(maxToolOutputBytes)) {
			t.Fatalf("%s: message must name the size and the limit: %q", name, ne.Message)
		}
	}
}

func TestCallToolAcceptsOutputAtTheCap(t *testing.T) {
	// Plain text keeps the parse cheap: the JSON decoder rejects it at the first byte.
	atCap := strings.Repeat("x", maxToolOutputBytes)
	tools := &fakeTools{respond: toolReply(atCap)}
	out, err := callTool(context.Background(), toolInput(tools), "x", nil)
	if err != nil {
		t.Fatalf("output of exactly the limit must pass: %v", err)
	}
	if text, _ := out["text"].(string); len(text) != maxToolOutputBytes {
		t.Fatalf("text has %d bytes, want %d", len(text), maxToolOutputBytes)
	}
}

func TestCallToolClassifiesByResponseStatusFirst(t *testing.T) {
	cases := []struct {
		name string
		resp ToolResponse
		code string
		msg  string
	}{
		{"error body, denied", ToolResponse{Output: `{"status":"error","message":"no go"}`, IsError: true, Status: "denied"}, "FLOW_TOOL_DENIED", "no go"},
		{"error body, needs setup", ToolResponse{Output: `{"status":"error","message":"set it up"}`, IsError: true, Status: "needs_setup"}, "FLOW_NODE_UNAVAILABLE", "set it up"},
		{"denied without IsError", ToolResponse{Output: "refused", Status: "denied"}, "FLOW_TOOL_DENIED", "refused"},
		{"policy_denied without IsError", ToolResponse{Output: `{"status":"success"}`, Status: "policy_denied"}, "FLOW_TOOL_DENIED", "the tool call was denied"},
		{"needs setup without IsError", ToolResponse{Output: "configure me", Status: "needs_setup"}, "FLOW_NODE_UNAVAILABLE", "configure me"},
		{"needs setup, empty output", ToolResponse{Status: "needs_setup", IsError: true}, "FLOW_NODE_UNAVAILABLE", "the tool is not set up"},
		{"status spelling is normalised", ToolResponse{Output: "refused", Status: " DENIED "}, "FLOW_TOOL_DENIED", "refused"},
		{"failed plain text", ToolResponse{Output: "boom", IsError: true, Status: "failed"}, "FLOW_TOOL_ERROR", "boom"},
		{"no status, IsError", ToolResponse{Output: "boom", IsError: true}, "FLOW_TOOL_ERROR", "boom"},
		{"error body, success status", ToolResponse{Output: `{"status":"error","message":"bad"}`, Status: "success"}, "FLOW_TOOL_ERROR", "bad"},
		{"denied body, no status", ToolResponse{Output: `{"status":"policy_denied","message":"blocked"}`}, "FLOW_TOOL_DENIED", "blocked"},
	}
	for _, tc := range cases {
		tc := tc
		tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) { return tc.resp, nil }}
		out, err := callTool(context.Background(), toolInput(tools), "x", nil)
		if err == nil {
			t.Fatalf("%s: want an error, got output %v", tc.name, out)
		}
		ne := asNodeError(err)
		if ne.Code != tc.code || ne.Message != tc.msg {
			t.Errorf("%s: got %s %q, want %s %q", tc.name, ne.Code, ne.Message, tc.code, tc.msg)
		}
		if out == nil {
			t.Errorf("%s: the parsed output must accompany the error", tc.name)
		}
	}
}

func TestCallToolRefusalKeepsBodyAndBoundsMessage(t *testing.T) {
	huge := strings.Repeat("ö", 100000)
	tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: `{"status":"error","message":"` + huge + `","code":7}`, IsError: true, Status: "denied"}, nil
	}}
	out, err := callTool(context.Background(), toolInput(tools), "x", nil)
	ne := asNodeError(err)
	if ne.Code != "FLOW_TOOL_DENIED" || out["code"] != 7.0 || out["status"] != "error" {
		t.Fatalf("out = %v, err = %v", out, err)
	}
	assertBoundedMessage(t, ne.Message)
}

func TestToolTextIsValidUTF8(t *testing.T) {
	got, ne := ParseToolOutput("plain \xff\xfe text")
	if ne != nil || got["text"] != "plain � text" {
		t.Fatalf("plain text = %q, %v", got["text"], ne)
	}
	got, _ = ParseToolOutput("Tool Output: <external_data>\nraw \xff here\n</external_data>")
	if s, _ := got["text"].(string); !utf8.ValidString(s) || !strings.Contains(s, "raw � here") {
		t.Fatalf("wrapped text = %q", got["text"])
	}
	got, _ = ParseToolOutput(strings.Repeat("\xff", 1000))
	if got["text"] != "�" {
		t.Fatalf("a run of invalid bytes becomes one replacement, got %q", got["text"])
	}
	_, ne = ParseToolOutput("{\"status\":\"error\",\"message\":\"bad \xff\xfe\"}")
	if ne == nil || !utf8.ValidString(ne.Message) {
		t.Fatalf("json message = %+v", ne)
	}

	// Through callTool: the plain-text failure path and the transport error path.
	tools := &fakeTools{respond: func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: "boom \xff\xfe" + strings.Repeat("x", 500), IsError: true}, nil
	}}
	_, err := callTool(context.Background(), toolInput(tools), "x", nil)
	ne = asNodeError(err)
	assertBoundedMessage(t, ne.Message)
	if _, merr := json.Marshal(map[string]any{"message": ne.Message}); merr != nil || !strings.HasPrefix(ne.Message, "boom �") {
		t.Fatalf("message = %q, marshal error = %v", truncateRunes(ne.Message, 20), merr)
	}
	_, err = callTool(context.Background(), toolInput(errorTools{errors.New("conn \xff failed")}), "x", nil)
	if ne := asNodeError(err); ne.Code != "FLOW_NODE_FAILED" || ne.Message != "conn � failed" {
		t.Fatalf("transport error = %+v", ne)
	}
}

func TestCallToolDefaultsEmptyInvokerErrorCode(t *testing.T) {
	for name, invokerErr := range map[string]error{
		"bare":    &NodeError{Message: "m"},
		"wrapped": fmt.Errorf("outer: %w", &NodeError{Message: "m"}),
	} {
		_, err := callTool(context.Background(), toolInput(errorTools{invokerErr}), "x", nil)
		ne := asNodeError(err)
		if ne.Code != "FLOW_NODE_FAILED" || ne.Message != "m" {
			t.Errorf("%s: got %q %q, want FLOW_NODE_FAILED \"m\"", name, ne.Code, ne.Message)
		}
	}
	// A code the invoker did set is kept.
	_, err := callTool(context.Background(), toolInput(errorTools{&NodeError{Code: "TOOL_BUSY", Message: "m"}}), "x", nil)
	if ne := asNodeError(err); ne.Code != "TOOL_BUSY" {
		t.Fatalf("code = %q", ne.Code)
	}
}

func TestCallToolPassesArgsThrough(t *testing.T) {
	var got []ToolRequest
	tools := &fakeTools{respond: func(req ToolRequest) (ToolResponse, error) {
		got = append(got, req)
		return ToolResponse{Output: "{}"}, nil
	}}
	if _, err := callTool(context.Background(), toolInput(tools), "x", nil); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"a": 1.0}
	if _, err := callTool(context.Background(), toolInput(tools), "x", args); err != nil {
		t.Fatal(err)
	}
	if got[0].Args == nil || len(got[0].Args) != 0 {
		t.Fatalf("nil args must reach the invoker as an empty map, got %#v", got[0].Args)
	}
	if reflect.ValueOf(got[1].Args).Pointer() != reflect.ValueOf(args).Pointer() {
		t.Fatal("non-nil args are passed on as they are, not cloned")
	}
}

func TestSharedFakesAreSafeForWorkers(t *testing.T) {
	tools := &fakeTools{}
	llm := &fakeLLM{responses: []LLMResponse{{Text: "ok"}}}
	const workers, perWorker = 8, 25
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				_, _ = tools.InvokeTool(context.Background(), ToolRequest{Tool: fmt.Sprintf("t%d", w)})
				_, _ = llm.Step(context.Background(), LLMRequest{Prompt: fmt.Sprintf("p%d", w)})
				_ = tools.allCalls()
				_ = tools.count()
				_ = llm.callCount()
				if llm.callCount() > 0 {
					_ = llm.callAt(0)
				}
				tools.setRespond(toolReply(`{"status":"success"}`))
			}
		}()
	}
	wg.Wait()
	calls := tools.allCalls()
	if len(calls) != workers*perWorker || tools.count() != len(calls) || llm.callCount() != workers*perWorker {
		t.Fatalf("tool calls = %d (count %d), llm calls = %d", len(calls), tools.count(), llm.callCount())
	}
	// allCalls hands out a copy.
	calls[0].Tool = "tampered"
	if tools.allCalls()[0].Tool == "tampered" {
		t.Fatal("allCalls must return a copy")
	}
	if got := llm.callAt(0).Prompt; !strings.HasPrefix(got, "p") {
		t.Fatalf("callAt(0) = %q", got)
	}
	// setRespond takes effect for later calls.
	tools.setRespond(toolReply(`{"status":"error","message":"late"}`))
	if _, err := callTool(context.Background(), toolInput(tools), "x", nil); asNodeError(err).Message != "late" {
		t.Fatalf("setRespond = %v", err)
	}
}

func TestFakeLLMCallAtPanicsOutOfRange(t *testing.T) {
	llm := &fakeLLM{}
	for _, i := range []int{-1, 0, 3} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("callAt(%d) on an empty fake must panic", i)
				}
			}()
			llm.callAt(i)
		}()
	}
}
