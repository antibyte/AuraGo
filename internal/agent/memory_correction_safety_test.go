package agent

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"aurago/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestCorrectMemoryPreservesLDAPArguments(t *testing.T) {
	args := `{"operation":"update_user","dn":"cn=fixture,dc=example,dc=test","changes":{"description":["Updated"],"title":[]}}`
	tc := NativeToolCallToToolCall(openai.ToolCall{ID: "review-ldap", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "ldap", Arguments: args}}, nil)
	if tc.NativeArgsMalformed {
		t.Errorf("valid native LDAP update rejected: %s", tc.NativeArgsError)
	}
	var tagged ToolCall
	if err := json.Unmarshal([]byte(`{"action":"ldap",`+args[1:]), &tagged); err != nil {
		t.Errorf("valid tagged LDAP update rejected: %v", err)
	}
	for _, call := range []ToolCall{tc, tagged} {
		decoded := decodeLDAPArgs(call)
		if decoded.Operation != "update_user" || decoded.DN != "cn=fixture,dc=example,dc=test" || !reflect.DeepEqual(decoded.Changes["description"], []string{"Updated"}) {
			t.Fatalf("LDAP arguments changed: %+v", decoded)
		}
		if values, exists := decoded.Changes["title"]; !exists || len(values) != 0 {
			t.Fatal("LDAP attribute removal was lost")
		}
	}
}

func TestCorrectMemoryNativeAndTaggedArguments(t *testing.T) {
	_, dc, id := correctionAgentFixture(t)
	call := correctionTestCall(id)
	args, err := json.Marshal(memoryCorrectionArgs{Operation: call.Operation, Reason: call.Reason, Changes: call.CorrectionChanges, Evidence: call.CorrectionEvidence})
	if err != nil {
		t.Fatal(err)
	}
	native := NativeToolCallToToolCall(openai.ToolCall{ID: "correction-preview", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "correct_memory", Arguments: string(args)}}, nil)
	var tagged ToolCall
	if err = json.Unmarshal([]byte(`{"action":"correct_memory",`+string(args[1:])), &tagged); err != nil {
		t.Fatal(err)
	}
	for _, decoded := range []ToolCall{native, tagged} {
		p, status := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), decoded, dc))
		if status != "success" || p == nil || len(p.Items) != 1 || p.Items[0].Value != call.CorrectionChanges[0].Value {
			t.Fatal("correction arguments lost during wire decoding", p, status)
		}
	}
}

func TestCorrectMemoryArchivePreservesGraphThroughMaintenance(t *testing.T) {
	s, dc, id := correctionAgentFixture(t)
	kg, err := memory.NewKnowledgeGraph(filepath.Join(t.TempDir(), "graph.db"), "", s.currentLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	if err = SyncCoreMemoryToKnowledgeGraph(t.Context(), dc.ShortTermMem, kg, s.currentLogger); err != nil {
		t.Fatal(err)
	}
	if err = kg.AddNode("other", "Unrelated fact", nil); err != nil {
		t.Fatal(err)
	}
	if err = kg.AddEdge("core_fact_"+id, "other", "related_to", map[string]string{"source": "user"}); err != nil {
		t.Fatal(err)
	}
	c := &memory.MemoryCorrectionService{Memory: dc.ShortTermMem, Graph: kg, Access: memory.CorrectionAccess{MemoryWrite: true, GraphWrite: true, ProfileEnabled: true}}
	p, err := c.Preview(t.Context(), dc.SessionID, "turn", "User withdraws incorrect fact", []memory.MemoryCorrectionChange{{Reference: memory.MemoryReference{Store: "core", ID: id}, Operation: "archive"}}, []memory.CorrectionEvidence{{Reference: "user:current", Kind: "user", Quote: "Withdraw this fact", ContentHash: "fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, dc.SessionID); err != nil {
		t.Fatal(err)
	}
	if n, err := kg.GetNode("core_fact_" + id); err != nil || n == nil {
		t.Fatal("archive removed entity before maintenance", err)
	}
	if err = SyncCoreMemoryToKnowledgeGraph(t.Context(), dc.ShortTermMem, kg, s.currentLogger); err != nil {
		t.Fatal(err)
	}
	if n, err := kg.GetNode("core_fact_" + id); err != nil || n == nil {
		t.Errorf("maintenance deleted corrected entity and its unrelated relationship: %v", err)
	}
	edges, err := kg.GetAllEdges(100)
	if err != nil || len(edges) != 1 || edges[0].Target != "other" || edges[0].Relation != "related_to" {
		t.Fatal("maintenance removed a foreign relationship", edges, err)
	}
	next, err := c.Preview(t.Context(), dc.SessionID, "fresh-turn", "User supplies a new preference", []memory.MemoryCorrectionChange{{Reference: memory.MemoryReference{Store: "core", ID: id}, Operation: "replace", Value: "User prefers green tea"}}, []memory.CorrectionEvidence{{Reference: "user:current", Kind: "user", Quote: "I prefer green tea", ContentHash: "fresh-fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), next.ID, dc.SessionID); err != nil {
		t.Fatal(err)
	}
	if err = SyncCoreMemoryToKnowledgeGraph(t.Context(), dc.ShortTermMem, kg, s.currentLogger); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), next.ID, dc.SessionID); err != nil {
		t.Fatal("maintenance invalidated a completed correction", err)
	}
}
