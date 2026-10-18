package server

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/memory"
)

func TestHandleMemoryConflictDismissKeepsBothFacts(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	for _, id := range []string{"a", "b"} {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	if err := stm.RegisterMemoryConflict("a", "b", "user|language", "german", "english", "review"); err != nil {
		t.Fatal(err)
	}
	items, _ := stm.GetOpenMemoryConflicts(1)
	s := &Server{ShortTermMem: stm}
	for _, item := range []struct {
		body   string
		status int
	}{
		{fmt.Sprintf(`{"conflict_id":%d,"action":"invalid"}`, items[0].ID), 400},
		{fmt.Sprintf(`{"conflict_id":%d,"action":"dismiss","winning_doc_id":"a"}`, items[0].ID), 400},
		{fmt.Sprintf(`{"conflict_id":%d,"action":"dismiss"}`, items[0].ID), 200},
		{fmt.Sprintf(`{"conflict_id":%d,"action":"dismiss"}`, items[0].ID), 409},
	} {
		rec := httptest.NewRecorder()
		handleMemoryConflictResolve(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/memory/conflicts/resolve", bytes.NewBufferString(item.body)))
		if rec.Code != item.status {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, id := range []string{"a", "b"} {
		meta, err := stm.GetMemoryMeta(id)
		if err != nil || memory.IsMemoryArchived(meta) {
			t.Fatalf("lost fact %s: %+v %v", id, meta, err)
		}
	}
}
