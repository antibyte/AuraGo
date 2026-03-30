package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/tools"
)

// handleCheatSheetRouter routes /api/cheatsheets/{id}[/attachments[/{aid}]] requests.
func handleCheatSheetRouter(s *Server) http.HandlerFunc {
	byID := handleCheatSheetByID(s)
	attachments := handleCheatSheetAttachments(s)
	attachmentByID := handleCheatSheetAttachmentByID(s)

	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/cheatsheets/")
		parts := strings.Split(path, "/")

		switch {
		case len(parts) >= 3 && parts[1] == "attachments" && parts[2] != "":
			attachmentByID.ServeHTTP(w, r)
		case len(parts) >= 2 && parts[1] == "attachments":
			attachments.ServeHTTP(w, r)
		default:
			byID.ServeHTTP(w, r)
		}
	}
}

// handleCheatSheets handles GET (list) and POST (create) for /api/cheatsheets.
func handleCheatSheets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.CheatsheetDB == nil {
			jsonError(w, "Cheatsheet DB not initialized", http.StatusInternalServerError)
			return
		}

		switch r.Method {
		case http.MethodGet:
			activeOnly := r.URL.Query().Get("active") == "true"
			sheets, err := tools.CheatsheetList(s.CheatsheetDB, activeOnly)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load cheat sheets", "Failed to list cheat sheets", err)
				return
			}
			if sheets == nil {
				sheets = []tools.CheatSheet{}
			}
			writeJSON(w, sheets)

		case http.MethodPost:
			var body struct {
				Name      string `json:"name"`
				Content   string `json:"content"`
				CreatedBy string `json:"created_by"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			if body.CreatedBy == "" {
				body.CreatedBy = "user"
			}
			sheet, err := tools.CheatsheetCreate(s.CheatsheetDB, body.Name, body.Content, body.CreatedBy)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create cheat sheet", "Failed to create cheat sheet", err, "name", body.Name)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, sheet)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleCheatSheetByID handles GET, PUT, DELETE for /api/cheatsheets/{id}.
func handleCheatSheetByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.CheatsheetDB == nil {
			jsonError(w, "Cheatsheet DB not initialized", http.StatusInternalServerError)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/cheatsheets/")
		if id == "" {
			jsonError(w, "missing id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			sheet, err := tools.CheatsheetGet(s.CheatsheetDB, id)
			if err != nil {
				if err == sql.ErrNoRows {
					jsonError(w, "not found", http.StatusNotFound)
				} else {
					jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to load cheat sheet", "Failed to load cheat sheet", err, "cheatsheet_id", id)
				}
				return
			}
			writeJSON(w, sheet)

		case http.MethodPut:
			var body struct {
				Name    *string `json:"name"`
				Content *string `json:"content"`
				Active  *bool   `json:"active"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			sheet, err := tools.CheatsheetUpdate(s.CheatsheetDB, id, body.Name, body.Content, body.Active)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					jsonError(w, "not found", http.StatusNotFound)
					return
				}
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update cheat sheet", "Failed to update cheat sheet", err, "cheatsheet_id", id)
				return
			}
			writeJSON(w, sheet)

		case http.MethodDelete:
			if err := tools.CheatsheetDelete(s.CheatsheetDB, id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					jsonError(w, "not found", http.StatusNotFound)
					return
				}
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete cheat sheet", "Failed to delete cheat sheet", err, "cheatsheet_id", id)
				return
			}
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleCheatSheetAttachments handles GET (list) and POST (add) for /api/cheatsheets/{id}/attachments.
func handleCheatSheetAttachments(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.CheatsheetDB == nil {
			jsonError(w, "Cheatsheet DB not initialized", http.StatusInternalServerError)
			return
		}

		// Extract cheatsheet ID: /api/cheatsheets/{id}/attachments
		path := strings.TrimPrefix(r.URL.Path, "/api/cheatsheets/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 || parts[0] == "" {
			jsonError(w, "missing cheatsheet id", http.StatusBadRequest)
			return
		}
		csID := parts[0]

		switch r.Method {
		case http.MethodGet:
			attachments, err := tools.CheatsheetAttachmentList(s.CheatsheetDB, csID)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list attachments", "Failed to list cheatsheet attachments", err, "cheatsheet_id", csID)
				return
			}
			if attachments == nil {
				attachments = []tools.CheatSheetAttachment{}
			}
			writeJSON(w, attachments)

		case http.MethodPost:
			contentType := r.Header.Get("Content-Type")

			var filename, source, content string

			if strings.HasPrefix(contentType, "multipart/form-data") {
				// File upload
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB max for text files
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					jsonError(w, "Upload too large or invalid", http.StatusBadRequest)
					return
				}
				file, header, err := r.FormFile("file")
				if err != nil {
					jsonError(w, "Missing file", http.StatusBadRequest)
					return
				}
				defer file.Close()

				filename = filepath.Base(header.Filename)
				source = "upload"

				data, err := io.ReadAll(io.LimitReader(file, 1<<20))
				if err != nil {
					jsonError(w, "Failed to read file", http.StatusBadRequest)
					return
				}
				content = string(data)

			} else {
				// JSON body — select from knowledge center
				var body struct {
					Source   string `json:"source"`
					Filename string `json:"filename"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					jsonError(w, "invalid JSON", http.StatusBadRequest)
					return
				}
				if body.Source != "knowledge" {
					jsonError(w, "source must be 'knowledge' for JSON requests", http.StatusBadRequest)
					return
				}

				safeName := filepath.Base(body.Filename)
				if safeName != body.Filename || safeName == "." || safeName == ".." || safeName == "" {
					jsonError(w, "Invalid filename", http.StatusBadRequest)
					return
				}

				knowledgeDir := s.knowledgeDir()
				if knowledgeDir == "" {
					jsonError(w, "Knowledge storage not configured", http.StatusServiceUnavailable)
					return
				}

				fullPath := filepath.Join(knowledgeDir, safeName)
				resolvedKnowledgeDir, err := filepath.Abs(knowledgeDir)
				if err != nil {
					jsonError(w, "Knowledge storage unavailable", http.StatusInternalServerError)
					return
				}
				resolvedPath, err := filepath.Abs(fullPath)
				if err != nil || !strings.HasPrefix(resolvedPath, resolvedKnowledgeDir+string(os.PathSeparator)) {
					jsonError(w, "Invalid filename", http.StatusBadRequest)
					return
				}

				data, err := os.ReadFile(fullPath)
				if err != nil {
					if os.IsNotExist(err) {
						jsonError(w, "Knowledge file not found", http.StatusNotFound)
					} else {
						jsonError(w, "Failed to read knowledge file", http.StatusInternalServerError)
					}
					return
				}

				filename = safeName
				source = "knowledge"
				content = string(data)
			}

			attachment, err := tools.CheatsheetAttachmentAdd(s.CheatsheetDB, csID, filename, source, content)
			if err != nil {
				statusCode := http.StatusBadRequest
				if strings.Contains(err.Error(), "not found") {
					statusCode = http.StatusNotFound
				}
				jsonError(w, err.Error(), statusCode)
				return
			}

			w.WriteHeader(http.StatusCreated)
			writeJSON(w, attachment)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleCheatSheetAttachmentByID handles DELETE for /api/cheatsheets/{id}/attachments/{aid}.
func handleCheatSheetAttachmentByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.CheatsheetDB == nil {
			jsonError(w, "Cheatsheet DB not initialized", http.StatusInternalServerError)
			return
		}

		// Extract IDs: /api/cheatsheets/{id}/attachments/{aid}
		path := strings.TrimPrefix(r.URL.Path, "/api/cheatsheets/")
		// path = "{id}/attachments/{aid}"
		pathParts := strings.Split(path, "/")
		if len(pathParts) < 3 || pathParts[0] == "" || pathParts[2] == "" {
			jsonError(w, "missing id", http.StatusBadRequest)
			return
		}
		csID := pathParts[0]
		aID := pathParts[2]

		switch r.Method {
		case http.MethodDelete:
			if err := tools.CheatsheetAttachmentRemove(s.CheatsheetDB, csID, aID); err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
