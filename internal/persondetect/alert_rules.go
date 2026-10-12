package persondetect

import "time"

// LastSent is, for one camera, the last time each action was SENT (the alert
// reported AlertSent). The zero time means never. An action that failed or was
// skipped does not count: the cooldown of an action only follows its own deliveries,
// so a failing Telegram message is not silenced by a desktop toast that went out.
// The mission trigger has no entry because it has no cooldown.
type LastSent struct{ Message, Agent, Desktop time.Time }

// decideFire applies the alert rules at event start. Message, agent and desktop
// fire when the action is enabled, the system is armed and the cooldown of that
// action since its own last sent alert (last) has elapsed; the mission trigger
// always fires (missions filter on their own) and has no cooldown. skipped records
// why an enabled action did not fire; "disarmed" takes precedence over "cooldown".
//
// A last time that is after now means the wall clock stepped back, because the
// timestamps are wall-clock values without a monotonic reading (the service
// truncates them to milliseconds UTC). That counts as an elapsed cooldown for that
// action; otherwise the step plus the cooldown would block alerts.
//
// The caller must read now in the same critical section as the LastSent values it
// passes. If an alert report records its sent time between the two reads, that time
// is later than a now read earlier and looks like a backward clock step, which would
// fire the action although its cooldown has not elapsed.
func decideFire(armed bool, now time.Time, last LastSent, cam Camera) (FireSet, map[string]AlertResult) {
	fire := FireSet{Mission: true}
	skipped := map[string]AlertResult{}
	for _, action := range []struct {
		name    string
		enabled bool
		lastAt  time.Time
		target  *bool
	}{
		{ActionMessage, cam.Actions.Message, last.Message, &fire.Message},
		{ActionAgent, cam.Actions.Agent, last.Agent, &fire.Agent},
		{ActionDesktop, cam.Actions.Desktop, last.Desktop, &fire.Desktop},
	} {
		cooled := action.lastAt.IsZero() || now.Before(action.lastAt) || now.Sub(action.lastAt) >= cam.Cooldown
		switch {
		case !action.enabled:
		case !armed:
			skipped[action.name] = AlertResult{Status: AlertSkipped, Detail: "disarmed"}
		case !cooled:
			skipped[action.name] = AlertResult{Status: AlertSkipped, Detail: "cooldown"}
		default:
			*action.target = true
		}
	}
	return fire, skipped
}
