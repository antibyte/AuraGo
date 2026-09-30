package agent

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"aurago/internal/memory"
)

func TestDeriveConflictSignalsDetectsDifferentLanguageClaims(t *testing.T) {
	signals := deriveConflictSignals("User prefers German")
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}
	if signals[0].Key != "user|preference" || signals[0].Value != "german" {
		t.Fatalf("unexpected signal: %+v", signals[0])
	}
}

func TestNormalizeConflictTextKeepsLegitimateBracketContent(t *testing.T) {
	input := `Alice prefers JSON ["home","lab"] backups`

	got := normalizeConflictText(input)

	if got != `Alice prefers JSON ["home","lab"] backups` {
		t.Fatalf("normalizeConflictText() = %q, want original content preserved", got)
	}
}

func TestNormalizeConflictTextStripsKnownSimilarityPrefix(t *testing.T) {
	input := `[Similarity: 0.87] Alice prefers rsync backups`

	got := normalizeConflictText(input)

	if got != "Alice prefers rsync backups" {
		t.Fatalf("normalizeConflictText() = %q, want prefix stripped", got)
	}
}

func TestConflictSignalsUnderstandStoredFactEnvelopes(t *testing.T) {
	for _, text := range []string{
		"User prefers Vim",
		"Editor preference\r\n\r\nUser prefers Vim",
		"[Similarity: 0.99] [aurago_memories] [Domain: ops] Editor preference\n\nUser prefers Vim",
		"[preference:workflow] User prefers Vim\n\nsource:memory_analysis session:default",
		"[arbitrary_category] User prefers Vim\n\nsource:memory_analysis session:default",
		"[correction:workflow] User prefers Vim",
		"User prefers Vim\nsource:memory_analysis session:default",
	} {
		signals := deriveConflictSignals(text)
		if len(signals) != 1 || signals[0].Key != "user|preference" || signals[0].Value != "vim" {
			t.Fatalf("text=%q signals=%v", text, signals)
		}
	}
	if got := normalizeConflictText("[custom] Alice prefers JSON [home,lab] backups"); got != "[custom] Alice prefers JSON [home,lab] backups" {
		t.Fatalf("legitimate brackets changed: %q", got)
	}
}

func TestMemoryConflictsCompareRawAndStoredFacts(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	for _, id := range []string{"old", "new"} {
		if err := stm.UpsertMemoryMeta(id); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vdb := &memorySafetyVector{archiveFilterVectorDB: archiveFilterVectorDB{byQuery: map[string][]memory.SearchResult{
		"user|preference": {{DocID: "old", Text: "Editor preference\n\nUser prefers Vim", Similarity: .99}},
	}}, stored: map[string]string{"new": "Editor preference\n\nUser prefers Emacs"}}
	if err := detectMemoryConflictsForDocIDsWithContext(context.Background(), logger, stm, vdb, []string{"new"}, "User prefers Emacs"); err != nil {
		t.Fatal(err)
	}
	conflicts, err := stm.GetOpenMemoryConflicts(10)
	if err != nil || len(conflicts) != 1 {
		t.Fatalf("conflicts=%v err=%v", conflicts, err)
	}
}

type cancellingConflictVector struct {
	memorySafetyVector
	cancel     context.CancelFunc
	queryCalls int
}

var _ memory.ContextScoredVectorDB = (*cancellingConflictVector)(nil)

func (v *cancellingConflictVector) SearchToolGuidesContext(ctx context.Context, _ string, _ int) ([]string, error) {
	return nil, ctx.Err()
}

func (v *cancellingConflictVector) SearchSimilarScoredContext(ctx context.Context, query string, topK int, _ ...string) ([]memory.SearchResult, error) {
	return v.SearchMemoriesOnlyScoredContext(ctx, query, topK)
}

func (v *cancellingConflictVector) SearchMemoriesOnlyScoredContext(ctx context.Context, _ string, _ int) ([]memory.SearchResult, error) {
	v.queryCalls++
	v.cancel()
	return nil, ctx.Err()
}

func TestMemoryConflictCheckHonorsCancellation(t *testing.T) {
	stm, _ := newMemorySafetyStore(t)
	if err := stm.UpsertMemoryMeta("new"); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	vdb := &cancellingConflictVector{cancel: cancel, memorySafetyVector: memorySafetyVector{reads: make(map[string]int), stored: map[string]string{"new": "User prefers Emacs"}}}
	if err := detectMemoryConflictsForDocIDsWithContext(ctx, logger, stm, vdb, []string{"new"}, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if vdb.queryCalls != 1 {
		t.Fatalf("query calls=%d", vdb.queryCalls)
	}
	if err := detectMemoryConflictsForDocIDsWithContext(ctx, logger, stm, vdb, []string{"new"}, ""); !errors.Is(err, context.Canceled) || vdb.reads["new"] != 1 {
		t.Fatalf("cancelled check read vector: err=%v reads=%v", err, vdb.reads)
	}
}
