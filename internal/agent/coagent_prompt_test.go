package agent

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

type coAgentContextVectorDB struct {
	results                  []string
	searchSimilarCalled      bool
	searchMemoriesOnlyCalled bool
}

func (f *coAgentContextVectorDB) StoreDocument(concept, content string) ([]string, error) {
	return nil, nil
}

func (f *coAgentContextVectorDB) StoreDocumentWithEmbedding(concept, content string, embedding []float32) (string, error) {
	return "", nil
}

func (f *coAgentContextVectorDB) StoreBatch(items []memory.ArchiveItem) ([]string, error) {
	return nil, nil
}

func (f *coAgentContextVectorDB) SearchSimilar(query string, topK int, excludeCollections ...string) ([]string, []string, error) {
	f.searchSimilarCalled = true
	return append([]string(nil), f.results...), nil, nil
}

func (f *coAgentContextVectorDB) SearchMemoriesOnly(query string, topK int) ([]string, []string, error) {
	f.searchMemoriesOnlyCalled = true
	return append([]string(nil), f.results...), nil, nil
}

func (f *coAgentContextVectorDB) GetByID(id string) (string, error) { return "", nil }
func (f *coAgentContextVectorDB) GetByIDFromCollection(id, collection string) (string, error) {
	return "", nil
}
func (f *coAgentContextVectorDB) DeleteDocument(id string) error { return nil }
func (f *coAgentContextVectorDB) DeleteDocumentFromCollection(id, collection string) error {
	return nil
}
func (f *coAgentContextVectorDB) Count() int       { return len(f.results) }
func (f *coAgentContextVectorDB) IsDisabled() bool { return false }
func (f *coAgentContextVectorDB) Close() error     { return nil }
func (f *coAgentContextVectorDB) StoreDocumentInCollection(concept, content, collection string) ([]string, error) {
	return nil, nil
}
func (f *coAgentContextVectorDB) StoreDocumentWithEmbeddingInCollection(concept, content string, embedding []float32, collection string) (string, error) {
	return "", nil
}
func (f *coAgentContextVectorDB) StoreCheatsheet(id, name, content string, attachments ...string) error { return nil }
func (f *coAgentContextVectorDB) DeleteCheatsheet(id string) error               { return nil }

func TestBuildContextSnapshotUsesMemoriesOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if _, err := stm.AddCoreMemoryFact("co-agent context fact"); err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	vdb := &coAgentContextVectorDB{results: []string{"memory hit one", "memory hit two"}}
	snapshot := buildContextSnapshot(CoAgentRequest{
		Task:         "Summarize the issue",
		ContextHints: []string{"hint one"},
	}, vdb, stm)

	if !vdb.searchMemoriesOnlyCalled {
		t.Fatal("expected SearchMemoriesOnly to be used for co-agent context")
	}
	if vdb.searchSimilarCalled {
		t.Fatal("did not expect SearchSimilar to be used for co-agent context")
	}
	if !strings.Contains(snapshot, "## Core Memory") {
		t.Fatalf("snapshot = %q, want core memory section", snapshot)
	}
	if !strings.Contains(snapshot, "memory hit one") {
		t.Fatalf("snapshot = %q, want local memory hit", snapshot)
	}
	if !strings.Contains(snapshot, "- hint one") {
		t.Fatalf("snapshot = %q, want normalized hint list", snapshot)
	}
}

func TestBuildContextSnapshotDesignerStaysLean(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	longFact := strings.Repeat("A", 450) + "TAILMARK"
	if _, err := stm.AddCoreMemoryFact(longFact); err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	vdb := &coAgentContextVectorDB{results: []string{
		"design memory one",
		"design memory two should be dropped for lean designer prompts",
	}}
	snapshot := buildContextSnapshot(CoAgentRequest{
		Task:         "Create a landing page concept",
		Specialist:   "designer",
		ContextHints: []string{"hint-1", "hint-2", "hint-3", "hint-4", "hint-5"},
	}, vdb, stm)

	if !vdb.searchMemoriesOnlyCalled {
		t.Fatal("expected SearchMemoriesOnly to be used")
	}
	if strings.Contains(snapshot, "TAILMARK") {
		t.Fatalf("snapshot should trim oversized core memory for designer prompts: %q", snapshot)
	}
	if !strings.Contains(snapshot, "design memory one") {
		t.Fatalf("snapshot = %q, want first design memory hit", snapshot)
	}
	if strings.Contains(snapshot, "design memory two should be dropped") {
		t.Fatalf("snapshot should keep only one RAG hit for designer prompts: %q", snapshot)
	}
	if strings.Contains(snapshot, "hint-5") {
		t.Fatalf("snapshot should cap designer hints: %q", snapshot)
	}
}

func TestBuildSpecialistSystemPromptInjectsLeanContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if _, err := stm.AddCoreMemoryFact(strings.Repeat("B", 450) + "ENDMARK"); err != nil {
		t.Fatalf("AddCoreMemoryFact: %v", err)
	}

	vdb := &coAgentContextVectorDB{results: []string{"memory one", "memory two"}}

	promptsDir := t.TempDir()
	templatesDir := filepath.Join(promptsDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	template := "Language={{LANGUAGE}}\n{{CONTEXT_SNAPSHOT}}\nTask={{TASK}}\n"
	if err := os.WriteFile(filepath.Join(templatesDir, "specialist_designer.md"), []byte(template), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.SystemLanguage = "de"
	cfg.Directories.PromptsDir = promptsDir

	prompt := buildSpecialistSystemPrompt(cfg, "designer", CoAgentRequest{
		Task:         "Design a compact status card",
		Specialist:   "designer",
		ContextHints: []string{"alpha", "beta", "gamma", "delta", "epsilon"},
	}, vdb, stm, nil)

	if !strings.Contains(prompt, "Language=de") {
		t.Fatalf("prompt = %q, want system language injected", prompt)
	}
	if !strings.Contains(prompt, "memory one") {
		t.Fatalf("prompt = %q, want first memory hit", prompt)
	}
	if strings.Contains(prompt, "memory two") {
		t.Fatalf("prompt should keep lean designer context: %q", prompt)
	}
	if strings.Contains(prompt, "ENDMARK") {
		t.Fatalf("prompt should trim oversized core memory: %q", prompt)
	}
	if strings.Contains(prompt, "epsilon") {
		t.Fatalf("prompt should cap designer hints: %q", prompt)
	}
	if !strings.Contains(prompt, "Task=Design a compact status card") {
		t.Fatalf("prompt = %q, want task injected", prompt)
	}
}
