package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/security"
)

func actionApprovalSession(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Session-ID"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("session_id"))
	}
	if id == "" {
		id = "default"
	}
	if len(id) > 256 || strings.ContainsAny(id, "\r\n\x00") {
		return ""
	}
	return id
}

func handleActionApprovals(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := actionApprovalSession(r)
		if sessionID == "" {
			jsonError(w, "Invalid session", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodGet {
			pending := agent.PendingActionApprovals(sessionID)
			if r.URL.Query().Get("all") == "1" {
				pending = agent.AdministrativePendingActionApprovals()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"pending": pending})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			ID       string `json:"id"`
			Decision string `json:"decision"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.ID == "" {
			jsonError(w, "Supply an action ID and approve/reject decision only", http.StatusBadRequest)
			return
		}
		result, err := agent.DecideActionApproval(r.Context(), sessionID, request.ID, request.Decision)
		if errors.Is(err, agent.ErrActionApprovalUnavailable) {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		if err != nil {
			jsonError(w, "Action decision could not be completed", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": result.Status, "result": security.Scrub(result.Output)})
	}
}

// Preview checks concrete actions without dispatch, persistence or approval
// creation. The same fields can be used in an administrator action rule.
func handleActionPolicyPreview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			Tool      string         `json:"tool"`
			Arguments map[string]any `json:"arguments"`
			MissionID string         `json:"mission_id"`
			FlowID    string         `json:"flow_id"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Tool == "" {
			jsonError(w, "Invalid action preview", http.StatusBadRequest)
			return
		}
		cfg := s.ConfigSnapshot().Clone()
		s.bindConfigAuthorization(cfg)
		dc := &agent.DispatchContext{Cfg: cfg, Logger: s.Logger, SessionID: actionApprovalSession(r), MissionID: request.MissionID, FlowID: request.FlowID}
		if request.FlowID != "" {
			dc.MessageSource = agent.MessageSourceFlow
		}
		action, state := agent.PreviewActionPolicy(agent.ToolCall{IsTool: true, Action: request.Tool, Params: request.Arguments}, dc)
		action.Target = security.RedactSensitiveInfo(security.Scrub(action.Target))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"action": action, "state": state})
	}
}

func handleActionPolicyCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agent.ConfiguredToolSchemas(s.ConfigSnapshot(), s.Logger))
	}
}
