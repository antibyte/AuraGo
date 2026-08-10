package agent

import (
	"testing"

	"aurago/internal/prompts"
)

func TestTurnContextSnapshotInvalidatesOnlyMutatedCategory(t *testing.T) {
	tests := []struct {
		name        string
		call        ToolCall
		wantMemory  bool
		wantNotes   bool
		wantPlan    bool
		wantPlanner bool
	}{
		{name: "memory read", call: ToolCall{Action: "manage_memory", Operation: "search"}},
		{name: "profile read", call: ToolCall{Action: "manage_memory", Operation: "view_profile"}},
		{name: "memory mutation", call: ToolCall{Action: "manage_memory", Operation: "add"}, wantMemory: true},
		{name: "knowledge mutation", call: ToolCall{Action: "knowledge_graph", Params: map[string]interface{}{"operation": "upsert"}}, wantMemory: true},
		{name: "knowledge health read", call: ToolCall{Action: "knowledge_graph", Operation: "graph_health"}},
		{name: "knowledge export read", call: ToolCall{Action: "knowledge_graph", Operation: "export_jsonld"}},
		{name: "journal mutation alias", call: ToolCall{Action: "manage_journal", ActionType: "add"}, wantMemory: true},
		{name: "notes read", call: ToolCall{Action: "manage_notes", Operation: "list"}},
		{name: "notes mutation", call: ToolCall{Action: "manage_notes", Operation: "complete"}, wantNotes: true},
		{name: "todo mutation refreshes planner and kg", call: ToolCall{Action: "manage_todos", Operation: "add"}, wantMemory: true, wantPlanner: true},
		{name: "appointment mutation refreshes planner and kg", call: ToolCall{Action: "manage_appointments", Operation: "complete"}, wantMemory: true, wantPlanner: true},
		{name: "todo read", call: ToolCall{Action: "manage_todos", Operation: "get"}},
		{name: "memory reflection", call: ToolCall{Action: "memory_reflect"}, wantMemory: true, wantPlanner: true},
		{name: "memory archive", call: ToolCall{Action: "archive_memory"}, wantMemory: true},
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
			if state.turnSnapshot.MemoryDirty != tt.wantMemory || state.turnSnapshot.NotesDirty != tt.wantNotes || state.turnSnapshot.PlanDirty != tt.wantPlan || state.turnSnapshot.PlannerDirty != tt.wantPlanner {
				t.Fatalf("dirty flags = memory:%v notes:%v plan:%v planner:%v", state.turnSnapshot.MemoryDirty, state.turnSnapshot.NotesDirty, state.turnSnapshot.PlanDirty, state.turnSnapshot.PlannerDirty)
			}
		})
	}
}

func TestDirtyMemorySnapshotBypassesToolResultThrottle(t *testing.T) {
	if !shouldRefreshTurnMemory(true, "original intent", "original intent", 0, true) {
		t.Fatal("dirty snapshot did not force a refresh after a tool result")
	}
	if shouldRefreshTurnMemory(false, "original intent", "original intent", 0, true) {
		t.Fatal("clean snapshot bypassed normal tool-result throttle")
	}
}

func TestTurnGuidePreparationDoesNotConsumeCompactIneligibility(t *testing.T) {
	if got := classifyTurnGuidePreparation(false, "compact", nil); got != turnGuidesNotEligible {
		t.Fatalf("compact state = %v, want not eligible", got)
	}
	if got := classifyTurnGuidePreparation(false, "full", nil); got != turnGuidesSearchEligible {
		t.Fatalf("full state = %v, want search eligible", got)
	}
	if got := classifyTurnGuidePreparation(true, "full", nil); got != turnGuidesResolvedWithoutSearch {
		t.Fatalf("suppressed state = %v, want resolved without search", got)
	}
}

func TestTurnContextSnapshotRestoresPlannerCategory(t *testing.T) {
	snapshot := &turnContextSnapshot{}
	snapshot.capturePlanner("planner", "reminder")
	contextText, reminder, ok := snapshot.restorePlanner()
	if !ok || contextText != "planner" || reminder != "reminder" {
		t.Fatalf("planner restore = (%q, %q, %v)", contextText, reminder, ok)
	}
	snapshot.PlannerDirty = true
	if _, _, ok := snapshot.restorePlanner(); ok {
		t.Fatal("dirty planner snapshot was restored")
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
