package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func newPublicationAgentVector(t *testing.T, stm *memory.SQLiteMemory, onEmbed func()) *memory.ChromemVectorDB {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onEmbed != nil {
			onEmbed()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"index": 0, "embedding": []float32{1, 0, 0}}}})
	}))
	t.Cleanup(server.Close)
	cfg := &config.Config{}
	cfg.Directories.VectorDBDir = filepath.Join(t.TempDir(), "vectors")
	cfg.Embeddings.Provider, cfg.Embeddings.BaseURL, cfg.Embeddings.Model = "external", server.URL+"/v1", "fixture"
	v, err := memory.NewChromemVectorDB(cfg, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	v.BindMemoryMetadata(stm)
	t.Cleanup(func() { _ = v.Close() })
	deadline := time.Now().Add(5 * time.Second)
	for !v.IsReady() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !v.IsReady() || v.IsDisabled() {
		t.Fatal("fixture embedding pipeline unavailable")
	}
	return v
}

func TestProactiveSummaryPrivacyAndReadOnly(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		readOnly, deleteMemories bool
		remaining                int
	}{
		{"delete memories", false, true, 0}, {"keep memories", false, false, 1}, {"read only", true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stm, _ := groundingTestStore(t)
			v := newPublicationAgentVector(t, stm, nil)
			cfg := enabledMemoryTestConfig()
			cfg.Tools.Memory.ReadOnly = tc.readOnly
			run := RunConfig{Config: cfg, ShortTermMem: stm, LongTermMem: v, SessionID: "default", Logger: testLogger}
			if err := publishProactiveHistorySummary(t.Context(), run, "The user stores bicycle equipment in the garden shed."); err != nil {
				t.Fatal(err)
			}
			metas, err := stm.GetAllMemoryMeta(10, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.readOnly && (len(metas) != 1 || metas[0].SourceType != "chat" || metas[0].VerificationStatus != "unverified") {
				t.Fatalf("summary provenance=%+v", metas)
			}
			kg, err := memory.NewKnowledgeGraph(filepath.Join(t.TempDir(), "kg.db"), "", testLogger)
			if err != nil {
				t.Fatal(err)
			}
			defer kg.Close()
			job, err := stm.PrepareChatPrivacyDeletion(t.Context(), "default", v, kg, tc.deleteMemories)
			if err != nil {
				t.Fatal(err)
			}
			if err := stm.ProcessDuePrivacyDeletionJobs(t.Context(), v, kg, time.Now().UTC(), 10); err != nil {
				t.Fatal(err)
			}
			if tc.deleteMemories {
				status, err := stm.GetPrivacyDeletionJob(t.Context(), job.DeletionJobID)
				if err != nil || status.DeletionStatus != "completed" {
					t.Fatalf("privacy status=%+v err=%v", status, err)
				}
			}
			if v.Count() != tc.remaining {
				t.Fatalf("documents=%d want=%d", v.Count(), tc.remaining)
			}
			if err := publishProactiveHistorySummary(t.Context(), run, "The user keeps spare bicycle parts in Berlin."); !tc.readOnly && err == nil {
				t.Fatal("deleted session accepted a late summary")
			}
		})
	}
}

func TestProactiveSummaryRechecksPermissionAfterEmbedding(t *testing.T) {
	stm, _ := groundingTestStore(t)
	baseline := enabledMemoryTestConfig()
	blocked := *baseline
	blocked.Tools.Memory.ReadOnly = true
	var current atomic.Pointer[config.Config]
	current.Store(baseline)
	var revoke atomic.Bool
	v := newPublicationAgentVector(t, stm, func() {
		if revoke.Load() {
			current.Store(&blocked)
		}
	})
	cfg := *baseline
	cfg.AuthorizationSnapshots = func() (*config.Config, *config.Config) { return baseline, current.Load() }
	revoke.Store(true)
	err := publishProactiveHistorySummary(t.Context(), RunConfig{Config: &cfg, ShortTermMem: stm, LongTermMem: v, SessionID: "default"}, "The user keeps spare bicycle parts in the garden shed.")
	if err == nil || v.Count() != 0 {
		t.Fatalf("late policy change: count=%d err=%v", v.Count(), err)
	}
}

func TestMemoryExpiryFailureDoesNotPublishPermanentFact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(path, testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	v := newPublicationAgentVector(t, stm, nil)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER expiry_failure BEFORE INSERT ON memory_expiry BEGIN SELECT RAISE(ABORT, 'fixture expiry failure'); END`); err != nil {
		t.Fatal(err)
	}
	result := memoryAnalysisResult{Facts: []extractedFact{{Content: "I am on vacation until tomorrow", Category: "personal", Confidence: .95, Quote: "I am on vacation until tomorrow", ExpiresOn: time.Now().AddDate(0, 0, 1).Format("2006-01-02")}}, human: "I am on vacation until tomorrow"}
	if count := applyMemoryAnalysisResultWithContext(context.Background(), enabledMemoryTestConfig(), testLogger, stm, v, "expiry-session", result); count != 0 {
		t.Fatalf("failed expiry counted as %d successful writes", count)
	}
	texts, _, err := v.SearchMemoriesOnly("vacation", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 0 {
		t.Fatalf("incomplete publication visible: %d", len(texts))
	}
}

func TestProactiveSummaryConcurrentDeletionCancelsPublication(t *testing.T) {
	stm, _ := groundingTestStore(t)
	var block atomic.Bool
	entered, release := make(chan struct{}, 1), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	v := newPublicationAgentVector(t, stm, func() {
		if block.Load() {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
		}
	})
	t.Cleanup(unblock)
	block.Store(true)
	done := make(chan error, 1)
	go func() {
		done <- publishProactiveHistorySummary(t.Context(), RunConfig{Config: enabledMemoryTestConfig(), ShortTermMem: stm, LongTermMem: v, SessionID: "default"}, "The user stores spare bicycle parts in the garden shed.")
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("embedding did not start")
	}
	deleted := make(chan error, 1)
	go func() { _, err := stm.PrepareChatPrivacyDeletion(t.Context(), "default", v, nil, true); deleted <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("deletion allowed a late summary")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deletion did not cancel publication")
	}
	unblock()
	if err := <-deleted; err != nil {
		t.Fatal(err)
	}
	if v.Count() != 0 {
		t.Fatalf("late summary documents=%d", v.Count())
	}
	block.Store(false)
	fresh, err := stm.ResolveUserSession(t.Context(), "default")
	if err != nil || fresh == "default" {
		t.Fatalf("new generation=%q err=%v", fresh, err)
	}
	if err := publishProactiveHistorySummary(t.Context(), RunConfig{Config: enabledMemoryTestConfig(), ShortTermMem: stm, LongTermMem: v, SessionID: fresh}, "The user stores a bicycle pump in the garage."); err != nil {
		t.Fatal(err)
	}
	metas, err := stm.GetAllMemoryMeta(10, 0)
	if err != nil || len(metas) != 1 {
		t.Fatalf("new generation metadata=%v err=%v", metas, err)
	}
	sources, err := stm.GetMemoryExtractionSources(metas[0].DocID)
	if err != nil || len(sources) != 1 || sources[0].SessionID != fresh {
		t.Fatalf("source generation=%v err=%v", sources, err)
	}
}

func TestProactiveSummaryDisabledMemoryStillCompressesHistory(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "read only"}[readOnly], func(t *testing.T) {
			previous := historyCompressionCoordinator
			historyCompressionCoordinator = &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}
			t.Cleanup(func() { historyCompressionCoordinator = previous })
			stm, history := seedPersistentCompressionHistory(t, "default")
			v := newPublicationAgentVector(t, stm, nil)
			cfg := enabledMemoryTestConfig()
			cfg.Tools.Memory.Enabled, cfg.Tools.Memory.ReadOnly = readOnly, readOnly
			cfg.LLM.Model, cfg.Agent.MemoryCompressionCharLimit = "fixture", 100
			group := NewAsyncTaskGroup(t.Context())
			t.Cleanup(func() { _ = group.Shutdown(5 * time.Second) })
			run := RunConfig{Config: cfg, Logger: testLogger, LLMClient: &mockChatClient{response: "The user keeps bicycle equipment in the garden shed."}, ShortTermMem: stm, LongTermMem: v, HistoryManager: history, SessionID: "default", SideEffects: group}
			if !ScheduleProactiveHistoryCompression(run) {
				t.Fatal("compression did not start")
			}
			deadline := time.Now().Add(5 * time.Second)
			for history.GetSummary() == "" && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if history.GetSummary() == "" {
				t.Fatal("normal history compression was disabled")
			}
			if err := group.Shutdown(5 * time.Second); err != nil {
				t.Fatal(err)
			}
			if v.Count() != 0 {
				t.Fatalf("disabled memory stored %d summaries", v.Count())
			}
		})
	}
}
