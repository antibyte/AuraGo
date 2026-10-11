package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

type correctionRetrievalVector struct {
	archiveFilterVectorDB
	documents map[string]string
}

func (v *correctionRetrievalVector) GetByID(id string) (string, error) {
	if text, ok := v.documents[id]; ok {
		return text, nil
	}
	return "", fmt.Errorf("missing test document %s", id)
}

func (v *correctionRetrievalVector) GetByIDFromCollection(id, _ string) (string, error) {
	return v.GetByID(id)
}

func (v *correctionRetrievalVector) SearchSimilar(query string, limit int, _ ...string) ([]string, []string, error) {
	return v.SearchMemoriesOnly(query, limit)
}

func (v *correctionRetrievalVector) StoreCorrectionDocument(_ context.Context, id, text string) error {
	v.documents[id] = text
	for query, hits := range v.byQuery {
		v.byQuery[query] = append(hits, memory.SearchResult{DocID: id, Text: text, Similarity: .99})
	}
	return nil
}

func TestCorrectMemoryFourStoresUseCurrentRetrieval(t *testing.T) {
	for _, operation := range []string{"replace", "archive"} {
		t.Run(operation, func(t *testing.T) {
			s, dc, id := correctionAgentFixture(t)
			const old = "User prefers black coffee"
			const updated = "User prefers green tea"
			const query = "User prefers"
			v := &correctionRetrievalVector{documents: map[string]string{"old-vector": old}, archiveFilterVectorDB: archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
				query: {{DocID: "old-vector", Text: old, Similarity: .99}},
				old:   {{DocID: "old-vector", Text: old, Similarity: .99}},
			}}}
			if err := dc.ShortTermMem.UpsertMemoryMetaWithDetails("old-vector", memory.MemoryMetaUpdate{VerificationStatus: "confirmed"}); err != nil {
				t.Fatal(err)
			}
			if err := dc.ShortTermMem.UpsertProfileEntry("prefs", "drink", old, "analysis"); err != nil {
				t.Fatal(err)
			}
			kg, err := memory.NewKnowledgeGraph(":memory:", "", dc.Logger)
			if err != nil {
				t.Fatal(err)
			}
			defer kg.Close()
			if err = kg.AddNode("core_fact_"+id, old, map[string]string{"source": "core_memory", "type": "concept", "content": old}); err != nil {
				t.Fatal(err)
			}
			dc.KG, dc.LongTermMem = kg, v
			dc.Cfg.Tools.KnowledgeGraph.Enabled = true
			call := correctionTestCall(id)
			call.CorrectionChanges[0].Operation = operation
			if operation == "archive" {
				call.CorrectionChanges[0].Value = ""
			}
			for _, ref := range []memory.MemoryReference{{Store: "profile", Category: "prefs", Key: "drink"}, {Store: "ltm", ID: "old-vector"}} {
				call.CorrectionChanges = append(call.CorrectionChanges, memory.MemoryCorrectionChange{Reference: ref, Operation: operation, Value: call.CorrectionChanges[0].Value})
			}
			p, status := decodeCorrectionTestResult(t, dispatchMemoryCorrection(t.Context(), call, dc))
			if p == nil || status != "success" || len(p.Items) != 4 {
				t.Fatal("four-store preview failed", p, status)
			}
			apply := ToolCall{Action: "correct_memory", Operation: "apply", ID: p.ID}
			out := dispatchMemoryCorrection(t.Context(), apply, dc)
			s.recordCorrectionEvidence(apply, out, true)
			p, status = decodeCorrectionTestResult(t, out)
			if status != "success" || p.Status != "applied" || !s.turnSnapshot.MemoryDirty {
				t.Fatal(out)
			}
			local, err := QueryMemoryForLocalAgent(query, 10, dc.ShortTermMem, v, kg, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(local)
			if strings.Contains(strings.ToLower(string(data)), "black coffee") {
				t.Fatal("normal/local search returned withdrawn fact", string(data))
			}
			if operation == "replace" {
				for _, source := range []string{"core_memory", "user_profile", "vector_db", "knowledge_graph"} {
					if !strings.Contains(string(data), `"source":"`+source+`"`) {
						t.Fatal("missing corrected source", source, string(data))
					}
				}
			}
			ranked, err := searchRankedMemoriesOnly(t.Context(), v, dc.ShortTermMem, query, 5, nil, time.Now())
			if err != nil || (operation == "archive" && len(ranked) != 0) || (operation == "replace" && (len(ranked) != 1 || ranked[0].text != updated)) {
				t.Fatal("Auto-RAG did not use current state", ranked, err)
			}
			recalled, err := RecallMemoryForLocalAgent("old-vector", dc.ShortTermMem, v)
			if err != nil || len(recalled["results"].([]interface{})) != 0 {
				t.Fatal("ID retrieval returned withdrawn fact", recalled, err)
			}
			if v.documents["old-vector"] != old {
				t.Fatal("historical vector lost")
			}
		})
	}
}

type interruptedCorrectionVector struct {
	*correctionRetrievalVector
}

func (v *interruptedCorrectionVector) StoreCorrectionDocument(ctx context.Context, id, text string) error {
	if err := v.correctionRetrievalVector.StoreCorrectionDocument(ctx, id, text); err != nil {
		return err
	}
	return fmt.Errorf("injected interruption after vector write")
}

func TestCorrectMemoryPendingReplacementRemainsInvisible(t *testing.T) {
	_, dc, _ := correctionAgentFixture(t)
	const old, updated, query = "User prefers black coffee", "User prefers green tea", "User prefers"
	v := &interruptedCorrectionVector{&correctionRetrievalVector{documents: map[string]string{"old-vector": old}, archiveFilterVectorDB: archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
		query: {{DocID: "old-vector", Text: old, Similarity: .99}},
		old:   {{DocID: "old-vector", Text: old, Similarity: .99}},
	}}}}
	if err := dc.ShortTermMem.UpsertMemoryMetaWithDetails("old-vector", memory.MemoryMetaUpdate{VerificationStatus: "confirmed"}); err != nil {
		t.Fatal(err)
	}
	c := &memory.MemoryCorrectionService{Memory: dc.ShortTermMem, Vector: v, Access: memory.CorrectionAccess{MemoryWrite: true, ProfileEnabled: true}}
	p, err := c.Preview(t.Context(), "chat", "turn", "Correct preference", []memory.MemoryCorrectionChange{{Reference: memory.MemoryReference{Store: "ltm", ID: "old-vector"}, Operation: "replace", Value: updated}}, []memory.CorrectionEvidence{{Reference: "user:current", Kind: "user", Quote: "I prefer green tea", ContentHash: "fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Apply(t.Context(), p.ID, "chat"); err == nil {
		t.Fatal("interruption was not injected")
	}
	replacement := p.Items[0].ReplacementID
	if err = dc.ShortTermMem.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: replacement, Action: "confirm"}, "admin", false); err == nil {
		t.Fatal("curation published an incomplete correction")
	}
	local, err := QueryMemoryForLocalAgent(query, 10, dc.ShortTermMem, v, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(local)
	if strings.Contains(string(data), updated) || !strings.Contains(string(data), old) {
		t.Fatal("normal/local retrieval exposed staged replacement or hid original", string(data))
	}
	ranked, err := searchRankedMemoriesOnly(t.Context(), v, dc.ShortTermMem, query, 5, nil, time.Now())
	if err != nil || len(ranked) != 1 || ranked[0].text != old {
		t.Fatal("Auto-RAG exposed staged replacement", ranked, err)
	}
	recalled, err := RecallMemoryForLocalAgent(replacement, dc.ShortTermMem, v)
	if err != nil || len(recalled["results"].([]interface{})) != 0 {
		t.Fatal("ID retrieval exposed staged replacement", recalled, err)
	}
}
