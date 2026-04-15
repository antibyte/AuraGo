package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"aurago/internal/tools"
)

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

		filter := tools.MissionHistoryFilter{
			MissionID:   r.URL.Query().Get("mission_id"),
			Result:      r.URL.Query().Get("result"),
			TriggerType: r.URL.Query().Get("trigger_type"),
			From:        r.URL.Query().Get("from"),
			To:          r.URL.Query().Get("to"),
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

		filter := tools.MissionHistoryFilter{
			MissionID:   r.URL.Query().Get("mission_id"),
			Result:      r.URL.Query().Get("result"),
			TriggerType: r.URL.Query().Get("trigger_type"),
			From:        r.URL.Query().Get("from"),
			To:          r.URL.Query().Get("to"),
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
