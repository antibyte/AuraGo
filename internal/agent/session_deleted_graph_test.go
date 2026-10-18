package agent

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestDeletedSessionCannotPublishNativeGraphMutation(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer kg.Close()
	if err := stm.PurgeChatSession("deleted"); err != nil {
		t.Fatal(err)
	}
	dc := &DispatchContext{Cfg: enabledMemoryTestConfig(), Logger: logger, ShortTermMem: stm, KG: kg, SessionID: "deleted"}
	out, handled := dispatchExec(context.Background(), ToolCall{Action: "manage_knowledge", Operation: "add_node", ID: "late", Label: "Late node"}, dc)
	if !handled || !strings.Contains(out, "unavailable") {
		t.Fatalf("late graph write accepted: %s", out)
	}
	if node, err := kg.GetNode("late"); err != nil || node != nil {
		t.Fatalf("late node recreated: %+v %v", node, err)
	}
	out, handled = dispatchExec(context.Background(), ToolCall{Action: "manage_knowledge", Operation: "get_node", ID: "late"}, dc)
	if !handled || strings.Contains(out, "session is unavailable") {
		t.Fatalf("read unexpectedly fenced: %s", out)
	}
}
