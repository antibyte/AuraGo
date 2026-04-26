package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

// docMaxBodyBytes is the upper bound for any documentation HTTP body.
// Twice the on-disk limit so JSON quoting/whitespace doesn't break legitimate writes.
const docMaxBodyBytes = 2 * tools.MaxSkillDocumentationBytes

// handleSkillDocumentation routes /api/skills/{id}/documentation by HTTP verb:
//
//	GET    -> read the markdown manual
//	PUT    -> replace the markdown manual (JSON body {"content": "..."})
//	DELETE -> remove the manual
func handleSkillDocumentation(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
		path = strings.TrimSuffix(path, "/documentation")
		id := strings.TrimSuffix(path, "/")
		if id == "" || strings.Contains(id, "/") {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			content, err := s.SkillManager.GetSkillDocumentation(id)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusNotFound, "Skill not found", "Failed to read skill documentation", err, "skill_id", id)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status":            "ok",
				"skill_id":          id,
				"has_documentation": content != "",
				"content":           content,
			})

		case http.MethodPut:
			s.CfgMu.RLock()
			readOnly := s.Cfg.Tools.SkillManager.ReadOnly
			s.CfgMu.RUnlock()
			if readOnly {
				jsonError(w, "Skill Manager is in read-only mode", http.StatusForbidden)
				return
			}
			var req struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, docMaxBodyBytes)).Decode(&req); err != nil {
				jsonError(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			if len(req.Content) > tools.MaxSkillDocumentationBytes {
				jsonError(w, "Documentation exceeds size limit", http.StatusRequestEntityTooLarge)
				return
			}
			if err := s.SkillManager.SetSkillDocumentation(id, req.Content, "user"); err != nil {
				if strings.Contains(err.Error(), "not found") {
					jsonError(w, err.Error(), http.StatusNotFound)
					return
				}
				if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "UTF-8") {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to save documentation", "Failed to save skill documentation", err, "skill_id", id)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "saved"})

		case http.MethodDelete:
			s.CfgMu.RLock()
			readOnly := s.Cfg.Tools.SkillManager.ReadOnly
			s.CfgMu.RUnlock()
			if readOnly {
				jsonError(w, "Skill Manager is in read-only mode", http.StatusForbidden)
				return
			}
			if err := s.SkillManager.DeleteSkillDocumentation(id, "user"); err != nil {
				if strings.Contains(err.Error(), "not found") {
					jsonError(w, err.Error(), http.StatusNotFound)
					return
				}
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete documentation", "Failed to delete skill documentation", err, "skill_id", id)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": "deleted"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleUploadSkillDocumentation accepts a multipart .md/.txt upload and
// persists it as the skill's manual.
//
// POST /api/skills/{id}/documentation/upload  (form field "file")
func handleUploadSkillDocumentation(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SkillManager == nil {
			jsonError(w, "Skill Manager is not enabled", http.StatusServiceUnavailable)
			return
		}
		s.CfgMu.RLock()
		readOnly := s.Cfg.Tools.SkillManager.ReadOnly
		allowUploads := s.Cfg.Tools.SkillManager.AllowUploads
		s.CfgMu.RUnlock()
		if readOnly || !allowUploads {
			jsonError(w, "Skill documentation upload is disabled", http.StatusForbidden)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/skills/")
		path = strings.TrimSuffix(path, "/documentation/upload")
		id := strings.TrimSuffix(path, "/")
		if id == "" || strings.Contains(id, "/") {
			jsonError(w, "Skill ID is required", http.StatusBadRequest)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, docMaxBodyBytes)
		if err := r.ParseMultipartForm(docMaxBodyBytes); err != nil {
			jsonError(w, "File too large or invalid form data", http.StatusRequestEntityTooLarge)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "File is required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		lower := strings.ToLower(header.Filename)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") && !strings.HasSuffix(lower, ".txt") {
			jsonError(w, "Only .md, .markdown or .txt files are allowed", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(io.LimitReader(file, docMaxBodyBytes+1))
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to read uploaded file", "Failed to read skill documentation upload", err, "skill_id", id)
			return
		}
		if len(data) > tools.MaxSkillDocumentationBytes {
			jsonError(w, "Documentation exceeds size limit", http.StatusRequestEntityTooLarge)
			return
		}
		if err := s.SkillManager.SetSkillDocumentation(id, string(data), "user"); err != nil {
			if strings.Contains(err.Error(), "not found") {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "limit") || strings.Contains(err.Error(), "UTF-8") {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to save documentation", "Failed to save skill documentation", err, "skill_id", id)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "uploaded",
			"size":     len(data),
			"filename": header.Filename,
		})
	}
}
