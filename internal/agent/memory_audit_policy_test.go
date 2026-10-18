package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestCoAgentOperationAliasesCannotReachHooks(t *testing.T) {
	for _, session := range []string{"coagent-test", "a2a-task-test"} {
		for _, call := range []ToolCall{
			{Action: "manage_memory", ActionType: " ADD "},
			{Action: "core_memory", Params: map[string]interface{}{"operation": " RESET_PROFILE "}},
			{Action: "manage_knowledge", Params: map[string]interface{}{"operation": "add"}},
			{Action: "manage_memory"},
			{Action: "manage_knowledge"},
			{Action: "manage_missions"},
		} {
			dc := &DispatchContext{Cfg: &config.Config{}, SessionID: session, IsCoAgent: session == "coagent-test", ExecutionHooks: &ExecutionHooks{
				BeforeTool: func(context.Context, ToolCall) error { t.Fatal("denied operation reached hook"); return nil },
			}}
			if got := dispatchInner(context.Background(), call, dc); classifyLegacyToolResult(got) != ToolResultDenied {
				t.Fatalf("session=%s call=%+v: %s", session, call, got)
			}
		}
	}
	call := prepareToolCall(ToolCall{Action: "core_memory", ActionType: " READ "}, &DispatchContext{Cfg: &config.Config{}})
	if call.Operation != "read" || decodeCoreMemoryArgs(call).Operation != "read" || roleToolRestriction(call, &DispatchContext{IsCoAgent: true}) != "" {
		t.Fatalf("read alias disagrees between policy and execution: %+v", call)
	}
}

func TestActivityEntitiesDoNotReplaceCanonicalNodes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	if err := kg.AddNode("docker", "Canonical Docker", map[string]string{"source": "user"}); err != nil {
		t.Fatal(err)
	}
	syncActivityTurnToKnowledgeGraph(kg, 1, "2026-10-18", "session", "web", memory.ActivityDigest{Entities: []string{"Docker"}}, "runtime", logger)
	nodes, err := kg.GetAllNodes(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.ID == "docker" && (node.Label != "Canonical Docker" || node.Properties["source"] != "user") {
			t.Fatalf("canonical node changed: %+v", node)
		}
	}
}
