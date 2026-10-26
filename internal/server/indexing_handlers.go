package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/chunking"
	"aurago/internal/config"

	"gopkg.in/yaml.v3"
)

// handleIndexingStatus returns the current indexer status.
// GET /api/indexing/status
func handleIndexingStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		chunkingStrategy := indexingChunkingStrategy(s)
		indexedDocuments := 0
		fileIndexer := s.fileIndexer()
		if fileIndexer == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled":           false,
				"chunking_strategy": chunkingStrategy,
				"indexed_documents": indexedDocuments,
			})
			return
		}

		status := fileIndexer.Status()
		chunkingStrategy = status.ChunkingStrategy
		indexedDocuments = status.IndexedDocuments
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":           true,
			"chunking_strategy": chunkingStrategy,
			"indexed_documents": indexedDocuments,
			"status":            status,
		})
	}
}

func indexingChunkingStrategy(s *Server) string {
	if s == nil || s.Cfg == nil {
		return chunking.DefaultStrategy
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	cfg := s.Cfg.Indexing.Chunking
	return chunking.NormalizeOptionsWithDefaults(chunking.Options{
		Strategy:     cfg.Strategy,
		MaxChars:     cfg.MaxChars,
		OverlapChars: cfg.OverlapChars,
		MaxChunks:    cfg.MaxChunksPerFile,
	}).Strategy
}

// handleIndexingRescan triggers an immediate rescan of all directories.
// POST /api/indexing/rescan
func handleIndexingRescan(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fileIndexer := s.fileIndexer()
		if fileIndexer == nil {
			jsonError(w, "Indexer not running", http.StatusServiceUnavailable)
			return
		}

		fileIndexer.Rescan()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Rescan gestartet"})
	}
}

// handleIndexingDirectories manages the list of indexed directories.
// GET  /api/indexing/directories → returns list; every directory carries its
// allow_media_analysis opt-in and "media_analysis_available" says whether a
// global capability (indexing.index_images or embeddings.multimodal) is on
// POST /api/indexing/directories → adds a directory {"path": "..."}; a new
// directory never has the media opt-in, it can only be enabled through PATCH
// PATCH /api/indexing/directories → sets the per-folder media opt-in
// {"path": "...", "allow_media_analysis": bool}
// DELETE /api/indexing/directories → removes a directory {"path": "..."}
func handleIndexingDirectories(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.CfgMu.RLock()
			dirs := append([]config.IndexingDirectory(nil), s.Cfg.Indexing.Directories...)
			mediaAvailable := s.Cfg.Indexing.IndexImages || s.Cfg.Embeddings.Multimodal
			s.CfgMu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"directories":              dirs,
				"media_analysis_available": mediaAvailable,
			})

		case http.MethodPatch:
			var req struct {
				Path               string `json:"path"`
				AllowMediaAnalysis *bool  `json:"allow_media_analysis"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Path) == "" {
				jsonError(w, "path is required", http.StatusBadRequest)
				return
			}
			if req.AllowMediaAnalysis == nil {
				jsonError(w, "allow_media_analysis is required", http.StatusBadRequest)
				return
			}
			allow := *req.AllowMediaAnalysis

			var absPath string
			if rejected := editIndexingDirectories(s, func(configDir string, dirs []config.IndexingDirectory) ([]config.IndexingDirectory, *indexingEditError) {
				absPath = resolveIndexingRequestPath(configDir, req.Path)
				found := false
				// Every entry of the path gets the flag: a duplicate must not keep the old permission.
				for i := range dirs {
					if sameIndexingPath(dirs[i].Path, absPath) {
						dirs[i].AllowMediaAnalysis = allow
						found = true
					}
				}
				if !found {
					return nil, &indexingEditError{status: http.StatusNotFound, message: "Verzeichnis nicht gefunden"}
				}
				return dirs, nil
			}); rejected != nil {
				jsonError(w, rejected.message, rejected.status)
				return
			}
			s.Logger.Info("[Indexer] Media analysis permission changed", "dir", absPath, "allow_media_analysis", allow)

			// The changed permission applies to the files of this directory right away.
			if fi := s.fileIndexer(); fi != nil {
				fi.Rescan()
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodPost:
			var req struct {
				Path       string `json:"path"`
				Collection string `json:"collection"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Path) == "" {
				jsonError(w, "path is required", http.StatusBadRequest)
				return
			}

			var absPath string
			if rejected := editIndexingDirectories(s, func(configDir string, dirs []config.IndexingDirectory) ([]config.IndexingDirectory, *indexingEditError) {
				// Resolve path relative to config dir
				absPath = resolveIndexingRequestPath(configDir, req.Path)
				if isRootIndexingPath(absPath) {
					return nil, &indexingEditError{status: http.StatusBadRequest, message: "Root directories cannot be indexed"}
				}

				// Check for duplicates
				for _, d := range dirs {
					if sameIndexingPath(d.Path, absPath) {
						return nil, &indexingEditError{status: http.StatusConflict, message: "Verzeichnis bereits in der Liste"}
					}
				}

				// A new directory never has the media opt-in; the request cannot carry it.
				return append(dirs, config.IndexingDirectory{
					Path:       absPath,
					Collection: strings.TrimSpace(req.Collection),
				}), nil
			}); rejected != nil {
				jsonError(w, rejected.message, rejected.status)
				return
			}

			// Create directory if it doesn't exist
			if err := os.MkdirAll(absPath, 0755); err != nil {
				s.Logger.Warn("[Indexer] Failed to create directory", "path", absPath, "error", err)
			}

			// Trigger rescan
			if fi := s.fileIndexer(); fi != nil {
				fi.Rescan()
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			var req struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Path) == "" {
				jsonError(w, "path is required", http.StatusBadRequest)
				return
			}

			var removed []config.IndexingDirectory
			if rejected := editIndexingDirectories(s, func(configDir string, dirs []config.IndexingDirectory) ([]config.IndexingDirectory, *indexingEditError) {
				absPath := resolveIndexingRequestPath(configDir, req.Path)
				kept := make([]config.IndexingDirectory, 0, len(dirs))
				for _, d := range dirs {
					if sameIndexingPath(d.Path, absPath) {
						removed = append(removed, d)
						continue
					}
					kept = append(kept, d)
				}
				if len(removed) == 0 {
					return nil, &indexingEditError{status: http.StatusNotFound, message: "Verzeichnis nicht gefunden"}
				}
				return kept, nil
			}); rejected != nil {
				jsonError(w, rejected.message, rejected.status)
				return
			}

			cleanupErrors := []string{}
			if fi := s.fileIndexer(); fi != nil {
				for _, dir := range removed {
					if errs := fi.CleanupDirectory(dir); len(errs) > 0 {
						s.Logger.Warn("[Indexer] Directory cleanup after removal produced errors", "path", dir.Path, "errors", errs)
						cleanupErrors = append(cleanupErrors, errs...)
					}
				}
				fi.Rescan()
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "cleanup_errors": len(cleanupErrors)})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// indexingEditError is a rejected directory edit: the HTTP status and message to answer with.
type indexingEditError struct {
	status  int
	message string
}

// editIndexingDirectories runs one read-modify-persist-update cycle on the list of
// indexed directories. The whole cycle holds s.CfgSaveMu, the lock that serializes
// config writers and is always taken before CfgMu. The list handed to mutate is read
// inside that lock, so a PATCH, POST or DELETE that raced another edit starts from its
// result instead of persisting an older copy that would write a removed entry (and its
// media permission) back. Only after the YAML was written does the runtime config
// follow, under CfgMu, and the running indexer is pointed at the very object that was
// updated. mutate gets a private copy of the list and the config directory; it returns
// the new list or the rejection to answer with.
func editIndexingDirectories(s *Server, mutate func(configDir string, dirs []config.IndexingDirectory) ([]config.IndexingDirectory, *indexingEditError)) *indexingEditError {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()

	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	enabled := s.Cfg.Indexing.Enabled
	current := append([]config.IndexingDirectory(nil), s.Cfg.Indexing.Directories...)
	s.CfgMu.RUnlock()

	next, rejected := mutate(filepath.Dir(configPath), current)
	if rejected != nil {
		return rejected
	}
	if err := patchIndexingDirsLocked(s, next, enabled); err != nil {
		s.Logger.Error("[Indexer] Failed to persist directory change", "error", err)
		return &indexingEditError{status: http.StatusInternalServerError, message: "Failed to save config"}
	}

	// Update the runtime config only after successful persistence.
	s.CfgMu.Lock()
	s.Cfg.Indexing.Directories = next
	if fi := s.fileIndexer(); fi != nil {
		fi.ReplaceConfigLocked(s.Cfg)
	}
	s.CfgMu.Unlock()
	return nil
}

// patchIndexingDirsLocked persists indexing directories to config.yaml, including each
// directory's allow_media_analysis opt-in. The caller must hold s.CfgSaveMu.
func patchIndexingDirsLocked(s *Server, dirs []config.IndexingDirectory, enabled bool) error {
	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		return err
	}

	// Build relative paths for storage in YAML (cleaner than absolute)
	configDir := filepath.Dir(configPath)

	relDirs := make([]interface{}, len(dirs))
	for i, d := range dirs {
		rel, err := filepath.Rel(configDir, d.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			relDirs[i] = d // keep absolute if can't relativize
		} else {
			relDirs[i] = config.IndexingDirectory{
				Path:               "./" + filepath.ToSlash(rel),
				Collection:         d.Collection,
				AllowMediaAnalysis: d.AllowMediaAnalysis,
			}
		}
	}

	// Ensure the indexing section exists
	indexing, ok := rawCfg["indexing"].(map[string]interface{})
	if !ok {
		indexing = make(map[string]interface{})
		rawCfg["indexing"] = indexing
	}
	indexing["directories"] = relDirs
	indexing["enabled"] = enabled

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}

	return config.WriteFileAtomic(configPath, out, 0o600)
}
