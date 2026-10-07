package server

import (
	"net/http"
	"time"

	"aurago/internal/tools"
)

func dispatchDesktopOwnedMission(s *Server, r *http.Request, id, trigger, data string) error {
	// A flow mission never enters the agent queue: RunNow and TriggerMission start a run in
	// the flow service. The owner path (QueueOwnedMission) would queue it for the agent, and
	// the dispatcher drops flow items, so route flows first. The Desktop write admission of
	// the request already ran.
	if mission, ok := s.MissionManagerV2.Get(id); ok && mission.ExecutionType == tools.ExecutionFlow {
		if trigger == "manual" {
			return s.MissionManagerV2.RunNow(id)
		}
		return s.MissionManagerV2.TriggerMission(id, trigger, data)
	}
	if grant, _ := r.Context().Value(desktopRunContextKey{}).(*desktopRunGrant); grant != nil && grant.server == s {
		ctx, release, err := s.beginDesktopBackgroundRun(40 * time.Minute)
		if err != nil {
			return err
		}
		return s.MissionManagerV2.QueueOwnedMission(ctx, release, id, trigger, data)
	}
	if trigger == "manual" {
		return s.MissionManagerV2.RunNow(id)
	}
	return s.MissionManagerV2.TriggerMission(id, trigger, data)
}
