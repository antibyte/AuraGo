package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
	"aurago/internal/tools"
)

type remoteMissionClient struct {
	hub *bridge.EggHub
	db  *sql.DB
}

type remoteMissionTarget struct {
	NestID      string `json:"nest_id"`
	NestName    string `json:"nest_name"`
	EggID       string `json:"egg_id"`
	EggName     string `json:"egg_name"`
	Connected   bool   `json:"connected"`
	HatchStatus string `json:"hatch_status"`
}

func newRemoteMissionClient(s *Server) *remoteMissionClient {
	return &remoteMissionClient{
		hub: s.EggHub,
		db:  s.InvasionDB,
	}
}

func (c *remoteMissionClient) SyncMission(ctx context.Context, mission tools.MissionV2, promptSnapshot string) error {
	if err := c.validateTarget(mission); err != nil {
		return err
	}
	payload, err := missionSyncPayloadFromMission(mission, promptSnapshot)
	if err != nil {
		return err
	}
	return runWithContext(ctx, func() error {
		return c.hub.SendMissionSync(mission.RemoteNestID, payload)
	})
}

func (c *remoteMissionClient) DeleteMission(ctx context.Context, mission tools.MissionV2) error {
	if mission.RemoteNestID == "" {
		return nil
	}
	if c.hub == nil || !c.hub.IsConnected(mission.RemoteNestID) {
		return fmt.Errorf("remote nest %s is not connected", mission.RemoteNestID)
	}
	payload := bridge.MissionDeletePayload{MissionID: mission.ID}
	return runWithContext(ctx, func() error {
		return c.hub.SendMissionDelete(mission.RemoteNestID, payload)
	})
}

func (c *remoteMissionClient) RunMission(ctx context.Context, mission tools.MissionV2, triggerType, triggerData string) error {
	if err := c.validateTarget(mission); err != nil {
		return err
	}
	payload := bridge.MissionRunPayload{
		MissionID:   mission.ID,
		TriggerType: triggerType,
		TriggerData: triggerData,
	}
	return runWithContext(ctx, func() error {
		return c.hub.SendMissionRun(mission.RemoteNestID, payload)
	})
}

func (c *remoteMissionClient) validateTarget(mission tools.MissionV2) error {
	if c.hub == nil {
		return fmt.Errorf("remote egg hub is not available")
	}
	if mission.RemoteNestID == "" {
		return fmt.Errorf("remote_nest_id is required for remote missions")
	}
	if mission.RemoteEggID == "" {
		return fmt.Errorf("remote_egg_id is required for remote missions")
	}
	if !c.hub.IsConnected(mission.RemoteNestID) {
		return fmt.Errorf("remote nest %s is not connected", mission.RemoteNestID)
	}
	if c.db != nil {
		nest, err := invasion.GetNest(c.db, mission.RemoteNestID)
		if err != nil {
			return fmt.Errorf("remote nest %s not found: %w", mission.RemoteNestID, err)
		}
		if nest.EggID != "" && nest.EggID != mission.RemoteEggID {
			return fmt.Errorf("remote nest %s is assigned to egg %s, not %s", mission.RemoteNestID, nest.EggID, mission.RemoteEggID)
		}
	}
	return nil
}

func missionSyncPayloadFromMission(mission tools.MissionV2, promptSnapshot string) (bridge.MissionSyncPayload, error) {
	var triggerConfig json.RawMessage
	if mission.TriggerConfig != nil {
		b, err := json.Marshal(mission.TriggerConfig)
		if err != nil {
			return bridge.MissionSyncPayload{}, fmt.Errorf("marshal trigger config: %w", err)
		}
		triggerConfig = b
	}
	return bridge.MissionSyncPayload{
		Revision:       mission.RemoteRevision,
		MissionID:      mission.ID,
		Name:           mission.Name,
		PromptSnapshot: promptSnapshot,
		ExecutionType:  string(mission.ExecutionType),
		Schedule:       mission.Schedule,
		TriggerType:    string(mission.TriggerType),
		TriggerConfig:  triggerConfig,
		Priority:       mission.Priority,
		Enabled:        mission.Enabled,
		Locked:         mission.Locked,
	}, nil
}

func runWithContext(ctx context.Context, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func handleMissionRemoteTargets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InvasionDB == nil || s.EggHub == nil {
			jsonError(w, "Invasion Control is not available", http.StatusServiceUnavailable)
			return
		}

		nests, err := invasion.ListNests(s.InvasionDB)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list remote targets", "Mission remote target list failed", err)
			return
		}
		eggs, err := invasion.ListEggs(s.InvasionDB)
		if err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list remote targets", "Mission remote egg list failed", err)
			return
		}
		eggNames := make(map[string]string, len(eggs))
		for _, egg := range eggs {
			eggNames[egg.ID] = egg.Name
		}

		targets := make([]remoteMissionTarget, 0)
		for _, nest := range nests {
			if nest.EggID == "" {
				continue
			}
			connected := s.EggHub.IsConnected(nest.ID)
			if !connected {
				continue
			}
			targets = append(targets, remoteMissionTarget{
				NestID:      nest.ID,
				NestName:    nest.Name,
				EggID:       nest.EggID,
				EggName:     eggNames[nest.EggID],
				Connected:   connected,
				HatchStatus: nest.HatchStatus,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"targets": targets,
		})
	}
}
