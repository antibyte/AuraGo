package server

import (
	"aurago/internal/config"
	"aurago/internal/memory"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type embeddingsResetMarker struct {
	RequestedAt time.Time `json:"requested_at"`
	Reason      string    `json:"reason"`
}

func embeddingsResetMarkerPath(cfg *config.Config) string {
	return filepath.Join(cfg.Directories.DataDir, "embeddings_reset_pending.json")
}

// WriteEmbeddingsResetMarker records that the embedding store must be rebuilt
// on the next process start. We do not delete the live VectorDB in-process
// because many running components still hold references to the current DB.
func WriteEmbeddingsResetMarker(cfg *config.Config, logger *slog.Logger, reason string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if err := os.MkdirAll(cfg.Directories.DataDir, 0750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	marker := embeddingsResetMarker{
		RequestedAt: time.Now().UTC(),
		Reason:      reason,
	}
	raw, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal marker: %w", err)
	}
	path := embeddingsResetMarkerPath(cfg)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}
	if logger != nil {
		logger.Warn("[Embeddings] Reset scheduled for next restart", "path", path, "reason", reason)
	}
	return nil
}

// ApplyPendingEmbeddingsReset clears the old embedding store and related SQLite
// tracking metadata before the new VectorDB instance is created.
func ApplyPendingEmbeddingsReset(cfg *config.Config, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, logger *slog.Logger) (bool, error) {
	if cfg == nil {
		return false, fmt.Errorf("config is required")
	}
	if stm == nil {
		return false, fmt.Errorf("short-term memory is required")
	}

	markerPath := embeddingsResetMarkerPath(cfg)
	if _, err := os.Stat(markerPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat reset marker: %w", err)
	}

	vectorDir, err := validateEmbeddingsVectorDir(cfg)
	if err != nil {
		return false, err
	}
	if err := os.RemoveAll(vectorDir); err != nil {
		return false, fmt.Errorf("remove vector db dir: %w", err)
	}
	if err := os.MkdirAll(vectorDir, 0750); err != nil {
		return false, fmt.Errorf("recreate vector db dir: %w", err)
	}
	if err := stm.ClearFileIndices(); err != nil {
		return false, fmt.Errorf("clear file indices: %w", err)
	}
	if err := stm.ClearMemoryMeta(); err != nil {
		return false, fmt.Errorf("clear memory meta: %w", err)
	}
	resetKGFileSyncEntities(kg, logger)

	if err := resetKGSemanticIndex(cfg, kg, logger); err != nil {
		return false, fmt.Errorf("reset KG semantic index: %w", err)
	}
	if err := os.Remove(markerPath); err != nil {
		return false, fmt.Errorf("remove reset marker: %w", err)
	}

	if logger != nil {
		logger.Warn("[Embeddings] Applied pending embeddings reset", "vector_dir", vectorDir)
	}
	return true, nil
}

func validateEmbeddingsVectorDir(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}
	vectorDir := strings.TrimSpace(cfg.Directories.VectorDBDir)
	if vectorDir == "" {
		return "", fmt.Errorf("vector db dir is empty")
	}
	absVectorDir, err := filepath.Abs(vectorDir)
	if err != nil {
		return "", fmt.Errorf("resolve vector db dir: %w", err)
	}
	absVectorDir = filepath.Clean(absVectorDir)
	if isFilesystemRoot(absVectorDir) {
		return "", fmt.Errorf("refusing to remove filesystem root as vector db dir: %s", absVectorDir)
	}

	var roots []string
	if dataDir := strings.TrimSpace(cfg.Directories.DataDir); dataDir != "" {
		if absDataDir, err := filepath.Abs(dataDir); err == nil {
			roots = append(roots, filepath.Clean(absDataDir))
		}
	}
	if configPath := strings.TrimSpace(cfg.ConfigPath); configPath != "" {
		if absConfigPath, err := filepath.Abs(configPath); err == nil {
			roots = append(roots, filepath.Dir(filepath.Clean(absConfigPath)))
		}
	}
	for _, root := range roots {
		if pathWithinRoot(absVectorDir, root) {
			return absVectorDir, nil
		}
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("cannot validate vector db dir without data dir or config path")
	}
	return "", fmt.Errorf("vector db dir %s is outside configured app roots", absVectorDir)
}

func isFilesystemRoot(path string) bool {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	root := volume + string(filepath.Separator)
	return clean == root || clean == string(filepath.Separator)
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func resetKGFileSyncEntities(kg *memory.KnowledgeGraph, logger *slog.Logger) {
	if kg == nil {
		return
	}

	nodes, edges, err := kg.FindOrphanedFileSyncEntities(nil)
	if err != nil {
		if logger != nil {
			logger.Warn("[Embeddings] Failed to find file-sync KG entities for reset", "error", err)
		}
		return
	}

	sourceFiles := make(map[string]struct{})
	edgeSourceFiles := make(map[string]struct{})
	for _, node := range nodes {
		if node.Properties == nil {
			continue
		}
		if sourceFile := strings.TrimSpace(node.Properties["source_file"]); sourceFile != "" {
			sourceFiles[sourceFile] = struct{}{}
		}
	}
	for _, edge := range edges {
		if edge.Properties == nil {
			continue
		}
		if sourceFile := strings.TrimSpace(edge.Properties["source_file"]); sourceFile != "" {
			sourceFiles[sourceFile] = struct{}{}
			edgeSourceFiles[sourceFile] = struct{}{}
		}
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

	deletedNodes := 0
	deletedEdges := 0
	for _, sourceFile := range paths {
		edgesDeleted, err := kg.DeleteEdgesBySourceFile(sourceFile)
		if err != nil && logger != nil {
			logger.Warn("[Embeddings] Failed to delete file-sync KG edges during reset", "source_file", sourceFile, "error", err)
		}
		nodesDeleted, err := kg.DeleteNodesBySourceFile(sourceFile)
		if err != nil && logger != nil {
			logger.Warn("[Embeddings] Failed to delete file-sync KG nodes during reset", "source_file", sourceFile, "error", err)
		}
		deletedEdges += edgesDeleted
		deletedNodes += nodesDeleted
	}

	if logger != nil && (deletedNodes > 0 || deletedEdges > 0) {
		logger.Warn("[Embeddings] Removed stale file-sync KG entities during reset",
			"nodes", deletedNodes, "edges", deletedEdges, "source_files", len(paths))
	}
}

func resetKGSemanticIndex(cfg *config.Config, kg *memory.KnowledgeGraph, logger *slog.Logger) error {
	// Use the provided KnowledgeGraph directly
	if kg == nil {
		if logger != nil {
			logger.Warn("[Embeddings] KG not available for semantic reset")
		}
		return nil
	}

	if err := kg.ResetSemanticIndex(); err != nil {
		if logger != nil {
			logger.Warn("[Embeddings] Failed to reset KG semantic index", "error", err)
		}
		return err
	}

	if logger != nil {
		logger.Info("[Embeddings] KG semantic index reset — will be rebuilt with new model")
	}
	return nil
}

func embeddingsConfigChanged(oldCfg, newCfg config.Config) bool {
	return oldCfg.Embeddings.Provider != newCfg.Embeddings.Provider ||
		oldCfg.Embeddings.ProviderType != newCfg.Embeddings.ProviderType ||
		oldCfg.Embeddings.BaseURL != newCfg.Embeddings.BaseURL ||
		oldCfg.Embeddings.Model != newCfg.Embeddings.Model ||
		oldCfg.Embeddings.InternalModel != newCfg.Embeddings.InternalModel ||
		oldCfg.Embeddings.ExternalURL != newCfg.Embeddings.ExternalURL ||
		oldCfg.Embeddings.ExternalModel != newCfg.Embeddings.ExternalModel ||
		oldCfg.Embeddings.Multimodal != newCfg.Embeddings.Multimodal ||
		oldCfg.Embeddings.MultimodalFormat != newCfg.Embeddings.MultimodalFormat ||
		oldCfg.Embeddings.LocalOllama != newCfg.Embeddings.LocalOllama
}

func handleEmbeddingsReset(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfgCopy := *s.Cfg
		s.CfgMu.RUnlock()

		if err := WriteEmbeddingsResetMarker(&cfgCopy, s.Logger, "config_ui_embedding_change"); err != nil {
			s.Logger.Error("[Embeddings] Failed to schedule reset", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Embeddings-Reset konnte nicht geplant werden.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Embeddings-Reset für den nächsten Neustart geplant.",
		})
	}
}
