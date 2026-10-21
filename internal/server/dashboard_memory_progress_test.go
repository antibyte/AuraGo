package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/embeddings"
	"aurago/internal/memory"
)

type dashboardCorpusVectorDB struct {
	memory.VectorDB
	status   embeddings.CorpusStatus
	count    int
	disabled bool
}

func (v dashboardCorpusVectorDB) Count() int { return v.count }

func (v dashboardCorpusVectorDB) IsDisabled() bool { return v.disabled }

func (v dashboardCorpusVectorDB) CorpusStatus(context.Context) embeddings.CorpusStatus {
	return v.status
}

func TestHandleDashboardMemoryProgressMetricsAndCorpus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	for _, concept := range []string{"pending one", "pending two", "exhausted"} {
		if err := stm.EnqueuePendingMemoryWrite(memory.PendingMemoryWrite{Concept: concept, Content: "payload " + concept, Domain: "test"}, nil); err != nil {
			t.Fatalf("EnqueuePendingMemoryWrite(%q): %v", concept, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	if _, err := db.Exec(`UPDATE pending_memory_writes SET status='exhausted',attempts=6,terminal_at=CURRENT_TIMESTAMP WHERE concept='exhausted';
		INSERT INTO pending_memory_write_tombstones(fingerprint,source_session_hash,terminal_at) VALUES('opaque-fixture-digest','','2026-10-20T00:00:00Z');
		INSERT INTO archived_messages(session_id,role,content,original_timestamp,consolidated,consolidation_status,consolidation_retries)
		VALUES('metrics-session','user','failed archive',CURRENT_TIMESTAMP,0,'failed',3)`); err != nil {
		_ = db.Close()
		t.Fatalf("seed dashboard metrics: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}

	vector := dashboardCorpusVectorDB{count: 7, status: embeddings.CorpusStatus{
		Available: true, Documents: 12, Dimensions: []int{384, 768}, DatabaseBytes: 4096, WALBytes: 512, VectorPayloadBytes: 4608,
	}}
	rec := httptest.NewRecorder()
	handleDashboardMemory(&Server{ShortTermMem: stm, LongTermMem: vector}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard/memory", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode dashboard payload: %v", err)
	}
	for key, want := range map[string]int{"pending_memory_writes": 2, "exhausted_memory_writes": 2, "terminal_consolidation_failures": 1} {
		got, ok := body[key].(float64)
		if !ok || int(got) != want {
			t.Errorf("%s=%v, want %d", key, body[key], want)
		}
	}
	health, ok := body["memory_health"].(map[string]any)
	if !ok {
		t.Fatalf("memory_health has type %T", body["memory_health"])
	}
	corpus, ok := health["corpus"].(map[string]any)
	if !ok {
		t.Fatalf("memory_health.corpus has type %T", health["corpus"])
	}
	if corpus["available"] != true || corpus["documents"] != float64(12) || corpus["database_bytes"] != float64(4096) || corpus["wal_bytes"] != float64(512) || corpus["vector_payload_bytes"] != float64(4608) {
		t.Fatalf("unexpected corpus status payload: %+v", corpus)
	}
	if got := corpus["dimensions"].([]any); len(got) != 2 || got[0] != float64(384) || got[1] != float64(768) {
		t.Fatalf("corpus dimensions=%v, want [384 768]", corpus["dimensions"])
	}
}

func TestHandleDashboardMemoryReportsUnavailableMetricsAsNull(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close memory store: %v", err)
	}

	vector := dashboardCorpusVectorDB{status: embeddings.CorpusStatus{Dimensions: []int{}, Error: "corpus_status_unavailable"}}
	rec := httptest.NewRecorder()
	handleDashboardMemory(&Server{ShortTermMem: stm, LongTermMem: vector}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard/memory", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode dashboard payload: %v", err)
	}
	for _, key := range []string{"exhausted_memory_writes", "terminal_consolidation_failures"} {
		if body[key] != nil {
			t.Errorf("%s=%v, want JSON null when the metric is unavailable", key, body[key])
		}
	}
	health := body["memory_health"].(map[string]any)
	corpus := health["corpus"].(map[string]any)
	if corpus["available"] != false || corpus["error"] != "corpus_status_unavailable" {
		t.Fatalf("unavailable corpus payload=%+v", corpus)
	}
	if corpus["documents"] != float64(0) {
		t.Fatalf("unavailable corpus documents=%v, want zero-valued aggregate", corpus["documents"])
	}
}
