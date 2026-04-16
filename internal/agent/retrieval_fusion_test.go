package agent

import (
	"aurago/internal/memory"
	"log/slog"
	"testing"
)

// ---------------------------------------------------------------------------
// containsString tests
// ---------------------------------------------------------------------------

func TestContainsString_Found(t *testing.T) {
	slice := []string{"alpha", "beta", "gamma"}
	if !containsString(slice, "beta") {
		t.Error("expected true for existing element")
	}
}

func TestContainsString_NotFound(t *testing.T) {
	slice := []string{"alpha", "beta", "gamma"}
	if containsString(slice, "delta") {
		t.Error("expected false for missing element")
	}
}

func TestContainsString_EmptySlice(t *testing.T) {
	if containsString(nil, "anything") {
		t.Error("expected false for nil slice")
	}
	if containsString([]string{}, "anything") {
		t.Error("expected false for empty slice")
	}
}

func TestContainsString_EmptyString(t *testing.T) {
	slice := []string{"", "alpha"}
	if !containsString(slice, "") {
		t.Error("expected true for empty string in slice")
	}
}

// ---------------------------------------------------------------------------
// truncateUTF8SafeAgent tests
// ---------------------------------------------------------------------------

func TestTruncateUTF8SafeAgent_ShortEnough(t *testing.T) {
	s := "hello"
	got := truncateUTF8SafeAgent(s, 10)
	if got != s {
		t.Errorf("truncateUTF8SafeAgent(%q, 10) = %q, want %q", s, got, s)
	}
}

func TestTruncateUTF8SafeAgent_ExactLength(t *testing.T) {
	s := "hello"
	got := truncateUTF8SafeAgent(s, 5)
	if got != s {
		t.Errorf("truncateUTF8SafeAgent(%q, 5) = %q, want %q", s, got, s)
	}
}

func TestTruncateUTF8SafeAgent_TruncateNoNewlines(t *testing.T) {
	s := "hello world"
	got := truncateUTF8SafeAgent(s, 5)
	if got != "hello" {
		t.Errorf("truncateUTF8SafeAgent(%q, 5) = %q, want %q", s, got, "hello")
	}
}

func TestTruncateUTF8SafeAgent_TruncateAtNewline(t *testing.T) {
	// 17 runes total; truncate to 8 → "line1\nli", break at last \n → "line1"
	s := "line1\nline2\nline3"
	got := truncateUTF8SafeAgent(s, 8)
	want := "line1"
	if got != want {
		t.Errorf("truncateUTF8SafeAgent(%q, 8) = %q, want %q", s, got, want)
	}
}

func TestTruncateUTF8SafeAgent_TruncateExactNoBreak(t *testing.T) {
	// When the truncated portion ends exactly at a line boundary (no \n in truncated part),
	// the function returns the full truncated string.
	s := "line1\nline2\nline3"
	got := truncateUTF8SafeAgent(s, 12)
	want := "line1\nline2"
	if got != want {
		t.Errorf("truncateUTF8SafeAgent(%q, 12) = %q, want %q", s, got, want)
	}
}

func TestTruncateUTF8SafeAgent_MultibyteUTF8(t *testing.T) {
	s := "Hällö Wörld 日本語テスト"
	got := truncateUTF8SafeAgent(s, 7)
	want := "Hällö W" // 7 runes
	if got != want {
		t.Errorf("truncateUTF8SafeAgent(%q, 7) = %q, want %q", s, got, want)
	}
}

func TestTruncateUTF8SafeAgent_EmptyString(t *testing.T) {
	got := truncateUTF8SafeAgent("", 5)
	if got != "" {
		t.Errorf("truncateUTF8SafeAgent(\"\", 5) = %q, want empty", got)
	}
}

func TestTruncateUTF8SafeAgent_ZeroMaxLen(t *testing.T) {
	got := truncateUTF8SafeAgent("hello", 0)
	if got != "" {
		t.Errorf("truncateUTF8SafeAgent(\"hello\", 0) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// extractKGEntityLabels tests
// ---------------------------------------------------------------------------

func TestExtractKGEntityLabels_BasicEntity(t *testing.T) {
	kgContext := "- [truenas] TrueNAS Server | type: device | ip: 192.168.1.5\n"
	labels := extractKGEntityLabels(kgContext, 3)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d: %v", len(labels), labels)
	}
	if labels[0] != "TrueNAS Server" {
		t.Errorf("label = %q, want %q", labels[0], "TrueNAS Server")
	}
}

func TestExtractKGEntityLabels_MultipleEntities(t *testing.T) {
	kgContext := `- [adguard] AdGuard | type: software
- [truenas] TrueNAS Server | type: device
- [john] John Doe | type: person
`
	labels := extractKGEntityLabels(kgContext, 3)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d: %v", len(labels), labels)
	}
	want := []string{"AdGuard", "TrueNAS Server", "John Doe"}
	for i, w := range want {
		if labels[i] != w {
			t.Errorf("labels[%d] = %q, want %q", i, labels[i], w)
		}
	}
}

func TestExtractKGEntityLabels_MaxLabels(t *testing.T) {
	kgContext := `- [a] Alpha | type: x
- [b] Beta | type: x
- [c] Charlie | type: x
- [d] Delta | type: x
`
	labels := extractKGEntityLabels(kgContext, 2)
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels (maxLabels=2), got %d", len(labels))
	}
}

func TestExtractKGEntityLabels_SkipIndentedEdgeLines(t *testing.T) {
	kgContext := "- [truenas] TrueNAS Server | type: device\n  - [adguard] -[runs_on]-> [truenas]\n"
	labels := extractKGEntityLabels(kgContext, 5)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label (indented edge line skipped), got %d: %v", len(labels), labels)
	}
	if labels[0] != "TrueNAS Server" {
		t.Errorf("label = %q, want %q", labels[0], "TrueNAS Server")
	}
}

func TestExtractKGEntityLabels_SkipUnknown(t *testing.T) {
	kgContext := "- [x] Unknown | type: x\n- [y] Valid Label | type: x\n"
	labels := extractKGEntityLabels(kgContext, 5)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label (Unknown skipped), got %d: %v", len(labels), labels)
	}
	if labels[0] != "Valid Label" {
		t.Errorf("label = %q, want %q", labels[0], "Valid Label")
	}
}

func TestExtractKGEntityLabels_SkipShortLabels(t *testing.T) {
	kgContext := "- [x] A | type: x\n- [y] Good Label | type: x\n"
	labels := extractKGEntityLabels(kgContext, 5)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label (single-char skipped), got %d: %v", len(labels), labels)
	}
	if labels[0] != "Good Label" {
		t.Errorf("label = %q, want %q", labels[0], "Good Label")
	}
}

func TestExtractKGEntityLabels_EmptyContext(t *testing.T) {
	labels := extractKGEntityLabels("", 5)
	if len(labels) != 0 {
		t.Fatalf("expected 0 labels for empty context, got %d", len(labels))
	}
}

func TestExtractKGEntityLabels_NoMatchingLines(t *testing.T) {
	kgContext := "some random text\nwithout entity lines\n"
	labels := extractKGEntityLabels(kgContext, 5)
	if len(labels) != 0 {
		t.Fatalf("expected 0 labels, got %d: %v", len(labels), labels)
	}
}

func TestExtractKGEntityLabels_TabIndentedSkipped(t *testing.T) {
	kgContext := "- [a] Alpha | type: x\n\t- [b] Beta | type: x\n"
	labels := extractKGEntityLabels(kgContext, 5)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label (tab-indented skipped), got %d: %v", len(labels), labels)
	}
	if labels[0] != "Alpha" {
		t.Errorf("label = %q, want %q", labels[0], "Alpha")
	}
}

// ---------------------------------------------------------------------------
// applyRetrievalFusion tests
// ---------------------------------------------------------------------------

func TestApplyRetrievalFusion_NoInputs(t *testing.T) {
	logger := slog.Default()
	result := applyRetrievalFusion(nil, "", nil, nil, logger)
	if result.EnrichedMemories != "" || result.EnrichedKGContext != "" {
		t.Error("expected empty result when no inputs provided")
	}
}

func TestApplyRetrievalFusion_RAGOnly_NoKG(t *testing.T) {
	logger := slog.Default()
	topMemories := []string{"memory one about servers", "memory two about networking"}
	result := applyRetrievalFusion(topMemories, "", nil, nil, logger)
	// Without KG, Direction 1 (KG→RAG) is skipped.
	// Without kg, Direction 2 (RAG→KG) is also skipped.
	if result.EnrichedMemories != "" {
		t.Error("expected no enriched memories without KG context")
	}
	if result.EnrichedKGContext != "" {
		t.Error("expected no enriched KG context without KnowledgeGraph")
	}
}

func TestApplyRetrievalFusion_KGOnly_NoVectorDB(t *testing.T) {
	logger := slog.Default()
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	result := applyRetrievalFusion(nil, kgContext, nil, nil, logger)
	// Without VectorDB, Direction 1 (KG→RAG) is skipped.
	// Without RAG, Direction 2 (RAG→KG) is skipped.
	if result.EnrichedMemories != "" {
		t.Error("expected no enriched memories without VectorDB")
	}
	if result.EnrichedKGContext != "" {
		t.Error("expected no enriched KG context without RAG memories")
	}
}

func TestApplyRetrievalFusion_WithBoth_NoImplementations(t *testing.T) {
	logger := slog.Default()
	topMemories := []string{"memory about something important"}
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	// Both subsystems report having data, but nil implementations mean
	// the actual lookups are skipped gracefully.
	result := applyRetrievalFusion(topMemories, kgContext, nil, nil, logger)
	// Direction 1 skipped: longTermMem is nil
	// Direction 2 skipped: kg is nil
	if result.EnrichedMemories != "" {
		t.Error("expected no enriched memories with nil VectorDB")
	}
	if result.EnrichedKGContext != "" {
		t.Error("expected no enriched KG context with nil KnowledgeGraph")
	}
}

func TestApplyRetrievalFusion_DisabledVectorDB(t *testing.T) {
	logger := slog.Default()
	topMemories := []string{"memory text"}
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	result := applyRetrievalFusion(topMemories, kgContext, &disabledVectorDB{}, nil, logger)
	// Direction 1 skipped: VectorDB is disabled
	if result.EnrichedMemories != "" {
		t.Error("expected no enriched memories with disabled VectorDB")
	}
}

// disabledVectorDB is a minimal mock that reports itself as disabled.
type disabledVectorDB struct{}

func (d *disabledVectorDB) IsDisabled() bool                            { return true }
func (d *disabledVectorDB) StoreDocument(_, _ string) ([]string, error) { return nil, nil }
func (d *disabledVectorDB) StoreDocumentWithEmbedding(_ string, _ string, _ []float32) (string, error) {
	return "", nil
}
func (d *disabledVectorDB) StoreDocumentInCollection(_, _, _ string) ([]string, error) {
	return nil, nil
}
func (d *disabledVectorDB) StoreDocumentWithEmbeddingInCollection(_ string, _ string, _ []float32, _ string) (string, error) {
	return "", nil
}
func (d *disabledVectorDB) StoreBatch(_ []memory.ArchiveItem) ([]string, error) { return nil, nil }
func (d *disabledVectorDB) SearchSimilar(_ string, _ int, _ ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (d *disabledVectorDB) SearchMemoriesOnly(_ string, _ int) ([]string, []string, error) {
	return nil, nil, nil
}
func (d *disabledVectorDB) GetByIDFromCollection(_, _ string) (string, error) { return "", nil }
func (d *disabledVectorDB) GetByID(_ string) (string, error)                  { return "", nil }
func (d *disabledVectorDB) DeleteDocument(_ string) error                     { return nil }
func (d *disabledVectorDB) DeleteDocumentFromCollection(_, _ string) error    { return nil }
func (d *disabledVectorDB) Count() int                                        { return 0 }
func (d *disabledVectorDB) Close() error                                      { return nil }
func (d *disabledVectorDB) StoreCheatsheet(_, _, _ string, _ ...string) error   { return nil }
func (d *disabledVectorDB) DeleteCheatsheet(_ string) error                   { return nil }
