package agent

import (
	"aurago/internal/memory"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
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
