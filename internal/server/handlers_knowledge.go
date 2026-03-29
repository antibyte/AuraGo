package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		knowledgeDir := s.knowledgeDir()
		if knowledgeDir == "" {
			jsonError(w, "Knowledge storage is not configured", http.StatusServiceUnavailable)
			return
		}

		entries, err := os.ReadDir(knowledgeDir)
		if err != nil {
			s.Logger.Error("Failed to read knowledge directory", "error", err)
			jsonError(w, "Knowledge storage is currently unavailable", http.StatusInternalServerError)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		knowledgeDir := s.knowledgeDir()
		if knowledgeDir == "" {
			jsonError(w, "Knowledge storage is not configured", http.StatusServiceUnavailable)
			return
		}

		// 32 MB max
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			jsonError(w, "Invalid upload request", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "Missing file upload", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Sanitize filename — prevent path traversal
		safeName := sanitizeFilename(filepath.Base(header.Filename))
		if safeName == "." || safeName == ".." || safeName == "" {
			jsonError(w, "Invalid filename", http.StatusBadRequest)
			return
		}
		if !isAllowedKnowledgeExtension(s, safeName) {
			jsonError(w, "This file type is not allowed for the Knowledge Center", http.StatusBadRequest)
			return
		}

		if err := os.MkdirAll(knowledgeDir, 0750); err != nil {
			s.Logger.Error("Failed to create knowledge directory", "error", err)
			jsonError(w, "Knowledge storage is currently unavailable", http.StatusInternalServerError)
			return
		}

		destPath := filepath.Join(knowledgeDir, safeName)
		resolvedKnowledgeDir, err := filepath.Abs(knowledgeDir)
		if err != nil {
			s.Logger.Error("Failed to resolve knowledge directory", "error", err)
			jsonError(w, "Knowledge storage is currently unavailable", http.StatusInternalServerError)
			return
		}
		resolvedDestPath, err := filepath.Abs(destPath)
		if err != nil || !strings.HasPrefix(resolvedDestPath, resolvedKnowledgeDir+string(os.PathSeparator)) {
			s.Logger.Warn("Rejected suspicious knowledge upload destination", "path", destPath)
			jsonError(w, "Invalid upload destination", http.StatusBadRequest)
			return
		}

		out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
		if err != nil {
			if os.IsExist(err) {
				jsonError(w, "A file with that name already exists", http.StatusConflict)
				return
			}
			s.Logger.Error("Failed to create knowledge file", "file", safeName, "error", err)
			jsonError(w, "Could not store uploaded file", http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, io.LimitReader(file, 32<<20)); err != nil {
			s.Logger.Error("Failed to write knowledge file", "file", safeName, "error", err)
			_ = os.Remove(destPath)
			jsonError(w, "Could not store uploaded file", http.StatusInternalServerError)
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
			jsonError(w, "Knowledge storage is not configured", http.StatusServiceUnavailable)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/api/knowledge/")
		if name == "" || name == "upload" {
			jsonError(w, "Missing filename", http.StatusBadRequest)
			return
		}

		// Sanitize — prevent path traversal
		safeName := filepath.Base(name)
		if safeName != name || safeName == "." || safeName == ".." {
			jsonError(w, "Invalid filename", http.StatusBadRequest)
			return
		}

		fullPath := filepath.Join(knowledgeDir, safeName)

		switch r.Method {
		case http.MethodGet:
			info, err := os.Stat(fullPath)
			if err != nil {
				jsonError(w, "File not found", http.StatusNotFound)
				return
			}
			if r.URL.Query().Get("inline") == "1" {
				w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, safeName))
			} else {
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, safeName))
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
			http.ServeFile(w, r, fullPath)

		case http.MethodDelete:
			if err := os.Remove(fullPath); err != nil {
				if os.IsNotExist(err) {
					jsonError(w, "File not found", http.StatusNotFound)
				} else {
					s.Logger.Error("Failed to delete knowledge file", "file", safeName, "error", err)
					jsonError(w, "Failed to delete file", http.StatusInternalServerError)
				}
				return
			}
			s.Logger.Info("Knowledge file deleted", "file", safeName)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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

var defaultKnowledgeExtensions = []string{
	".txt", ".md", ".json", ".csv", ".log", ".yaml", ".yml", ".pdf", ".docx", ".xlsx", ".pptx", ".odt", ".rtf",
}

func isAllowedKnowledgeExtension(s *Server, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	s.CfgMu.RLock()
	allowed := append([]string(nil), s.Cfg.Indexing.Extensions...)
	s.CfgMu.RUnlock()
	if len(allowed) == 0 {
		allowed = defaultKnowledgeExtensions
	}
	for i, candidate := range allowed {
		allowed[i] = strings.ToLower(strings.TrimSpace(candidate))
	}
	return slices.Contains(allowed, ext)
}
