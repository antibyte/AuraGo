package persondetect

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

var storeT0 = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func pdTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestEventStore(t *testing.T) *EventStore {
	t.Helper()
	st, err := OpenEventStore(t.TempDir(), pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestOpenEventStoreLayoutAndPermissions(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore: %v", err)
	}
	defer st.Close()
	dbPath, root := DetectionsPaths(dir)
	if dbPath != filepath.Join(dir, "go2rtc", "detections.db") || root != filepath.Join(dir, "go2rtc", "detections") || st.Root() != root {
		t.Fatalf("paths = %s, %s, root %s", dbPath, root, st.Root())
	}
	if st.Cursor() != 0 {
		t.Fatalf("fresh cursor = %d", st.Cursor())
	}
	// A write makes SQLite create its WAL and SHM files, which take the
	// database file's mode.
	if _, err := st.CreateEvent(context.Background(), NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
				t.Fatalf("%s mode = %v, %v; want 0640", filepath.Base(path), modeOf(info), err)
			}
		}
		if info, err := os.Stat(root); err != nil || info.Mode().Perm() != 0o750 {
			t.Fatalf("image root mode = %v, %v; want 0750", modeOf(info), err)
		}
	}
	if _, err := OpenEventStore("  ", pdTestLogger()); err == nil {
		t.Fatal("empty data directory accepted")
	}
}

func modeOf(info os.FileInfo) os.FileMode {
	if info == nil {
		return 0
	}
	return info.Mode().Perm()
}

func TestOpenEventStoreRecoversOnlyCorruptDatabases(t *testing.T) {
	dir := t.TempDir()
	dbPath, _ := DetectionsPaths(dir)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, bytes.Repeat([]byte("not a database "), 200), 0o640); err != nil {
		t.Fatal(err)
	}
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore on a corrupt file: %v", err)
	}
	moved, _ := filepath.Glob(dbPath + ".corrupt-*")
	if len(moved) != 1 {
		t.Fatalf("corrupt database moved aside as %v, want one .corrupt-<time> file", moved)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(dbPath); err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("recreated database mode = %v, %v; want 0640", modeOf(info), err)
		}
	}
	if _, err := st.CreateEvent(context.Background(), NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
		t.Fatalf("CreateEvent on the fresh database: %v", err)
	}
	// A database written by a newer schema is refused and left where it is.
	if _, err := st.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenEventStore(dir, pdTestLogger()); err == nil {
		t.Fatal("a database from a newer schema was opened")
	}
	if moved, _ := filepath.Glob(dbPath + ".corrupt-*"); len(moved) != 1 {
		t.Fatalf("a newer database must not be moved aside: %v", moved)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("the newer database must stay: %v", err)
	}
}

func TestEventStoreEventLifecycleAndCover(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	id, err := st.CreateEvent(ctx, NewEvent{
		CameraID: "front-door", CameraName: "Front door", StartedAt: storeT0, Armed: true,
		Stats:  EventStats{MaxScore: 0.6, MaxPersons: 1, Zones: []string{"Driveway"}},
		Alerts: map[string]AlertResult{ActionAgent: {Status: AlertSkipped, Detail: "disarmed"}},
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	f1, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_thumb.jpg", Size: 100, CapturedAt: storeT0, Width: 640, Height: 360, Boxes: []Box{{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.6}}, Score: 0.6})
	if err != nil {
		t.Fatalf("AddFrame 1: %v", err)
	}
	f2, err := st.AddFrame(ctx, id, StoredFrame{Seq: 2, Path: "b.jpg", ThumbPath: "b_thumb.jpg", Size: 50, CapturedAt: storeT0.Add(5 * time.Second), Width: 640, Height: 360, Score: 0.9})
	if err != nil {
		t.Fatalf("AddFrame 2: %v", err)
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 3, Path: "c.jpg", ThumbPath: "c_thumb.jpg", Size: 100, CapturedAt: storeT0.Add(10 * time.Second), Width: 640, Height: 360, Score: 0.7}); err != nil {
		t.Fatalf("AddFrame 3: %v", err)
	}
	if err := st.UpdateEventStats(ctx, id, EventStats{MaxScore: 0.9, MaxPersons: 2, Zones: []string{"Driveway", "Porch"}}); err != nil {
		t.Fatalf("UpdateEventStats: %v", err)
	}
	if err := st.SetAlertResult(ctx, id, ActionMission, AlertResult{Status: AlertSent}); err != nil {
		t.Fatalf("SetAlertResult: %v", err)
	}
	if err := st.EndEvent(ctx, id, storeT0.Add(12*time.Second)); err != nil {
		t.Fatalf("EndEvent: %v", err)
	}
	d, err := st.Event(ctx, id)
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if d.CoverFrameID == nil || *d.CoverFrameID != f2 || d.FrameCount != 3 || d.MaxScore != 0.9 || d.MaxPersons != 2 || !d.Armed {
		t.Fatalf("detail = %+v, want cover %d", d.EventSummary, f2)
	}
	if !reflect.DeepEqual(d.Zones, []string{"Driveway", "Porch"}) || d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(12*time.Second)) {
		t.Fatalf("zones %v ended %v", d.Zones, d.EndedAt)
	}
	if d.Alerts[ActionAgent] != (AlertResult{Status: AlertSkipped, Detail: "disarmed"}) || d.Alerts[ActionMission] != (AlertResult{Status: AlertSent}) {
		t.Fatalf("alerts = %+v", d.Alerts)
	}
	if len(d.Frames) != 3 || d.Frames[0].ID != f1 || d.Frames[0].Boxes[0].Score != 0.6 || d.Frames[0].Width != 640 || d.Frames[1].Boxes == nil {
		t.Fatalf("frames = %+v", d.Frames)
	}
	if total, err := st.TotalBytes(ctx); err != nil || total != 250 {
		t.Fatalf("TotalBytes = %d, %v", total, err)
	}
	if path, thumb, err := st.Frame(ctx, f1); err != nil || path != "a.jpg" || thumb != "a_thumb.jpg" {
		t.Fatalf("Frame = %q %q %v", path, thumb, err)
	}
	if _, _, err := st.Frame(ctx, 999); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown frame err = %v", err)
	}
	if _, err := st.Event(ctx, 999); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown event err = %v", err)
	}
	if err := st.UpdateEventStats(ctx, 999, EventStats{}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("stats of unknown event err = %v", err)
	}
	if err := st.SetAlertResult(ctx, 999, ActionAgent, AlertResult{Status: AlertSent}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("alert of unknown event err = %v", err)
	}
	if _, err := st.AddFrame(ctx, 999, StoredFrame{Seq: 1, Path: "z.jpg", ThumbPath: "z_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("frame of unknown event err = %v", err)
	}
}

func TestEventStoreRevisionsDriveChanges(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	c0 := st.Cursor()
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	c1 := st.Cursor()
	events, last, full, err := st.EventsBetween(ctx, c0, c1, 10)
	if err != nil || len(events) != 1 || events[0].ID != id || last != c1 || full || c1 <= c0 {
		t.Fatalf("EventsBetween(c0,c1) = %d events, last %d, full %v, err %v", len(events), last, full, err)
	}
	if events, _, _, _ := st.EventsBetween(ctx, c1, c1, 10); len(events) != 0 {
		t.Fatal("no change after the cursor expected")
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 10, CapturedAt: storeT0, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	c2 := st.Cursor()
	if events, _, _, _ := st.EventsBetween(ctx, c1, c2, 10); len(events) != 1 || events[0].FrameCount != 1 {
		t.Fatalf("a frame insert must bump its event: %+v", events)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: storeT0, ChangedBy: ActorUser}); err != nil {
		t.Fatal(err)
	}
	c3 := st.Cursor()
	entries, armLast, _, err := st.ArmLogBetween(ctx, c2, c3, 10)
	if err != nil || len(entries) != 1 || armLast != c3 || entries[0].Mode != ArmOn || !entries[0].Armed || entries[0].Actor != ActorUser || !entries[0].At.Equal(storeT0) {
		t.Fatalf("arm log = %+v, %v", entries, err)
	}
	for i := 0; i < 3; i++ {
		if _, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
			t.Fatal(err)
		}
	}
	events, last, full, err = st.EventsBetween(ctx, c3, st.Cursor(), 2)
	if err != nil || len(events) != 2 || !full || last != c3+2 {
		t.Fatalf("capped = %d events, last %d (want %d), full %v", len(events), last, c3+2, full)
	}
	if _, _, _, err := st.EventsBetween(ctx, c0, st.Cursor(), 0); err == nil {
		t.Fatal("EventsBetween accepted a row limit of 0")
	}
	if _, _, _, err := st.ArmLogBetween(ctx, c0, st.Cursor(), 0); err == nil {
		t.Fatal("ArmLogBetween accepted a row limit of 0")
	}
}

// One write can stamp several rows (CloseOpenEvents). A capped page must not
// split that revision: the next call continues after the page's last revision,
// so rows of that revision left out of the page would never be delivered.
func TestEventStoreBetweenKeepsRevisionsWhole(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	var ids []int64
	for i := 0; i < 3; i++ {
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 3 {
		t.Fatalf("CloseOpenEvents = %d, %v", n, err)
	}
	closedRev := st.Cursor()
	newest, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	upTo := st.Cursor()

	events, last, full, err := st.EventsBetween(ctx, 0, upTo, 2)
	if err != nil || len(events) != 3 || last != closedRev || !full {
		t.Fatalf("first page = %d events, last %d (want %d), full %v, err %v", len(events), last, closedRev, full, err)
	}
	for i, ev := range events {
		if ev.ID != ids[i] || ev.EndedAt == nil {
			t.Fatalf("first page row %d = %+v, want closed event %d", i, ev, ids[i])
		}
	}
	events, last, full, err = st.EventsBetween(ctx, last, upTo, 2)
	if err != nil || len(events) != 1 || events[0].ID != newest || last != upTo || full {
		t.Fatalf("second page = %+v, last %d, full %v, err %v", events, last, full, err)
	}
}

// EventsLatest and ArmLogLatest back the Changes(0) snapshot: the newest rows
// by revision up to a cursor, returned oldest revision first.
func TestEventStoreLatestRows(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	if events, err := st.EventsLatest(ctx, st.Cursor(), 10); err != nil || events == nil || len(events) != 0 {
		t.Fatalf("EventsLatest on an empty store = %#v, %v", events, err)
	}
	if entries, err := st.ArmLogLatest(ctx, st.Cursor(), 10); err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("ArmLogLatest on an empty store = %#v, %v", entries, err)
	}

	var ids, eventRevs []int64
	for i := 0; i < 5; i++ {
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		ids, eventRevs = append(ids, id), append(eventRevs, st.Cursor())
	}
	// Changing the oldest event gives it the newest revision.
	if err := st.SetAlertResult(ctx, ids[0], ActionAgent, AlertResult{Status: AlertSent}); err != nil {
		t.Fatal(err)
	}
	snapshot := st.Cursor()
	// A row written after the snapshot cursor stays out.
	if _, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	eventIDs := func(events []EventSummary) []int64 {
		out := []int64{}
		for _, ev := range events {
			out = append(out, ev.ID)
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		upTo  int64
		limit int
		want  []int64
	}{
		{"more rows than the limit", snapshot, 3, []int64{ids[3], ids[4], ids[0]}},
		{"fewer rows than the limit", snapshot, 100, []int64{ids[1], ids[2], ids[3], ids[4], ids[0]}},
		{"cursor below the newest revision", eventRevs[2], 2, []int64{ids[1], ids[2]}},
		{"cursor before every row", 0, 10, []int64{}},
	} {
		events, err := st.EventsLatest(ctx, tc.upTo, tc.limit)
		if err != nil || !reflect.DeepEqual(eventIDs(events), tc.want) {
			t.Fatalf("EventsLatest %s = %v, %v; want %v", tc.name, eventIDs(events), err, tc.want)
		}
		if events == nil {
			t.Fatalf("EventsLatest %s returned a nil list", tc.name)
		}
	}
	if events, err := st.EventsLatest(ctx, snapshot, 1); err != nil || len(events) != 1 || events[0].Alerts[ActionAgent].Status != AlertSent {
		t.Fatalf("EventsLatest must return the changed row in full: %+v, %v", events, err)
	}

	var armRevs []int64
	modes := []ArmMode{ArmOn, ArmOff, ArmSchedule, ArmOn, ArmOff}
	for i, mode := range modes {
		if err := st.RecordArm(ctx, armRecord{Mode: mode, Armed: mode == ArmOn, ChangedAt: storeT0.Add(time.Duration(i) * time.Minute), ChangedBy: ActorUser}); err != nil {
			t.Fatal(err)
		}
		armRevs = append(armRevs, st.Cursor())
	}
	armModes := func(entries []ArmLogEntry) []ArmMode {
		out := []ArmMode{}
		for _, e := range entries {
			out = append(out, e.Mode)
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		upTo  int64
		limit int
		want  []ArmMode
	}{
		{"more rows than the limit", armRevs[4], 2, []ArmMode{ArmOn, ArmOff}},
		{"fewer rows than the limit", armRevs[4], 50, modes},
		{"cursor below the newest revision", armRevs[2], 2, []ArmMode{ArmOff, ArmSchedule}},
		{"cursor before every row", snapshot, 10, []ArmMode{}},
	} {
		entries, err := st.ArmLogLatest(ctx, tc.upTo, tc.limit)
		if err != nil || !reflect.DeepEqual(armModes(entries), tc.want) {
			t.Fatalf("ArmLogLatest %s = %v, %v; want %v", tc.name, armModes(entries), err, tc.want)
		}
		if entries == nil {
			t.Fatalf("ArmLogLatest %s returned a nil list", tc.name)
		}
	}
	if entries, _ := st.ArmLogLatest(ctx, armRevs[4], 2); !entries[0].At.Equal(storeT0.Add(3*time.Minute)) || entries[1].ID <= entries[0].ID {
		t.Fatalf("ArmLogLatest rows = %+v", entries)
	}
	if _, err := st.EventsLatest(ctx, snapshot, 0); err == nil {
		t.Fatal("EventsLatest accepted a row limit of 0")
	}
	if _, err := st.ArmLogLatest(ctx, snapshot, -1); err == nil {
		t.Fatal("ArmLogLatest accepted a negative row limit")
	}
}

func TestEventStorePagingAndFilters(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		camera := "a"
		if i%2 == 1 {
			camera = "b"
		}
		if _, err := st.CreateEvent(ctx, NewEvent{CameraID: camera, CameraName: camera, StartedAt: storeT0.Add(time.Duration(i) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(page EventPage) []int64 {
		out := []int64{}
		for _, e := range page.Events {
			out = append(out, e.ID)
		}
		return out
	}
	page, err := st.Events(ctx, EventQuery{Limit: 2})
	if err != nil || !reflect.DeepEqual(ids(page), []int64{5, 4}) || page.NextBefore == nil || *page.NextBefore != 4 {
		t.Fatalf("page 1 = %v next %v err %v", ids(page), page.NextBefore, err)
	}
	page, _ = st.Events(ctx, EventQuery{Limit: 2, Before: 4})
	if !reflect.DeepEqual(ids(page), []int64{3, 2}) || page.NextBefore == nil || *page.NextBefore != 2 {
		t.Fatalf("page 2 = %v next %v", ids(page), page.NextBefore)
	}
	page, _ = st.Events(ctx, EventQuery{Limit: 2, Before: 2})
	if !reflect.DeepEqual(ids(page), []int64{1}) || page.NextBefore != nil {
		t.Fatalf("page 3 = %v next %v", ids(page), page.NextBefore)
	}
	if page, _ := st.Events(ctx, EventQuery{CameraID: "a"}); !reflect.DeepEqual(ids(page), []int64{5, 3, 1}) {
		t.Fatalf("camera filter = %v", ids(page))
	}
	if page, _ := st.Events(ctx, EventQuery{From: storeT0.Add(time.Minute), To: storeT0.Add(3 * time.Minute)}); !reflect.DeepEqual(ids(page), []int64{3, 2}) {
		t.Fatalf("time filter = %v", ids(page))
	}
	if page, _ := st.Events(ctx, EventQuery{Limit: 500}); len(page.Events) != 5 {
		t.Fatalf("large limit = %d events", len(page.Events))
	}
	if page, _ := st.Events(ctx, EventQuery{CameraID: "none"}); page.Events == nil || len(page.Events) != 0 {
		t.Fatalf("empty page must be an empty list: %#v", page.Events)
	}
}

// Every list the store returns is a JSON array (and the alerts map an object),
// never null: the API hands these values to the browser as they are.
func TestEventStoreListsAreNeverNil(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	requireArray := func(name string, obj map[string]interface{}, key string) {
		t.Helper()
		if list, ok := obj[key].([]interface{}); !ok || len(list) != 0 {
			t.Fatalf("%s: %q = %#v, want []", name, key, obj[key])
		}
	}
	requireObject := func(name string, obj map[string]interface{}, key string) {
		t.Helper()
		if m, ok := obj[key].(map[string]interface{}); !ok || len(m) != 0 {
			t.Fatalf("%s: %q = %#v, want {}", name, key, obj[key])
		}
	}

	// Nothing stored yet.
	page, err := st.Events(ctx, EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	requireArray("empty page", jsonObjectOf(t, page), "events")
	if events, _, _, err := st.EventsBetween(ctx, 0, st.Cursor()+1, 10); err != nil || events == nil || string(mustMarshal(t, events)) != "[]" {
		t.Fatalf("EventsBetween on an empty store = %#v, %v", events, err)
	}
	if entries, _, _, err := st.ArmLogBetween(ctx, 0, st.Cursor()+1, 10); err != nil || entries == nil || string(mustMarshal(t, entries)) != "[]" {
		t.Fatalf("ArmLogBetween on an empty store = %#v, %v", entries, err)
	}
	if last, err := st.LastEvents(ctx); err != nil || last == nil {
		t.Fatalf("LastEvents on an empty store = %#v, %v", last, err)
	}

	// An event created without zones or alerts, before any frame.
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.Event(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	obj := jsonObjectOf(t, d)
	requireArray("event without frames", obj, "zones")
	requireArray("event without frames", obj, "frames")
	requireObject("event without frames", obj, "alerts")
	events, _, _, err := st.EventsBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("EventsBetween = %+v, %v", events, err)
	}
	requireArray("changed event", jsonObjectOf(t, events[0]), "zones")
	requireObject("changed event", jsonObjectOf(t, events[0]), "alerts")
	latest, err := st.EventsLatest(ctx, st.Cursor(), 10)
	if err != nil || len(latest) != 1 {
		t.Fatalf("EventsLatest = %+v, %v", latest, err)
	}
	requireArray("latest event", jsonObjectOf(t, latest[0]), "zones")
	requireObject("latest event", jsonObjectOf(t, latest[0]), "alerts")

	// A frame stored without boxes, and stats without zones.
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEventStats(ctx, id, EventStats{MaxScore: 0.9, MaxPersons: 1}); err != nil {
		t.Fatal(err)
	}
	d, err = st.Event(ctx, id)
	if err != nil || len(d.Frames) != 1 {
		t.Fatalf("Event = %+v, %v", d, err)
	}
	requireArray("event with stats without zones", jsonObjectOf(t, d), "zones")
	requireArray("frame without boxes", jsonObjectOf(t, d.Frames[0]), "boxes")

	// Rows whose JSON columns hold null (or damage) still decode to empty lists.
	if _, err := st.db.Exec(`UPDATE events SET zones = 'null', alerts = 'null' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE frames SET boxes = 'not json' WHERE event_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	d, err = st.Event(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	obj = jsonObjectOf(t, d)
	requireArray("stored null zones", obj, "zones")
	requireObject("stored null alerts", obj, "alerts")
	requireArray("damaged boxes", jsonObjectOf(t, d.Frames[0]), "boxes")
}

// Timestamps come back in UTC, truncated to milliseconds, whatever location and
// precision they were written with.
func TestEventStoreReturnsMillisecondUTC(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	zone := time.FixedZone("UTC+2", 2*60*60)
	started := time.Date(2026, 10, 11, 14, 0, 0, 123456789, zone)
	captured := started.Add(time.Second + 999999*time.Nanosecond)
	ended := started.Add(2*time.Second + time.Nanosecond)
	armedAt := started.Add(3*time.Second + 555555*time.Nanosecond)
	requireMS := func(name string, got, written time.Time) {
		t.Helper()
		want := written.Truncate(time.Millisecond)
		if !got.Equal(want) || got.Location() != time.UTC || got.Nanosecond()%int(time.Millisecond) != 0 {
			t.Fatalf("%s = %s (%s), want %s in UTC", name, got.Format(time.RFC3339Nano), got.Location(), want.UTC().Format(time.RFC3339Nano))
		}
	}

	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 1, CapturedAt: captured, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if err := st.EndEvent(ctx, id, ended); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: armedAt, ChangedBy: ActorUser}); err != nil {
		t.Fatal(err)
	}

	d, err := st.Event(ctx, id)
	if err != nil || d.EndedAt == nil || len(d.Frames) != 1 {
		t.Fatalf("Event = %+v, %v", d, err)
	}
	requireMS("Event.StartedAt", d.StartedAt, started)
	requireMS("Event.EndedAt", *d.EndedAt, ended)
	requireMS("Event.Frames[0].CapturedAt", d.Frames[0].CapturedAt, captured)
	if !reflect.DeepEqual(d.StartedAt, time.Date(2026, 10, 11, 12, 0, 0, 123000000, time.UTC)) {
		t.Fatalf("StartedAt = %#v, want the plain UTC value", d.StartedAt)
	}
	if got := string(mustMarshal(t, d.StartedAt)); got != `"2026-10-11T12:00:00.123Z"` {
		t.Fatalf("StartedAt JSON = %s", got)
	}

	page, err := st.Events(ctx, EventQuery{})
	if err != nil || len(page.Events) != 1 {
		t.Fatalf("Events = %+v, %v", page, err)
	}
	requireMS("Events.StartedAt", page.Events[0].StartedAt, started)
	requireMS("Events.EndedAt", *page.Events[0].EndedAt, ended)
	between, _, _, err := st.EventsBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(between) != 1 {
		t.Fatalf("EventsBetween = %+v, %v", between, err)
	}
	requireMS("EventsBetween.StartedAt", between[0].StartedAt, started)
	latest, err := st.EventsLatest(ctx, st.Cursor(), 10)
	if err != nil || len(latest) != 1 {
		t.Fatalf("EventsLatest = %+v, %v", latest, err)
	}
	requireMS("EventsLatest.EndedAt", *latest[0].EndedAt, ended)

	entries, _, _, err := st.ArmLogBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ArmLogBetween = %+v, %v", entries, err)
	}
	requireMS("ArmLogBetween.At", entries[0].At, armedAt)
	if entries, err = st.ArmLogLatest(ctx, st.Cursor(), 10); err != nil || len(entries) != 1 {
		t.Fatalf("ArmLogLatest = %+v, %v", entries, err)
	}
	requireMS("ArmLogLatest.At", entries[0].At, armedAt)
	rec, err := st.LoadArm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireMS("LoadArm.ChangedAt", rec.ChangedAt, armedAt)
	last, err := st.LastEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireMS("LastEvents.at", last["cam"].at, started)
}

func TestEventStoreArmStateDefaultsOffAndPersists(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.LoadArm(ctx)
	if err != nil || rec.Mode != ArmOff || rec.Armed || !rec.ChangedAt.IsZero() || rec.ChangedBy != "" {
		t.Fatalf("fresh arm state = %+v, %v", rec, err)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmSchedule, Armed: true, ChangedAt: storeT0, ChangedBy: ActorAgent}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rec, err = st.LoadArm(ctx)
	if err != nil || rec.Mode != ArmSchedule || !rec.Armed || !rec.ChangedAt.Equal(storeT0) || rec.ChangedBy != ActorAgent {
		t.Fatalf("reloaded arm state = %+v, %v", rec, err)
	}
	if st.Cursor() == 0 {
		t.Fatal("the cursor must survive a reopen")
	}
}

func TestEventStoreCloseOpenEventsAndRemove(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	withFrame, _ := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if _, err := st.AddFrame(ctx, withFrame, StoredFrame{Seq: 1, Path: "x.jpg", ThumbPath: "x_t.jpg", Size: 1, CapturedAt: storeT0.Add(7 * time.Second), Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	noFrames, _ := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Minute)})
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 2 {
		t.Fatalf("CloseOpenEvents = %d, %v", n, err)
	}
	cursor := st.Cursor()
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 0 || st.Cursor() != cursor {
		t.Fatalf("second CloseOpenEvents = %d, %v; cursor moved %d -> %d", n, err, cursor, st.Cursor())
	}
	if d, _ := st.Event(ctx, withFrame); d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(7*time.Second)) {
		t.Fatalf("event with frames ended at %v", d.EndedAt)
	}
	if d, _ := st.Event(ctx, noFrames); d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(time.Minute)) {
		t.Fatalf("event without frames ended at %v", d.EndedAt)
	}

	id, _ := st.CreateEvent(ctx, NewEvent{CameraID: "front-door", CameraName: "Front", StartedAt: storeT0})
	rel, thumb := "2026/10/11/front-door/9_1.jpg", "2026/10/11/front-door/9_1_thumb.jpg"
	for _, p := range []string{rel, thumb} {
		full := filepath.Join(st.Root(), filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("jpeg"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: rel, ThumbPath: thumb, Size: 8, CapturedAt: storeT0, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	freed, err := st.RemoveEvent(ctx, id)
	if err != nil || freed != 8 {
		t.Fatalf("RemoveEvent = %d, %v", freed, err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), "2026")); !os.IsNotExist(err) {
		t.Fatalf("empty date directories must be removed: %v", err)
	}
	if _, err := os.Stat(st.Root()); err != nil {
		t.Fatalf("the image root itself must stay: %v", err)
	}
	if _, err := st.RemoveEvent(ctx, id); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("second RemoveEvent err = %v", err)
	}
}

func TestEventStoreLastEventsPerCamera(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	_, _ = st.CreateEvent(ctx, NewEvent{CameraID: "a", CameraName: "A", StartedAt: storeT0})
	lastA, _ := st.CreateEvent(ctx, NewEvent{CameraID: "a", CameraName: "A", StartedAt: storeT0.Add(time.Minute)})
	lastB, _ := st.CreateEvent(ctx, NewEvent{CameraID: "b", CameraName: "B", StartedAt: storeT0.Add(2 * time.Minute)})
	last, err := st.LastEvents(ctx)
	if err != nil || len(last) != 2 || last["a"].id != lastA || !last["a"].at.Equal(storeT0.Add(time.Minute)) || last["b"].id != lastB {
		t.Fatalf("LastEvents = %+v, %v", last, err)
	}
}
