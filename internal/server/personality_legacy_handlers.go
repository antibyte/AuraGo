package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"aurago/internal/memory"
)

func handlePersonalityLegacy(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Personality state unavailable", http.StatusServiceUnavailable)
			return
		}
		after, limit := int64(0), 50
		var err error
		if value := r.URL.Query().Get("after"); value != "" {
			after, err = strconv.ParseInt(value, 10, 64)
			if err != nil || after < 0 {
				jsonError(w, "Invalid pagination", http.StatusBadRequest)
				return
			}
		}
		if value := r.URL.Query().Get("limit"); value != "" {
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				jsonError(w, "Invalid page size", http.StatusBadRequest)
				return
			}
		}
		entries, err := s.ShortTermMem.ListPersonalityLegacy(r.Context(), after, limit)
		if err != nil {
			jsonError(w, "Legacy review unavailable", http.StatusInternalServerError)
			return
		}
		next := after
		if len(entries) > 0 {
			next = entries[len(entries)-1].ID
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries, "next_after": next, "has_more": len(entries) == limit})
	}
}

func handlePersonalityLegacyReview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Personality state unavailable", http.StatusServiceUnavailable)
			return
		}
		var req struct {
			ID          int64  `json:"id"`
			Fingerprint string `json:"fingerprint"`
			Action      string `json:"action"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil || req.ID <= 0 || req.Fingerprint == "" {
			jsonError(w, "Invalid legacy review", http.StatusBadRequest)
			return
		}
		if err := s.ShortTermMem.ReviewPersonalityLegacy(req.ID, req.Fingerprint, req.Action); err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, memory.ErrPersonalityEventConflict) || errors.Is(err, sql.ErrNoRows) {
				status = http.StatusConflict
			}
			jsonError(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
