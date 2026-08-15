package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

func handlePersonalityCharacterNotes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.ShortTermMem == nil || !s.Cfg.Personality.Engine {
			jsonError(w, "Personality engine is disabled", http.StatusConflict)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.buildPersonalityStatePayload())
		case http.MethodPost:
			var req struct {
				Action    string `json:"action"`
				ID        int64  `json:"id"`
				Visible   *bool  `json:"visible"`
				Label     string `json:"label"`
				Protected *bool  `json:"protected"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, "Bad request", http.StatusBadRequest)
				return
			}
			switch strings.ToLower(strings.TrimSpace(req.Action)) {
			case "delete":
				if err := s.ShortTermMem.DeleteCharacterNote(req.ID); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
			case "protect", "unprotect":
				protected := req.Action == "protect"
				if req.Protected != nil {
					protected = *req.Protected
				}
				if err := s.ShortTermMem.SetCharacterNoteProtected(req.ID, protected); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
			case "narrative":
				if req.Visible == nil {
					jsonError(w, "visible is required", http.StatusBadRequest)
					return
				}
				if err := s.ShortTermMem.SetNarrativeVisible(*req.Visible); err != nil {
					jsonError(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case "review_milestone":
				if err := s.ShortTermMem.MarkMilestoneReviewed(req.Label); err != nil {
					jsonError(w, err.Error(), http.StatusBadRequest)
					return
				}
			default:
				jsonError(w, "Unknown action", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.buildPersonalityStatePayload())
		case http.MethodDelete:
			id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
			if err := s.ShortTermMem.DeleteCharacterNote(id); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s.buildPersonalityStatePayload())
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
