package persondetect

import "time"

// Alert actions.
const (
	ActionMessage = "message"
	ActionAgent   = "agent"
	ActionDesktop = "desktop"
	ActionMission = "mission"
)

// FireSet says which actions fire for an event.
type FireSet struct{ Message, Agent, Desktop, Mission bool }

// Fired lists the fired actions in the order message, agent, desktop, mission.
func (f FireSet) Fired() []string {
	out := make([]string, 0, 4)
	if f.Message {
		out = append(out, ActionMessage)
	}
	if f.Agent {
		out = append(out, ActionAgent)
	}
	if f.Desktop {
		out = append(out, ActionDesktop)
	}
	if f.Mission {
		out = append(out, ActionMission)
	}
	return out
}

// AlertEvent is handed to the AlertDispatcher once, at event start.
type AlertEvent struct {
	EventID    int64
	CameraID   string
	CameraName string
	Zones      []string
	Score      float64
	Persons    int
	Armed      bool
	StartedAt  time.Time
	CoverPath  string // absolute path of the cover frame; empty when no start frame could be written (for example, disk full)
	CoverBoxes []Box
	Actions    Actions
	Fire       FireSet
	Language   string
}

// AlertResult is the outcome of one action. For ActionMission, "sent" means the
// event was handed to the mission manager; whether a mission then runs is up to
// the mission's own filters.
type AlertResult struct {
	Status string `json:"status"` // sent | failed | skipped
	Detail string `json:"detail,omitempty"`
}

// AlertDispatcher delivers alerts (slice 3).
type AlertDispatcher interface {
	// Dispatch must not block; report is called once per fired action, from any
	// goroutine. A report that arrives after Service.Shutdown is dropped, so after a
	// restart the stored event may lack the result of an action that was still in
	// flight.
	Dispatch(ev AlertEvent, report func(action string, result AlertResult))
}
