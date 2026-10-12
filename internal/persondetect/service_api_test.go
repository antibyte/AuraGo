package persondetect_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
)

func TestServiceArmedEventFiresEnabledActions(t *testing.T) {
	h := newSvcHarness(t, true)
	h.start(svcCamera("front-door"))
	state, err := h.svc.SetArmMode(context.Background(), persondetect.ArmOn, persondetect.ActorUser)
	if err != nil || state.Mode != persondetect.ArmOn || !state.Armed || state.ChangedBy != persondetect.ActorUser || state.NextChange != nil {
		t.Fatalf("SetArmMode = %+v, %v", state, err)
	}
	h.analyze("front-door", svcPerson(0.9))
	sent := h.alerts.Events()
	if len(sent) != 1 || sent[0].Fire != (persondetect.FireSet{Message: true, Desktop: true, Mission: true}) || !sent[0].Armed || sent[0].Actions.Channel != "all" {
		t.Fatalf("dispatched = %+v", sent)
	}
	summary := h.eventList()[0]
	if !summary.Armed {
		t.Fatal("event not marked armed")
	}
	for _, action := range []string{persondetect.ActionMessage, persondetect.ActionDesktop, persondetect.ActionMission} {
		if summary.Alerts[action] != (persondetect.AlertResult{Status: persondetect.AlertSent}) {
			t.Fatalf("%s = %+v", action, summary.Alerts[action])
		}
	}
	if _, ok := summary.Alerts[persondetect.ActionAgent]; ok {
		t.Fatal("a disabled action must not get a result")
	}
}

func TestServiceCooldownCountsOnlySentAlerts(t *testing.T) {
	h := newSvcHarness(t, true)
	cam := svcCamera("front-door")
	cam.Actions.Desktop = false
	h.start(cam)
	if _, err := h.svc.SetArmMode(context.Background(), persondetect.ArmOn, persondetect.ActorUser); err != nil {
		t.Fatal(err)
	}
	h.alerts.SetResult(persondetect.ActionMessage, persondetect.AlertResult{Status: persondetect.AlertFailed, Detail: "telegram: 502"})
	h.analyze(cam.ID, svcPerson(0.9)) // event 1 at 0 s: the message fails
	h.clock.Advance(31 * time.Second)
	h.analyze(cam.ID)
	h.alerts.SetResult(persondetect.ActionMessage, persondetect.AlertResult{Status: persondetect.AlertSent})
	h.clock.Advance(10 * time.Second)
	h.analyze(cam.ID, svcPerson(0.9)) // event 2 at 41 s: a failed alert starts no cooldown
	h.clock.Advance(31 * time.Second)
	h.analyze(cam.ID)
	h.clock.Advance(10 * time.Second)
	h.analyze(cam.ID, svcPerson(0.9)) // event 3 at 82 s: 41 s after the sent alert
	h.clock.Advance(31 * time.Second)
	h.analyze(cam.ID)
	h.clock.Advance(300 * time.Second)
	h.analyze(cam.ID, svcPerson(0.9)) // event 4 at 413 s: cooldown over
	var fired []bool
	for _, ev := range h.alerts.Events() {
		if !ev.Fire.Mission {
			t.Fatal("the mission trigger must fire for every event")
		}
		fired = append(fired, ev.Fire.Message)
	}
	if len(fired) != 4 || !fired[0] || !fired[1] || fired[2] || !fired[3] {
		t.Fatalf("message fired = %v, want [true true false true]", fired)
	}
	events := h.eventList() // newest first
	if events[1].Alerts[persondetect.ActionMessage] != (persondetect.AlertResult{Status: persondetect.AlertSkipped, Detail: "cooldown"}) {
		t.Fatalf("event 3 message = %+v", events[1].Alerts[persondetect.ActionMessage])
	}
	if events[3].Alerts[persondetect.ActionMessage] != (persondetect.AlertResult{Status: persondetect.AlertFailed, Detail: "telegram: 502"}) {
		t.Fatalf("event 1 message = %+v", events[3].Alerts[persondetect.ActionMessage])
	}
}

func TestServiceScheduleTransitionsAreLogged(t *testing.T) {
	h := newSvcHarness(t, false)
	settings := h.settings(svcCamera("front-door"))
	settings.Schedule = []persondetect.Window{{Days: []time.Weekday{time.Wednesday}, From: 13 * 60, To: 14 * 60}}
	h.svc.Configure(settings)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	ctx := context.Background()
	c0 := h.svc.Status().Cursor
	state, err := h.svc.SetArmMode(ctx, persondetect.ArmSchedule, persondetect.ActorAgent)
	if err != nil || state.Armed || state.NextChange == nil || !state.NextChange.Equal(svcStart.Add(time.Hour)) || state.ChangedBy != persondetect.ActorAgent {
		t.Fatalf("SetArmMode(schedule) = %+v, %v", state, err)
	}
	h.clock.Set(svcStart.Add(90 * time.Minute))
	h.svc.EvaluateArmingForTest(ctx, h.clock.Now())
	arm := h.svc.Status().Arm
	if !arm.Armed || arm.ChangedBy != persondetect.ActorSchedule || arm.NextChange == nil || !arm.NextChange.Equal(svcStart.Add(2*time.Hour)) {
		t.Fatalf("armed by schedule = %+v", arm)
	}
	changes, err := h.svc.Changes(ctx, c0)
	if err != nil || len(changes.ArmLog) != 2 {
		t.Fatalf("arm log = %+v, %v", changes.ArmLog, err)
	}
	if first := changes.ArmLog[0]; first.Mode != persondetect.ArmSchedule || first.Armed || first.Actor != persondetect.ActorAgent {
		t.Fatalf("first entry = %+v", first)
	}
	if second := changes.ArmLog[1]; !second.Armed || second.Actor != persondetect.ActorSchedule {
		t.Fatalf("second entry = %+v", second)
	}
	h.svc.EvaluateArmingForTest(ctx, h.clock.Now())
	if again, _ := h.svc.Changes(ctx, changes.Cursor); len(again.ArmLog) != 0 {
		t.Fatalf("an unchanged schedule logged again: %+v", again.ArmLog)
	}
	if _, err := h.svc.SetArmMode(ctx, persondetect.ArmSchedule, persondetect.ActorUser); err != nil {
		t.Fatal(err)
	}
	if again, _ := h.svc.Changes(ctx, changes.Cursor); len(again.ArmLog) != 0 {
		t.Fatal("setting the current mode again must not be logged")
	}
	if _, err := h.svc.SetArmMode(ctx, "panic", persondetect.ActorUser); !errors.Is(err, persondetect.ErrInvalidArmMode) {
		t.Fatalf("invalid mode err = %v", err)
	}
	if _, err := h.svc.SetArmMode(ctx, persondetect.ArmOn, persondetect.ActorSchedule); !errors.Is(err, persondetect.ErrInvalidArmMode) {
		t.Fatalf("schedule actor err = %v", err)
	}
}

func TestServiceArmModeStartsOffAndPersists(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	if arm := h.svc.Status().Arm; arm.Mode != persondetect.ArmOff || arm.Armed || arm.ChangedAt != nil {
		t.Fatalf("fresh arm state = %+v", arm)
	}
	if _, err := h.svc.SetArmMode(ctx, persondetect.ArmOn, persondetect.ActorUser); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	next := persondetect.NewManualServiceForTest(persondetect.Deps{
		Logger: svcDiscardLogger(), NewDetector: persondetecttest.NewFactory(persondetecttest.CPUInfo()).New, Now: h.clock.Now,
	})
	t.Cleanup(func() { _ = next.Shutdown(context.Background()) })
	next.Configure(h.settings(svcCamera("front-door")))
	persondetecttest.WaitDetector(t, next, persondetect.DetectorReady)
	arm := next.Status().Arm
	if arm.Mode != persondetect.ArmOn || !arm.Armed || arm.ChangedBy != persondetect.ActorUser || arm.ChangedAt == nil || !arm.ChangedAt.Equal(svcStart) {
		t.Fatalf("reloaded arm state = %+v", arm)
	}
	disabled := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger()})
	t.Cleanup(func() { _ = disabled.Shutdown(context.Background()) })
	disabled.Configure(persondetect.Settings{DataDir: t.TempDir()})
	if _, err := disabled.SetArmMode(ctx, persondetect.ArmOn, persondetect.ActorUser); !errors.Is(err, persondetect.ErrDisabled) {
		t.Fatalf("SetArmMode while disabled = %v", err)
	}
}

func TestServiceChangesReturnRowsAfterTheCursor(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	c0 := h.svc.Status().Cursor
	h.analyze("front-door", svcPerson(0.9))
	changes, err := h.svc.Changes(ctx, c0)
	if err != nil || len(changes.Events) != 1 || changes.Cursor <= c0 || changes.Status.Cursor != changes.Cursor || len(changes.ArmLog) != 0 {
		t.Fatalf("changes = %+v, %v", changes, err)
	}
	if again, _ := h.svc.Changes(ctx, changes.Cursor); len(again.Events) != 0 {
		t.Fatalf("no change expected: %+v", again.Events)
	}
	h.clock.Advance(5 * time.Second)
	h.analyze("front-door", svcPerson(0.8))
	updated, _ := h.svc.Changes(ctx, changes.Cursor)
	if len(updated.Events) != 1 || updated.Events[0].FrameCount != 2 {
		t.Fatalf("a stored frame must resend its event: %+v", updated.Events)
	}
	if all, _ := h.svc.Changes(ctx, -5); len(all.Events) != 1 {
		t.Fatalf("a negative cursor reads from the start: %+v", all.Events)
	}
}

// Changes(0) is a snapshot of the newest rows at the current cursor; an
// incremental poll that hits the cap continues from the last delivered revision.
func TestServiceChangesSnapshotAndCappedPolls(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	st := h.svc.StoreForTest()
	c0 := h.svc.Status().Cursor
	const total = 205 // one write, and so one revision, per event
	ids := make([]int64, 0, total)
	for i := 0; i < total; i++ {
		id, err := st.CreateEvent(ctx, persondetect.NewEvent{CameraID: "front-door", CameraName: "Front-door", StartedAt: svcStart.Add(time.Duration(i) * time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	snapshot, err := h.svc.Changes(ctx, 0)
	if err != nil || len(snapshot.Events) != 200 || snapshot.Events[0].ID != ids[5] || snapshot.Events[199].ID != ids[total-1] ||
		snapshot.Cursor != h.svc.Status().Cursor || snapshot.Cursor != snapshot.Status.Cursor {
		t.Fatalf("snapshot: %d events, cursor %d (status %d), %v", len(snapshot.Events), snapshot.Cursor, snapshot.Status.Cursor, err)
	}
	first, err := h.svc.Changes(ctx, c0)
	if err != nil || len(first.Events) != 200 || first.Events[0].ID != ids[0] || first.Events[199].ID != ids[199] || first.Cursor >= first.Status.Cursor {
		t.Fatalf("capped poll: %d events, cursor %d (status %d), %v", len(first.Events), first.Cursor, first.Status.Cursor, err)
	}
	rest, err := h.svc.Changes(ctx, first.Cursor)
	if err != nil || len(rest.Events) != 5 || rest.Events[0].ID != ids[200] || rest.Cursor != rest.Status.Cursor {
		t.Fatalf("next poll: %d events, cursor %d (status %d), %v", len(rest.Events), rest.Cursor, rest.Status.Cursor, err)
	}
}

func TestServiceDeleteEventRemovesFilesAndResetsTheOpenEvent(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.analyze("front-door", svcPerson(0.9))
	ev := h.eventList()[0]
	path, err := h.svc.FrameFile(ctx, *ev.CoverFrameID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.svc.DeleteEvent(ctx, ev.ID); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("frame file survived: %v", err)
	}
	if _, err := h.svc.Event(ctx, ev.ID); !errors.Is(err, persondetect.ErrUnknownEvent) {
		t.Fatalf("deleted event still readable: %v", err)
	}
	if err := h.svc.DeleteEvent(ctx, ev.ID); !errors.Is(err, persondetect.ErrUnknownEvent) {
		t.Fatalf("second delete = %v", err)
	}
	if st := h.svc.Status(); st.Storage.Bytes != 0 || st.Cameras[0].LastEventID != nil {
		t.Fatalf("status after delete = %+v", st)
	}
	h.clock.Advance(2 * time.Second)
	h.analyze("front-door", svcPerson(0.9))
	events := h.eventList()
	if len(events) != 1 || events[0].ID == ev.ID || events[0].FrameCount != 1 {
		t.Fatalf("the next hit must start a new event: %+v", events)
	}
}

func TestServiceTestMarksBoxesAgainstZonesAndThreshold(t *testing.T) {
	h := newSvcHarness(t, false)
	cam := svcCamera("front-door")
	cam.Zones = []persondetect.Zone{{Name: "Left", Points: []persondetect.Point{{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 0.5, Y: 1}, {X: 0, Y: 1}}}}
	h.start(cam)
	ctx := context.Background()
	h.factory.Script.SetBoxes(
		persondetect.Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.6},  // inside Left: counts
		persondetect.Box{X1: 0.6, Y1: 0.2, X2: 0.9, Y2: 0.9, Score: 0.9},  // outside every zone
		persondetect.Box{X1: 0.1, Y1: 0.2, X2: 0.2, Y2: 0.8, Score: 0.35}, // inside Left, below 0.5
	)
	h.frames.Set(cam.ID, persondetecttest.JPEG(640, 360))
	res, err := h.svc.Test(ctx, cam.ID, nil)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if !strings.HasPrefix(res.Image, "data:image/jpeg;base64,") || res.Width != 640 || res.Height != 360 || res.Backend != persondetect.BackendCPU || res.WallMS != persondetecttest.FakeWallMS || len(res.Boxes) != 3 {
		t.Fatalf("result = %+v", res)
	}
	if !res.Boxes[0].Counts || res.Boxes[1].Counts || res.Boxes[2].Counts {
		t.Fatalf("verdicts = %v %v %v", res.Boxes[0].Counts, res.Boxes[1].Counts, res.Boxes[2].Counts)
	}
	if len(res.Boxes[0].Zones) != 1 || res.Boxes[1].Zones == nil || len(res.Boxes[1].Zones) != 0 || len(res.Boxes[2].Zones) != 1 {
		t.Fatalf("zones = %v %v %v", res.Boxes[0].Zones, res.Boxes[1].Zones, res.Boxes[2].Zones)
	}
	calls := h.factory.Script.Calls()
	if calls[len(calls)-1].MinScore != persondetect.TestMinScore {
		t.Fatalf("Test must ask for every box from %v up", persondetect.TestMinScore)
	}

	h.frames.Set("garage", persondetecttest.JPEG(320, 180))
	override := persondetect.Camera{MinScore: 0.3}
	res, err = h.svc.Test(ctx, "garage", &override)
	if err != nil || !res.Boxes[0].Counts || !res.Boxes[1].Counts || !res.Boxes[2].Counts {
		t.Fatalf("override without zones = %+v, %v", res.Boxes, err)
	}
	if _, err := h.svc.Test(ctx, "garage", nil); !errors.Is(err, persondetect.ErrUnknownCamera) {
		t.Fatalf("unknown camera err = %v", err)
	}
	h.frames.Fail(cam.ID, errors.New("offline"))
	if _, err := h.svc.Test(ctx, cam.ID, nil); !errors.Is(err, persondetect.ErrFrameUnavailable) {
		t.Fatalf("frame failure err = %v", err)
	}

	waiting := newSvcHarness(t, false)
	waiting.factory.SetGate(make(chan struct{}))
	waiting.svc.Configure(waiting.settings(svcCamera("front-door")))
	if _, err := waiting.svc.Test(ctx, "front-door", nil); !errors.Is(err, persondetect.ErrNotReady) {
		t.Fatalf("detector not ready err = %v", err)
	}
	off := newSvcHarness(t, false)
	if _, err := off.svc.Test(ctx, "front-door", nil); !errors.Is(err, persondetect.ErrDisabled) {
		t.Fatalf("disabled err = %v", err)
	}
}

// listKeys are the JSON keys whose values are lists in the C4 shapes.
var listKeys = map[string]bool{"events": true, "arm_log": true, "cameras": true, "providers": true, "zones": true, "boxes": true, "frames": true}

// requireNoNullLists fails when a list key anywhere in value's JSON is null.
func requireNoNullLists(t *testing.T, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for key, child := range v {
				if child == nil && listKeys[key] {
					t.Errorf("%s: %s.%s is null in %s", name, path, key, raw)
				}
				walk(path+"."+key, child)
			}
		case []any:
			for i, child := range v {
				walk(fmt.Sprintf("%s[%d]", path, i), child)
			}
		}
	}
	walk("", doc)
}

// Every list in a JSON response is [], never null, also when it is empty; and
// Changes never fails because detection is off or its database is not open.
func TestServiceEmptyResponsesHaveNoNullLists(t *testing.T) {
	ctx := context.Background()
	fresh := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger()})
	persondetecttest.StopOnCleanup(t, fresh)
	changes, err := fresh.Changes(ctx, 0)
	if err != nil || changes.Events == nil || changes.ArmLog == nil {
		t.Fatalf("Changes while disabled = %+v, %v", changes, err)
	}
	requireNoNullLists(t, "changes while disabled", changes)
	requireNoNullLists(t, "status without cameras", fresh.Status())

	broken := newSvcHarness(t, false)
	broken.svc.SetStoreOpenerForTest(func(string, *slog.Logger) (*persondetect.EventStore, error) {
		return nil, errors.New("database disk image is malformed")
	})
	broken.svc.Configure(broken.settings(svcCamera("front-door")))
	persondetecttest.WaitDetector(t, broken.svc, persondetect.DetectorError)
	changes, err = broken.svc.Changes(ctx, 0)
	if err != nil || len(changes.Events) != 0 || !strings.Contains(changes.Status.Detector.Error, "malformed") {
		t.Fatalf("Changes without a database = %+v, %v", changes, err)
	}
	requireNoNullLists(t, "changes without a database", changes)

	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	changes, err = h.svc.Changes(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	requireNoNullLists(t, "changes without rows", changes)
	id, err := h.svc.StoreForTest().CreateEvent(ctx, persondetect.NewEvent{CameraID: "front-door", CameraName: "Front-door", StartedAt: svcStart})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := h.svc.Event(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	requireNoNullLists(t, "event without frames", detail)
	h.frames.Set("front-door", persondetecttest.JPEG(64, 36))
	res, err := h.svc.Test(ctx, "front-door", nil)
	if err != nil {
		t.Fatal(err)
	}
	requireNoNullLists(t, "test without boxes", res)
}

// ArmState names the zone the schedule windows are evaluated in.
func TestServiceArmStateNamesTheServerTimeZone(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	h := newSvcHarness(t, false)
	settings := h.settings(svcCamera("front-door"))
	settings.Location = berlin
	h.svc.Configure(settings)
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if arm := h.svc.Status().Arm; arm.TimeZone != "CEST" || arm.UTCOffset != "+02:00" {
		t.Fatalf("summer arm state = %+v", arm)
	}
	h.clock.Set(time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC))
	state, err := h.svc.SetArmMode(context.Background(), persondetect.ArmOn, persondetect.ActorUser)
	if err != nil || state.TimeZone != "CET" || state.UTCOffset != "+01:00" {
		t.Fatalf("winter arm state = %+v, %v", state, err)
	}
}

// Switching detection off ends the open event at its last hit; the change feed
// still delivers that end, and a snapshot while off includes the arm history.
func TestServiceChangesDeliverRowsWhileDetectionIsOff(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	if _, err := h.svc.SetArmMode(ctx, persondetect.ArmOn, persondetect.ActorUser); err != nil {
		t.Fatal(err)
	}
	h.analyze("front-door", svcPerson(0.9))
	r := h.svc.Status().Cursor
	off := h.settings(svcCamera("front-door"))
	off.Enabled = false
	h.svc.Configure(off)
	if !h.svc.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("background work did not finish")
	}
	changes, err := h.svc.Changes(ctx, r)
	if err != nil || changes.Status.Enabled || len(changes.Events) != 1 || changes.Events[0].EndedAt == nil ||
		!changes.Events[0].EndedAt.Equal(svcStart) || changes.Cursor <= r || changes.Cursor != changes.Status.Cursor {
		t.Fatalf("changes after switching off = %+v, %v (cursor before %d)", changes, err, r)
	}
	snapshot, err := h.svc.Changes(ctx, 0)
	if err != nil || len(snapshot.ArmLog) != 1 || snapshot.ArmLog[0].Mode != persondetect.ArmOn || len(snapshot.Events) != 1 || snapshot.Cursor != changes.Cursor {
		t.Fatalf("snapshot while off = %+v, %v", snapshot, err)
	}
}

// Switching detection off (for privacy, say) must not keep recorded frames from
// being deleted: DeleteEvent needs only the open database.
func TestServiceDeletesEventsWhileDetectionIsOff(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.analyze("front-door", svcPerson(0.9))
	ev := h.eventList()[0]
	path, err := h.svc.FrameFile(ctx, *ev.CoverFrameID, false)
	if err != nil {
		t.Fatal(err)
	}
	off := h.settings(svcCamera("front-door"))
	off.Enabled = false
	h.svc.Configure(off)
	if !h.svc.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("background work did not finish")
	}
	if err := h.svc.DeleteEvent(ctx, ev.ID); err != nil {
		t.Fatalf("DeleteEvent while off: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("frame file survived: %v", err)
	}
	if _, err := h.svc.Event(ctx, ev.ID); !errors.Is(err, persondetect.ErrUnknownEvent) {
		t.Fatalf("deleted event still readable: %v", err)
	}
	if st := h.svc.Status(); st.Enabled || st.Storage.Bytes != 0 {
		t.Fatalf("status after delete = %+v", st)
	}
}
