package server

import (
	"database/sql"
	"log/slog"
	"time"

	"aurago/internal/planner"
	"aurago/internal/ruview"
)

// bindRuViewOperationalHints records planner issues for RuView edge hints.
// The callback is the only writer. RuView does not import the planner and does
// not send a notice. Titles and details are fixed sentences.
func bindRuViewOperationalHints(m *ruview.Manager, db *sql.DB, logger *slog.Logger) {
	if m == nil || db == nil {
		return
	}
	m.SetOperationalHint(func(event string, open bool) {
		title, detail, resolution, ok := ruviewOperationalHintCopy(event)
		if !ok {
			return
		}
		fingerprint := "ruview:" + event
		var err error
		if open {
			_, err = planner.RecordOperationalIssue(db, planner.OperationalIssue{
				Source:      "ruview",
				Context:     "ruview",
				Title:       title,
				Detail:      detail,
				Severity:    "warning",
				Kind:        planner.OperationalIssueKindRuntimeFailure,
				Reference:   event,
				Fingerprint: fingerprint,
			})
		} else {
			_, err = planner.ResolveOperationalIssue(db, fingerprint, resolution, time.Now())
		}
		if err != nil && logger != nil {
			logger.Warn("RuView operational hint was not saved", "event", event, "open", open)
		}
	})
}

func ruviewOperationalHintCopy(event string) (title, detail, resolution string, ok bool) {
	switch event {
	case "sensing_stale":
		return "RuView sensing is stale",
			"A fresh ESP32 probe was followed by a stale or disconnected probe.",
			"RuView sensing is fresh again.",
			true
	case "storage_limit":
		return "RuView storage budget is full",
			"RuView reached its storage budget.",
			"RuView storage is under the budget again.",
			true
	case "stop_unconfirmed":
		return "RuView stop was not confirmed",
			"RuView returned from a stop without releasing the runtime.",
			"RuView confirmed a later stop or start.",
			true
	default:
		return "", "", "", false
	}
}
