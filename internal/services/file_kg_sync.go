package services

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	syncFile  func(path, collection string, opts FileKGSyncOptions) FileKGSyncResult
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

	if opts.MaxFiles > 0 && len(files) > opts.MaxFiles {
		files = files[:opts.MaxFiles]
	}

	workerCount := s.fileSyncWorkerCount(len(files))
	if workerCount <= 1 {
		for _, path := range files {
			fileResult := s.runSyncFile(path, collection, opts)
			result.FilesProcessed += fileResult.FilesProcessed
			result.FilesSkipped += fileResult.FilesSkipped
			result.NodesExtracted += fileResult.NodesExtracted
			result.EdgesExtracted += fileResult.EdgesExtracted
			result.Errors = append(result.Errors, fileResult.Errors...)
		}
		return result
	}

	jobs := make(chan string)
	results := make(chan FileKGSyncResult, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				results <- s.runSyncFile(path, collection, opts)
			}
		}()
	}

	go func() {
		for _, path := range files {
			jobs <- path
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for fileResult := range results {
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
	// Track original length before preparation for confidence scoring.
	originalLength := len(content)
	wasTruncated := originalLength > maxContentBytes
	segments := prepareContentSegmentsForExtraction(path, content)
	if len(segments) == 0 {
		result.FilesSkipped++
		s.logger.Debug("[FileKGSync] Skipping file: no extractable prepared content", "path", path)
		return result
	}

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
	var nodes []memory.Node
	var edges []memory.Edge
	for i, segment := range segments {
		segmentNodes, segmentEdges, err := kgextraction.ExtractKGFromText(s.cfg, s.logger, s.llmClient, segment, existingNodesString)
		if err != nil {
			s.logger.Warn("[FileKGSync] KG extraction failed", "path", path, "segment", i+1, "segments", len(segments), "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("extraction %s segment %d/%d: %v", path, i+1, len(segments), err))
			continue
		}
		nodes = append(nodes, segmentNodes...)
		edges = append(edges, segmentEdges...)
	}
	nodes = mergeNodesByID(nodes)
	edges = mergeEdgesByKey(edges)
	if len(result.Errors) > 0 && len(nodes) == 0 && len(edges) == 0 {
		return result
	}
	if len(nodes) == 0 && len(edges) == 0 {
		result.FilesSkipped++
		s.logger.Debug("[FileKGSync] No entities extracted", "path", path)
		return result
	}

	// 3b. Compute extraction confidence based on heuristics.
	confidenceScore := kgextraction.ComputeConfidence(kgextraction.ConfidenceInput{
		SourceType:    "file_sync",
		FilePath:      path,
		ContentLength: originalLength,
		NodeCount:     len(nodes),
		EdgeCount:     len(edges),
		WasTruncated:  wasTruncated,
	})
	confidenceStr := kgextraction.FormatConfidence(confidenceScore)

	// 4. Annotate with source metadata and confidence (minimal evidence link for first draft).
	now := time.Now().Format("2006-01-02")
	for i := range nodes {
		if nodes[i].Properties == nil {
			nodes[i].Properties = make(map[string]string)
		}
		nodes[i].Properties["source"] = "file_sync"
		nodes[i].Properties["source_file"] = path
		nodes[i].Properties["extracted_at"] = now
		nodes[i].Properties["confidence"] = confidenceStr
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
		edges[i].Properties["confidence"] = confidenceStr
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
		deletedEdges, err := s.kg.DeleteEdgesBySourceFile(path)
		if err != nil {
			s.logger.Error("[FileKGSync] Failed to replace stale file edges before reindex", "path", path, "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("replace stale edges %s: %v", path, err))
			return result
		}
		deletedNodes, err := s.kg.DeleteNodesBySourceFile(path)
		if err != nil {
			s.logger.Error("[FileKGSync] Failed to replace stale file nodes before reindex", "path", path, "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("replace stale nodes %s: %v", path, err))
			return result
		}
		if deletedNodes > 0 || deletedEdges > 0 {
			s.logger.Info("[FileKGSync] Replacing stale file entities before reindex",
				"path", path, "deleted_nodes", deletedNodes, "deleted_edges", deletedEdges)
		}
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

func (s *FileKGSyncer) runSyncFile(path, collection string, opts FileKGSyncOptions) FileKGSyncResult {
	if s.syncFile != nil {
		return s.syncFile(path, collection, opts)
	}
	return s.SyncFile(path, collection, opts)
}

func (s *FileKGSyncer) fileSyncWorkerCount(fileCount int) int {
	if fileCount <= 1 {
		return fileCount
	}
	workerCount := 4
	if fileCount < workerCount {
		workerCount = fileCount
	}
	return workerCount
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

	deletedEdges, err := s.kg.DeleteEdgesBySourceFile(path)
	if err != nil {
		s.logger.Warn("[FileKGSync] Failed to delete edges by source file", "path", path, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("delete edges for %s: %v", path, err))
	}
	deletedNodes, err := s.kg.DeleteNodesBySourceFile(path)
	if err != nil {
		s.logger.Warn("[FileKGSync] Failed to delete nodes by source file", "path", path, "error", err)
		result.Errors = append(result.Errors, fmt.Sprintf("delete nodes for %s: %v", path, err))
	}

	s.logger.Info("[FileKGSync] Cleaned up file entities", "path", path,
		"deleted_nodes", deletedNodes, "deleted_edges", deletedEdges)
	result.NodesExtracted = deletedNodes
	result.EdgesExtracted = deletedEdges
	return result
}

// CleanupOrphans removes file_sync KG entities whose source_file is no longer
// tracked by STM. It covers rename/delete/reset cleanup after the file index
// has changed.
func (s *FileKGSyncer) CleanupOrphans(dryRun bool) FileKGSyncResult {
	var result FileKGSyncResult
	orphanNodes, orphanEdges, err := s.FindOrphans()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("find orphans: %v", err))
		return result
	}

	sourceFiles := make(map[string]struct{})
	edgeSourceFiles := make(map[string]struct{})
	for _, node := range orphanNodes {
		if node.Properties != nil {
			if sourceFile := strings.TrimSpace(node.Properties["source_file"]); sourceFile != "" {
				sourceFiles[sourceFile] = struct{}{}
			}
		}
	}
	for _, edge := range orphanEdges {
		if edge.Properties != nil {
			if sourceFile := strings.TrimSpace(edge.Properties["source_file"]); sourceFile != "" {
				sourceFiles[sourceFile] = struct{}{}
				edgeSourceFiles[sourceFile] = struct{}{}
			}
		}
	}
	if len(sourceFiles) == 0 {
		return result
	}

	paths := make([]string, 0, len(sourceFiles))
	for sourceFile := range sourceFiles {
		paths = append(paths, sourceFile)
	}
	sort.Slice(paths, func(i, j int) bool {
		_, iHasEdges := edgeSourceFiles[paths[i]]
		_, jHasEdges := edgeSourceFiles[paths[j]]
		if iHasEdges != jHasEdges {
			return iHasEdges
		}
		return paths[i] < paths[j]
	})

	if dryRun {
		result.FilesProcessed = len(paths)
		result.NodesExtracted = len(orphanNodes)
		result.EdgesExtracted = len(orphanEdges)
		s.logger.Info("[FileKGSync] Dry-run: would cleanup orphan file entities",
			"files", len(paths), "nodes", len(orphanNodes), "edges", len(orphanEdges))
		return result
	}

	for _, path := range paths {
		cleanup := s.CleanupFile(path, "", false)
		result.FilesProcessed++
		result.NodesExtracted += cleanup.NodesExtracted
		result.EdgesExtracted += cleanup.EdgesExtracted
		result.Errors = append(result.Errors, cleanup.Errors...)
	}
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
// Approximately 32000 characters keeps broad document coverage while leaving
// room for prompt context and response on common helper models.
const maxContentBytes = 32000

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
	segments := prepareContentSegmentsForExtraction(filePath, content)
	if len(segments) == 0 {
		return ""
	}
	return segments[0]
}

func prepareContentSegmentsForExtraction(filePath, content string) []string {
	content = strings.TrimSpace(content)
	if isGenericMultimodalPlaceholder(content) {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".md":
		content = prepareMarkdownContent(content)
	case ".pdf", ".docx":
		content = prepareDocumentExtractionContent(content)
	}

	// Universal cleanup: collapse excessive blank lines.
	content = multiBlankLineRe.ReplaceAllString(content, "\n\n")

	if len(content) <= maxContentBytes {
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []string{strings.TrimSpace(content)}
	}

	return representativeExtractionSegments(content, maxContentBytes)
}

func representativeExtractionSegments(content string, segmentBytes int) []string {
	if segmentBytes <= 0 || len(content) <= segmentBytes {
		return []string{strings.TrimSpace(content)}
	}
	start := truncateUTF8(content, segmentBytes)
	midStart := (len(content) - segmentBytes) / 2
	midStart = adjustUTF8Boundary(content, midStart)
	middle := truncateUTF8(content[midStart:], segmentBytes)
	endStart := len(content) - segmentBytes
	endStart = adjustUTF8Boundary(content, endStart)
	end := content[endStart:]
	return []string{
		strings.TrimSpace(start) + "\n\n[... content truncated for extraction ...]\n[beginning segment]",
		"[... middle segment ...]\n\n" + strings.TrimSpace(middle),
		"[... ending segment ...]\n\n" + strings.TrimSpace(end),
	}
}

func isGenericMultimodalPlaceholder(content string) bool {
	return strings.HasPrefix(content, "Bild-Datei: ") ||
		strings.HasPrefix(content, "Audio-Datei: ") ||
		strings.HasPrefix(content, "PDF (gescannt): ")
}

func truncateUTF8(content string, maxBytes int) string {
	if len(content) <= maxBytes {
		return content
	}
	for maxBytes > 0 && !utf8.ValidString(content[:maxBytes]) {
		maxBytes--
	}
	return content[:maxBytes]
}

func adjustUTF8Boundary(content string, index int) int {
	if index <= 0 {
		return 0
	}
	if index >= len(content) {
		return len(content)
	}
	for index < len(content) && !utf8.RuneStart(content[index]) {
		index++
	}
	return index
}

func mergeNodesByID(nodes []memory.Node) []memory.Node {
	merged := make(map[string]memory.Node, len(nodes))
	for _, node := range nodes {
		if strings.TrimSpace(node.ID) == "" {
			continue
		}
		existing, ok := merged[node.ID]
		if !ok {
			merged[node.ID] = node
			continue
		}
		if strings.TrimSpace(existing.Label) == "" {
			existing.Label = node.Label
		}
		if existing.Properties == nil {
			existing.Properties = make(map[string]string)
		}
		for key, value := range node.Properties {
			if strings.TrimSpace(existing.Properties[key]) == "" && strings.TrimSpace(value) != "" {
				existing.Properties[key] = value
			}
		}
		merged[node.ID] = existing
	}
	out := make([]memory.Node, 0, len(merged))
	for _, node := range merged {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func mergeEdgesByKey(edges []memory.Edge) []memory.Edge {
	merged := make(map[string]memory.Edge, len(edges))
	for _, edge := range edges {
		if strings.TrimSpace(edge.Source) == "" || strings.TrimSpace(edge.Target) == "" || strings.TrimSpace(edge.Relation) == "" {
			continue
		}
		key := edge.Source + "\x00" + edge.Target + "\x00" + edge.Relation
		existing, ok := merged[key]
		if !ok {
			merged[key] = edge
			continue
		}
		if existing.Properties == nil {
			existing.Properties = make(map[string]string)
		}
		for propKey, value := range edge.Properties {
			if strings.TrimSpace(existing.Properties[propKey]) == "" && strings.TrimSpace(value) != "" {
				existing.Properties[propKey] = value
			}
		}
		merged[key] = existing
	}
	out := make([]memory.Edge, 0, len(merged))
	for _, edge := range merged {
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Source + "\x00" + out[i].Target + "\x00" + out[i].Relation
		right := out[j].Source + "\x00" + out[j].Target + "\x00" + out[j].Relation
		return left < right
	})
	return out
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
