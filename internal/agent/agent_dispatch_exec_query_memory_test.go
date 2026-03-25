package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type fakeVectorDB struct {
	searchSimilarCalled      bool
	searchMemoriesOnlyCalled bool
}

func (f *fakeVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, nil
}

func (f *fakeVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}

func (f *fakeVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}

func (f *fakeVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	f.searchSimilarCalled = true
	return []string{"[tool_guides] wrong hit"}, []string{"tool-1"}, nil
}

func (f *fakeVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	f.searchMemoriesOnlyCalled = true
	return []string{"Vincenzo memory hit"}, []string{"mem-1"}, nil
}

func (f *fakeVectorDB) GetByID(id string) (string, error) { return "", nil }
func (f *fakeVectorDB) DeleteDocument(id string) error    { return nil }
func (f *fakeVectorDB) Count() int                        { return 1 }
func (f *fakeVectorDB) IsDisabled() bool                  { return false }
func (f *fakeVectorDB) Close() error                      { return nil }

func TestDispatchExecQueryMemoryUsesMemoriesOnlyForVectorDB(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	logger := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	vdb := &fakeVectorDB{}

	out := dispatchExec(
		context.Background(),
		ToolCall{Action: "query_memory", Query: "Vincenzo", Sources: []string{"vector_db"}},
		cfg,
		logger,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		vdb,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		false,
		"",
		nil,
		"",
		nil,
		nil,
	)

	if !vdb.searchMemoriesOnlyCalled {
		t.Fatal("expected SearchMemoriesOnly to be called")
	}
	if vdb.searchSimilarCalled {
		t.Fatal("did not expect SearchSimilar to be called")
	}
	if !strings.Contains(out, "Vincenzo memory hit") {
		t.Fatalf("output = %q, want memory hit", out)
	}
	if strings.Contains(out, "tool_guides") {
		t.Fatalf("output = %q, did not expect tool guide hit", out)
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}
