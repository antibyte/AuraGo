package server

import (
	"aurago/internal/tools"
	"encoding/json"
	"net/http"
	"strings"
)

// handleListMissionsV2 returns all missions V2 with queue status
func handleListMissionsV2(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var mission tools.MissionV2
		if err := json.NewDecoder(r.Body).Decode(&mission); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		if mission.Name == "" || mission.Prompt == "" {
			http.Error(w, "name and prompt are required", http.StatusBadRequest)
			return
		}

		// Validate execution type
		switch mission.ExecutionType {
		case tools.ExecutionManual, tools.ExecutionScheduled, tools.ExecutionTriggered:
			// Valid
		case "":
			mission.ExecutionType = tools.ExecutionManual
		default:
			http.Error(w, "Invalid execution_type. Use: manual, scheduled, triggered", http.StatusBadRequest)
			return
		}

		// Validate trigger configuration
		if mission.ExecutionType == tools.ExecutionTriggered {
			if mission.TriggerType == "" {
				http.Error(w, "trigger_type is required for triggered missions", http.StatusBadRequest)
				return
			}
			if mission.TriggerConfig == nil {
				http.Error(w, "trigger_config is required for triggered missions", http.StatusBadRequest)
				return
			}
		}

		if err := s.MissionManagerV2.Create(&mission); err != nil {
			s.Logger.Error("Failed to create mission", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "ok",
			"mission":   mission,
		})
	}
}

// handleMissionV2ByID handles GET, PUT, DELETE, POST .../run, POST .../trigger
func handleMissionV2ByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/missions/v2/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "mission ID required", http.StatusBadRequest)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleMissionGetV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	mission, ok := s.MissionManagerV2.Get(id)
	if !ok {
		http.Error(w, "mission not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mission)
}

func handleMissionUpdateV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	var updated tools.MissionV2
	if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.MissionManagerV2.Update(id, &updated); err != nil {
		s.Logger.Error("Failed to update mission", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMissionDeleteV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if err := s.MissionManagerV2.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleMissionRunV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.MissionManagerV2.RunNow(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

func handleMissionTriggerV2(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		TriggerData string `json:"trigger_data,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&payload)

	if err := s.MissionManagerV2.TriggerMission(id, "api", payload.TriggerData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
}

func handleMissionRemoveFromQueue(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	queue, running := s.MissionManagerV2.GetQueue()
	if running == id {
		http.Error(w, "Cannot remove running mission from queue", http.StatusConflict)
		return
	}
	if removed := queue.Remove(id); removed {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
		return
	}
	http.Error(w, "mission not in queue", http.StatusNotFound)
}

// handleMissionQueue returns the current queue status
func handleMissionQueue(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		execType := r.URL.Query().Get("type")
		if execType == "" {
			http.Error(w, "type parameter required", http.StatusBadRequest)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sourceID := r.URL.Query().Get("source_id")
		if sourceID == "" {
			http.Error(w, "source_id parameter required", http.StatusBadRequest)
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
