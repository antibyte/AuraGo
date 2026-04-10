package services

import (
	"fmt"
	"log/slog"
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
		collections = []string{indexerCollection}
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
