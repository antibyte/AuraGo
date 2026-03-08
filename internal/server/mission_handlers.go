package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"aurago/internal/tools"
)

// handleListMissions returns all missions as JSON.
func handleListMissions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		missions := s.MissionManager.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(missions)
	}
}

// handleCreateMission creates a new mission.
func handleCreateMission(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var mission tools.Mission
		if err := json.NewDecoder(r.Body).Decode(&mission); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if mission.Name == "" || mission.Prompt == "" {
			http.Error(w, "name and prompt are required", http.StatusBadRequest)
			return
		}
		if err := s.MissionManager.Create(mission); err != nil {
			s.Logger.Error("Failed to create mission", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// handleMissionByID routes PUT, DELETE and POST .../run for a specific mission.
func handleMissionByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path: /api/missions/{id} or /api/missions/{id}/run
		path := strings.TrimPrefix(r.URL.Path, "/api/missions/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "mission ID required", http.StatusBadRequest)
			return
		}
		id := parts[0]

		// Handle /api/missions/{id}/run
		if len(parts) >= 2 && parts[1] == "run" {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := s.MissionManager.RunNow(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "running"})
			return
		}

		switch r.Method {
		case http.MethodPut:
			var updated tools.Mission
			if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
				http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
				return
			}
			if err := s.MissionManager.Update(id, updated); err != nil {
				s.Logger.Error("Failed to update mission", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			if err := s.MissionManager.Delete(id); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
