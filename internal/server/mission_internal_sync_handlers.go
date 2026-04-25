package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/invasion/bridge"
	"aurago/internal/tools"
)

func handleInternalMissionSync(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		var payload bridge.MissionSyncPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		mission, err := missionFromSyncPayload(payload)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.MissionManagerV2.ApplySyncedMission(mission); err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleInternalMissionRun(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/internal/missions/")
		id = strings.TrimSuffix(id, "/run")
		if id == "" {
			jsonError(w, "Mission ID is required", http.StatusBadRequest)
			return
		}
		if !s.MissionManagerV2.IsSyncedFromMaster(id) {
			jsonError(w, "Mission is not synced from master", http.StatusForbidden)
			return
		}

		var payload struct {
			TriggerType string `json:"trigger_type,omitempty"`
			TriggerData string `json:"trigger_data,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err.Error() != "EOF" {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		var err error
		if payload.TriggerType == "" || payload.TriggerType == "manual" {
			err = s.MissionManagerV2.RunNow(id)
		} else {
			err = s.MissionManagerV2.TriggerMission(id, payload.TriggerType, payload.TriggerData)
		}
		if err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
	}
}

func handleInternalMissionDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isValidInternalLoopbackToken(r, s.internalToken) {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/internal/missions/")
		if id == "" {
			jsonError(w, "Mission ID is required", http.StatusBadRequest)
			return
		}
		if err := s.MissionManagerV2.DeleteSyncedMission(id); err != nil {
			jsonError(w, err.Error(), missionErrorStatus(err))
			return
		}
		broadcastMissionState(s)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func missionFromSyncPayload(payload bridge.MissionSyncPayload) (*tools.MissionV2, error) {
	if payload.MissionID == "" {
		return nil, fmt.Errorf("mission_id is required")
	}
	var triggerConfig *tools.TriggerConfig
	if len(payload.TriggerConfig) > 0 {
		var cfg tools.TriggerConfig
		if err := json.Unmarshal(payload.TriggerConfig, &cfg); err != nil {
			return nil, fmt.Errorf("invalid trigger config: %w", err)
		}
		triggerConfig = &cfg
	}
	createdAt := payload.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return &tools.MissionV2{
		ID:               payload.MissionID,
		Name:             payload.Name,
		Prompt:           payload.PromptSnapshot,
		ExecutionType:    tools.ExecutionType(payload.ExecutionType),
		Schedule:         payload.Schedule,
		TriggerType:      tools.TriggerType(payload.TriggerType),
		TriggerConfig:    triggerConfig,
		Priority:         payload.Priority,
		Enabled:          payload.Enabled,
		Locked:           payload.Locked,
		CreatedAt:        createdAt,
		AutoPrepare:      payload.AutoPrepare,
		SyncedFromMaster: true,
	}, nil
}
