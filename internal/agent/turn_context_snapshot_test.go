package agent

import (
	"testing"

	"aurago/internal/prompts"
)

func TestTurnContextSnapshotInvalidatesOnlyMutatedCategory(t *testing.T) {
	tests := []struct {
		name       string
		call       ToolCall
		wantMemory bool
		wantNotes  bool
		wantPlan   bool
	}{
		{name: "memory read", call: ToolCall{Action: "manage_memory", Operation: "search"}},
		{name: "memory mutation", call: ToolCall{Action: "manage_memory", Operation: "add"}, wantMemory: true},
		{name: "knowledge mutation", call: ToolCall{Action: "knowledge_graph", Params: map[string]interface{}{"operation": "upsert"}}, wantMemory: true},
		{name: "journal mutation alias", call: ToolCall{Action: "manage_journal", ActionType: "add"}, wantMemory: true},
		{name: "notes read", call: ToolCall{Action: "manage_notes", Operation: "list"}},
		{name: "notes mutation", call: ToolCall{Action: "manage_notes", Operation: "complete"}, wantNotes: true},
		{name: "remember task", call: ToolCall{Action: "remember", Category: "task"}, wantNotes: true},
		{name: "remember auto", call: ToolCall{Action: "remember"}, wantMemory: true, wantNotes: true},
		{name: "plan read", call: ToolCall{Action: "manage_plan", Operation: "get"}},
		{name: "plan mutation", call: ToolCall{Action: "manage_plan", Operation: "update"}, wantPlan: true},
		{name: "todo payload", call: ToolCall{Action: "shell", Todo: "- [ ] verify"}, wantPlan: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &agentLoopState{turnSnapshot: &turnContextSnapshot{MemoryReady: true, NotesReady: true, PlanReady: true}}
			invalidateTurnSnapshotAfterTool(state, tt.call, false)
			if state.turnSnapshot.MemoryDirty != tt.wantMemory || state.turnSnapshot.NotesDirty != tt.wantNotes || state.turnSnapshot.PlanDirty != tt.wantPlan {
				t.Fatalf("dirty flags = memory:%v notes:%v plan:%v", state.turnSnapshot.MemoryDirty, state.turnSnapshot.NotesDirty, state.turnSnapshot.PlanDirty)
			}
		})
	}
}

func TestTurnContextSnapshotFailedMutationDoesNotInvalidate(t *testing.T) {
	state := &agentLoopState{turnSnapshot: &turnContextSnapshot{MemoryReady: true}}
	invalidateTurnSnapshotAfterTool(state, ToolCall{Action: "manage_memory", Operation: "add"}, true)
	if state.turnSnapshot.MemoryDirty {
		t.Fatal("failed mutation invalidated memory snapshot")
	}
}

func TestTurnContextSnapshotRestoresCapturedMemory(t *testing.T) {
	snapshot := &turnContextSnapshot{}
	flags := &prompts.ContextFlags{
		RetrievedMemories:              "retrieved",
		PredictedMemories:              "predicted",
		KnowledgeContext:               "knowledge",
		AvailableMemoryContextIndex:    "memory-index",
		AvailableKnowledgeContextIndex: "knowledge-index",
		RecentActivityOverview:         "activity",
	}
	snapshot.captureMemory(flags, map[string]string{"memory:1": "value"}, nil)
	flags.RetrievedMemories = "changed"
	candidates, _, ok := snapshot.restoreMemory(flags)
	if !ok || flags.RetrievedMemories != "retrieved" || candidates["memory:1"] != "value" {
		t.Fatalf("snapshot restore failed: ok=%v flags=%+v candidates=%v", ok, flags, candidates)
	}
	candidates["memory:1"] = "mutated"
	second, _, _ := snapshot.restoreMemory(flags)
	if second["memory:1"] != "value" {
		t.Fatal("restored candidate map aliases mutable caller state")
	}
}

func TestMergeTurnGuideSnapshotPrioritizesNewExplicitGuides(t *testing.T) {
	got := mergeTurnGuideSnapshot(
		[]string{"explicit guide", "shared guide"},
		[]string{"semantic guide", "shared guide"},
		3,
	)
	want := []string{"explicit guide", "shared guide", "semantic guide"}
	if len(got) != len(want) {
		t.Fatalf("merged guides = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merged guides = %v, want %v", got, want)
		}
	}
}
