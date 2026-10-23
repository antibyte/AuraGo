package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/planner"
	"aurago/internal/tools"
)

func (s *Server) setMissionError(ctx context.Context, missionID, title, detail string) {
	if s.missionRunTracker().consumeCancelled(missionID) {
		s.Logger.Info("[MissionV2] Mission run cancelled by user", "mission_id", missionID)
		s.MissionManagerV2.SetResult(missionID, tools.MissionResultError, tools.MissionCancelledOutput)
		broadcastMissionState(s)
		return
	}
	if ctx.Err() == context.Canceled {
		s.Logger.Info("[MissionV2] Mission run interrupted by owner cancellation", "mission_id", missionID)
		s.MissionManagerV2.SetResult(missionID, tools.MissionResultError, "Interrupted because the mission owner was cancelled")
		broadcastMissionState(s)
		return
	}
	if s.PlannerDB != nil {
		missionLabel := missionID
		if mission, ok := s.MissionManagerV2.Get(missionID); ok && strings.TrimSpace(mission.Name) != "" {
			missionLabel = strings.TrimSpace(mission.Name)
		}
		if title == "" {
			title = fmt.Sprintf("Mission %s failed", missionLabel)
		}
		issue := planner.OperationalIssue{
			Source:      "mission",
			Context:     missionID,
			Title:       title,
			Detail:      detail,
			Severity:    "error",
			Reference:   missionID,
			Fingerprint: "mission|" + missionID,
			OccurredAt:  time.Now(),
		}
		if issueID, err := planner.RecordOperationalIssue(s.PlannerDB, issue); err != nil {
			s.Logger.Warn("[MissionV2] Failed to record internal operational issue", "mission_id", missionID, "error", err)
		} else {
			s.MissionManagerV2.NotifyPlannerOperationalIssue(issueID, issue.Source, issue.Severity, issue.Title)
		}
	}
	s.MissionManagerV2.SetResult(missionID, tools.MissionResultError, detail)
	broadcastMissionState(s)
}
