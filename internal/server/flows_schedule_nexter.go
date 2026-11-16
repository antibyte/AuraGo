package server

import (
	"time"

	"aurago/internal/flows"
)

// flowScheduleNexter gives the flow service the schedule half of its next run: the cron
// engine's next fire of the flow's schedule triggers (flows.ScheduleNexter). The flows
// package does not import the cron code.
type flowScheduleNexter struct{ s *Server }

var _ flows.ScheduleNexter = flowScheduleNexter{}

// NextScheduleRun implements flows.ScheduleNexter.
func (n flowScheduleNexter) NextScheduleRun(flowID string, now time.Time) (time.Time, string, bool) {
	if n.s == nil || n.s.MissionManagerV2 == nil {
		return time.Time{}, "", false
	}
	return n.s.MissionManagerV2.NextFlowScheduleRun(flowID, now)
}
