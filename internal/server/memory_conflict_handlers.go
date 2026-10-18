package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

func handleMemoryConflictResolve(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.ShortTermMem == nil {
			jsonError(w, "Memory subsystem not available", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			ConflictID   int64  `json:"conflict_id"`
			WinningDocID string `json:"winning_doc_id"`
			Reason       string `json:"reason"`
			Action       string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		req.WinningDocID = strings.TrimSpace(req.WinningDocID)
		if req.Action != "" && req.Action != "resolve" && req.Action != "dismiss" {
			jsonError(w, "Invalid conflict action", http.StatusBadRequest)
			return
		}
		if req.ConflictID <= 0 || (req.Action != "dismiss" && req.WinningDocID == "") || (req.Action == "dismiss" && req.WinningDocID != "") {
			jsonError(w, "conflict_id and winning_doc_id are required", http.StatusBadRequest)
			return
		}

		var actionErr error
		if req.Action == "dismiss" {
			actionErr = s.ShortTermMem.DismissMemoryConflict(req.ConflictID, req.Reason)
		} else {
			actionErr = s.ShortTermMem.ResolveMemoryConflict(req.ConflictID, req.WinningDocID, req.Reason)
		}
		if err := actionErr; err != nil {
			status := http.StatusInternalServerError
			errText := err.Error()
			if strings.Contains(errText, "does not belong") {
				status = http.StatusBadRequest
			} else if strings.Contains(errText, "is not open") {
				status = http.StatusConflict
			}
			jsonError(w, "Failed to resolve memory conflict", status)
			return
		}

		payload := map[string]interface{}{
			"status":         "ok",
			"conflict_id":    req.ConflictID,
			"winning_doc_id": req.WinningDocID,
		}
		if conflict, err := s.ShortTermMem.GetMemoryConflictByID(req.ConflictID); err == nil {
			payload["superseded_doc_id"] = conflict.SupersededDocID
		}
		writeJSON(w, payload)
	}
}
