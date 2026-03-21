package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type knowledgeFileEntry struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	Extension string `json:"extension"`
}

// handleKnowledgeFiles handles GET (list) on /api/knowledge.
func handleKnowledgeFiles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		knowledgeDir := s.knowledgeDir()
		if knowledgeDir == "" {
			http.Error(w, `{"error":"no knowledge directory configured"}`, http.StatusServiceUnavailable)
			return
		}

		entries, err := os.ReadDir(knowledgeDir)
		if err != nil {
			http.Error(w, `{"error":"failed to read knowledge directory"}`, http.StatusInternalServerError)
			return
		}

		var files []knowledgeFileEntry
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, knowledgeFileEntry{
				Name:      e.Name(),
				Size:      info.Size(),
				Modified:  info.ModTime().UTC().Format(time.RFC3339),
				Extension: strings.TrimPrefix(filepath.Ext(e.Name()), "."),
			})
		}
		if files == nil {
			files = []knowledgeFileEntry{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(files)
	}
}

// handleKnowledgeUpload handles POST /api/knowledge/upload (multipart file upload).
func handleKnowledgeUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		knowledgeDir := s.knowledgeDir()
		if knowledgeDir == "" {
			http.Error(w, `{"error":"no knowledge directory configured"}`, http.StatusServiceUnavailable)
			return
		}

		// 32 MB max
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, `{"error":"failed to parse multipart form"}`, http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, `{"error":"missing file field"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Sanitize filename — prevent path traversal
		safeName := filepath.Base(header.Filename)
		if safeName == "." || safeName == ".." || safeName == "" {
			http.Error(w, `{"error":"invalid filename"}`, http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(knowledgeDir, 0750); err != nil {
			http.Error(w, `{"error":"failed to create knowledge directory"}`, http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(knowledgeDir, safeName)
		out, err := os.Create(destPath)
		if err != nil {
			http.Error(w, `{"error":"failed to create file"}`, http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, io.LimitReader(file, 32<<20)); err != nil {
			http.Error(w, `{"error":"failed to write file"}`, http.StatusInternalServerError)
			return
		}

		s.Logger.Info("Knowledge file uploaded", "file", safeName)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "uploaded", "name": safeName})
	}
}

// handleKnowledgeFile handles GET (download) and DELETE on /api/knowledge/{filename}.
func handleKnowledgeFile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		knowledgeDir := s.knowledgeDir()
		if knowledgeDir == "" {
			http.Error(w, `{"error":"no knowledge directory configured"}`, http.StatusServiceUnavailable)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/api/knowledge/")
		if name == "" || name == "upload" {
			http.Error(w, `{"error":"missing filename"}`, http.StatusBadRequest)
			return
		}

		// Sanitize — prevent path traversal
		safeName := filepath.Base(name)
		if safeName != name || safeName == "." || safeName == ".." {
			http.Error(w, `{"error":"invalid filename"}`, http.StatusBadRequest)
			return
		}

		fullPath := filepath.Join(knowledgeDir, safeName)

		switch r.Method {
		case http.MethodGet:
			info, err := os.Stat(fullPath)
			if err != nil {
				http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
			http.ServeFile(w, r, fullPath)

		case http.MethodDelete:
			if err := os.Remove(fullPath); err != nil {
				if os.IsNotExist(err) {
					http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
				} else {
					http.Error(w, `{"error":"failed to delete file"}`, http.StatusInternalServerError)
				}
				return
			}
			s.Logger.Info("Knowledge file deleted", "file", safeName)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// knowledgeDir returns the first configured indexing directory (knowledge folder).
func (s *Server) knowledgeDir() string {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if len(s.Cfg.Indexing.Directories) > 0 {
		return s.Cfg.Indexing.Directories[0]
	}
	return ""
}
