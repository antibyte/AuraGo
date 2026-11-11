package server

import (
	"fmt"

	"aurago/internal/invasion"
	"aurago/internal/invasion/bridge"
)

func (s *Server) validateInvasionWork(nestID, eggID, taskID, missionID string, required bool) error {
	nest, err := invasion.GetNest(s.InvasionDB, nestID)
	if err != nil || !nest.Active || nest.EggID != eggID {
		return fmt.Errorf("inactive or reassigned nest")
	}
	egg, err := invasion.GetEgg(s.InvasionDB, eggID)
	if err != nil || !egg.Active {
		return fmt.Errorf("inactive egg")
	}

	if taskID == "" && missionID == "" && required {
		return fmt.Errorf("artifact requires an admitted task or mission")
	}
	if taskID != "" {
		task, err := invasion.GetTaskByID(s.InvasionDB, taskID)
		if err != nil || task.NestID != nestID || task.EggID != eggID {
			return fmt.Errorf("task does not belong to authenticated nest and egg")
		}
	}
	if missionID != "" {
		if s.MissionManagerV2 == nil {
			return fmt.Errorf("mission is unavailable")
		}
		mission, ok := s.MissionManagerV2.Get(missionID)
		if !ok || mission.RunnerType != "remote" || mission.RemoteNestID != nestID || mission.RemoteEggID != eggID {
			return fmt.Errorf("mission does not belong to authenticated nest and egg")
		}
	}
	return nil
}

func (s *Server) validateInvasionArtifacts(nestID, eggID, taskID, missionID string, ids []string) error {
	for _, id := range ids {
		artifact, err := invasion.GetArtifact(s.InvasionDB, id)
		if err != nil || artifact.NestID != nestID || artifact.EggID != eggID || artifact.TaskID != taskID || artifact.MissionID != missionID {
			return fmt.Errorf("artifact does not belong to reported work")
		}
	}
	return nil
}

func (s *Server) recordInvasionTaskResult(nestID string, result bridge.ResultPayload) error {
	if !s.invasionAllowed(false) {
		return fmt.Errorf("Invasion Control is disabled")
	}
	nest, err := invasion.GetNest(s.InvasionDB, nestID)
	if err != nil {
		return err
	}
	if err := s.validateInvasionWork(nestID, nest.EggID, result.TaskID, "", true); err != nil {
		return err
	}
	if err := s.validateInvasionArtifacts(nestID, nest.EggID, result.TaskID, "", result.ArtifactIDs); err != nil {
		return err
	}
	status := "completed"
	if result.Status == "failure" {
		status = "failed"
	}
	return invasion.UpdateTaskResult(s.InvasionDB, result.TaskID, status, result.Output, result.Error, result.ArtifactIDs)
}
