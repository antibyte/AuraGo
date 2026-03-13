package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

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
				jsonError(w, err.Error(), http.StatusInternalServerError)
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
				jsonError(w, err.Error(), http.StatusBadRequest)
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
					jsonError(w, err.Error(), http.StatusInternalServerError)
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
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, sheet)

		case http.MethodDelete:
			if err := tools.CheatsheetDelete(s.CheatsheetDB, id); err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]string{"status": "deleted"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
