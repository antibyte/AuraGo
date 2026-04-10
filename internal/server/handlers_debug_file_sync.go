package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/memory"
	"aurago/internal/services"
)

// handleDebugKGFileSyncStats returns KG statistics filtered to file_sync source.
// GET /api/debug/kg-file-sync-stats
func handleDebugKGFileSyncStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		stats, err := s.KG.GetFileSyncStats()
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to get KG file sync stats", "error", err)
			}
			jsonError(w, "Failed to retrieve KG statistics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// handleDebugKGOrphans returns orphaned KG nodes that have no edges.
// GET /api/debug/kg-orphans?source=file_sync
func handleDebugKGOrphans(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
		if sourceFilter == "" {
			sourceFilter = "file_sync"
		}

		// Get active files from STM for orphan detection
		activeFiles, err := s.getActiveIndexedFiles()
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Failed to get active indexed files for orphan detection", "error", err)
			}
			activeFiles = []string{}
		}

		orphanNodes, orphanEdges, err := s.KG.FindOrphanedFileSyncEntities(activeFiles)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to find orphaned entities", "error", err)
			}
			jsonError(w, "Failed to find orphaned entities", http.StatusInternalServerError)
			return
		}

		// Filter by source if specified (via source property)
		if sourceFilter != "" {
			var filteredNodes []memory.Node
			for _, n := range orphanNodes {
				if props, ok := n.Properties["source"]; ok && props == sourceFilter {
					filteredNodes = append(filteredNodes, n)
				}
			}
			orphanNodes = filteredNodes

			var filteredEdges []memory.Edge
			for _, e := range orphanEdges {
				if props, ok := e.Properties["source"]; ok && props == sourceFilter {
					filteredEdges = append(filteredEdges, e)
				}
			}
			orphanEdges = filteredEdges
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"source":       sourceFilter,
			"orphan_nodes": orphanNodes,
			"orphan_edges": orphanEdges,
			"total":        len(orphanNodes) + len(orphanEdges),
		})
	}
}

// handleDebugFileSyncStatus returns sync status per collection.
// GET /api/debug/file-sync-status
func handleDebugFileSyncStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"indexer":     nil,
			"collections": []map[string]interface{}{},
		}

		// Get FileIndexer status if available
		if s.FileIndexer != nil {
			status := s.FileIndexer.Status()
			response["indexer"] = map[string]interface{}{
				"running":            status.Running,
				"total_files":        status.TotalFiles,
				"indexed_files":      status.IndexedFiles,
				"last_scan_at":       status.LastScanAt,
				"last_scan_duration": status.LastScanDuration,
				"directories":        status.Directories,
				"errors":             status.Errors,
			}
		} else {
			response["indexer"] = map[string]interface{}{
				"running": false,
			}
		}

		// Get per-collection KG stats if KG is available
		if s.KG != nil {
			collections := s.discoverIndexingCollections()
			var collStats []map[string]interface{}
			for _, coll := range collections {
				stats, err := s.KG.GetCollectionFileSyncStats(coll)
				if err != nil {
					if s.Logger != nil {
						s.Logger.Warn("Failed to get collection stats", "collection", coll, "error", err)
					}
					continue
				}
				collStats = append(collStats, map[string]interface{}{
					"collection":   coll,
					"node_count":   stats.NodeCount,
					"edge_count":   stats.EdgeCount,
					"file_count":   stats.FileCount,
					"last_sync_at": stats.LastSyncAt,
				})
			}
			response["collections"] = collStats
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleDebugFileSyncLastRun returns last sync timestamps.
// GET /api/debug/file-sync-last-run
func handleDebugFileSyncLastRun(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		response := map[string]interface{}{
			"global":         nil,
			"per_collection": []map[string]interface{}{},
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		// Get global last sync time (max of all extracted_at values)
		globalLastSync, err := s.KG.GetLastFileSyncTime("")
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Failed to get global last sync time", "error", err)
			}
		}
		response["global"] = globalLastSync

		// Get per-collection last sync times
		collections := s.discoverIndexingCollections()
		var collTimes []map[string]interface{}
		for _, coll := range collections {
			lastSync, err := s.KG.GetLastFileSyncTime(coll)
			if err != nil {
				if s.Logger != nil {
					s.Logger.Warn("Failed to get collection last sync time", "collection", coll, "error", err)
				}
				continue
			}
			collTimes = append(collTimes, map[string]interface{}{
				"collection":   coll,
				"last_sync_at": lastSync,
			})
		}
		response["per_collection"] = collTimes

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleDebugKGFileEntities returns all KG nodes and edges originating from a specific file.
// GET /api/debug/kg-file-entities?path=<filepath>&limit=100
func handleDebugKGFileEntities(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		filePath := strings.TrimSpace(r.URL.Query().Get("path"))
		if filePath == "" {
			jsonError(w, "Missing required query parameter: path", http.StatusBadRequest)
			return
		}

		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		nodes, err := s.KG.GetNodesBySourceFile(filePath, limit)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to get nodes by source file", "path", filePath, "error", err)
			}
			jsonError(w, "Failed to retrieve nodes", http.StatusInternalServerError)
			return
		}

		edges, err := s.KG.GetEdgesBySourceFile(filePath, limit)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to get edges by source file", "path", filePath, "error", err)
			}
			jsonError(w, "Failed to retrieve edges", http.StatusInternalServerError)
			return
		}

		if nodes == nil {
			nodes = []memory.Node{}
		}
		if edges == nil {
			edges = []memory.Edge{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":       filePath,
			"nodes":      nodes,
			"edges":      edges,
			"node_count": len(nodes),
			"edge_count": len(edges),
		})
	}
}

// handleDebugKGNodeSources returns all source files contributing to a specific KG node.
// GET /api/debug/kg-node-sources?id=<nodeID>&limit=100
func handleDebugKGNodeSources(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		nodeID := strings.TrimSpace(r.URL.Query().Get("id"))
		if nodeID == "" {
			jsonError(w, "Missing required query parameter: id", http.StatusBadRequest)
			return
		}

		limit := 100
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
				limit = parsed
			}
		}

		// Verify the node exists
		node, err := s.KG.GetNode(nodeID)
		if err != nil || node == nil {
			jsonError(w, fmt.Sprintf("Node %q not found", nodeID), http.StatusNotFound)
			return
		}

		files, err := s.KG.GetSourceFilesByNodeID(nodeID, limit)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to get source files for node", "id", nodeID, "error", err)
			}
			jsonError(w, "Failed to retrieve source files", http.StatusInternalServerError)
			return
		}

		if files == nil {
			files = []string{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"node_id":      nodeID,
			"node_label":   node.Label,
			"source_files": files,
			"file_count":   len(files),
		})
	}
}

// handleDebugKGFileSyncCleanup removes orphaned file_sync entities from the KG.
// POST /api/debug/kg-file-sync-cleanup?dry_run=true
// With dry_run=true (default), only reports what would be deleted without actually deleting.
func handleDebugKGFileSyncCleanup(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.KG == nil {
			jsonError(w, "Knowledge graph not available", http.StatusServiceUnavailable)
			return
		}

		// dry_run defaults to true for safety
		dryRun := true
		if dr := strings.ToLower(r.URL.Query().Get("dry_run")); dr == "false" || dr == "0" {
			dryRun = false
		}

		// Get active files from STM for orphan detection
		activeFiles, err := s.getActiveIndexedFiles()
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Failed to get active indexed files for cleanup", "error", err)
			}
			activeFiles = []string{}
		}

		orphanNodes, orphanEdges, err := s.KG.FindOrphanedFileSyncEntities(activeFiles)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Error("Failed to find orphaned entities for cleanup", "error", err)
			}
			jsonError(w, "Failed to find orphaned entities", http.StatusInternalServerError)
			return
		}

		// Collect unique orphaned source files
		orphanFiles := make(map[string]bool)
		for _, n := range orphanNodes {
			if sf, ok := n.Properties["source_file"]; ok && sf != "" {
				orphanFiles[sf] = true
			}
		}
		for _, e := range orphanEdges {
			if sf, ok := e.Properties["source_file"]; ok && sf != "" {
				orphanFiles[sf] = true
			}
		}

		result := map[string]interface{}{
			"dry_run":       dryRun,
			"orphan_nodes":  len(orphanNodes),
			"orphan_edges":  len(orphanEdges),
			"orphan_files":  len(orphanFiles),
			"deleted_nodes": 0,
			"deleted_edges": 0,
		}

		if !dryRun && (len(orphanNodes) > 0 || len(orphanEdges) > 0) {
			var totalDeletedNodes, totalDeletedEdges int

			// Delete orphaned nodes grouped by source file
			for sf := range orphanFiles {
				deleted, err := s.KG.DeleteNodesBySourceFile(sf)
				if err != nil {
					if s.Logger != nil {
						s.Logger.Warn("Failed to delete orphaned nodes", "source_file", sf, "error", err)
					}
					continue
				}
				totalDeletedNodes += deleted

				deletedEdges, err := s.KG.DeleteEdgesBySourceFile(sf)
				if err != nil {
					if s.Logger != nil {
						s.Logger.Warn("Failed to delete orphaned edges", "source_file", sf, "error", err)
					}
					continue
				}
				totalDeletedEdges += deletedEdges
			}

			result["deleted_nodes"] = totalDeletedNodes
			result["deleted_edges"] = totalDeletedEdges
			if s.Logger != nil {
				s.Logger.Info("FileSync cleanup completed",
					"deleted_nodes", totalDeletedNodes,
					"deleted_edges", totalDeletedEdges,
					"orphan_files", len(orphanFiles),
				)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// discoverIndexingCollections returns the set of collections from config directories.
func (s *Server) discoverIndexingCollections() []string {
	seen := make(map[string]bool)
	var collections []string
	s.CfgMu.RLock()
	if s.Cfg != nil {
		for _, dir := range s.Cfg.Indexing.Directories {
			coll := dir.Collection
			if coll == "" {
				coll = services.IndexerCollection
			}
			if !seen[coll] {
				seen[coll] = true
				collections = append(collections, coll)
			}
		}
	}
	s.CfgMu.RUnlock()
	if len(collections) == 0 {
		collections = []string{services.IndexerCollection}
	}
	return collections
}

// getActiveIndexedFiles returns all indexed file paths from STM across all collections.
func (s *Server) getActiveIndexedFiles() ([]string, error) {
	if s.ShortTermMem == nil {
		return []string{}, nil
	}

	collections := s.discoverIndexingCollections()
	var allFiles []string
	seen := make(map[string]bool)

	for _, coll := range collections {
		files, err := s.ShortTermMem.ListIndexedFiles(coll)
		if err != nil {
			continue
		}
		for _, f := range files {
			if !seen[f] {
				seen[f] = true
				allFiles = append(allFiles, f)
			}
		}
	}
	return allFiles, nil
}
