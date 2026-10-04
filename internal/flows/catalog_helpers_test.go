package flows

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// fakeTools records tool calls and answers with respond (default: a success object).
type fakeTools struct {
	mu      sync.Mutex
	calls   []ToolRequest
	respond func(ToolRequest) (ToolResponse, error)
}

func (f *fakeTools) InvokeTool(_ context.Context, req ToolRequest) (ToolResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	respond := f.respond
	f.mu.Unlock()
	if respond == nil {
		return ToolResponse{Output: `Tool Output: {"status":"success"}`, Status: "success"}, nil
	}
	return respond(req)
}

func (f *fakeTools) last(t *testing.T) ToolRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		t.Fatal("no tool was called")
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeTools) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// allCalls returns a copy of the recorded calls. Use it, not f.calls, when the
// fake is called from other goroutines (the runner runs nodes on workers).
func (f *fakeTools) allCalls() []ToolRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ToolRequest(nil), f.calls...)
}

// setRespond replaces the responder under the lock, so a test can change it while
// workers are calling the fake. Assigning f.respond directly is only safe before
// the fake is shared.
func (f *fakeTools) setRespond(respond func(ToolRequest) (ToolResponse, error)) {
	f.mu.Lock()
	f.respond = respond
	f.mu.Unlock()
}

// toolReply answers every call with the given raw tool output.
func toolReply(output string) func(ToolRequest) (ToolResponse, error) {
	return func(ToolRequest) (ToolResponse, error) {
		return ToolResponse{Output: output, Status: "success"}, nil
	}
}

// fakeLLM returns the queued responses in order (the last one repeats).
type fakeLLM struct {
	mu        sync.Mutex
	calls     []LLMRequest
	responses []LLMResponse
	err       error
}

func (f *fakeLLM) Step(_ context.Context, req LLMRequest) (LLMResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, req)
	if f.err != nil {
		return LLMResponse{}, f.err
	}
	if len(f.responses) == 0 {
		return LLMResponse{}, nil
	}
	resp := f.responses[0]
	if len(f.responses) > 1 {
		f.responses = f.responses[1:]
	}
	return resp, nil
}

// callCount returns the number of recorded Step calls.
func (f *fakeLLM) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// callAt returns the i-th recorded Step call (0 is the first). It panics when
// there is no such call, which fails the test loudly instead of returning a zero
// request that could pass an assertion by accident.
func (f *fakeLLM) callAt(i int) LLMRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i < 0 || i >= len(f.calls) {
		panic(fmt.Sprintf("fakeLLM.callAt(%d): %d calls recorded", i, len(f.calls)))
	}
	return f.calls[i]
}

// execDef runs a definition like the engine: parameter defaults first, then Execute.
func execDef(def *NodeDef, params map[string]any, svc *Services, inputs ...NodeInput) (ExecResult, error) {
	if params == nil {
		params = map[string]any{}
	}
	if svc == nil {
		svc = &Services{}
	}
	node := &Node{ID: testNodeID(1), Key: "node", Type: def.Type, Params: params}
	return def.Execute(context.Background(), ExecInput{
		Node: node, Params: withDefaults(def, params), Inputs: inputs, Services: svc,
		Run: RunInfo{ID: "run_test", FlowID: "flow_test", Mode: ModeTest},
	})
}

func lookupDef(t *testing.T, reg *Registry, typ string) *NodeDef {
	t.Helper()
	def, ok := reg.Lookup(typ)
	if !ok {
		t.Fatalf("%s is not registered", typ)
	}
	return def
}
