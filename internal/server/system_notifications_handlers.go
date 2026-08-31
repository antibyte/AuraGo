package server

import (
	"encoding/json"
	"net/http"
)

func handleSystemNotifications(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		notifications, err := s.ShortTermMem.GetUnreadSystemNotifications()
		if err != nil {
			s.Logger.Error("Failed to fetch typed system notifications", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "ok",
			"notifications": notifications,
		})
	}
}

func handleSystemNotificationsRead(s *Server) http.HandlerFunc {
	type request struct {
		IDs []int64 `json:"ids"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
			jsonError(w, "notification ids are required", http.StatusBadRequest)
			return
		}
		if err := s.ShortTermMem.MarkNotificationsReadByIDs(body.IDs); err != nil {
			jsonError(w, "Invalid notification ids", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "read_ids": body.IDs})
	}
}
