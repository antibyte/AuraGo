package persondetect_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"aurago/internal/persondetect"
	"aurago/internal/persondetect/persondetecttest"
)

func svcPerson(score float64) persondetect.Box {
	return persondetect.Box{X1: 0.4, Y1: 0.3, X2: 0.5, Y2: 0.9, Score: score}
}

// analyze fetches and analyzes one frame of cameraID at the clock's current time
// and waits for the frame storage and alert dispatch it queued; the detector
// returns boxes.
func (h *svcHarness) analyze(cameraID string, boxes ...persondetect.Box) {
	h.t.Helper()
	h.factory.Script.SetBoxes(boxes...)
	h.frames.Set(cameraID, persondetecttest.JPEG(640, 360))
	h.svc.FetchOnceForTest(context.Background(), cameraID)
	if !h.svc.ProcessOneForTest(context.Background()) {
		h.t.Fatalf("no frame of %s was queued", cameraID)
	}
	h.waitStorage()
}

func (h *svcHarness) waitStorage() {
	h.t.Helper()
	if !h.svc.WaitStorageForTest(5 * time.Second) {
		h.t.Fatal("frame storage did not finish")
	}
}

func (h *svcHarness) eventList() []persondetect.EventSummary {
	h.t.Helper()
	page, err := h.svc.Events(context.Background(), persondetect.EventQuery{Limit: 100})
	if err != nil {
		h.t.Fatalf("Events: %v", err)
	}
	return page.Events
}

func TestServiceStartsAnEventOnTwoHitsAndStoresFrames(t *testing.T) {
	h := newSvcHarness(t, true)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.analyze("front-door", svcPerson(0.6))
	if len(h.eventList()) != 0 {
		t.Fatal("one weak hit started an event")
	}
	h.clock.Advance(2 * time.Second)
	h.analyze("front-door")
	h.clock.Advance(2 * time.Second)
	h.analyze("front-door", svcPerson(0.65))
	events := h.eventList()
	if len(events) != 1 {
		t.Fatalf("events = %d", len(events))
	}
	ev := events[0]
	if !ev.StartedAt.Equal(svcStart) || ev.EndedAt != nil || ev.FrameCount != 2 || ev.MaxScore != 0.65 || ev.MaxPersons != 1 ||
		ev.Armed || ev.CoverFrameID == nil || ev.CameraName != "Front-door" || ev.CameraID != "front-door" {
		t.Fatalf("event = %+v", ev)
	}
	detail, err := h.svc.Event(ctx, ev.ID)
	if err != nil || len(detail.Frames) != 2 || detail.Frames[1].ID != *ev.CoverFrameID || len(detail.Frames[0].Boxes) != 1 || detail.Frames[0].Width != 640 {
		t.Fatalf("detail = %+v, %v", detail, err)
	}
	path, err := h.svc.FrameFile(ctx, *ev.CoverFrameID, false)
	if err != nil || filepath.Base(filepath.Dir(path)) != "front-door" || !strings.HasSuffix(filepath.ToSlash(path), "/2026/10/14/front-door/"+filepath.Base(path)) {
		t.Fatalf("frame path = %q, %v", path, err)
	}
	thumbPath, err := h.svc.FrameFile(ctx, *ev.CoverFrameID, true)
	if err != nil {
		t.Fatal(err)
	}
	thumb, _ := os.ReadFile(thumbPath)
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb)); err != nil || cfg.Width != persondetect.ThumbnailWidth {
		t.Fatalf("thumbnail = %+v, %v", cfg, err)
	}
	if _, err := h.svc.FrameFile(ctx, 9999, false); !errors.Is(err, persondetect.ErrUnknownEvent) {
		t.Fatalf("unknown frame err = %v", err)
	}
	st := h.svc.Status()
	cam := st.Cameras[0]
	if cam.State != persondetect.CameraActive || cam.LastEventID == nil || *cam.LastEventID != ev.ID || cam.AnalysesPerMinute != 3 {
		t.Fatalf("camera status = %+v", cam)
	}
	if st.Detector.WallMS != persondetecttest.FakeWallMS || st.Detector.CPUMS != persondetecttest.FakeCPUMS || st.Storage.Bytes <= 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestServiceThrottlesFramesAndEndsThirtySecondsAfterTheLastHit(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	for i := 0; i < 12; i++ {
		h.clock.Advance(time.Second)
		h.analyze("front-door", svcPerson(0.7))
	}
	if ev := h.eventList()[0]; ev.FrameCount != 3 || ev.EndedAt != nil {
		t.Fatalf("event after 12 s of hits = %+v, want 3 frames (0, 5, 10 s)", ev)
	}
	h.clock.Advance(29 * time.Second)
	h.analyze("front-door")
	if h.eventList()[0].EndedAt != nil {
		t.Fatal("ended 29 s after the last hit")
	}
	h.clock.Advance(time.Second)
	h.analyze("front-door")
	if ev := h.eventList()[0]; ev.EndedAt == nil || !ev.EndedAt.Equal(svcStart.Add(12*time.Second)) {
		t.Fatalf("ended_at = %v, want the last hit", ev.EndedAt)
	}
}

func TestServiceCapsFramesPerEvent(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	for i := 0; i < 30; i++ {
		h.clock.Advance(5 * time.Second)
		h.analyze("front-door", svcPerson(0.7))
	}
	if ev := h.eventList()[0]; ev.FrameCount != persondetect.EventMaxFrames {
		t.Fatalf("frames = %d, want %d", ev.FrameCount, persondetect.EventMaxFrames)
	}
}

func TestServiceDisarmedEventFiresOnlyTheMissionTrigger(t *testing.T) {
	h := newSvcHarness(t, true)
	cam := svcCamera("front-door")
	cam.Actions.Agent = true
	h.start(cam)
	h.analyze(cam.ID, svcPerson(0.85))
	sent := h.alerts.Events()
	if len(sent) != 1 {
		t.Fatalf("dispatched %d events", len(sent))
	}
	ev := sent[0]
	if ev.Fire != (persondetect.FireSet{Mission: true}) || ev.Armed || ev.CameraName != "Front-door" || ev.Persons != 1 ||
		ev.Score != 0.85 || ev.Language != "de" || len(ev.CoverBoxes) != 1 || ev.Zones == nil || len(ev.Zones) != 0 || !ev.StartedAt.Equal(svcStart) {
		t.Fatalf("alert event = %+v", ev)
	}
	if _, err := os.Stat(ev.CoverPath); err != nil {
		t.Fatalf("cover path: %v", err)
	}
	alerts := h.eventList()[0].Alerts
	for _, action := range []string{persondetect.ActionMessage, persondetect.ActionAgent, persondetect.ActionDesktop} {
		if alerts[action] != (persondetect.AlertResult{Status: persondetect.AlertSkipped, Detail: "disarmed"}) {
			t.Fatalf("%s = %+v", action, alerts[action])
		}
	}
	if alerts[persondetect.ActionMission] != (persondetect.AlertResult{Status: persondetect.AlertSent}) {
		t.Fatalf("mission = %+v", alerts[persondetect.ActionMission])
	}
}

func TestServiceWithoutDispatcherReportsAlertsUnavailable(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	alerts := h.eventList()[0].Alerts
	if alerts[persondetect.ActionMission] != (persondetect.AlertResult{Status: persondetect.AlertSkipped, Detail: "alerts unavailable"}) ||
		alerts[persondetect.ActionMessage].Detail != "disarmed" {
		t.Fatalf("alerts = %+v", alerts)
	}
}

func TestServiceZonesFilterHits(t *testing.T) {
	h := newSvcHarness(t, false)
	cam := svcCamera("front-door")
	cam.Zones = []persondetect.Zone{{Name: "Left", Points: []persondetect.Point{{X: 0, Y: 0}, {X: 0.5, Y: 0}, {X: 0.5, Y: 1}, {X: 0, Y: 1}}}}
	h.start(cam)
	right := persondetect.Box{X1: 0.6, Y1: 0.2, X2: 0.8, Y2: 0.9, Score: 0.95}
	h.analyze(cam.ID, right)
	if len(h.eventList()) != 0 {
		t.Fatal("a person outside every zone started an event")
	}
	h.clock.Advance(2 * time.Second)
	left := persondetect.Box{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.95}
	h.analyze(cam.ID, left, right)
	ev := h.eventList()[0]
	if len(ev.Zones) != 1 || ev.Zones[0] != "Left" || ev.MaxPersons != 1 {
		t.Fatalf("event = %+v", ev)
	}
	detail, _ := h.svc.Event(context.Background(), ev.ID)
	if len(detail.Frames) != 1 || len(detail.Frames[0].Boxes) != 1 || detail.Frames[0].Boxes[0] != left {
		t.Fatalf("stored frames keep only the boxes that count (S2 Task 5 decision): %+v", detail.Frames)
	}
}

func TestServiceNewestFrameWinsAndCamerasTakeTurns(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("a"), svcCamera("b"))
	ctx := context.Background()
	a1, a2, b1 := persondetecttest.JPEG(64, 36), persondetecttest.JPEG(80, 45), persondetecttest.JPEG(96, 54)
	h.frames.Set("a", a1)
	h.svc.FetchOnceForTest(ctx, "a")
	h.frames.Set("a", a2)
	h.svc.FetchOnceForTest(ctx, "a")
	h.frames.Set("b", b1)
	h.svc.FetchOnceForTest(ctx, "b")
	processed := 0
	for h.svc.ProcessOneForTest(ctx) {
		processed++
	}
	calls := h.factory.Script.Calls()
	if processed != 2 || len(calls) != 2 || !bytes.Equal(calls[0].JPEG, a2) || !bytes.Equal(calls[1].JPEG, b1) || calls[0].MinScore != 0.5 {
		t.Fatalf("processed %d, detector calls %d (newest a first, then b)", processed, len(calls))
	}
}

func TestServiceWorkerRestartSkipsAndFailurePausesDetection(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.frames.Set("front-door", persondetecttest.JPEG(64, 36))

	h.factory.Script.Push(persondetecttest.Result{Err: persondetect.ErrWorkerRestarting})
	h.svc.FetchOnceForTest(ctx, "front-door")
	if !h.svc.ProcessOneForTest(ctx) {
		t.Fatal("no frame queued")
	}
	if st := h.svc.Status(); st.Detector.State != persondetect.DetectorReady || st.Cameras[0].State == persondetect.CameraError || len(h.eventList()) != 0 {
		t.Fatalf("restarting worker = %+v", st)
	}

	h.factory.Script.Push(persondetecttest.Result{Err: fmt.Errorf("%w: worker crashed 5 times", persondetect.ErrDetectorFailed)})
	h.svc.FetchOnceForTest(ctx, "front-door")
	h.svc.ProcessOneForTest(ctx)
	if d := h.svc.Status().Detector; d.State != persondetect.DetectorError || !strings.Contains(d.Error, "crashed 5 times") {
		t.Fatalf("failed detector = %+v", d)
	}
	persondetecttest.WaitFor(t, 5*time.Second, "failed detector closed", func() bool { return h.factory.Created()[0].Closed() })
	before := h.frames.Calls("front-door")
	if delay := h.svc.FetchOnceForTest(ctx, "front-door"); delay != 2*time.Second || h.frames.Calls("front-door") != before {
		t.Fatal("frames were requested while detection was paused")
	}
	if err := h.svc.Retry(); err != nil {
		t.Fatalf("Retry: %v", err)
	}
	persondetecttest.WaitDetector(t, h.svc, persondetect.DetectorReady)
	if h.factory.Calls() != 2 {
		t.Fatalf("setups = %d", h.factory.Calls())
	}

	h.frames.Set("front-door", []byte("not a jpeg"))
	h.svc.FetchOnceForTest(ctx, "front-door")
	h.svc.ProcessOneForTest(ctx)
	if cam := h.svc.Status().Cameras[0]; cam.State != persondetect.CameraError || cam.LastError == "" {
		t.Fatalf("camera after a bad frame = %+v", cam)
	}
}

func TestServiceSweepEndsEventsWithoutFrames(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	h.svc.SweepForTest(svcStart.Add(29 * time.Second))
	if h.eventList()[0].EndedAt != nil {
		t.Fatal("swept after 29 s")
	}
	h.svc.SweepForTest(svcStart.Add(30 * time.Second))
	if ev := h.eventList()[0]; ev.EndedAt == nil || !ev.EndedAt.Equal(svcStart) {
		t.Fatalf("ended_at = %v", ev.EndedAt)
	}
}

func TestServiceShutdownClosesOpenEventsAtTheLastHit(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	h.clock.Advance(3 * time.Second)
	h.analyze("front-door", svcPerson(0.9))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	reader := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger()})
	t.Cleanup(func() { _ = reader.Shutdown(context.Background()) })
	reader.Configure(persondetect.Settings{DataDir: h.dataDir})
	if !reader.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("the reader did not open the database")
	}
	page, err := reader.Events(context.Background(), persondetect.EventQuery{})
	if err != nil || len(page.Events) != 1 || page.Events[0].EndedAt == nil || !page.Events[0].EndedAt.Equal(svcStart.Add(3*time.Second)) {
		t.Fatalf("events after shutdown = %+v, %v", page.Events, err)
	}
	if reader.Status().Enabled {
		t.Fatal("a disabled reader reports enabled")
	}
}

func TestServiceRemovingACameraClosesItsEvent(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("a"), svcCamera("b"))
	h.analyze("a", svcPerson(0.9))
	h.svc.Configure(h.settings(svcCamera("b")))
	if !h.svc.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("background work did not finish")
	}
	if ev := h.eventList()[0]; ev.EndedAt == nil || !ev.EndedAt.Equal(svcStart) {
		t.Fatalf("event of the removed camera = %+v", ev)
	}
	if st := h.svc.Status(); len(st.Cameras) != 1 || st.Cameras[0].ID != "b" {
		t.Fatalf("cameras = %+v", st.Cameras)
	}
}

func TestServiceDiskFullWarningTriggersRetention(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.svc.NoteWriteErrorForTest(&os.PathError{Op: "write", Path: "f.jpg", Err: syscall.ENOSPC})
	if st := h.svc.Status(); st.Storage.Warning != persondetect.StorageFullWarning {
		t.Fatalf("storage = %+v", st.Storage)
	}
	if !h.svc.RetentionRequestedForTest() {
		t.Fatal("a full disk must request an immediate retention run")
	}
	h.analyze("front-door", svcPerson(0.9))
	if st := h.svc.Status(); st.Storage.Warning != "" || st.Storage.Bytes <= 0 {
		t.Fatalf("a successful write must clear the warning: %+v", st.Storage)
	}
}

func TestServiceRetentionRunUsesTheSettings(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.analyze("front-door", svcPerson(0.9))
	h.clock.Advance(31 * time.Second)
	h.analyze("front-door")
	if h.eventList()[0].EndedAt == nil {
		t.Fatal("event did not end")
	}
	h.clock.Advance(15 * 24 * time.Hour)
	res, err := h.svc.RunRetentionForTest(context.Background())
	if err != nil || res.DeletedEvents != 1 || len(h.eventList()) != 0 || h.svc.Status().Storage.Bytes != 0 {
		t.Fatalf("retention = %+v, %v, events %d, bytes %d", res, err, len(h.eventList()), h.svc.Status().Storage.Bytes)
	}
}

func TestServiceRealLoopsCreateAndCloseEvents(t *testing.T) {
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	factory.Script.SetBoxes(svcPerson(0.9))
	frames := persondetecttest.NewFrames()
	frames.Set("front-door", persondetecttest.JPEG(64, 36))
	dir := t.TempDir()
	svc := persondetect.NewService(persondetect.Deps{Logger: svcDiscardLogger(), Frames: frames, NewDetector: factory.New})
	persondetecttest.StopOnCleanup(t, svc)
	cam := svcCamera("front-door")
	cam.Interval = 20 * time.Millisecond
	svc.Configure(persondetect.Settings{Enabled: true, DataDir: dir, Cameras: []persondetect.Camera{cam}, Location: time.UTC})
	persondetecttest.WaitFor(t, 10*time.Second, "an event from the background loops", func() bool {
		page, err := svc.Events(context.Background(), persondetect.EventQuery{})
		return err == nil && len(page.Events) == 1
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	reader := persondetect.NewManualServiceForTest(persondetect.Deps{Logger: svcDiscardLogger()})
	t.Cleanup(func() { _ = reader.Shutdown(context.Background()) })
	reader.Configure(persondetect.Settings{DataDir: dir})
	if !reader.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("the reader did not open the database")
	}
	page, err := reader.Events(context.Background(), persondetect.EventQuery{})
	if err != nil || len(page.Events) != 1 || page.Events[0].EndedAt == nil || page.Events[0].FrameCount < 1 {
		t.Fatalf("events after shutdown = %+v, %v", page.Events, err)
	}
}

// An event ends after max(30 s, 2 × interval + 5 s) without a hit, and a settings
// change of the interval applies to the open event.
func TestServiceEndAfterFollowsTheCameraInterval(t *testing.T) {
	h := newSvcHarness(t, false)
	cam := svcCamera("front-door")
	cam.Interval = 15 * time.Second
	h.start(cam)
	h.analyze(cam.ID, svcPerson(0.9))
	h.svc.SweepForTest(svcStart.Add(34 * time.Second))
	if h.eventList()[0].EndedAt != nil {
		t.Fatal("a camera at 15 s ended its event 34 s after the last hit, want 2 × 15 s + 5 s")
	}
	cam.Interval = 2 * time.Second
	h.svc.Configure(h.settings(cam))
	if !h.svc.WaitBackgroundForTest(5 * time.Second) {
		t.Fatal("background work did not finish")
	}
	h.svc.SweepForTest(svcStart.Add(34 * time.Second))
	if ev := h.eventList()[0]; ev.EndedAt == nil || !ev.EndedAt.Equal(svcStart) {
		t.Fatalf("after the interval change to 2 s: ended_at = %v, want the last hit", ev.EndedAt)
	}
}

// A hit captured before the end deadline but still waiting for analysis must not
// see its event ended by a sweep first, which would split the event.
func TestServiceSweepSkipsACameraWithAWaitingFrame(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	ctx := context.Background()
	h.analyze("front-door", svcPerson(0.9))
	h.clock.Advance(29 * time.Second)
	h.factory.Script.SetBoxes(svcPerson(0.7))
	h.frames.Set("front-door", persondetecttest.JPEG(640, 360))
	h.svc.FetchOnceForTest(ctx, "front-door")
	h.svc.SweepForTest(svcStart.Add(31 * time.Second))
	if h.eventList()[0].EndedAt != nil {
		t.Fatal("the sweep ended an event while a hit of its camera waited for analysis")
	}
	if !h.svc.ProcessOneForTest(ctx) {
		t.Fatal("no frame queued")
	}
	h.waitStorage()
	h.svc.SweepForTest(svcStart.Add(58 * time.Second))
	if h.eventList()[0].EndedAt != nil {
		t.Fatal("ended 29 s after the waiting hit")
	}
	h.svc.SweepForTest(svcStart.Add(59 * time.Second))
	if ev := h.eventList()[0]; ev.EndedAt == nil || !ev.EndedAt.Equal(svcStart.Add(29*time.Second)) || len(h.eventList()) != 1 {
		t.Fatalf("event = %+v, want one event ending at the waiting hit", ev)
	}
}

// When the event row cannot be written, the tracker forgets the event, so the
// next hit starts one instead of extending an event that does not exist.
func TestServiceRetriesAnEventStartThatCouldNotBeRecorded(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	if err := h.svc.SetStoreReadOnlyForTest(true); err != nil {
		t.Fatal(err)
	}
	h.analyze("front-door", svcPerson(0.9))
	if err := h.svc.SetStoreReadOnlyForTest(false); err != nil {
		t.Fatal(err)
	}
	if n := len(h.eventList()); n != 0 {
		t.Fatalf("events while the database refused writes = %d", n)
	}
	h.clock.Advance(time.Second)
	h.analyze("front-door", svcPerson(0.9))
	events := h.eventList()
	if len(events) != 1 || !events[0].StartedAt.Equal(svcStart.Add(time.Second)) || events[0].FrameCount != 1 {
		t.Fatalf("events = %+v, want a new event at the next hit", events)
	}
}

// sqliteFullError returns a real SQLITE_FULL error ("database or disk is full").
func sqliteFullError(t *testing.T) error {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "full.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{`CREATE TABLE t(b BLOB)`, `PRAGMA max_page_count = 2`} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	_, err = db.Exec(`INSERT INTO t VALUES (zeroblob(100000))`)
	if err == nil || !strings.Contains(err.Error(), "full") {
		t.Fatalf("insert beyond max_page_count = %v, want SQLITE_FULL", err)
	}
	return err
}

// SQLite reports a full disk as SQLITE_FULL, not as an OS error; it counts as a
// full disk all the same.
func TestServiceSQLiteFullCountsAsStorageFull(t *testing.T) {
	h := newSvcHarness(t, false)
	h.start(svcCamera("front-door"))
	h.svc.NoteWriteErrorForTest(fmt.Errorf("persondetect: add frame: %w", sqliteFullError(t)))
	if st := h.svc.Status(); st.Storage.Warning != persondetect.StorageFullWarning {
		t.Fatalf("storage after SQLITE_FULL = %+v", st.Storage)
	}
	if !h.svc.RetentionRequestedForTest() {
		t.Fatal("SQLITE_FULL must request an immediate retention run")
	}
}

// The cooldown of an action follows only its own sent alerts.
func TestServiceCooldownIsPerAction(t *testing.T) {
	h := newSvcHarness(t, true)
	cam := svcCamera("front-door")
	cam.Actions.Agent = true
	h.start(cam)
	h.svc.SetArmModeForTest(persondetect.ArmOn)
	failed := persondetect.AlertResult{Status: persondetect.AlertFailed, Detail: "502"}
	sent := persondetect.AlertResult{Status: persondetect.AlertSent}
	cooldown := persondetect.AlertResult{Status: persondetect.AlertSkipped, Detail: "cooldown"}
	event := func(at time.Duration) {
		t.Helper()
		h.clock.Set(svcStart.Add(at))
		h.analyze(cam.ID, svcPerson(0.9))
		h.clock.Advance(31 * time.Second)
		h.analyze(cam.ID)
	}
	h.alerts.SetResult(persondetect.ActionMessage, failed)
	h.alerts.SetResult(persondetect.ActionAgent, failed)
	event(0) // message and agent fail, the desktop toast is sent
	h.alerts.SetResult(persondetect.ActionMessage, sent)
	event(41 * time.Second) // the message is sent, the agent fails again; the desktop cools down
	event(82 * time.Second) // the agent failed at 41 s: it fires; message and desktop cool down
	fires := h.alerts.Events()
	want := []persondetect.FireSet{
		{Message: true, Agent: true, Desktop: true, Mission: true},
		{Message: true, Agent: true, Mission: true},
		{Agent: true, Mission: true},
	}
	if len(fires) != len(want) {
		t.Fatalf("dispatched %d events", len(fires))
	}
	for i := range want {
		if fires[i].Fire != want[i] {
			t.Fatalf("event %d fired %+v, want %+v", i+1, fires[i].Fire, want[i])
		}
	}
	events := h.eventList() // newest first
	if events[1].Alerts[persondetect.ActionDesktop] != cooldown || events[1].Alerts[persondetect.ActionMessage] != sent {
		t.Fatalf("event 2 alerts = %+v", events[1].Alerts)
	}
	if events[0].Alerts[persondetect.ActionMessage] != cooldown || events[0].Alerts[persondetect.ActionAgent] != failed {
		t.Fatalf("event 3 alerts = %+v", events[0].Alerts)
	}
}

type slowDetector struct {
	persondetect.Detector
	delay time.Duration
}

func (d slowDetector) Detect(ctx context.Context, data []byte, minScore float64) (persondetect.DetectResult, error) {
	time.Sleep(d.delay)
	return d.Detector.Detect(ctx, data, minScore)
}

// When detection is slower than frames arrive, the slots never empty; the sweep
// must still get its turn.
func TestServiceSweepRunsWhileDetectionIsSaturated(t *testing.T) {
	factory := persondetecttest.NewFactory(persondetecttest.CPUInfo())
	newSlow := func(ctx context.Context, opts persondetect.DetectorOptions) (persondetect.Detector, error) {
		det, err := factory.New(ctx, opts)
		if err != nil {
			return nil, err
		}
		return slowDetector{Detector: det, delay: 10 * time.Millisecond}, nil
	}
	frames := persondetecttest.NewFrames()
	dir := t.TempDir()
	svc := persondetect.NewService(persondetect.Deps{Logger: svcDiscardLogger(), Frames: frames, NewDetector: newSlow})
	svc.SetSweepIntervalForTest(20 * time.Millisecond)
	persondetecttest.StopOnCleanup(t, svc)
	settings := persondetect.Settings{Enabled: true, DataDir: dir, Location: time.UTC}
	for _, id := range []string{"a", "b"} {
		frames.Set(id, persondetecttest.JPEG(32, 18))
		cam := svcCamera(id)
		cam.Interval = 2 * time.Millisecond
		settings.Cameras = append(settings.Cameras, cam)
	}
	svc.Configure(settings)
	persondetecttest.WaitFor(t, 5*time.Second, "saturated detection", func() bool { return len(factory.Script.Calls()) >= 5 })
	start := svc.SweepsForTest()
	persondetecttest.WaitFor(t, 5*time.Second, "sweeps while detection is saturated", func() bool { return svc.SweepsForTest() >= start+5 })
}

// Thumbnails are made off the detection loop: a slow thumbnail of one camera does
// not hold up the analysis of another. Frames at most ThumbnailWidth wide are
// their own thumbnail.
func TestServiceSlowThumbnailDoesNotDelayOtherCameras(t *testing.T) {
	h := newSvcHarness(t, false)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	h.svc.SetThumbnailerForTest(func(data []byte) ([]byte, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		return persondetecttest.JPEG(320, 180), nil
	})
	t.Cleanup(unblock)
	h.start(svcCamera("a"), svcCamera("b"))
	ctx := context.Background()
	h.factory.Script.SetBoxes(svcPerson(0.9))
	small := persondetecttest.JPEG(320, 180)
	h.frames.Set("a", persondetecttest.JPEG(640, 360))
	h.frames.Set("b", small)
	go func() {
		h.svc.FetchOnceForTest(ctx, "a")
		h.svc.ProcessOneForTest(ctx)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the thumbnail of camera a was never made")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.svc.FetchOnceForTest(ctx, "b")
		h.svc.ProcessOneForTest(ctx)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the analysis of camera b waited for the thumbnail of camera a")
	}
	var bEvent persondetect.EventSummary
	persondetecttest.WaitFor(t, 5*time.Second, "camera b's frame stored", func() bool {
		page, err := h.svc.Events(ctx, persondetect.EventQuery{CameraID: "b"})
		if err != nil || len(page.Events) != 1 || page.Events[0].FrameCount != 1 {
			return false
		}
		bEvent = page.Events[0]
		return true
	})
	unblock()
	h.waitStorage()
	if page, err := h.svc.Events(ctx, persondetect.EventQuery{CameraID: "a"}); err != nil || len(page.Events) != 1 || page.Events[0].FrameCount != 1 {
		t.Fatalf("camera a's events = %+v, %v", page.Events, err)
	}
	thumbPath, err := h.svc.FrameFile(ctx, *bEvent.CoverFrameID, true)
	if err != nil {
		t.Fatal(err)
	}
	if thumb, err := os.ReadFile(thumbPath); err != nil || !bytes.Equal(thumb, small) {
		t.Fatalf("thumbnail of a 320 px frame = %d bytes, %v; want the frame itself", len(thumb), err)
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A retention pass cut short by the shutdown is not a failure worth a warning.
func TestServiceRetentionCancelledAtShutdownIsNotAWarning(t *testing.T) {
	var logs lockedBuffer
	dir := t.TempDir()
	svc := persondetect.NewManualServiceForTest(persondetect.Deps{
		Logger:      slog.New(slog.NewTextHandler(&logs, nil)),
		NewDetector: persondetecttest.NewFactory(persondetecttest.CPUInfo()).New,
	})
	persondetecttest.StopOnCleanup(t, svc)
	svc.Configure(persondetect.Settings{Enabled: true, DataDir: dir, Location: time.UTC})
	persondetecttest.WaitDetector(t, svc, persondetect.DetectorReady)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.RunRetentionForTest(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("retention with a cancelled context = %v", err)
	}
	if strings.Contains(logs.String(), "Retention run failed") {
		t.Fatalf("a cancelled retention pass logged a failure:\n%s", logs.String())
	}
}
