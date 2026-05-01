package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

type questionResponseRequest struct {
	SessionID     string `json:"session_id"`
	SelectedValue string `json:"selected_value"`
	FreeText      string `json:"free_text"`
}

func handleQuestionStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session"))
		if sessionID == "" {
			sessionID = "default"
		}
		q := tools.GetPendingQuestion(sessionID)
		if q == nil {
			writeJSON(w, map[string]interface{}{"status": "none"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "pending", "question": q})
	}
}

func handleQuestionResponse(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req questionResponseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.SessionID) == "" {
			req.SessionID = "default"
		}
		resp := tools.QuestionResponse{Status: "ok", Selected: strings.TrimSpace(req.SelectedValue), FreeText: strings.TrimSpace(req.FreeText)}
		if !tools.CompleteQuestion(req.SessionID, resp) {
			writeJSON(w, map[string]interface{}{"status": "not_found"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok"})
	}
}
