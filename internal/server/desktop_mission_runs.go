package server

import (
	"net/http"
	"time"
)

func dispatchDesktopOwnedMission(s *Server, r *http.Request, id, trigger, data string) error {
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
