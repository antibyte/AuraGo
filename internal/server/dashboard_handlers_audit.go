package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/memory"
)

type dashboardAuditDeleteRequest struct {
	ID       int64  `json:"id,omitempty"`
	All      bool   `json:"all,omitempty"`
	Q        string `json:"q,omitempty"`
	Source   string `json:"source,omitempty"`
	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	Confirm  string `json:"confirm,omitempty"`
}

// handleDashboardAudit serves central dashboard audit search and confirmed bulk deletion.
func handleDashboardAudit(s *Server, _ *SSEBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.ShortTermMem == nil {
			jsonError(w, "Audit log not available", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			filter, ok := auditFilterFromRequest(w, r)
			if !ok {
				return
			}
			page, err := s.ShortTermMem.SearchAuditEvents(filter)
			if err != nil {
				if s.Logger != nil {
					s.Logger.Error("Failed to query audit log", "error", err)
				}
				jsonError(w, "Failed to query audit log", http.StatusInternalServerError)
				return
			}
			writeJSON(w, page)
		case http.MethodDelete:
			var req dashboardAuditDeleteRequest
			if r.Body != nil && r.ContentLength != 0 {
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					jsonError(w, "Invalid audit delete request", http.StatusBadRequest)
					return
				}
			}
			if req.ID > 0 {
				if err := s.ShortTermMem.DeleteAuditEvent(req.ID); err != nil {
					jsonError(w, "Failed to delete audit event", http.StatusInternalServerError)
					return
				}
				writeJSON(w, map[string]interface{}{"deleted": 1})
				return
			}
			filter := memory.AuditFilter{
				Q:        firstNonEmptyQuery(req.Q, r.URL.Query().Get("q")),
				Source:   firstNonEmptyQuery(req.Source, r.URL.Query().Get("source")),
				Status:   firstNonEmptyQuery(req.Status, r.URL.Query().Get("status")),
				Type:     firstNonEmptyQuery(req.Type, r.URL.Query().Get("type")),
				From:     firstNonEmptyQuery(req.From, r.URL.Query().Get("from")),
				To:       firstNonEmptyQuery(req.To, r.URL.Query().Get("to")),
				TargetID: firstNonEmptyQuery(req.TargetID, r.URL.Query().Get("target_id")),
			}
			if !validateAuditDateFilters(w, filter.From, filter.To) {
				return
			}
			confirm := firstNonEmptyQuery(req.Confirm, r.URL.Query().Get("confirm"))
			deleted, err := s.ShortTermMem.DeleteAuditEvents(filter, confirm)
			if err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]interface{}{"deleted": deleted})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardAuditByID serves DELETE /api/dashboard/audit/{id}.
func handleDashboardAuditByID(s *Server, _ *SSEBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s == nil || s.ShortTermMem == nil {
			jsonError(w, "Audit log not available", http.StatusServiceUnavailable)
			return
		}
		rawID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/dashboard/audit/"), "/")
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil || id <= 0 {
			jsonError(w, "Invalid audit event id", http.StatusBadRequest)
			return
		}
		if err := s.ShortTermMem.DeleteAuditEvent(id); err != nil {
			jsonError(w, "Failed to delete audit event", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{"deleted": 1})
	}
}

func auditFilterFromRequest(w http.ResponseWriter, r *http.Request) (memory.AuditFilter, bool) {
	q := r.URL.Query()
	filter := memory.AuditFilter{
		Q:        q.Get("q"),
		Source:   q.Get("source"),
		Status:   q.Get("status"),
		Type:     q.Get("type"),
		From:     q.Get("from"),
		To:       q.Get("to"),
		TargetID: q.Get("target_id"),
	}
	if !validateAuditDateFilters(w, filter.From, filter.To) {
		return memory.AuditFilter{}, false
	}
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.Limit = n
		}
	}
	if raw := q.Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			filter.Offset = n
		}
	}
	return filter, true
}

func validateAuditDateFilters(w http.ResponseWriter, from, to string) bool {
	if msg, ok := validateDateParam(from, "from"); !ok {
		jsonError(w, msg, http.StatusBadRequest)
		return false
	}
	if msg, ok := validateDateParam(to, "to"); !ok {
		jsonError(w, msg, http.StatusBadRequest)
		return false
	}
	return true
}

func firstNonEmptyQuery(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
