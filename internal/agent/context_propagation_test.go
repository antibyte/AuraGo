package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

type dispatchContextMarker struct{}

func TestDispatchExecCheatsheetAbstractUsesDispatchContext(t *testing.T) {
	db, err := tools.InitCheatsheetDB(filepath.Join(t.TempDir(), "cheatsheets.db"))
	if err != nil {
		t.Fatalf("InitCheatsheetDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var abstractCtxErr error
	sawDispatchValue := false
	previous := generateCheatsheetAbstractFunc
	generateCheatsheetAbstractFunc = func(ctx context.Context, _ *config.Config, _ *slog.Logger, _, _ string) (string, error) {
		sawDispatchValue = ctx.Value(dispatchContextMarker{}) == "dispatch"
		abstractCtxErr = ctx.Err()
		return "", ctx.Err()
	}
	t.Cleanup(func() { generateCheatsheetAbstractFunc = previous })

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), dispatchContextMarker{}, "dispatch"))
	cancel()
	out, handled := dispatchExec(ctx, ToolCall{
		Action: "cheatsheet",
		Params: map[string]interface{}{"operation": "create", "name": "Cancelled abstract", "content": "body"},
	}, &DispatchContext{
		Cfg:          &config.Config{},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		CheatsheetDB: db,
		LongTermMem:  &fakeVectorDB{},
	})
	if !handled {
		t.Fatal("expected dispatchExec to handle cheatsheet")
	}
	if !sawDispatchValue {
		t.Fatal("abstract generation did not derive from the dispatch context")
	}
	if !errors.Is(abstractCtxErr, context.Canceled) {
		t.Fatalf("abstract context error = %v, want canceled", abstractCtxErr)
	}
	if !strings.Contains(out, `"status":"ok"`) {
		t.Fatalf("cheat sheet creation must still succeed without an abstract: %s", out)
	}
}

type ctxRecordingVectorDB struct {
	partialSearchVectorDB
	called bool
	ctxErr error
}

func (v *ctxRecordingVectorDB) SearchMemoriesOnlyScoredContext(ctx context.Context, _ string, _ int) ([]memory.SearchResult, error) {
	v.called = true
	v.ctxErr = ctx.Err()
	return nil, ctx.Err()
}

func (v *ctxRecordingVectorDB) SearchSimilarScoredContext(ctx context.Context, _ string, _ int, _ ...string) ([]memory.SearchResult, error) {
	return nil, ctx.Err()
}

func (v *ctxRecordingVectorDB) SearchToolGuidesContext(ctx context.Context, _ string, _ int) ([]string, error) {
	return nil, ctx.Err()
}

var _ memory.ContextScoredVectorDB = (*ctxRecordingVectorDB)(nil)

func TestBuildContextSnapshotPassesCallerContextToMemorySearch(t *testing.T) {
	run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "co_agent")
	defer cleanup()
	vdb := &ctxRecordingVectorDB{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = buildContextSnapshot(ctx, CoAgentRequest{Task: "Explain NAS backups"}, vdb, run.ShortTermMem)
	if !vdb.called {
		t.Fatal("co-agent snapshot did not search memory through the context-aware API")
	}
	if !errors.Is(vdb.ctxErr, context.Canceled) {
		t.Fatalf("memory search context error = %v, want canceled", vdb.ctxErr)
	}
}
