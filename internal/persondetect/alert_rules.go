package persondetect

import "time"

// decideFire applies the alert rules at event start. Message, agent and desktop
// fire when the action is enabled, the system is armed and the camera's cooldown
// since its last sent alert has elapsed; the mission trigger always fires (missions
// filter on their own). skipped records why an enabled action did not fire.
func decideFire(armed bool, now, lastSent time.Time, cam Camera) (FireSet, map[string]AlertResult) {
	fire := FireSet{Mission: true}
	skipped := map[string]AlertResult{}
	// The timestamps are wall-clock values without a monotonic reading, so now can
	// precede lastSent when the clock stepped back. That counts as an elapsed
	// cooldown; otherwise the step plus the cooldown would block alerts.
	cooled := lastSent.IsZero() || now.Before(lastSent) || now.Sub(lastSent) >= cam.Cooldown
	for _, action := range []struct {
		name    string
		enabled bool
		target  *bool
	}{
		{ActionMessage, cam.Actions.Message, &fire.Message},
		{ActionAgent, cam.Actions.Agent, &fire.Agent},
		{ActionDesktop, cam.Actions.Desktop, &fire.Desktop},
	} {
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
