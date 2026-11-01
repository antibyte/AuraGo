package agent

import (
	"aurago/internal/memory"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestRecallMemoryHonorsCurrentArchiveAndMetadataFailures(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	vdb := &fakeVectorDB{documents: map[string]string{"active": "active content", "archived": "archived content", "legacy": "legacy content"}}
	for _, id := range []string{"active", "archived"} {
		if err := stm.EnsureMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "archived", Action: "archive"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"active", "legacy", "archived"} {
		out, err := RecallMemoryForLocalAgent(id, stm, vdb)
		if err != nil {
			t.Fatal(err)
		}
		results := out["results"].([]interface{})
		if id == "archived" {
			if len(results) != 0 {
				t.Fatalf("archived result=%v", out)
			}
		} else if len(results) != 1 {
			t.Fatalf("available result=%v", out)
		}
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "active", Action: "archive"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	raw, err := executeRecallMemory(ToolCall{ID: "active"}, "", stm, vdb)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, "Tool Output: ")), &out); err != nil {
		t.Fatal(err)
	}
	if len(out["results"].([]interface{})) != 0 {
		t.Fatalf("native stale archive lookup=%s", raw)
	}
	if _, err := RecallMemoryForLocalAgent("legacy", nil, vdb); err == nil {
		t.Fatal("nil metadata store allowed recall")
	}
	if err := stm.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := RecallMemoryForLocalAgent("legacy", stm, vdb); err == nil {
		t.Fatal("closed metadata store allowed recall")
	}
}

func TestExplicitRecallCountsOnlySuccessfulAccesses(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	vdb := &fakeVectorDB{documents: map[string]string{"active": "active content", "archived": "old", "empty": ""}}
	for _, id := range []string{"active", "archived", "empty", "missing"} {
		if err := stm.EnsureMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "archived", Action: "archive"}, "admin", false); err != nil {
		t.Fatal(err)
	}
	if _, err := executeRecallMemory(ToolCall{IDs: []string{"active", "active", "archived", "empty", "missing"}}, "native-session", stm, vdb); err != nil {
		t.Fatal(err)
	}
	if _, err := RecallMemoryForLocalAgent("active", stm, vdb); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"active", "archived", "empty", "missing"} {
		meta, err := stm.GetMemoryMeta(id)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if id == "active" {
			want = 2
		}
		if meta.AccessCount != want {
			t.Fatalf("%s access=%d want=%d", id, meta.AccessCount, want)
		}
	}
	stats, err := stm.GetMemoryUsageStats(7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.TopReused) != 1 || stats.TopReused[0].MemoryID != "active" || stats.TopReused[0].Count != 2 {
		t.Fatalf("usage=%+v", stats)
	}
}

func TestAgentSearchAndRecallHonorExpiryAndKeepProtection(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	vector := newPublicationAgentVector(t, stm, nil)

	ids := make(map[string]string, 3)
	for _, name := range []string{"expired", "protected", "keep-forever"} {
		stored, err := memory.StoreAutomaticMemoryDocumentWithContext(t.Context(), stm, vector,
			"[fixture] recall eligibility "+name,
			"durable marker: recall-eligibility-"+name,
			memory.MemoryMetaUpdate{VerificationStatus: "confirmed", SourceType: "user", SourceReliability: 1},
		)
		if err != nil {
			t.Fatalf("store %s fixture: %v", name, err)
		}
		if len(stored.CreatedIDs) != 1 {
			t.Fatalf("store %s IDs=%v, want one created document", name, stored.CreatedIDs)
		}
		ids[name] = stored.CreatedIDs[0]
	}
	if err := stm.SetMemoryExpiry(t.Context(), []string{ids["expired"], ids["protected"], ids["keep-forever"]}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMetaProtection(ids["protected"], true, false); err != nil {
		t.Fatal(err)
	}
	if err := stm.SetMemoryMetaProtection(ids["keep-forever"], false, true); err != nil {
		t.Fatal(err)
	}

	search, err := QueryMemoryForLocalAgent("recall eligibility", 10, stm, vector, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	searchResults, err := json.Marshal(search["results"])
	if err != nil {
		t.Fatal(err)
	}
	searchText := string(searchResults)
	for _, name := range []string{"protected", "keep-forever"} {
		if !strings.Contains(searchText, "recall-eligibility-"+name) {
			t.Errorf("search omitted %s memory: %s", name, searchText)
		}
	}
	if strings.Contains(searchText, "recall-eligibility-expired") {
		t.Fatalf("search exposed expired unprotected memory: %s", searchText)
	}

	recalled := make(map[string]bool, 3)
	missing := make(map[string]bool, 3)
	for _, name := range []string{"expired", "protected", "keep-forever"} {
		out, err := RecallMemoryForLocalAgent(ids[name], stm, vector)
		if err != nil {
			t.Fatalf("recall %s: %v", name, err)
		}
		for _, result := range out["results"].([]interface{}) {
			recalled[result.(map[string]interface{})["id"].(string)] = true
		}
		if ids, ok := out["missing"].([]interface{}); ok {
			for _, id := range ids {
				missing[id.(string)] = true
			}
		}
	}
	if recalled[ids["expired"]] || !missing[ids["expired"]] {
		t.Fatalf("expired unprotected recall was not missing: recalled=%v missing=%v", recalled, missing)
	}
	for _, name := range []string{"protected", "keep-forever"} {
		if !recalled[ids[name]] || missing[ids[name]] {
			t.Errorf("expired %s memory was not recalled: recalled=%v missing=%v", name, recalled, missing)
		}
	}

	for _, name := range []string{"expired", "protected", "keep-forever"} {
		meta, err := stm.GetMemoryMeta(ids[name])
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if name != "expired" {
			want = 1
		}
		if meta.AccessCount != want {
			t.Errorf("%s access count=%d want=%d", name, meta.AccessCount, want)
		}
	}
	stats, err := stm.GetMemoryUsageStats(7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.TopReused) != 2 {
		t.Fatalf("usage stats include wrong memories: %+v", stats.TopReused)
	}
	for _, item := range stats.TopReused {
		if item.MemoryID == ids["expired"] {
			t.Fatalf("expired memory was included in usage stats: %+v", stats.TopReused)
		}
	}
}
