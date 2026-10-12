package persondetect

import (
	"testing"
	"time"
)

// allSent is a LastSent with the same time for every action.
func allSent(at time.Time) LastSent {
	return LastSent{Message: at, Agent: at, Desktop: at}
}

// wantSkipped checks that skipped holds exactly the given actions, each skipped for reason.
func wantSkipped(t *testing.T, skipped map[string]AlertResult, reason string, actions ...string) {
	t.Helper()
	if len(skipped) != len(actions) {
		t.Fatalf("skipped = %+v, want exactly %v skipped for %q", skipped, actions, reason)
	}
	for _, action := range actions {
		if got := skipped[action]; got != (AlertResult{Status: AlertSkipped, Detail: reason}) {
			t.Fatalf("skipped[%s] = %+v, want skipped %q (all: %+v)", action, got, reason, skipped)
		}
	}
}

func TestDecideFire(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}

	fire, skipped := decideFire(true, now, LastSent{}, cam)
	if fire != (FireSet{Message: true, Agent: true, Desktop: true, Mission: true}) || len(skipped) != 0 {
		t.Fatalf("armed, never sent = %+v %+v", fire, skipped)
	}
	fire, skipped = decideFire(false, now, LastSent{}, cam)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("disarmed = %+v", fire)
	}
	wantSkipped(t, skipped, "disarmed", ActionMessage, ActionAgent, ActionDesktop)

	fire, skipped = decideFire(true, now, allSent(now.Add(-time.Minute)), cam)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("cooldown = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionMessage, ActionAgent, ActionDesktop)

	fire, skipped = decideFire(true, now, allSent(now.Add(-5*time.Minute)), cam)
	if fire != (FireSet{Message: true, Agent: true, Desktop: true, Mission: true}) || len(skipped) != 0 {
		t.Fatalf("cooldown elapsed exactly = %+v %+v", fire, skipped)
	}
	noCooldown := cam
	noCooldown.Cooldown = 0
	if fire, skipped := decideFire(true, now, allSent(now), noCooldown); !fire.Message || !fire.Agent || !fire.Desktop || len(skipped) != 0 {
		t.Fatalf("cooldown 0 = %+v %+v", fire, skipped)
	}
	desktopOnly := Camera{Cooldown: time.Minute, Actions: Actions{Desktop: true}}
	fire, skipped = decideFire(false, now, LastSent{}, desktopOnly)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("disarmed desktop-only = %+v", fire)
	}
	wantSkipped(t, skipped, "disarmed", ActionDesktop)
	fire, skipped = decideFire(true, now, allSent(now.Add(-time.Second)), desktopOnly)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("cooldown desktop-only = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionDesktop)
}

// The cooldown belongs to each action: an action that was not sent (failed or skipped)
// must not be silenced by another action that was.
func TestDecideFireCooldownIsPerAction(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}

	// The message failed at the last event while the desktop toast was sent: the next
	// event must still send the message, only the desktop toast is in its cooldown.
	fire, skipped := decideFire(true, now, LastSent{Desktop: now.Add(-time.Minute)}, cam)
	if fire != (FireSet{Message: true, Agent: true, Mission: true}) {
		t.Fatalf("message failed, desktop sent = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionDesktop)

	recent := now.Add(-time.Minute)
	for _, tc := range []struct {
		name string
		last LastSent
		fire FireSet
		cool string
	}{
		{"only the message was sent", LastSent{Message: recent}, FireSet{Agent: true, Desktop: true, Mission: true}, ActionMessage},
		{"only the agent was sent", LastSent{Agent: recent}, FireSet{Message: true, Desktop: true, Mission: true}, ActionAgent},
		{"only the desktop was sent", LastSent{Desktop: recent}, FireSet{Message: true, Agent: true, Mission: true}, ActionDesktop},
	} {
		fire, skipped := decideFire(true, now, tc.last, cam)
		if fire != tc.fire {
			t.Fatalf("%s: fire = %+v, want %+v", tc.name, fire, tc.fire)
		}
		wantSkipped(t, skipped, "cooldown", tc.cool)
	}

	// Each action has its own age: the message is cooled, the agent is not, the desktop was never sent.
	fire, skipped = decideFire(true, now, LastSent{Message: now.Add(-10 * time.Minute), Agent: now.Add(-4 * time.Minute)}, cam)
	if fire != (FireSet{Message: true, Desktop: true, Mission: true}) {
		t.Fatalf("mixed ages = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionAgent)

	// A cooldown only ever concerns actions the camera has enabled.
	messageOnly := Camera{Cooldown: 5 * time.Minute, Actions: Actions{Message: true}}
	fire, skipped = decideFire(true, now, LastSent{Message: now.Add(-10 * time.Minute), Desktop: recent}, messageOnly)
	if fire != (FireSet{Message: true, Mission: true}) || len(skipped) != 0 {
		t.Fatalf("message-only camera = %+v %+v", fire, skipped)
	}

	// The mission trigger has no cooldown, and "disarmed" wins over "cooldown".
	fire, skipped = decideFire(true, now, allSent(now), cam)
	if !fire.Mission || fire.Message || fire.Agent || fire.Desktop {
		t.Fatalf("everything in cooldown = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionMessage, ActionAgent, ActionDesktop)
	fire, skipped = decideFire(false, now, allSent(now), cam)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("disarmed, in cooldown = %+v", fire)
	}
	wantSkipped(t, skipped, "disarmed", ActionMessage, ActionAgent, ActionDesktop)
}

// Timestamps carry no monotonic reading (the service truncates them to ms UTC), so a
// wall clock that stepped back makes now earlier than the last sent alert. That must
// count as an elapsed cooldown: otherwise the step plus the cooldown would block alerts.
func TestDecideFireTreatsABackwardClockStepAsElapsedCooldown(t *testing.T) {
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}
	lastSent := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, step := range []time.Duration{time.Millisecond, time.Minute, time.Hour, 24 * time.Hour} {
		now := lastSent.Add(-step)
		fire, skipped := decideFire(true, now, allSent(lastSent), cam)
		if fire != (FireSet{Message: true, Agent: true, Desktop: true, Mission: true}) || len(skipped) != 0 {
			t.Fatalf("clock %v behind the last sent alert = %+v %+v, want every enabled action fired", step, fire, skipped)
		}
	}

	// The step does not weaken the other rules or the cooldown once the clock runs forward again.
	now := lastSent.Add(-time.Hour)
	fire, skipped := decideFire(false, now, allSent(lastSent), cam)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("disarmed after a clock step = %+v", fire)
	}
	wantSkipped(t, skipped, "disarmed", ActionMessage, ActionAgent, ActionDesktop)
	fire, skipped = decideFire(true, lastSent.Add(time.Minute), allSent(lastSent), cam)
	if fire != (FireSet{Mission: true}) {
		t.Fatalf("one minute after the last sent alert = %+v", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionMessage, ActionAgent, ActionDesktop)
}

// The clock-step rule is applied to each action's own timestamp.
func TestDecideFireClockStepConcernsOnlyTheActionsSentInTheFuture(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}
	last := LastSent{
		Message: now.Add(time.Hour),         // sent "in the future": the clock stepped back since
		Agent:   now.Add(-time.Minute),      // sent a minute ago: still in its cooldown
		Desktop: now.Add(-10 * time.Minute), // sent long ago: elapsed
	}
	fire, skipped := decideFire(true, now, last, cam)
	if fire != (FireSet{Message: true, Desktop: true, Mission: true}) {
		t.Fatalf("fire = %+v, want message (clock step) and desktop (elapsed) but not the agent", fire)
	}
	wantSkipped(t, skipped, "cooldown", ActionAgent)

	// Just in front of now by a millisecond is a clock step too; exactly now is not.
	fire, _ = decideFire(true, now, LastSent{Message: now.Add(time.Millisecond), Agent: now}, cam)
	if !fire.Message || fire.Agent {
		t.Fatalf("fire = %+v, want the message fired (1 ms ahead) and the agent in cooldown (same instant)", fire)
	}
}

// The full rule as a table: an enabled action fires when armed and its own cooldown
// has elapsed (never sent, sent in the future, or sent at least Cooldown ago); a skipped
// enabled action carries "disarmed" when disarmed, else "cooldown"; the mission always fires.
func TestDecideFireTruthTable(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	ages := []time.Time{
		{}, now.Add(-time.Minute), now.Add(-5 * time.Minute), now.Add(-10 * time.Minute), now, now.Add(time.Millisecond),
	}
	rows := 0
	for _, armed := range []bool{false, true} {
		for bits := 0; bits < 8; bits++ {
			acts := Actions{Message: bits&1 != 0, Agent: bits&2 != 0, Desktop: bits&4 != 0}
			for _, cooldown := range []time.Duration{0, 5 * time.Minute, -time.Second} {
				for _, m := range ages {
					for _, a := range ages {
						for _, d := range ages {
							cam := Camera{ID: "c", Cooldown: cooldown, Actions: acts}
							fire, skipped := decideFire(armed, now, LastSent{Message: m, Agent: a, Desktop: d}, cam)
							rows++
							check := func(action string, enabled, fired bool, last time.Time) {
								cooled := last.IsZero() || now.Before(last) || now.Sub(last) >= cooldown
								if want := enabled && armed && cooled; fired != want {
									t.Fatalf("armed=%v cooldown=%v %s enabled=%v last=%v: fired=%v, want %v", armed, cooldown, action, enabled, last, fired, want)
								}
								res, recorded := skipped[action]
								switch {
								case !enabled && recorded:
									t.Fatalf("disabled %s recorded as %+v", action, res)
								case enabled && !fired && !recorded:
									t.Fatalf("enabled %s neither fired nor recorded", action)
								case fired && recorded:
									t.Fatalf("fired %s also recorded as %+v", action, res)
								case recorded && !armed && res != (AlertResult{Status: AlertSkipped, Detail: "disarmed"}):
									t.Fatalf("%s skipped while disarmed as %+v", action, res)
								case recorded && armed && res != (AlertResult{Status: AlertSkipped, Detail: "cooldown"}):
									t.Fatalf("%s skipped while armed as %+v", action, res)
								}
							}
							check(ActionMessage, acts.Message, fire.Message, m)
							check(ActionAgent, acts.Agent, fire.Agent, a)
							check(ActionDesktop, acts.Desktop, fire.Desktop, d)
							if !fire.Mission {
								t.Fatalf("mission did not fire (armed=%v)", armed)
							}
							if _, recorded := skipped[ActionMission]; recorded {
								t.Fatal("mission recorded as skipped")
							}
						}
					}
				}
			}
		}
	}
	if rows != 2*8*3*len(ages)*len(ages)*len(ages) {
		t.Fatalf("checked %d rows", rows)
	}
}
