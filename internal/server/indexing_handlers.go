package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

		if s.FileIndexer == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
			})
			return
		}

		status := s.FileIndexer.Status()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": true,
			"status":  status,
		})
	}
}

// handleIndexingRescan triggers an immediate rescan of all directories.
// POST /api/indexing/rescan
func handleIndexingRescan(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.FileIndexer == nil {
			jsonError(w, "Indexer not running", http.StatusServiceUnavailable)
			return
		}

		s.FileIndexer.Rescan()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Rescan gestartet"})
	}
}

// handleIndexingDirectories manages the list of indexed directories.
// GET  /api/indexing/directories → returns list
// POST /api/indexing/directories → adds a directory {"path": "..."}
// DELETE /api/indexing/directories → removes a directory {"path": "..."}
func handleIndexingDirectories(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.CfgMu.RLock()
			dirs := s.Cfg.Indexing.Directories
			s.CfgMu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"directories": dirs})

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

			// Resolve path relative to config dir
			configDir := filepath.Dir(s.Cfg.ConfigPath)
			absPath := resolveIndexingRequestPath(configDir, req.Path)
			if isRootIndexingPath(absPath) {
				jsonError(w, "Root directories cannot be indexed", http.StatusBadRequest)
				return
			}

			// Check for duplicates
			s.CfgMu.RLock()
			for _, d := range s.Cfg.Indexing.Directories {
				if sameIndexingPath(d.Path, absPath) {
					s.CfgMu.RUnlock()
					jsonError(w, "Verzeichnis bereits in der Liste", http.StatusConflict)
					return
				}
			}
			s.CfgMu.RUnlock()

			// Create directory if it doesn't exist
			if err := os.MkdirAll(absPath, 0755); err != nil {
				s.Logger.Warn("[Indexer] Failed to create directory", "path", absPath, "error", err)
			}

			// Update config
			s.CfgMu.Lock()
			s.Cfg.Indexing.Directories = append(s.Cfg.Indexing.Directories, config.IndexingDirectory{
				Path:       absPath,
				Collection: strings.TrimSpace(req.Collection),
			})
			s.CfgMu.Unlock()

			// Persist to YAML
			if err := patchIndexingDirs(s); err != nil {
				s.Logger.Error("[Indexer] Failed to persist directory change", "error", err)
				jsonError(w, "Failed to save config", http.StatusInternalServerError)
				return
			}

			// Trigger rescan
			if s.FileIndexer != nil {
				s.FileIndexer.Rescan()
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

			configDir := filepath.Dir(s.Cfg.ConfigPath)
			absPath := resolveIndexingRequestPath(configDir, req.Path)

			s.CfgMu.Lock()
			newDirs := make([]config.IndexingDirectory, 0, len(s.Cfg.Indexing.Directories))
			found := false
			var removed config.IndexingDirectory
			for _, d := range s.Cfg.Indexing.Directories {
				if sameIndexingPath(d.Path, absPath) {
					found = true
					removed = d
					continue
				}
				newDirs = append(newDirs, d)
			}
			if !found {
				s.CfgMu.Unlock()
				jsonError(w, "Verzeichnis nicht gefunden", http.StatusNotFound)
				return
			}
			s.Cfg.Indexing.Directories = newDirs
			s.CfgMu.Unlock()

			// Persist
			if err := patchIndexingDirs(s); err != nil {
				s.Logger.Error("[Indexer] Failed to persist directory change", "error", err)
				jsonError(w, "Failed to save config", http.StatusInternalServerError)
				return
			}

			cleanupErrors := []string{}
			if s.FileIndexer != nil {
				cleanupErrors = s.FileIndexer.CleanupDirectory(removed)
				if len(cleanupErrors) > 0 {
					s.Logger.Warn("[Indexer] Directory cleanup after removal produced errors", "path", removed.Path, "errors", cleanupErrors)
				}
				s.FileIndexer.Rescan()
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "cleanup_errors": len(cleanupErrors)})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// patchIndexingDirs persists the current indexing directories to config.yaml.
func patchIndexingDirs(s *Server) error {
	configPath := s.Cfg.ConfigPath
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
	s.CfgMu.RLock()
	dirs := make([]config.IndexingDirectory, len(s.Cfg.Indexing.Directories))
	copy(dirs, s.Cfg.Indexing.Directories)
	s.CfgMu.RUnlock()

	relDirs := make([]interface{}, len(dirs))
	for i, d := range dirs {
		rel, err := filepath.Rel(configDir, d.Path)
		if err != nil || strings.HasPrefix(rel, "..") {
			relDirs[i] = d // keep absolute if can't relativize
		} else {
			relDirs[i] = config.IndexingDirectory{
				Path:       "./" + filepath.ToSlash(rel),
				Collection: d.Collection,
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
	indexing["enabled"] = true

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		return err
	}

	return config.WriteFileAtomic(configPath, out, 0o600)
}
