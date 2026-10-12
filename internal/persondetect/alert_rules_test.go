package persondetect

import (
	"testing"
	"time"
)

func TestDecideFire(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}

	fire, skipped := decideFire(true, now, time.Time{}, cam)
	if fire != (FireSet{Message: true, Agent: true, Desktop: true, Mission: true}) || len(skipped) != 0 {
		t.Fatalf("armed, never sent = %+v %+v", fire, skipped)
	}
	fire, skipped = decideFire(false, now, time.Time{}, cam)
	if fire != (FireSet{Mission: true}) || len(skipped) != 3 || skipped[ActionMessage] != (AlertResult{Status: AlertSkipped, Detail: "disarmed"}) {
		t.Fatalf("disarmed = %+v %+v", fire, skipped)
	}
	fire, skipped = decideFire(true, now, now.Add(-time.Minute), cam)
	if fire != (FireSet{Mission: true}) || skipped[ActionDesktop] != (AlertResult{Status: AlertSkipped, Detail: "cooldown"}) {
		t.Fatalf("cooldown = %+v %+v", fire, skipped)
	}
	if fire, _ := decideFire(true, now, now.Add(-5*time.Minute), cam); !fire.Message || !fire.Agent || !fire.Desktop {
		t.Fatalf("cooldown elapsed exactly = %+v", fire)
	}
	noCooldown := cam
	noCooldown.Cooldown = 0
	if fire, _ := decideFire(true, now, now, noCooldown); !fire.Message {
		t.Fatalf("cooldown 0 = %+v", fire)
	}
	desktopOnly := Camera{Cooldown: time.Minute, Actions: Actions{Desktop: true}}
	if _, skipped := decideFire(false, now, time.Time{}, desktopOnly); len(skipped) != 1 || skipped[ActionDesktop].Detail != "disarmed" {
		t.Fatalf("disabled actions must not be recorded: %+v", skipped)
	}
}

// Timestamps carry no monotonic reading (the service truncates them to ms UTC), so a
// wall clock that stepped back makes now earlier than the last sent alert. That must
// count as an elapsed cooldown: otherwise the step plus the cooldown would block alerts.
func TestDecideFireTreatsABackwardClockStepAsElapsedCooldown(t *testing.T) {
	cam := Camera{ID: "cam", Cooldown: 5 * time.Minute, Actions: Actions{Message: true, Agent: true, Desktop: true}}
	lastSent := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, step := range []time.Duration{time.Millisecond, time.Minute, time.Hour, 24 * time.Hour} {
		now := lastSent.Add(-step)
		fire, skipped := decideFire(true, now, lastSent, cam)
		if fire != (FireSet{Message: true, Agent: true, Desktop: true, Mission: true}) || len(skipped) != 0 {
			t.Fatalf("clock %v behind the last sent alert = %+v %+v, want every enabled action fired", step, fire, skipped)
		}
	}

	// The step does not weaken the other rules or the cooldown once the clock runs forward again.
	now := lastSent.Add(-time.Hour)
	if fire, skipped := decideFire(false, now, lastSent, cam); fire != (FireSet{Mission: true}) || skipped[ActionMessage].Detail != "disarmed" {
		t.Fatalf("disarmed after a clock step = %+v %+v", fire, skipped)
	}
	if fire, skipped := decideFire(true, lastSent.Add(time.Minute), lastSent, cam); fire != (FireSet{Mission: true}) || skipped[ActionMessage].Detail != "cooldown" {
		t.Fatalf("one minute after the last sent alert = %+v %+v, want the cooldown to hold", fire, skipped)
	}
}
