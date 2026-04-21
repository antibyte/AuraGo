package agent

import "testing"

func TestQueuePendingToolCallsUpdatesLoopState(t *testing.T) {
	state := &agentLoopState{
		pendingTCs: []ToolCall{
			{Action: "existing", NativeCallID: "call_existing"},
		},
	}
	existing := append([]ToolCall(nil), state.pendingTCs...)
	newCalls := []ToolCall{
		{Action: "second", NativeCallID: "call_second"},
		{Action: "third", NativeCallID: "call_third"},
	}

	queued := queuePendingToolCalls(state, existing, newCalls)

	if len(queued) != 3 {
		t.Fatalf("len(queued) = %d, want 3", len(queued))
	}
	if len(state.pendingTCs) != 3 {
		t.Fatalf("len(state.pendingTCs) = %d, want 3", len(state.pendingTCs))
	}
	if state.pendingTCs[1].NativeCallID != "call_second" {
		t.Fatalf("state.pendingTCs[1].NativeCallID = %q, want call_second", state.pendingTCs[1].NativeCallID)
	}
	if state.pendingTCs[2].NativeCallID != "call_third" {
		t.Fatalf("state.pendingTCs[2].NativeCallID = %q, want call_third", state.pendingTCs[2].NativeCallID)
	}
}

func TestQueuePendingToolCallsNoopForEmptyInput(t *testing.T) {
	state := &agentLoopState{
		pendingTCs: []ToolCall{
			{Action: "existing", NativeCallID: "call_existing"},
		},
	}
	existing := append([]ToolCall(nil), state.pendingTCs...)

	queued := queuePendingToolCalls(state, existing, nil)

	if len(queued) != 1 {
		t.Fatalf("len(queued) = %d, want 1", len(queued))
	}
	if len(state.pendingTCs) != 1 {
		t.Fatalf("len(state.pendingTCs) = %d, want 1", len(state.pendingTCs))
	}
	if state.pendingTCs[0].NativeCallID != "call_existing" {
		t.Fatalf("state.pendingTCs[0].NativeCallID = %q, want call_existing", state.pendingTCs[0].NativeCallID)
	}
}
