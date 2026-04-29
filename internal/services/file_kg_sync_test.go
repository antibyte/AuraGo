package services

import (
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestFileKGSyncer_SyncAll_NilDependencies(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	// Should not panic when KG or STM are nil.
	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, nil)
	result := syncer.SyncAll(FileKGSyncOptions{})

	if result.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed, got %d", result.FilesProcessed)
	}
	if result.NodesExtracted != 0 {
		t.Errorf("expected 0 nodes extracted, got %d", result.NodesExtracted)
	}
}

func TestFileKGSyncer_SyncFile_VectorDBDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	// Create a disabled VectorDB stub.
	vectorDB := &memory.ChromemVectorDB{}

	// STM and KG are nil — SyncAll should bail early.
	syncer := NewFileKGSyncer(cfg, logger, nil, vectorDB, nil, nil)
	result := syncer.SyncAll(FileKGSyncOptions{})

	if result.FilesProcessed != 0 {
		t.Errorf("expected 0 files processed with nil STM/KG, got %d", result.FilesProcessed)
	}
}

func TestFileKGSyncer_CleanupFile_NilKG(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, nil)
	result := syncer.CleanupFile("/docs/a.txt", "file_index", false)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors with nil KG, got %v", result.Errors)
	}
}

func TestFileKGSyncer_CleanupFile_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}

	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	syncer := NewFileKGSyncer(cfg, logger, nil, nil, nil, kg)
	result := syncer.CleanupFile("/docs/a.txt", "file_index", true)

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors in dry-run, got %v", result.Errors)
	}
}

func TestFileKGSyncer_CleanupOrphans_RemovesRenamedAndDeletedFileEntities(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	defer kg.Close()

	activePath := "/docs/new-name.md"
	renamedPath := "/docs/old-name.md"
	deletedPath := "/docs/deleted.md"
	if err := stm.UpdateFileIndexWithDocs(activePath, IndexerCollection, time.Now(), []string{"doc-new"}); err != nil {
		t.Fatalf("UpdateFileIndexWithDocs active: %v", err)
	}
	if err := kg.BulkMergeExtractedEntities([]memory.Node{
		{ID: "active-node", Label: "Active", Properties: map[string]string{"source": "file_sync", "source_file": activePath}},
		{ID: "renamed-node", Label: "Renamed", Properties: map[string]string{"source": "file_sync", "source_file": renamedPath}},
		{ID: "deleted-node", Label: "Deleted", Properties: map[string]string{"source": "file_sync", "source_file": deletedPath}},
	}, []memory.Edge{
		{Source: "renamed-node", Target: "deleted-node", Relation: "related_to", Properties: map[string]string{"source": "file_sync", "source_file": renamedPath}},
	}); err != nil {
		t.Fatalf("BulkMergeExtractedEntities: %v", err)
	}

	syncer := NewFileKGSyncer(cfg, logger, nil, nil, stm, kg)
	result := syncer.CleanupOrphans(false)
	if len(result.Errors) != 0 {
		t.Fatalf("CleanupOrphans errors: %v", result.Errors)
	}
	if result.FilesProcessed != 2 {
		t.Fatalf("expected 2 orphan source files cleaned, got %#v", result)
	}
	if result.NodesExtracted != 2 {
		t.Fatalf("expected 2 orphan nodes removed, got %#v", result)
	}
	if result.EdgesExtracted != 1 {
		t.Fatalf("expected 1 orphan edge removed, got %#v", result)
	}

	nodes, err := kg.GetAllNodes(20)
	if err != nil {
		t.Fatalf("GetAllNodes: %v", err)
	}
	seen := make(map[string]bool)
	for _, node := range nodes {
		seen[node.ID] = true
	}
	if !seen["active-node"] {
		t.Fatal("expected active file node to remain")
	}
	if seen["renamed-node"] || seen["deleted-node"] {
		t.Fatalf("expected orphan nodes to be removed, got nodes %#v", seen)
	}

	orphanNodes, orphanEdges, err := syncer.FindOrphans()
	if err != nil {
		t.Fatalf("FindOrphans after cleanup: %v", err)
	}
	if len(orphanNodes) != 0 || len(orphanEdges) != 0 {
		t.Fatalf("expected no orphan entities after cleanup, got nodes=%d edges=%d", len(orphanNodes), len(orphanEdges))
	}
}

// ---------------------------------------------------------------------------
// Content preparation tests
// ---------------------------------------------------------------------------

func TestPrepareContent_Markdown_HeadingOutline(t *testing.T) {
	input := `# My Server Setup

Some intro text.

## Hardware

Details about the hardware.

### Network

Network config here.

## Software

Software details.`

	result := prepareContentForExtraction("/docs/setup.md", input)

	// Should contain structural outline.
	if !strings.Contains(result, "[Document Structure]") {
		t.Error("expected [Document Structure] prefix for Markdown files")
	}
	if !strings.Contains(result, "[Content]") {
		t.Error("expected [Content] separator in Markdown preparation")
	}
	// Outline should list headings.
	if !strings.Contains(result, "My Server Setup") {
		t.Error("expected heading 'My Server Setup' in outline")
	}
	if !strings.Contains(result, "Hardware") {
		t.Error("expected heading 'Hardware' in outline")
	}
	if !strings.Contains(result, "Network") {
		t.Error("expected heading 'Network' in outline")
	}
	// Original content should still be present.
	if !strings.Contains(result, "Details about the hardware.") {
		t.Error("expected original content to be preserved")
	}
	// Indentation: level-3 heading should be indented.
	if !strings.Contains(result, "    - Network") {
		t.Error("expected level-3 heading 'Network' to be indented with 4 spaces")
	}
}

func TestPrepareContent_Markdown_NoHeadings(t *testing.T) {
	input := "Just some plain text without any headings at all."
	result := prepareContentForExtraction("/docs/notes.md", input)

	// Should NOT add structure prefix when no headings exist.
	if strings.Contains(result, "[Document Structure]") {
		t.Error("expected no structure prefix for Markdown without headings")
	}
	if result != input {
		t.Errorf("expected content unchanged, got:\n%s", result)
	}
}

func TestPrepareContent_PDF_CleansArtifacts(t *testing.T) {
	// Simulate PDF extraction with form-feed and excessive spaces.
	input := "Page 1 content\x0cPage 2 content   with   extra   spaces\x0bhere"
	result := prepareContentForExtraction("/docs/manual.pdf", input)

	if strings.Contains(result, "\x0c") {
		t.Error("expected form-feed characters to be removed")
	}
	if strings.Contains(result, "\x0b") {
		t.Error("expected vertical-tab characters to be removed")
	}
	if strings.Contains(result, "   ") {
		t.Error("expected excessive spaces to be collapsed")
	}
	if !strings.Contains(result, "Page 1 content") {
		t.Error("expected original text to be preserved")
	}
}

func TestPrepareContent_DOCX_CleansArtifacts(t *testing.T) {
	input := "Some   text   from   docx\x0cwith artifacts"
	result := prepareContentForExtraction("/docs/report.docx", input)

	if strings.Contains(result, "\x0c") {
		t.Error("expected form-feed characters to be removed for DOCX")
	}
	if strings.Contains(result, "   ") {
		t.Error("expected excessive spaces to be collapsed for DOCX")
	}
}

func TestPrepareContent_Truncation(t *testing.T) {
	// Create content exceeding the limit.
	longContent := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 1600) // ~41,600 chars
	result := prepareContentForExtraction("/docs/large.txt", longContent)

	if len(result) > maxContentBytes+100 { // allow for truncation notice
		t.Errorf("expected content to be truncated to ~%d chars, got %d", maxContentBytes, len(result))
	}
	if !strings.Contains(result, "[... content truncated for extraction ...]") {
		t.Error("expected truncation notice in output")
	}
}

func TestPrepareContent_SkipsGenericMultimodalPlaceholders(t *testing.T) {
	for _, input := range []string{
		"Bild-Datei: photo.jpg (Pfad: photo.jpg)",
		"Audio-Datei: clip.mp3 (Pfad: clip.mp3)",
		"PDF (gescannt): scan.pdf (Pfad: scan.pdf)",
	} {
		if got := prepareContentForExtraction("/docs/file", input); got != "" {
			t.Fatalf("prepareContentForExtraction(%q) = %q, want empty placeholder skipped", input, got)
		}
	}
}

func TestPrepareContent_TruncationPreservesShortContent(t *testing.T) {
	input := "This is a short file that should pass through unchanged."
	result := prepareContentForExtraction("/docs/short.txt", input)

	if strings.Contains(result, "[... content truncated") {
		t.Error("short content should not be truncated")
	}
	if result != input {
		t.Errorf("expected content unchanged, got:\n%s", result)
	}
}

func TestPrepareContent_WhitespaceNormalization(t *testing.T) {
	input := "Line 1\n\n\n\n\nLine 2\n\n\n\n\n\nLine 3"
	result := prepareContentForExtraction("/docs/messy.txt", input)

	// Should collapse 3+ blank lines to 2 (one blank line between paragraphs).
	if strings.Contains(result, "\n\n\n") {
		t.Error("expected excessive blank lines to be collapsed")
	}
	if !strings.Contains(result, "Line 1") || !strings.Contains(result, "Line 3") {
		t.Error("expected all content lines to be preserved")
	}
}

func TestPrepareContent_PlainText_PassThrough(t *testing.T) {
	input := "Hello world, this is a plain text file."
	result := prepareContentForExtraction("/docs/readme.txt", input)

	if result != input {
		t.Errorf("expected plain text to pass through unchanged, got:\n%s", result)
	}
}

func TestPrepareContent_EmptyContent(t *testing.T) {
	result := prepareContentForExtraction("/docs/empty.md", "")
	if result != "" {
		t.Errorf("expected empty string for empty input, got:\n%s", result)
	}
}

func TestPrepareContent_MarkdownOutline_Truncated(t *testing.T) {
	// Markdown with many headings and long content should still be truncated.
	var sb strings.Builder
	sb.WriteString("# Main Title\n\n")
	for i := 0; i < 500; i++ {
		sb.WriteString("## Section ")
		sb.WriteString(strings.Repeat("x", 20))
		sb.WriteString("\n\n")
		sb.WriteString(strings.Repeat("Content line here.\n", 10))
	}

	result := prepareContentForExtraction("/docs/huge.md", sb.String())
	if len(result) > maxContentBytes+100 {
		t.Errorf("expected Markdown content to be truncated, got %d chars", len(result))
	}
	// Structure prefix should still be present.
	if !strings.Contains(result, "[Document Structure]") {
		t.Error("expected document structure prefix even for truncated Markdown")
	}
}

func TestFileKGSyncer_SyncCollectionAggregatesParallelResults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := &config.Config{}
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	defer stm.Close()

	for _, path := range []string{"/docs/a.md", "/docs/b.md", "/docs/c.md"} {
		if err := stm.UpdateFileIndexWithDocs(path, IndexerCollection, time.Now(), []string{"doc-" + path}); err != nil {
			t.Fatalf("UpdateFileIndexWithDocs(%s): %v", path, err)
		}
	}

	var calls atomic.Int32
	syncer := NewFileKGSyncer(cfg, logger, nil, nil, stm, nil)
	syncer.syncFile = func(path, collection string, opts FileKGSyncOptions) FileKGSyncResult {
		calls.Add(1)
		return FileKGSyncResult{
			FilesProcessed: 1,
			NodesExtracted: 2,
			EdgesExtracted: 1,
			Errors:         []string{"warn:" + path},
		}
	}

	result := syncer.SyncCollection(IndexerCollection, FileKGSyncOptions{})
	if calls.Load() != 3 {
		t.Fatalf("expected syncFile to be called 3 times, got %d", calls.Load())
	}
	if result.FilesProcessed != 3 {
		t.Fatalf("expected 3 processed files, got %d", result.FilesProcessed)
	}
	if result.NodesExtracted != 6 || result.EdgesExtracted != 3 {
		t.Fatalf("unexpected aggregate result: %#v", result)
	}
	if len(result.Errors) != 3 {
		t.Fatalf("expected 3 aggregated errors, got %#v", result.Errors)
	}
}
