package persondetect

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"
)

func jsonKeysOf(t *testing.T, value interface{}) []string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestAPITypesMatchContractJSONShapes(t *testing.T) {
	for name, tc := range map[string]struct {
		value interface{}
		want  []string
	}{
		"status":   {Status{}, []string{"arm", "cameras", "cursor", "detector", "enabled", "storage"}},
		"detector": {DetectorStatus{}, []string{"backend", "cpu_ms", "download", "error", "fallback_reason", "model", "model_name", "phase", "providers", "requested_backend", "runtime", "self_test", "state", "wall_ms"}},
		"arm":      {ArmState{}, []string{"armed", "changed_at", "changed_by", "mode", "next_change"}},
		"camera":   {CameraStatus{}, []string{"analyses_per_minute", "id", "last_error", "last_event_at", "last_event_id", "name", "state"}},
		"storage":  {StorageStatus{}, []string{"bytes", "limit_bytes", "warning"}},
		"changes":  {Changes{}, []string{"arm_log", "cursor", "events", "status"}},
		"event":    {EventSummary{}, []string{"alerts", "armed", "camera_id", "camera_name", "cover_frame_id", "ended_at", "frame_count", "id", "max_persons", "max_score", "started_at", "zones"}},
		"detail":   {EventDetail{}, []string{"alerts", "armed", "camera_id", "camera_name", "cover_frame_id", "ended_at", "frame_count", "frames", "id", "max_persons", "max_score", "started_at", "zones"}},
		"frame":    {EventFrame{}, []string{"boxes", "captured_at", "height", "id", "score", "seq", "width"}},
		"arm log":  {ArmLogEntry{}, []string{"actor", "armed", "at", "id", "mode"}},
		"page":     {EventPage{}, []string{"events", "next_before"}},
		"test":     {TestResult{}, []string{"backend", "boxes", "height", "image", "wall_ms", "width"}},
		"test box": {TestBox{}, []string{"counts", "score", "x1", "x2", "y1", "y2", "zones"}},
		"alert":    {AlertResult{Status: AlertFailed, Detail: "telegram: 502"}, []string{"detail", "status"}},
	} {
		if got := jsonKeysOf(t, tc.value); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s keys = %v, want %v", name, got, tc.want)
		}
	}
	encoded, _ := json.Marshal(ArmState{Mode: ArmOff})
	if string(encoded) != `{"mode":"off","armed":false,"next_change":null,"changed_at":null,"changed_by":""}` {
		t.Fatalf("ArmState JSON = %s", encoded)
	}
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	encoded, _ = json.Marshal(ArmLogEntry{ID: 1, At: at, Mode: ArmOn, Armed: true, Actor: ActorUser})
	if string(encoded) != `{"id":1,"at":"2026-10-11T12:00:00Z","mode":"on","armed":true,"actor":"user"}` {
		t.Fatalf("ArmLogEntry JSON = %s", encoded)
	}
}

func TestFireSetFiredOrderAndArmModes(t *testing.T) {
	all := FireSet{Message: true, Agent: true, Desktop: true, Mission: true}
	if got := all.Fired(); !reflect.DeepEqual(got, []string{ActionMessage, ActionAgent, ActionDesktop, ActionMission}) {
		t.Fatalf("Fired() = %v", got)
	}
	if got := (FireSet{Mission: true}).Fired(); !reflect.DeepEqual(got, []string{"mission"}) {
		t.Fatalf("mission only = %v", got)
	}
	if got := (FireSet{}).Fired(); len(got) != 0 {
		t.Fatalf("nothing fired = %v", got)
	}
	for _, mode := range []string{"off", "on", "schedule"} {
		if !ValidArmMode(mode) {
			t.Fatalf("%q rejected", mode)
		}
	}
	for _, mode := range []string{"", "ON", "armed"} {
		if ValidArmMode(mode) {
			t.Fatalf("%q accepted", mode)
		}
	}
}
