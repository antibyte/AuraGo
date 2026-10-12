package tools

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"
)

func personDetectedMission(id string, cfg *TriggerConfig) *MissionV2 {
	return &MissionV2{
		ID: id, Name: id, Prompt: "check the camera", ExecutionType: ExecutionTriggered,
		TriggerType: TriggerPersonDetected, TriggerConfig: cfg, Priority: "medium", Enabled: true, Status: MissionStatusIdle,
	}
}

func personDetectedTestEvent() PersonDetectedEvent {
	return PersonDetectedEvent{
		EventID: 42, CameraID: "garden", CameraName: "Garten", Zones: []string{"Einfahrt"},
		MaxScore: 0.91, Persons: 2, Armed: true,
		StartedAt: time.Date(2026, 10, 11, 19, 14, 3, 0, time.UTC),
		ImagePath: "/data/go2rtc/detections/2026/10/11/garden/42_1.jpg",
	}
}

func queuedMissionIDs(mm *MissionManagerV2) []string {
	ids := []string{}
	for _, item := range mm.queue.List() {
		ids = append(ids, item.MissionID)
	}
	sort.Strings(ids)
	return ids
}

func personDetectedManager(t *testing.T) *MissionManagerV2 {
	t.Helper()
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.missions["any"] = personDetectedMission("any", nil)
	mm.missions["garden"] = personDetectedMission("garden", &TriggerConfig{PersonCameraID: "Garden"})
	mm.missions["door"] = personDetectedMission("door", &TriggerConfig{PersonCameraID: "door"})
	mm.missions["zone"] = personDetectedMission("zone", &TriggerConfig{PersonZone: " einfahrt "})
	mm.missions["other_zone"] = personDetectedMission("other_zone", &TriggerConfig{PersonZone: "Tor"})
	mm.missions["disarmed_too"] = personDetectedMission("disarmed_too", &TriggerConfig{PersonIncludeDisarmed: true})
	off := personDetectedMission("off", nil)
	off.Enabled = false
	mm.missions["off"] = off
	return mm
}

func TestNotifyPersonDetectedFiltersCameraZoneAndArmedState(t *testing.T) {
	mm := personDetectedManager(t)
	mm.NotifyPersonDetected(personDetectedTestEvent())
	if got, want := strings.Join(queuedMissionIDs(mm), ","), "any,disarmed_too,garden,zone"; got != want {
		t.Fatalf("armed event queued %s, want %s", got, want)
	}
	if mm.missions["garden"].Status != MissionStatusQueued {
		t.Fatalf("garden status = %q", mm.missions["garden"].Status)
	}

	mm = personDetectedManager(t)
	ev := personDetectedTestEvent()
	ev.Armed = false
	mm.NotifyPersonDetected(ev)
	if got := strings.Join(queuedMissionIDs(mm), ","); got != "disarmed_too" {
		t.Fatalf("disarmed event queued %s, want only disarmed_too", got)
	}

	// An event without zones (whole picture) never matches a zone filter.
	mm = personDetectedManager(t)
	ev = personDetectedTestEvent()
	ev.Zones = nil
	mm.NotifyPersonDetected(ev)
	if got := strings.Join(queuedMissionIDs(mm), ","); got != "any,disarmed_too,garden" {
		t.Fatalf("zoneless event queued %s", got)
	}
}

func TestNotifyPersonDetectedTriggerData(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.missions["any"] = personDetectedMission("any", nil)
	mm.NotifyPersonDetected(personDetectedTestEvent())
	items := mm.queue.List()
	if len(items) != 1 || items[0].TriggerType != string(TriggerPersonDetected) {
		t.Fatalf("queue = %+v", items)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(items[0].TriggerData), &data); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"event_id": 42.0, "camera_id": "garden", "camera_name": "Garten", "zones": []any{"Einfahrt"},
		"max_score": 0.91, "persons": 2.0, "armed": true, "started_at": "2026-10-11T19:14:03Z",
		"image_path": "/data/go2rtc/detections/2026/10/11/garden/42_1.jpg",
	}
	if len(data) != len(want) {
		t.Fatalf("trigger data keys = %v", data)
	}
	for key, value := range want {
		if got, _ := json.Marshal(data[key]); string(got) != personJSON(t, value) {
			t.Errorf("%s = %s, want %s", key, got, personJSON(t, value))
		}
	}
}

// Event ids are int64 around 1.8e12 and event times have milliseconds (C4); the trigger data
// keeps both exactly as the detection API reports them.
func TestNotifyPersonDetectedKeepsLargeIDsAndMilliseconds(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.missions["any"] = personDetectedMission("any", nil)
	ev := personDetectedTestEvent()
	ev.EventID = 1791234567890
	ev.StartedAt = time.Date(2026, 10, 11, 21, 14, 3, 250_987_000, time.FixedZone("CEST", 2*3600))
	mm.NotifyPersonDetected(ev)
	items := mm.queue.List()
	if len(items) != 1 {
		t.Fatalf("queue = %+v", items)
	}
	raw := items[0].TriggerData
	for _, want := range []string{`"event_id":1791234567890`, `"started_at":"2026-10-11T19:14:03.25Z"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("trigger data %s lacks %s", raw, want)
		}
	}
}

func personJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNotifyPersonDetectedHonorsMinInterval(t *testing.T) {
	mm := NewMissionManagerV2(tempSystemTaskDir(t), nil)
	mm.missions["throttled"] = personDetectedMission("throttled", &TriggerConfig{MinIntervalSeconds: 600})
	mm.missions["free"] = personDetectedMission("free", nil)
	mm.NotifyPersonDetected(personDetectedTestEvent())
	mm.queue.Remove("throttled")
	mm.queue.Remove("free")
	ev := personDetectedTestEvent()
	ev.EventID = 43
	mm.NotifyPersonDetected(ev)
	if got := strings.Join(queuedMissionIDs(mm), ","); got != "free" {
		t.Fatalf("second event queued %s, want only free", got)
	}
}

func TestPersonDetectedFlowTriggersMatchFilters(t *testing.T) {
	mm, hooks, _ := newFlowTestManager(t)
	id := publishTestFlow(t, mm,
		FlowTriggerSpec{NodeID: "n_ffffffff", TriggerType: TriggerPersonDetected, TriggerConfig: &TriggerConfig{PersonCameraID: "garden"}},
	)
	ev := personDetectedTestEvent()
	ev.CameraID = "door"
	mm.NotifyPersonDetected(ev)
	hooks.expectNoStart(t)
	ev = personDetectedTestEvent()
	ev.Armed = false
	mm.NotifyPersonDetected(ev)
	hooks.expectNoStart(t)
	mm.NotifyPersonDetected(personDetectedTestEvent())
	c := hooks.waitStart(t)
	if c.missionID != id || c.nodeID != "n_ffffffff" || c.triggerType != "person_detected" ||
		!strings.Contains(c.data, `"camera_id":"garden"`) || !strings.Contains(c.data, `"armed":true`) {
		t.Fatalf("flow start = %+v", c)
	}
	if queue, _ := mm.GetQueue(); len(queue.List()) != 0 {
		t.Fatalf("flow events must not use the agent queue: %+v", queue.List())
	}
}
