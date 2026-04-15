package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/tools"
)

// validateDateParam checks if a query parameter is a valid RFC3339 timestamp.
// Returns empty string if valid (or empty), or an error message.
func validateDateParam(value, paramName string) (string, bool) {
	if value == "" {
		return "", true
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return "Invalid " + paramName + " format, expected RFC3339 (e.g. 2026-04-15T12:00:00Z)", false
	}
	return "", true
}

// handleDashboardMissionHistory returns paginated mission execution history for the dashboard.
// GET /api/dashboard/mission-history?limit=10&offset=0&mission_id=&result=&trigger_type=&from=&to=
func handleDashboardMissionHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.MissionHistoryDB == nil {
			jsonError(w, "Mission history not available", http.StatusServiceUnavailable)
			return
		}

		fromVal := r.URL.Query().Get("from")
		toVal := r.URL.Query().Get("to")
		if msg, ok := validateDateParam(fromVal, "from"); !ok {
			jsonError(w, msg, http.StatusBadRequest)
			return
		}
		if msg, ok := validateDateParam(toVal, "to"); !ok {
			jsonError(w, msg, http.StatusBadRequest)
			return
		}

		filter := tools.MissionHistoryFilter{
			MissionID:   r.URL.Query().Get("mission_id"),
			Result:      r.URL.Query().Get("result"),
			TriggerType: r.URL.Query().Get("trigger_type"),
			From:        fromVal,
			To:          toVal,
		}

		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Offset = n
			}
		}

		page, err := tools.QueryMissionHistory(s.MissionHistoryDB, filter)
		if err != nil {
			s.Logger.Error("Failed to query mission history", "error", err)
			jsonError(w, "Failed to query mission history", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}
}

// handleMissionV2History returns paginated mission execution history for the missions page.
// GET /api/missions/v2/history?limit=10&offset=0&mission_id=&result=&trigger_type=&from=&to=
func handleMissionV2History(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.MissionHistoryDB == nil {
			jsonError(w, "Mission history not available", http.StatusServiceUnavailable)
			return
		}

		fromVal := r.URL.Query().Get("from")
		toVal := r.URL.Query().Get("to")
		if msg, ok := validateDateParam(fromVal, "from"); !ok {
			jsonError(w, msg, http.StatusBadRequest)
			return
		}
		if msg, ok := validateDateParam(toVal, "to"); !ok {
			jsonError(w, msg, http.StatusBadRequest)
			return
		}

		filter := tools.MissionHistoryFilter{
			MissionID:   r.URL.Query().Get("mission_id"),
			Result:      r.URL.Query().Get("result"),
			TriggerType: r.URL.Query().Get("trigger_type"),
			From:        fromVal,
			To:          toVal,
		}

		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				filter.Offset = n
			}
		}

		page, err := tools.QueryMissionHistory(s.MissionHistoryDB, filter)
		if err != nil {
			s.Logger.Error("Failed to query mission history", "error", err)
			jsonError(w, "Failed to query mission history", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(page)
	}
}
