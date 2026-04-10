package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestExtractKGEntityLabels(t *testing.T) {
	tests := []struct {
		name      string
		kgContext string
		maxLabels int
		want      []string
	}{
		{
			name: "single entity with properties",
			kgContext: "- [server_01] Web Server | type: device | ip: 192.168.1.10\n" +
				"  - [server_01] -[runs]-> [nginx]",
			maxLabels: 3,
			want:      []string{"Web Server"},
		},
		{
			name: "multiple entities",
			kgContext: "- [server_01] Web Server | type: device\n" +
				"  - [server_01] -[runs]-> [nginx]\n" +
				"- [db_01] Database Server | type: service\n" +
				"  - [db_01] -[connects_to]-> [server_01]\n" +
				"- [alice] Alice | type: person | role: admin\n",
			maxLabels: 3,
			want:      []string{"Web Server", "Database Server", "Alice"},
		},
		{
			name: "unknown label skipped",
			kgContext: "- [node_1] Unknown\n" +
				"- [node_2] MyService | type: service\n",
			maxLabels: 3,
			want:      []string{"MyService"},
		},
		{
			name:      "empty context",
			kgContext: "",
			maxLabels: 3,
			want:      nil,
		},
		{
			name: "max labels respected",
			kgContext: "- [a] Alpha | type: test\n" +
				"- [b] Beta | type: test\n" +
				"- [c] Gamma | type: test\n",
			maxLabels: 2,
			want:      []string{"Alpha", "Beta"},
		},
		{
			name: "short label skipped",
			kgContext: "- [x] X\n" +
				"- [y] ValidLabel | type: test\n",
			maxLabels: 3,
			want:      []string{"ValidLabel"},
		},
		{
			name: "edge-only lines skipped",
			kgContext: "  - [src] -[connects]-> [tgt]\n" +
				"- [node_1] MyNode | type: device\n",
			maxLabels: 3,
			want:      []string{"MyNode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractKGEntityLabels(tt.kgContext, tt.maxLabels)
			if len(got) != len(tt.want) {
				t.Errorf("extractKGEntityLabels() = %v, want %v", got, tt.want)
				return
			}
			for i, label := range got {
				if label != tt.want[i] {
					t.Errorf("extractKGEntityLabels()[%d] = %q, want %q", i, label, tt.want[i])
				}
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"alpha", "beta", "gamma"}
	if !containsString(slice, "beta") {
		t.Error("expected to find 'beta'")
	}
	if containsString(slice, "delta") {
		t.Error("did not expect to find 'delta'")
	}
	if containsString(nil, "anything") {
		t.Error("did not expect to find 'anything' in nil slice")
	}
}

func TestTruncateUTF8SafeAgent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "truncated at newline boundary",
			input:  "line1\nline2\nline3",
			maxLen: 10,
			want:   "line1",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "unicode preserved",
			input:  "Hällo Wörld ünïcödë",
			maxLen: 8,
			want:   "Hällo Wö",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateUTF8SafeAgent(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateUTF8SafeAgent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyRetrievalFusion_NoResults(t *testing.T) {
	// When both RAG and KG are empty, fusion should return empty results.
	result := applyRetrievalFusion(nil, "", nil, nil, nil)
	if result.EnrichedMemories != "" {
		t.Errorf("expected empty EnrichedMemories, got %q", result.EnrichedMemories)
	}
	if result.EnrichedKGContext != "" {
		t.Errorf("expected empty EnrichedKGContext, got %q", result.EnrichedKGContext)
	}
}

func TestApplyRetrievalFusion_RAGOnly(t *testing.T) {
	// When only RAG has results (no KG), only RAG→KG direction should be attempted.
	// With a nil KG, no enrichment should happen.
	topMemories := []string{"User prefers dark mode in the IDE"}
	result := applyRetrievalFusion(topMemories, "", nil, nil, nil)
	if result.EnrichedMemories != "" {
		t.Errorf("expected empty EnrichedMemories with nil LTM, got %q", result.EnrichedMemories)
	}
	if result.EnrichedKGContext != "" {
		t.Errorf("expected empty EnrichedKGContext with nil KG, got %q", result.EnrichedKGContext)
	}
}

func TestApplyRetrievalFusion_KGOnly(t *testing.T) {
	// When only KG has results (no RAG), only KG→RAG direction should be attempted.
	// With a nil LTM, no enrichment should happen.
	kgContext := "- [server_01] Web Server | type: device | ip: 192.168.1.10\n"
	result := applyRetrievalFusion(nil, kgContext, nil, nil, nil)
	if result.EnrichedMemories != "" {
		t.Errorf("expected empty EnrichedMemories with nil LTM, got %q", result.EnrichedMemories)
	}
	if result.EnrichedKGContext != "" {
		t.Errorf("expected empty EnrichedKGContext with no RAG, got %q", result.EnrichedKGContext)
	}
}

// mockFusionVectorDB is a minimal mock for testing the KG→RAG direction.
type mockFusionVectorDB struct {
	results map[string][]string
}

func (m *mockFusionVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, nil
}
func (m *mockFusionVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}
func (m *mockFusionVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (m *mockFusionVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (m *mockFusionVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}
func (m *mockFusionVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	return nil, nil, nil
}
func (m *mockFusionVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	if m.results != nil {
		if r, ok := m.results[strings.ToLower(query)]; ok {
			return r, nil, nil
		}
	}
	return nil, nil, nil
}
func (m *mockFusionVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (m *mockFusionVectorDB) GetByID(id string) (string, error)                        { return "", nil }
func (m *mockFusionVectorDB) DeleteDocument(id string) error                           { return nil }
func (m *mockFusionVectorDB) DeleteDocumentFromCollection(id, collection string) error { return nil }
func (m *mockFusionVectorDB) Count() int                                               { return 0 }
func (m *mockFusionVectorDB) IsDisabled() bool                                         { return false }
func (m *mockFusionVectorDB) Close() error                                             { return nil }
func (m *mockFusionVectorDB) StoreCheatsheet(id, name, content string) error           { return nil }
func (m *mockFusionVectorDB) DeleteCheatsheet(id string) error                         { return nil }

func TestApplyRetrievalFusion_KGToRAG(t *testing.T) {
	// Test KG→RAG direction: entity labels from KG are used to search LTM.
	kgContext := "- [server_01] Web Server | type: device\n" +
		"- [db_01] Database | type: service\n"

	mockLTM := &mockFusionVectorDB{
		results: map[string][]string{
			"web server": {"Found memory about web server configuration"},
			"database":   {"Found memory about database setup"},
		},
	}

	result := applyRetrievalFusion(nil, kgContext, mockLTM, nil, nil)

	if result.EnrichedMemories == "" {
		t.Error("expected EnrichedMemories to be populated from KG→RAG fusion")
	}
	if !strings.Contains(result.EnrichedMemories, "[Related Memories via Knowledge Graph]") {
		t.Errorf("expected fusion prefix in EnrichedMemories, got %q", result.EnrichedMemories)
	}
	if !strings.Contains(result.EnrichedMemories, "web server") && !strings.Contains(result.EnrichedMemories, "database") {
		t.Errorf("expected memory content in EnrichedMemories, got %q", result.EnrichedMemories)
	}
}

func TestApplyRetrievalFusion_BudgetRespected(t *testing.T) {
	// Verify that the fusion result respects the character budget.
	kgContext := "- [server_01] Web Server | type: device\n"

	// Create a mock that returns a very long string.
	longMem := strings.Repeat("x", 1000)
	mockLTM := &mockFusionVectorDB{
		results: map[string][]string{
			"web server": {longMem},
		},
	}

	result := applyRetrievalFusion(nil, kgContext, mockLTM, nil, nil)

	if len(result.EnrichedMemories) > fusionCharBudget+50 { // Allow some overhead for the prefix
		t.Errorf("EnrichedMemories exceeds budget: got %d chars (budget %d)",
			len(result.EnrichedMemories), fusionCharBudget)
	}
}

func TestApplyRetrievalFusion_Deduplication(t *testing.T) {
	// Verify that memories already in topMemories are not duplicated.
	existingMemory := "Already retrieved memory about web server"
	topMemories := []string{existingMemory}

	kgContext := "- [server_01] Web Server | type: device\n"

	mockLTM := &mockFusionVectorDB{
		results: map[string][]string{
			"web server": {existingMemory}, // Same as topMemories[0]
		},
	}

	result := applyRetrievalFusion(topMemories, kgContext, mockLTM, nil, nil)

	// Should be empty because the only result is a duplicate.
	if result.EnrichedMemories != "" {
		t.Errorf("expected empty EnrichedMemories due to dedup, got %q", result.EnrichedMemories)
	}
}
