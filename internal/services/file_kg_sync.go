package services

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/kgextraction"
	"aurago/internal/llm"
	"aurago/internal/memory"
)

// FileKGSyncOptions controls the behavior of the file-to-KG sync operation.
type FileKGSyncOptions struct {
	DryRun   bool // If true, log what would be done without modifying the KG.
	Backfill bool // If true, process all tracked files regardless of prior sync state.
	MaxFiles int  // Limit the number of files to process (0 = unlimited).
}

// FileKGSyncResult reports the outcome of a sync run.
type FileKGSyncResult struct {
	FilesProcessed int
	FilesSkipped   int
	NodesExtracted int
	EdgesExtracted int
	Errors         []string
}

// FileKGSyncer performs incremental synchronization of indexed file contents
// into the Knowledge Graph using the reusable ExtractKGFromText extraction logic.
type FileKGSyncer struct {
	cfg       *config.Config
	logger    *slog.Logger
	llmClient llm.ChatClient
	vectorDB  memory.VectorDB
	stm       *memory.SQLiteMemory
	kg        *memory.KnowledgeGraph
}

// NewFileKGSyncer creates a new file-to-KG sync service.
func NewFileKGSyncer(cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vectorDB memory.VectorDB, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) *FileKGSyncer {
	return &FileKGSyncer{
		cfg:       cfg,
		logger:    logger,
		llmClient: llmClient,
		vectorDB:  vectorDB,
		stm:       stm,
		kg:        kg,
	}
}

// SyncAll synchronizes all tracked files across all collections.
func (s *FileKGSyncer) SyncAll(opts FileKGSyncOptions) FileKGSyncResult {
	var result FileKGSyncResult
	if s.kg == nil || s.stm == nil {
		s.logger.Warn("[FileKGSync] KG or STM not available, skipping sync")
		return result
	}
	if s.vectorDB != nil && s.vectorDB.IsDisabled() {
		s.logger.Warn("[FileKGSync] VectorDB is disabled, skipping sync")
		return result
	}

	// Derive collections from config directories (same logic as FileIndexer).
	collections := s.discoverCollections()
	if len(collections) == 0 {
		collections = []string{IndexerCollection}
	}

	for _, collection := range collections {
		colResult := s.SyncCollection(collection, opts)
		result.FilesProcessed += colResult.FilesProcessed
		result.FilesSkipped += colResult.FilesSkipped
		result.NodesExtracted += colResult.NodesExtracted
		result.EdgesExtracted += colResult.EdgesExtracted
		result.Errors = append(result.Errors, colResult.Errors...)

		if opts.MaxFiles > 0 && result.FilesProcessed >= opts.MaxFiles {
			break
		}
	}

	s.logger.Info("[FileKGSync] Sync complete",
		"files_processed", result.FilesProcessed,
		"files_skipped", result.FilesSkipped,
		"nodes_extracted", result.NodesExtracted,
		"edges_extracted", result.EdgesExtracted,
		"errors", len(result.Errors))
	return result
}

// discoverCollections returns the set of collections configured for indexing.
func (s *FileKGSyncer) discoverCollections() []string {
	seen := make(map[string]struct{})
	var collections []string
	for _, dir := range s.cfg.Indexing.Directories {
		col := getDirCollection(dir)
		if _, ok := seen[col]; !ok {
			seen[col] = struct{}{}
			collections = append(collections, col)
		}
	}
	return collections
}

// SyncCollection synchronizes all tracked files within a single collection.
func (s *FileKGSyncer) SyncCollection(collection string, opts FileKGSyncOptions) FileKGSyncResult {
	var result FileKGSyncResult
	files, err := s.stm.ListIndexedFiles(collection)
	if err != nil {
		s.logger.Error("[FileKGSync] Failed to list indexed files", "collection", collection, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("list indexed files %s: %v", collection, err))
		return result
	}

	for _, path := range files {
		if opts.MaxFiles > 0 && result.FilesProcessed >= opts.MaxFiles {
			break
		}
		fileResult := s.SyncFile(path, collection, opts)
		result.FilesProcessed += fileResult.FilesProcessed
		result.FilesSkipped += fileResult.FilesSkipped
		result.NodesExtracted += fileResult.NodesExtracted
		result.EdgesExtracted += fileResult.EdgesExtracted
		result.Errors = append(result.Errors, fileResult.Errors...)
	}
	return result
}

// SyncFile synchronizes a single tracked file into the Knowledge Graph.
func (s *FileKGSyncer) SyncFile(path, collection string, opts FileKGSyncOptions) FileKGSyncResult {
	var result FileKGSyncResult

	// 1. Resolve document content from VectorDB via tracked doc IDs.
	content, err := s.resolveFileContent(path, collection)
	if err != nil {
		s.logger.Warn("[FileKGSync] Failed to resolve file content", "path", path, "collection", collection, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("resolve content %s: %v", path, err))
		return result
	}
	if len(strings.TrimSpace(content)) < 50 {
		result.FilesSkipped++
		s.logger.Debug("[FileKGSync] Skipping file: content too short", "path", path)
		return result
	}

	// 1b. Prepare content based on file type for better extraction quality.
	content = prepareContentForExtraction(path, content)

	// 2. Build existing nodes context for the LLM (reuse IDs where possible).
	existingNodesString := ""
	if s.kg != nil {
		if existingNodes, err := s.kg.GetAllNodes(150); err == nil && len(existingNodes) > 0 {
			var contexts []string
			for _, n := range existingNodes {
				contexts = append(contexts, fmt.Sprintf("- ID: %s, Label: %s", n.ID, n.Label))
			}
			existingNodesString = "Existing Nodes (reuse IDs if possible):\n" + strings.Join(contexts, "\n") + "\n\n"
		}
	}

	// 3. Extract entities and relationships via the reusable extraction package.
	nodes, edges, err := kgextraction.ExtractKGFromText(s.cfg, s.logger, s.llmClient, content, existingNodesString)
	if err != nil {
		s.logger.Warn("[FileKGSync] KG extraction failed", "path", path, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("extraction %s: %v", path, err))
		return result
	}
	if len(nodes) == 0 && len(edges) == 0 {
		result.FilesSkipped++
		s.logger.Debug("[FileKGSync] No entities extracted", "path", path)
		return result
	}

	// 4. Annotate with source metadata (minimal evidence link for first draft).
	now := time.Now().Format("2006-01-02")
	for i := range nodes {
		if nodes[i].Properties == nil {
			nodes[i].Properties = make(map[string]string)
		}
		nodes[i].Properties["source"] = "file_sync"
		nodes[i].Properties["source_file"] = path
		nodes[i].Properties["extracted_at"] = now
		if collection != "" {
			nodes[i].Properties["collection"] = collection
		}
	}
	for i := range edges {
		if edges[i].Properties == nil {
			edges[i].Properties = make(map[string]string)
		}
		edges[i].Properties["source"] = "file_sync"
		edges[i].Properties["source_file"] = path
		edges[i].Properties["extracted_at"] = now
		if collection != "" {
			edges[i].Properties["collection"] = collection
		}
	}

	// 5. Upsert into KG (or log in dry-run mode).
	if opts.DryRun {
		s.logger.Info("[FileKGSync] Dry-run: would upsert entities",
			"path", path,
			"nodes", len(nodes),
			"edges", len(edges))
	} else {
		if err := s.kg.BulkMergeExtractedEntities(nodes, edges); err != nil {
			s.logger.Error("[FileKGSync] Failed to bulk-merge entities", "path", path, "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("bulk merge %s: %v", path, err))
			return result
		}
		s.logger.Info("[FileKGSync] Upserted entities", "path", path, "nodes", len(nodes), "edges", len(edges))
	}

	result.FilesProcessed++
	result.NodesExtracted += len(nodes)
	result.EdgesExtracted += len(edges)
	return result
}

// CleanupFile removes KG nodes and edges that were extracted from the given file.
// It uses the lightweight source_file property annotation for evidence mapping.
func (s *FileKGSyncer) CleanupFile(path, collection string, dryRun bool) FileKGSyncResult {
	var result FileKGSyncResult
	if s.kg == nil {
		s.logger.Warn("[FileKGSync] KG not available, skipping cleanup", "path", path)
		return result
	}

	if dryRun {
		s.logger.Info("[FileKGSync] Dry-run: would cleanup file entities", "path", path, "collection", collection)
		return result
	}

	deletedNodes, err := s.kg.DeleteNodesBySourceFile(path)
	if err != nil {
		s.logger.Warn("[FileKGSync] Failed to delete nodes by source file", "path", path, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("delete nodes for %s: %v", path, err))
	}
	deletedEdges, err := s.kg.DeleteEdgesBySourceFile(path)
	if err != nil {
		s.logger.Warn("[FileKGSync] Failed to delete edges by source file", "path", path, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("delete edges for %s: %v", path, err))
	}

	s.logger.Info("[FileKGSync] Cleaned up file entities", "path", path,
		"deleted_nodes", deletedNodes, "deleted_edges", deletedEdges)
	result.NodesExtracted = deletedNodes
	result.EdgesExtracted = deletedEdges
	return result
}

// FindOrphans returns KG nodes and edges whose source_file no longer matches
// any tracked file in the STM. This is a lightweight orphan detection based on
// the source_file property annotation.
func (s *FileKGSyncer) FindOrphans() ([]memory.Node, []memory.Edge, error) {
	if s.kg == nil || s.stm == nil {
		return nil, nil, fmt.Errorf("KG or STM not available")
	}

	// Gather all active tracked files across all collections.
	collections := s.discoverCollections()
	if len(collections) == 0 {
		collections = []string{IndexerCollection}
	}
	activeSet := make(map[string]struct{})
	for _, col := range collections {
		files, err := s.stm.ListIndexedFiles(col)
		if err != nil {
			s.logger.Warn("[FileKGSync] Failed to list indexed files for orphan check", "collection", col, "error", err)
			continue
		}
		for _, f := range files {
			activeSet[f] = struct{}{}
		}
	}

	var activeFiles []string
	for f := range activeSet {
		activeFiles = append(activeFiles, f)
	}
	return s.kg.FindOrphanedFileSyncEntities(activeFiles)
}

// resolveFileContent reconstructs the full text content of a file from its
// tracked VectorDB document chunks.
func (s *FileKGSyncer) resolveFileContent(path, collection string) (string, error) {
	docIDs, err := s.stm.GetFileEmbeddingDocIDs(path, collection)
	if err != nil {
		return "", fmt.Errorf("get doc ids: %w", err)
	}
	if len(docIDs) == 0 {
		return "", fmt.Errorf("no tracked doc ids for file")
	}

	var parts []string
	for _, docID := range docIDs {
		var content string
		var getErr error
		if collection != "" && s.vectorDB != nil {
			content, getErr = s.vectorDB.GetByIDFromCollection(docID, collection)
		} else if s.vectorDB != nil {
			content, getErr = s.vectorDB.GetByID(docID)
		}
		if getErr != nil {
			// Best-effort: skip missing chunks rather than failing entirely.
			s.logger.Debug("[FileKGSync] Skipping missing chunk", "doc_id", docID, "error", getErr)
			continue
		}
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n"), nil
}

// maxContentBytes limits the prepared content sent to the LLM for KG extraction.
// Approximately 8000 characters ≈ 2000 tokens, leaving room for the prompt and response.
const maxContentBytes = 8000

// headingRe matches Markdown ATX headings (# through ######).
var headingRe = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

// multiBlankLineRe matches 3 or more consecutive blank lines.
var multiBlankLineRe = regexp.MustCompile(`\n{3,}`)

// formFeedRe matches form-feed and vertical-tab characters common in PDF extractions.
var formFeedRe = regexp.MustCompile(`[\x0b\x0c]`)

// prepareContentForExtraction applies file-type-specific content preparation before
// passing text to the KG extraction LLM. It is a pure text transformation that
// does not invoke any LLM calls.
//
// Strategy:
//   - Markdown (.md): Extract heading outline as structural context prefix.
//   - PDF / DOCX (.pdf, .docx): Clean extraction artifacts (form-feeds, excess whitespace).
//   - All types: Normalize whitespace and truncate to maxContentBytes if needed.
func prepareContentForExtraction(filePath, content string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".md":
		content = prepareMarkdownContent(content)
	case ".pdf", ".docx":
		content = prepareDocumentExtractionContent(content)
	}

	// Universal cleanup: collapse excessive blank lines.
	content = multiBlankLineRe.ReplaceAllString(content, "\n\n")

	// Truncate if content exceeds the limit.
	if len(content) > maxContentBytes {
		content = content[:maxContentBytes] + "\n\n[... content truncated for extraction ...]"
	}

	return strings.TrimSpace(content)
}

// prepareMarkdownContent extracts the heading outline from Markdown content and
// prepends it as structural context, helping the LLM understand document organization.
func prepareMarkdownContent(content string) string {
	matches := headingRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content
	}

	var outline strings.Builder
	outline.WriteString("[Document Structure]\n")
	for _, m := range matches {
		level := len(m[1]) // number of # characters
		title := strings.TrimSpace(m[2])
		indent := strings.Repeat("  ", level-1)
		outline.WriteString(fmt.Sprintf("%s- %s\n", indent, title))
	}
	outline.WriteString("\n[Content]\n")

	return outline.String() + content
}

// prepareDocumentExtractionContent cleans common artifacts from PDF/DOCX text extraction,
// such as form-feed characters, excessive whitespace, and broken line breaks.
func prepareDocumentExtractionContent(content string) string {
	// Remove form-feed and vertical-tab characters.
	content = formFeedRe.ReplaceAllString(content, " ")

	// Collapse runs of spaces (common in PDF column layouts).
	content = regexp.MustCompile(` {3,}`).ReplaceAllString(content, " ")

	return content
}
