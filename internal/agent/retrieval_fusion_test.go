package agent

import (
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"context"
	"log/slog"
	"strings"
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
	result := applyRetrievalFusion(context.Background(), nil, "", nil, nil, nil, logger)
	if len(result.EnrichedMemories) != 0 || len(result.EnrichedKGContexts) != 0 {
		t.Error("expected empty result when no inputs provided")
	}
}

func TestApplyRetrievalFusion_RAGOnly_NoKG(t *testing.T) {
	logger := slog.Default()
	topMemories := []rankedMemory{{text: "memory one about servers", docID: "mem-1"}, {text: "memory two about networking", docID: "mem-2"}}
	result := applyRetrievalFusion(context.Background(), topMemories, "", nil, nil, nil, logger)
	// Without KG, Direction 1 (KG→RAG) is skipped.
	// Without kg, Direction 2 (RAG→KG) is also skipped.
	if len(result.EnrichedMemories) != 0 {
		t.Error("expected no enriched memories without KG context")
	}
	if len(result.EnrichedKGContexts) != 0 {
		t.Error("expected no enriched KG context without KnowledgeGraph")
	}
}

func TestApplyRetrievalFusion_KGOnly_NoVectorDB(t *testing.T) {
	logger := slog.Default()
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	result := applyRetrievalFusion(context.Background(), nil, kgContext, nil, nil, nil, logger)
	// Without VectorDB, Direction 1 (KG→RAG) is skipped.
	// Without RAG, Direction 2 (RAG→KG) is skipped.
	if len(result.EnrichedMemories) != 0 {
		t.Error("expected no enriched memories without VectorDB")
	}
	if len(result.EnrichedKGContexts) != 0 {
		t.Error("expected no enriched KG context without RAG memories")
	}
}

func TestApplyRetrievalFusion_WithBoth_NoImplementations(t *testing.T) {
	logger := slog.Default()
	topMemories := []rankedMemory{{text: "memory about something important", docID: "mem-1"}}
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	// Both subsystems report having data, but nil implementations mean
	// the actual lookups are skipped gracefully.
	result := applyRetrievalFusion(context.Background(), topMemories, kgContext, nil, nil, nil, logger)
	// Direction 1 skipped: longTermMem is nil
	// Direction 2 skipped: kg is nil
	if len(result.EnrichedMemories) != 0 {
		t.Error("expected no enriched memories with nil VectorDB")
	}
	if len(result.EnrichedKGContexts) != 0 {
		t.Error("expected no enriched KG context with nil KnowledgeGraph")
	}
}

func TestApplyRetrievalFusion_DisabledVectorDB(t *testing.T) {
	logger := slog.Default()
	topMemories := []rankedMemory{{text: "memory text", docID: "mem-1"}}
	kgContext := "- [truenas] TrueNAS Server | type: device\n"
	result := applyRetrievalFusion(context.Background(), topMemories, kgContext, &disabledVectorDB{}, nil, nil, logger)
	// Direction 1 skipped: VectorDB is disabled
	if len(result.EnrichedMemories) != 0 {
		t.Error("expected no enriched memories with disabled VectorDB")
	}
}

func TestApplyRetrievalFusionDeduplicatesCanonicalMemoryText(t *testing.T) {
	logger := slog.Default()
	topMemories := []rankedMemory{
		{text: "[Similarity: 0.88] Docker backup policy should keep seven daily snapshots and verify restores weekly.", docID: "doc-base"},
	}
	kgContext := "- [docker] Docker | type: software\n"
	vdb := &fusionVectorDB{
		memories: []string{
			"Docker backup policy should keep seven daily snapshots and verify restores weekly.",
		},
		docIDs: []string{"doc-duplicate"},
	}

	result := applyRetrievalFusion(context.Background(), topMemories, kgContext, vdb, nil, nil, logger)
	if len(result.EnrichedMemories) != 0 {
		t.Fatalf("EnrichedMemories = %+v, want duplicate to be suppressed", result.EnrichedMemories)
	}
}

func TestApplyRetrievalFusionPreservesMemoryIDsAndStructuredKGIDs(t *testing.T) {
	t.Run("KG to RAG keeps stable memory ID", func(t *testing.T) {
		run, _, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
		defer cleanup()
		vdb := &fusionVectorDB{
			memories: []string{"NAS backup policy keeps seven daily snapshots."},
			docIDs:   []string{"ltm-real-id"},
		}
		result := applyRetrievalFusion(context.Background(), nil, "- [nas] NAS | type: device", vdb, run.ShortTermMem, nil, slog.Default())
		if len(result.EnrichedMemories) != 1 || result.EnrichedMemories[0].docID != "ltm-real-id" {
			t.Fatalf("fusion memory result = %+v, want stable LTM ID", result.EnrichedMemories)
		}
		formatted := formatFusionMemoriesForPrompt(result.EnrichedMemories, fusionCharBudget)
		if !strings.Contains(formatted, "NAS backup policy keeps seven daily snapshots") || strings.Contains(formatted, "[fusion:") {
			t.Fatalf("formatted fusion memory lost fact or invented an ID: %q", formatted)
		}
	})

	t.Run("RAG to KG searches the complete fact and keeps node IDs", func(t *testing.T) {
		kg, err := memory.NewKnowledgeGraph(":memory:", "", slog.Default())
		if err != nil {
			t.Fatal(err)
		}
		defer kg.Close()
		if err := kg.AddNode("tailmarker-id", "TailMarker", map[string]string{"type": "software"}); err != nil {
			t.Fatal(err)
		}
		fullFact := strings.Repeat("ordinary backup details ", 12) + "TailMarker"
		result := applyRetrievalFusion(context.Background(), []rankedMemory{{text: fullFact, docID: "memory-input"}}, "- [base-node] Existing", nil, nil, kg, slog.Default())
		if len(result.EnrichedKGContexts) == 0 {
			t.Fatalf("fusion did not find node referenced after the former 120-byte cut: %+v", result)
		}
		foundID := false
		for _, contextResult := range result.EnrichedKGContexts {
			for _, node := range contextResult.Nodes {
				if node.ID == "tailmarker-id" {
					foundID = true
				}
			}
		}
		if !foundID {
			t.Fatalf("structured fusion result lost the KG node ID: %+v", result.EnrichedKGContexts)
		}
		formatted := formatFusionKGContextsForPrompt(result.EnrichedKGContexts, fusionCharBudget)
		if !strings.Contains(formatted, "[tailmarker-id]") || strings.Contains(formatted, "[fusion:") {
			t.Fatalf("formatted KG context lost the real ID or invented one: %q", formatted)
		}
	})
}

func TestDeduplicateFusionKGNodesAndEdgesAgainstBaseContext(t *testing.T) {
	edge := memory.Edge{Source: "base", Target: "new", Relation: "links"}
	base := "- [base] Base\n  - [base] Base -[links]-> [new] New"
	result := memory.KnowledgeGraphContextResult{
		Nodes: []memory.Node{
			{ID: "base", Label: "Base"},
			{ID: "new", Label: "New"},
		},
		EdgesByNodeID: map[string][]memory.Edge{
			"base": {edge},
			"new":  {edge},
		},
	}

	got := dedupeKGContextResults(base, []memory.KnowledgeGraphContextResult{result, result})
	if len(got) != 1 || len(got[0].Nodes) != 1 || got[0].Nodes[0].ID != "new" {
		t.Fatalf("deduped results = %+v, want only new node", got)
	}
	if len(got[0].EdgesByNodeID["new"]) != 0 {
		t.Fatalf("duplicate edge survived dedupe: %+v", got[0].EdgesByNodeID)
	}
	formatted := formatFusionKGContextsForPrompt(got, fusionCharBudget)
	if strings.Count(formatted, "[new]") != 1 || strings.Contains(formatted, "-[links]->") {
		t.Fatalf("formatted fusion context duplicated base content: %q", formatted)
	}
}

func TestFormatFusionMemoryBudgetKeepsRecallRefsAndShowsUnreferenceableOmissions(t *testing.T) {
	got := formatFusionMemoriesForPrompt([]rankedMemory{
		{text: strings.Repeat("oversized fact ", 60), docID: "memory-long"},
		{text: strings.Repeat("oversized fact ", 60)},
	}, fusionCharBudget)
	if !strings.Contains(got, "[memory:memory-long] (content omitted; use recall_memory)") {
		t.Fatalf("oversized memory lost its recall reference: %q", got)
	}
	if !strings.Contains(got, "[memory omitted: no stable recall ID]") {
		t.Fatalf("oversized memory without ID was silently omitted: %q", got)
	}
	if len(got) > fusionCharBudget {
		t.Fatalf("fusion memory output length %d exceeds %d bytes", len(got), fusionCharBudget)
	}
}

func TestFusionPromptEntriesKeepRealIDsAndWholeFacts(t *testing.T) {
	longFact := "NAS backup policy: " + strings.Repeat("retain a verified daily snapshot; ", 20)
	memories := []rankedMemory{
		{text: "Keep seven verified backup snapshots.", docID: "memory-real-1"},
		{text: longFact, docID: "memory-real-2"},
	}
	kgResults := []memory.KnowledgeGraphContextResult{{Nodes: []memory.Node{{ID: "kg-real-1", Label: "NAS Backup"}}}}
	flags := prompts.ContextFlags{
		Tier:                           "full",
		TokenBudget:                    12000,
		RetrievedMemories:              formatFusionMemoriesForPrompt(memories, fusionCharBudget),
		KnowledgeContext:               formatFusionKGContextsForPrompt(kgResults, fusionCharBudget),
		AvailableMemoryContextIndex:    buildAvailableMemoryIndex(memories, fusionCharBudget),
		AvailableKnowledgeContextIndex: formatFusionKGIndexes(kgResults, fusionCharBudget),
	}
	prompt, _ := prompts.BuildSystemPrompt(t.TempDir(), &flags, "", slog.Default())
	for _, want := range []string{"Keep seven verified backup snapshots.", "[memory:memory-real-2] (content omitted; use recall_memory)", "[kg-real-1] NAS Backup", "[memory:memory-real-1]", "[kg:kg-real-1]"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("actual system prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "[fusion:") {
		t.Fatalf("actual system prompt contains synthetic fusion IDs:\n%s", prompt)
	}
}

func TestFusionPromptSmallBudgetKeepsUnicodeAndWholeIDs(t *testing.T) {
	const budget = 120
	longID := "memory-" + strings.Repeat("長い識別子", 30)
	memories := []rankedMemory{
		{text: "Das NAS nutzt ein privates VLAN. 🌐", docID: "memory-漢"},
		{text: strings.Repeat("oversized fact ", 40), docID: longID},
	}
	kgResults := []memory.KnowledgeGraphContextResult{{Nodes: []memory.Node{{ID: "kg-" + strings.Repeat("長い識別子", 30), Label: "NAS"}}}}
	flags := prompts.ContextFlags{
		Tier:                           "full",
		TokenBudget:                    12000,
		RetrievedMemories:              formatFusionMemoriesForPrompt(memories, budget),
		KnowledgeContext:               formatFusionKGContextsForPrompt(kgResults, budget),
		AvailableMemoryContextIndex:    buildAvailableMemoryIndex(memories, budget),
		AvailableKnowledgeContextIndex: formatFusionKGIndexes(kgResults, budget),
	}
	prompt, _ := prompts.BuildSystemPrompt(t.TempDir(), &flags, "", slog.Default())
	if !strings.Contains(prompt, "Das NAS nutzt ein privates VLAN. 🌐") || !strings.Contains(prompt, "[memory:memory-漢]") || !strings.Contains(prompt, "[additional entries omitted]") {
		t.Fatalf("small-budget prompt lost Unicode fact or visible omission marker:\n%s", prompt)
	}
	if strings.Contains(prompt, "memory-長") || strings.Contains(prompt, "kg-長") {
		t.Fatalf("small-budget prompt contains a partial/unreferenceable long ID:\n%s", prompt)
	}
	if got := len(formatFusionMemoriesForPrompt(memories, budget)); got > budget {
		t.Fatalf("memory fusion output uses %d bytes, exceeds budget %d", got, budget)
	}
	if got := len(formatFusionKGContextsForPrompt(kgResults, budget)); got > budget {
		t.Fatalf("KG fusion output uses %d bytes, exceeds budget %d", got, budget)
	}
	if strings.Contains(prompt, longID) {
		t.Fatalf("prompt contains the full over-budget memory ID:\n%s", prompt)
	}
}

// disabledVectorDB is a minimal mock that reports itself as disabled.
type disabledVectorDB struct{}

func (d *disabledVectorDB) IsDisabled() bool                            { return true }
func (d *disabledVectorDB) IsReady() bool                               { return true }
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
func (d *disabledVectorDB) StoreCheatsheet(_, _, _ string, _ ...string) error { return nil }
func (d *disabledVectorDB) DeleteCheatsheet(_ string) error                   { return nil }
func (d *disabledVectorDB) RegisterCollections(_ []string)                    {}

type fusionVectorDB struct {
	disabledVectorDB
	memories []string
	docIDs   []string
}

func (f *fusionVectorDB) IsDisabled() bool { return false }
func (f *fusionVectorDB) IsReady() bool    { return true }
func (f *fusionVectorDB) SearchMemoriesOnly(_ string, _ int) ([]string, []string, error) {
	return f.memories, f.docIDs, nil
}
