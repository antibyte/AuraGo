package dockerwatch

import (
	"strconv"
	"strings"
	"time"

	"aurago/internal/dockerhost"
)

const (
	labelProject = "com.docker.compose.project"
	labelService = "com.docker.compose.service"
)

// containerActions are the container events that change what the UI shows.
// Everything else the Engine reports for containers (exec_*, attach, detach,
// resize, top, commit, export, copy, archive-path, extract-to-dir, prune, ...)
// is dropped. health_status is handled apart: its detail follows a colon.
var containerActions = map[string]bool{
	"create": true, "start": true, "restart": true, "die": true, "stop": true, "kill": true,
	"pause": true, "unpause": true, "destroy": true, "rename": true, "oom": true, "update": true,
}

// shortID is the first 12 characters of an id.
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// normalizeEvent turns an Engine event into the Event the UI receives. It
// reports false for events that are not forwarded. Only the few attributes the
// UI needs are copied; label values and environment never leave this function.
func normalizeEvent(host string, ev dockerhost.EngineEvent, now time.Time) (Event, bool) {
	out := Event{
		Host:    host,
		Type:    ev.Type,
		ID:      ev.ID,
		Name:    strings.TrimPrefix(ev.Attributes["name"], "/"),
		Project: ev.Attributes[labelProject],
		Service: ev.Attributes[labelService],
	}
	when := ev.Time
	if when.IsZero() {
		when = now
	}
	out.T = when.UnixMilli()
	switch ev.Type {
	case "container":
		action, detail, _ := strings.Cut(ev.Action, ":")
		action, detail = strings.TrimSpace(action), strings.TrimSpace(detail)
		switch {
		case action == "health_status":
			out.Action, out.Health = action, detail
		case containerActions[action]:
			out.Action = action
		default:
			return Event{}, false
		}
		out.ShortID = shortID(ev.ID)
		if action == "die" {
			if code, err := strconv.Atoi(ev.Attributes["exitCode"]); err == nil {
				out.ExitCode = &code
			}
		}
	case "image", "volume", "network":
		out.Action = ev.Action
	default:
		return Event{}, false
	}
	return out, true
}
