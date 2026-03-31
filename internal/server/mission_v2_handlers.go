package server

import (
	"aurago/internal/tools"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/robfig/cron/v3"
)

const (
	maxMissionNameLen   = 200
	maxMissionPromptLen = 10000
)

// broadcastMissionState pushes current mission list + queue state to all SSE clients.
func broadcastMissionState(s *Server) {
	if s.SSE == nil {
		return
	}
	missions := s.MissionManagerV2.List()
	queue, running := s.MissionManagerV2.GetQueue()
	s.SSE.BroadcastType(EventMissionUpdate, map[string]interface{}{
		"missions": missions,
		"queue": map[string]interface{}{
			"items":   queue.List(),
			"running": running,
		},
	})
}

// validateCronExpr checks whether expr is a valid cron expression.
func validateCronExpr(expr string) bool {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(expr)
	return err == nil
}

// handleListMissionsV2 returns all missions V2 with queue status
func handleListMissionsV2(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		missions := s.MissionManagerV2.List()
		queue, running := s.MissionManagerV2.GetQueue()

		response := map[string]interface{}{
			"missions": missions,
			"queue": map[string]interface{}{
				"items":   queue.List(),
				"running": running,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleCreateMissionV2 creates a new mission
func handleCreateMissionV2(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var mission tools.MissionV2
		if err := json.NewDecoder(r.Body).Decode(&mission); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if mission.Name == "" || mission.Prompt == "" {
			jsonError(w, "name and prompt are required", http.StatusBadRequest)
			return
		}

		if len(mission.Name) > maxMissionNameLen {
			jsonError(w, "name exceeds maximum length", http.StatusBadRequest)
			return
		}
		if len(mission.Prompt) > maxMissionPromptLen {
			jsonError(w, "prompt exceeds maximum length", http.StatusBadRequest)
			return
		}

		// Validate execution type
		switch mission.ExecutionType {
		case tools.ExecutionManual, tools.ExecutionScheduled, tools.ExecutionTriggered:
			// Valid
		case "":
			mission.ExecutionType = tools.ExecutionManual
		default:
			jsonError(w, "Invalid execution_type. Use: manual, scheduled, triggered", http.StatusBadRequest)
			return
		}

		// Validate trigger configuration
		if mission.ExecutionType == tools.ExecutionTriggered {
			if mission.TriggerType == "" {
				jsonError(w, "trigger_type is required for triggered missions", http.StatusBadRequest)
				return
			}
			if mission.TriggerConfig == nil {
				jsonError(w, "trigger_config is required for triggered missions", http.StatusBadRequest)
				return
			}
		}

		// Validate cron schedule for scheduled missions
		if mission.ExecutionType == tools.ExecutionScheduled {
			if mission.Schedule == "" {
				jsonError(w, "schedule (cron expression) is required for scheduled missions", http.StatusBadRequest)
				return
			}
			if !validateCronExpr(mission.Schedule) {
				jsonError(w, "invalid cron expression in schedule", http.StatusBadRequest)
				return
			}
		}

		if err := s.MissionManagerV2.Create(&mission); err != nil {
			s.Logger.Error("Failed to create mission", "error", err)
			jsonError(w, "Failed to create mission", http.StatusInternalServerError)
			return
		}
		broadcastMissionState(s)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"mission": mission,
		})
	}
}

// handleMissionV2ByID handles GET, PUT, DELETE, POST .../run, POST .../trigger
func handleMissionV2ByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/missions/v2/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			jsonError(w, "mission ID required", http.StatusBadRequest)
			return
		}
		id := parts[0]

		// Handle sub-routes
		if len(parts) >= 2 {
			switch parts[1] {
			case "run":
				handleMissionRunV2(s, w, r, id)
				return
			case "trigger":
				handleMissionTriggerV2(s, w, r, id)
				return
			case "queue":
				if r.Method == http.MethodDelete {
					handleMissionRemoveFromQueue(s, w, r, id)
					return
				}
			case "prepare":
				handleMissionPrepare(s, w, r, id)
				return
			case "prepared":
				handleMissionPrepared(s, w, r, id)
				return
			}
		}

		switch r.Method {
		case http.MethodGet:
			handleMissionGetV2(s, w, r, id)
		case http.MethodPut:
			handleMissionUpdateV2(s, w, r, id)
		case http.MethodDelete:
			handleMissionDeleteV2(s, w, r, id)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleMissionGetV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	mission, ok := s.MissionManagerV2.Get(id)
	if !ok {
		jsonError(w, "mission not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mission)
}

func handleMissionUpdateV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	var updated tools.MissionV2
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(updated.Name) > maxMissionNameLen {
		jsonError(w, "name exceeds maximum length", http.StatusBadRequest)
		return
	}
	if len(updated.Prompt) > maxMissionPromptLen {
		jsonError(w, "prompt exceeds maximum length", http.StatusBadRequest)
		return
	}
	if updated.Schedule != "" && !validateCronExpr(updated.Schedule) {
		jsonError(w, "invalid cron expression in schedule", http.StatusBadRequest)
		return
	}

	if err := s.MissionManagerV2.Update(id, &updated); err != nil {
		s.Logger.Error("Failed to update mission", "error", err)
		jsonError(w, "Failed to update mission", http.StatusInternalServerError)
		return
	}
	broadcastMissionState(s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMissionDeleteV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if err := s.MissionManagerV2.Delete(id); err != nil {
		jsonError(w, "Mission not found", http.StatusNotFound)
		return
	}
	broadcastMissionState(s)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMissionRunV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.MissionManagerV2.RunNow(id); err != nil {
		jsonError(w, "Mission not found", http.StatusNotFound)
		return
	}
	broadcastMissionState(s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

func handleMissionTriggerV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		TriggerData string `json:"trigger_data,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	if err := s.MissionManagerV2.TriggerMission(id, "api", payload.TriggerData); err != nil {
		jsonError(w, "Failed to trigger mission", http.StatusBadRequest)
		return
	}
	broadcastMissionState(s)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

func handleMissionRemoveFromQueue(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	queue, running := s.MissionManagerV2.GetQueue()
	if running == id {
		jsonError(w, "Cannot remove running mission from queue", http.StatusConflict)
		return
	}
	if removed := queue.Remove(id); removed {
		broadcastMissionState(s)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
		return
	}
	jsonError(w, "mission not in queue", http.StatusNotFound)
}

// handleMissionQueue returns the current queue status
func handleMissionQueue(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		queue, running := s.MissionManagerV2.GetQueue()
		response := map[string]interface{}{
			"items":   queue.List(),
			"running": running,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleMissionsV2ByExecution returns missions filtered by execution type
func handleMissionsV2ByExecution(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		execType := r.URL.Query().Get("type")
		if execType == "" {
			jsonError(w, "type parameter required", http.StatusBadRequest)
			return
		}

		all := s.MissionManagerV2.List()
		filtered := make([]*tools.MissionV2, 0)
		for _, m := range all {
			if string(m.ExecutionType) == execType {
				filtered = append(filtered, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(filtered)
	}
}

// handleMissionDependencies returns missions that depend on a given mission
func handleMissionDependencies(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sourceID := r.URL.Query().Get("source_id")
		if sourceID == "" {
			jsonError(w, "source_id parameter required", http.StatusBadRequest)
			return
		}

		all := s.MissionManagerV2.List()
		dependents := make([]*tools.MissionV2, 0)
		for _, m := range all {
			if m.ExecutionType == tools.ExecutionTriggered &&
				m.TriggerType == tools.TriggerMissionCompleted &&
				m.TriggerConfig != nil &&
				m.TriggerConfig.SourceMissionID == sourceID {
				dependents = append(dependents, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dependents)
	}
}

// handleMissionPrepare triggers manual preparation or returns/deletes prepared context.
// POST /api/missions/v2/{id}/prepare — trigger preparation
func handleMissionPrepare(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.PreparationService == nil {
		jsonError(w, "Mission preparation is not enabled", http.StatusServiceUnavailable)
		return
	}

	// Verify mission exists
	if _, ok := s.MissionManagerV2.Get(id); !ok {
		jsonError(w, "Mission not found", http.StatusNotFound)
		return
	}

	// Run preparation asynchronously
	go func() {
		if _, err := s.PreparationService.PrepareMission(r.Context(), id); err != nil {
			s.Logger.Error("Manual mission preparation failed", "mission", id, "error", err)
		}
		broadcastMissionState(s)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "preparing"})
}

// handleMissionPrepared returns or deletes the prepared context for a mission.
// GET /api/missions/v2/{id}/prepared — get prepared context
// DELETE /api/missions/v2/{id}/prepared — invalidate prepared context
func handleMissionPrepared(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if s.PreparedMissionsDB == nil {
		jsonError(w, "Mission preparation is not enabled", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		pm, err := tools.GetPreparedMission(s.PreparedMissionsDB, id)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to get preparation", "Failed to get prepared mission", err, "mission", id)
			return
		}
		if pm == nil {
			jsonError(w, "No preparation found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pm)

	case http.MethodDelete:
		if s.PreparationService != nil {
			s.PreparationService.InvalidateMission(id)
		} else {
			tools.InvalidatePreparedMission(s.PreparedMissionsDB, id)
		}
		broadcastMissionState(s)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "invalidated"})

	default:
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
