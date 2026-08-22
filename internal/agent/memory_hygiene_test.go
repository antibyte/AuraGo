package agent

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"

	_ "modernc.org/sqlite"
)

func TestBuildMemoryReflectionReviewIssueTriggersOnActionableCuratorCounts(t *testing.T) {
	issue, ok := buildMemoryReflectionReviewIssue("recent", memory.MemoryCuratorDryRun{
		StaleCandidates:     30,
		VerificationBacklog: 75,
		Contradictions:      1,
	})
	if !ok {
		t.Fatal("expected memory reflection review issue")
	}
	if issue.Fingerprint != "memory_reflect|recent|curator_review" {
		t.Fatalf("fingerprint = %q, want stable memory reflection fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "contradictions=1") || !strings.Contains(issue.Detail, "verification_backlog=75") {
		t.Fatalf("issue detail = %q, want curator counts", issue.Detail)
	}
}

func TestBuildMemoryReflectionReviewIssueSkipsNoise(t *testing.T) {
	if _, ok := buildMemoryReflectionReviewIssue("recent", memory.MemoryCuratorDryRun{StaleCandidates: 2}); ok {
		t.Fatal("unexpected issue for low curator noise")
	}
}

func TestBuildKnowledgeGraphSparseIssueRequiresCoreFacts(t *testing.T) {
	if _, ok := buildKnowledgeGraphSparseIssue(nil, 0, 0); ok {
		t.Fatal("unexpected issue without core facts")
	}
	issue, ok := buildKnowledgeGraphSparseIssue([]string{"User: Andi", "Agent: Nova"}, 1, 0)
	if !ok {
		t.Fatal("expected sparse KG issue with core facts")
	}
	if issue.Fingerprint != "memory_maintenance|kg_sparse|core_facts_present" {
		t.Fatalf("fingerprint = %q, want stable sparse KG fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "core_facts=2") || !strings.Contains(issue.Detail, "nodes=1") {
		t.Fatalf("issue detail = %q, want KG counts", issue.Detail)
	}
}

func TestBuildKnowledgeGraphDuplicateIssue(t *testing.T) {
	if _, ok := buildKnowledgeGraphDuplicateIssue(&memory.KnowledgeGraphQualityReport{DuplicateGroups: 2, IDDuplicateGroups: 2}); ok {
		t.Fatal("unexpected duplicate issue below threshold")
	}
	if _, ok := buildKnowledgeGraphDuplicateIssue(&memory.KnowledgeGraphQualityReport{
		DuplicateGroups: 4,
		DuplicateNodes:  9,
		DuplicateCandidates: []memory.KnowledgeGraphDuplicateCandidate{
			{Label: "NAS", IDs: []string{"nas_a", "nas_b", "nas_c"}},
		},
	}); ok {
		t.Fatal("same-label review candidates must not create an operational issue")
	}

	idIssue, ok := buildKnowledgeGraphDuplicateIssue(&memory.KnowledgeGraphQualityReport{
		DuplicateGroups:   12,
		DuplicateNodes:    25,
		IDDuplicateGroups: 4,
		IDDuplicateNodes:  8,
		IDDuplicateCandidates: []memory.KnowledgeGraphDuplicateCandidate{
			{Label: "TrueNAS", IDs: []string{"truenas", "true_nas"}},
		},
	})
	if !ok {
		t.Fatal("expected duplicate issue for id duplicate groups above threshold")
	}
	if idIssue.Fingerprint != "maintenance|knowledge_graph|duplicates" {
		t.Fatalf("fingerprint = %q, want duplicates fingerprint", idIssue.Fingerprint)
	}
	if idIssue.Title != "Knowledge graph ID duplicates detected" {
		t.Fatalf("title = %q, want explicit ID duplicate title", idIssue.Title)
	}
	if !strings.Contains(idIssue.Detail, "id_duplicate_groups=4") || !strings.Contains(idIssue.Detail, "same-label review candidates") || !strings.Contains(idIssue.Detail, "truenas") {
		t.Fatalf("id issue detail = %q, want id duplicate counts and sample IDs", idIssue.Detail)
	}
}

func TestRecordKnowledgeGraphQualityIssuesLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	plannerDB, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer plannerDB.Close()

	kg, err := memory.NewKnowledgeGraph(filepath.Join(t.TempDir(), "knowledge_graph.db"), "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	pairs := [][2]string{
		{"alpha_service", "alphaservice"},
		{"beta_tool", "betatool"},
		{"gamma_agent", "gammaagent"},
		{"delta_server", "deltaserver"},
	}
	for i, pair := range pairs {
		properties := map[string]string{"type": "service", "source": fmt.Sprintf("pair-%d", i)}
		if err := kg.AddNode(pair[0], fmt.Sprintf("Canonical %d", i), properties); err != nil {
			t.Fatalf("AddNode target %s: %v", pair[0], err)
		}
		if err := kg.AddNode(pair[1], fmt.Sprintf("Variant %d", i), properties); err != nil {
			t.Fatalf("AddNode source %s: %v", pair[1], err)
		}
	}

	assertIssueStatus := func(wantStatus string, wantOccurrences int) {
		t.Helper()
		page, err := planner.ListOperationalIssues(plannerDB, planner.OperationalIssueListFilter{Status: "all", Limit: 20})
		if err != nil {
			t.Fatalf("ListOperationalIssues: %v", err)
		}
		for _, issue := range page.Items {
			if issue.Fingerprint == "maintenance|knowledge_graph|duplicates" {
				if issue.Status != wantStatus || issue.Occurrences != wantOccurrences {
					t.Fatalf("issue status/occurrences = %s/%d, want %s/%d", issue.Status, issue.Occurrences, wantStatus, wantOccurrences)
				}
				return
			}
		}
		t.Fatal("knowledge graph duplicate issue not found")
	}

	recordKnowledgeGraphQualityIssues(plannerDB, kg, logger)
	assertIssueStatus("open", 1)

	for _, pair := range pairs {
		if err := kg.MergeNodes(pair[0], pair[1]); err != nil {
			t.Fatalf("MergeNodes %s <- %s: %v", pair[0], pair[1], err)
		}
	}
	recordKnowledgeGraphQualityIssues(plannerDB, kg, logger)
	assertIssueStatus("done", 1)

	for i, pair := range pairs {
		if err := kg.AddNode(pair[1], fmt.Sprintf("Reopened Variant %d", i), map[string]string{"type": "service", "source": fmt.Sprintf("pair-%d", i)}); err != nil {
			t.Fatalf("re-add source %s: %v", pair[1], err)
		}
	}
	recordKnowledgeGraphQualityIssues(plannerDB, kg, logger)
	assertIssueStatus("open", 2)
}

func TestKGDroppedAccessHitsBaselinePersists(t *testing.T) {
	kgDroppedAccessHitsStateMu.Lock()
	lastRecordedKGDroppedAccessHits = 0
	kgDroppedAccessHitsStateLoaded = false
	kgDroppedAccessHitsStateMu.Unlock()

	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	defer db.Close()

	if err := planner.SetPlannerMeta(db, plannerMetaKGDroppedAccessHitsKey, "42"); err != nil {
		t.Fatalf("SetPlannerMeta: %v", err)
	}

	ensureKGDroppedAccessHitsBaseline(db, nil)

	kgDroppedAccessHitsStateMu.Lock()
	got := lastRecordedKGDroppedAccessHits
	kgDroppedAccessHitsStateMu.Unlock()
	if got != 42 {
		t.Fatalf("baseline = %d, want 42", got)
	}
}

func TestBuildKnowledgeGraphDroppedAccessHitsIssue(t *testing.T) {
	if _, ok := buildKnowledgeGraphDroppedAccessHitsIssue(0); ok {
		t.Fatal("unexpected dropped access issue for zero delta")
	}
	issue, ok := buildKnowledgeGraphDroppedAccessHitsIssue(12)
	if !ok {
		t.Fatal("expected dropped access issue for positive delta")
	}
	if issue.Fingerprint != "maintenance|knowledge_graph|dropped_access_hits" {
		t.Fatalf("fingerprint = %q, want dropped access hits fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "Dropped 12") {
		t.Fatalf("issue detail = %q, want dropped delta", issue.Detail)
	}
}

func TestBuildKnowledgeGraphSemanticReindexBacklogIssue(t *testing.T) {
	if _, ok := buildKnowledgeGraphSemanticReindexBacklogIssue(100, 100); ok {
		t.Fatal("unexpected backlog issue below threshold")
	}
	issue, ok := buildKnowledgeGraphSemanticReindexBacklogIssue(5001, 12)
	if !ok {
		t.Fatal("expected backlog issue when dirty nodes exceed batch size")
	}
	if issue.Fingerprint != "maintenance|knowledge_graph|semantic_reindex_backlog" {
		t.Fatalf("fingerprint = %q, want semantic reindex backlog fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "dirty_nodes=5001") {
		t.Fatalf("issue detail = %q, want dirty node count", issue.Detail)
	}
}

func TestBuildCoreMemoryReviewIssueFlagsTestFacts(t *testing.T) {
	issue, ok := buildCoreMemoryReviewIssue([]string{"This is a test fact", "User: Andi"})
	if !ok {
		t.Fatal("expected core memory review issue for test fact")
	}
	if issue.Fingerprint != "memory_maintenance|core_memory_review|low_signal" {
		t.Fatalf("fingerprint = %q, want stable core memory review fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "test fact") {
		t.Fatalf("issue detail = %q, want test fact detail", issue.Detail)
	}
}

func TestBuildCoreMemoryReviewIssueUsesCoreMemoryPolicy(t *testing.T) {
	issue, ok := buildCoreMemoryReviewIssue([]string{
		"KI-News Aktualisierung am 2026-06-13: 25 Artikel mit Quellen.",
		"User prefers German responses.",
	})
	if !ok {
		t.Fatal("expected core memory review issue for operational core-memory junk")
	}
	if issue.Fingerprint != "memory_maintenance|core_memory_review|low_signal" {
		t.Fatalf("fingerprint = %q, want stable core memory review fingerprint", issue.Fingerprint)
	}
	if !strings.Contains(issue.Detail, "KI-News") {
		t.Fatalf("issue detail = %q, want operational fact detail", issue.Detail)
	}
	if strings.Contains(issue.Detail, "User prefers German responses") {
		t.Fatalf("issue detail = %q, should not include durable fact", issue.Detail)
	}
}

func TestRunAutomaticMemoryHygieneLimitsNoteAutoArchivePerRun(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stm.db")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}

	for i := 0; i < maxNotesAutoArchivePerHygieneRun+5; i++ {
		if _, err := stm.AddNote("general", fmt.Sprintf("old note %d", i), "stale", 1, ""); err != nil {
			t.Fatalf("AddNote %d: %v", i, err)
		}
	}
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = rawDB.Close() })
	old := time.Now().UTC().Add(-120 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := rawDB.Exec(`UPDATE notes SET created_at = ?, updated_at = ?`, old, old); err != nil {
		t.Fatalf("backdate notes: %v", err)
	}

	cfg := &config.Config{}
	cfg.Tools.Notes.Enabled = true
	stats := runAutomaticMemoryHygiene(cfg, logger, stm, nil)
	if stats.NotesArchived != maxNotesAutoArchivePerHygieneRun {
		t.Fatalf("NotesArchived = %d, want cap %d", stats.NotesArchived, maxNotesAutoArchivePerHygieneRun)
	}
}
