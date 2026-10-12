package persondetect

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"aurago/internal/onnxrt"
)

func mustMarshal(t *testing.T, value interface{}) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %T: %v", value, err)
	}
	return encoded
}

func jsonKeysOf(t *testing.T, value interface{}) []string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(mustMarshal(t, value), &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// jsonObjectOf decodes the JSON of value with numbers kept as json.Number, so a
// test can tell a number from a string.
func jsonObjectOf(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(mustMarshal(t, value)))
	decoder.UseNumber()
	var fields map[string]interface{}
	if err := decoder.Decode(&fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestAPITypesMatchContractJSONShapes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  []string
	}{
		{"status", Status{}, []string{"arm", "cameras", "cursor", "detector", "enabled", "storage"}},
		{"detector", DetectorStatus{}, []string{"backend", "cpu_ms", "download", "error", "fallback_reason", "model", "model_name", "phase", "providers", "requested_backend", "runtime", "self_test", "state", "wall_ms"}},
		{"arm", ArmState{}, []string{"armed", "changed_at", "changed_by", "mode", "next_change", "time_zone", "utc_offset"}},
		{"camera", CameraStatus{}, []string{"analyses_per_minute", "id", "last_error", "last_event_at", "last_event_id", "name", "state"}},
		{"storage", StorageStatus{}, []string{"bytes", "limit_bytes", "warning"}},
		{"changes", Changes{}, []string{"arm_log", "cursor", "events", "status"}},
		{"event", EventSummary{}, []string{"alerts", "armed", "camera_id", "camera_name", "cover_frame_id", "ended_at", "frame_count", "id", "max_persons", "max_score", "started_at", "zones"}},
		{"detail", EventDetail{}, []string{"alerts", "armed", "camera_id", "camera_name", "cover_frame_id", "ended_at", "frame_count", "frames", "id", "max_persons", "max_score", "started_at", "zones"}},
		{"frame", EventFrame{}, []string{"boxes", "captured_at", "height", "id", "score", "seq", "width"}},
		{"arm log", ArmLogEntry{}, []string{"actor", "armed", "at", "id", "mode"}},
		{"page", EventPage{}, []string{"events", "next_before"}},
		{"test", TestResult{}, []string{"backend", "boxes", "height", "image", "wall_ms", "width"}},
		{"test box", TestBox{}, []string{"counts", "score", "x1", "x2", "y1", "y2", "zones"}},
		{"alert", AlertResult{Status: AlertFailed, Detail: "telegram: 502"}, []string{"detail", "status"}},
		// DetectorStatus nests these two; a change here silently changes the HTTP API.
		{"download", onnxrt.DownloadStatus{}, []string{"downloaded", "percent", "total"}},
		{"download asset", onnxrt.DownloadStatus{Asset: "model.onnx"}, []string{"asset", "downloaded", "percent", "total"}},
		{"self test", SelfTest{}, []string{"cpu_cpu_ms", "cpu_wall_ms", "passed", "reference_persons"}},
		{"self test full", SelfTest{Reason: "r", GPUWallMS: 1, GPUCPUMS: 1}, []string{"cpu_cpu_ms", "cpu_wall_ms", "gpu_cpu_ms", "gpu_wall_ms", "passed", "reason", "reference_persons"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonKeysOf(t, tc.value); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("keys = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPITypesEncodeExactJSON(t *testing.T) {
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"arm off", ArmState{Mode: ArmOff}, `{"mode":"off","armed":false,"next_change":null,"changed_at":null,"changed_by":"","time_zone":"","utc_offset":""}`},
		{"arm berlin", ArmState{Mode: ArmOn, Armed: true, TimeZone: "CET", UTCOffset: "+01:00"}, `{"mode":"on","armed":true,"next_change":null,"changed_at":null,"changed_by":"","time_zone":"CET","utc_offset":"+01:00"}`},
		{"arm log", ArmLogEntry{ID: 1, At: at, Mode: ArmOn, Armed: true, Actor: ActorUser}, `{"id":1,"at":"2026-10-11T12:00:00Z","mode":"on","armed":true,"actor":"user"}`},
		{"alert sent", AlertResult{Status: AlertSent}, `{"status":"sent"}`},
		{"alert failed", AlertResult{Status: AlertFailed, Detail: "telegram: 502"}, `{"status":"failed","detail":"telegram: 502"}`},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := string(mustMarshal(t, tc.value)); got != tc.want {
				t.Fatalf("JSON = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEventJSONValueKinds(t *testing.T) {
	started := time.Date(2026, 10, 11, 12, 0, 0, 123_000_000, time.UTC)
	ended := started.Add(42 * time.Second)
	coverID := int64(42)

	t.Run("filled", func(t *testing.T) {
		event := EventSummary{
			ID: 7, CameraID: "front", CameraName: "Front door", StartedAt: started, EndedAt: &ended,
			MaxScore: 0.91, MaxPersons: 2, Zones: []string{"Driveway"}, Armed: true, CoverFrameID: &coverID, FrameCount: 3,
			Alerts: map[string]AlertResult{ActionMessage: {Status: AlertSent}},
		}
		fields := jsonObjectOf(t, event)
		for _, key := range []string{"id", "cover_frame_id", "max_persons", "frame_count", "max_score"} {
			if _, ok := fields[key].(json.Number); !ok {
				t.Errorf("%s = %#v, want a JSON number", key, fields[key])
			}
		}
		if got, ok := fields["id"].(json.Number); !ok || got.String() != "7" {
			t.Errorf("id = %#v, want 7", fields["id"])
		}
		if got, ok := fields["cover_frame_id"].(json.Number); !ok || got.String() != "42" {
			t.Errorf("cover_frame_id = %#v, want 42", fields["cover_frame_id"])
		}
		if got := fields["started_at"]; got != "2026-10-11T12:00:00.123Z" {
			t.Errorf("started_at = %#v, want a millisecond UTC string", got)
		}
		if got := fields["ended_at"]; got != "2026-10-11T12:00:42.123Z" {
			t.Errorf("ended_at = %#v, want a millisecond UTC string", got)
		}
		if _, ok := fields["zones"].([]interface{}); !ok {
			t.Errorf("zones = %#v, want an array", fields["zones"])
		}
		if _, ok := fields["alerts"].(map[string]interface{}); !ok {
			t.Errorf("alerts = %#v, want an object", fields["alerts"])
		}
	})

	t.Run("unset optionals are null", func(t *testing.T) {
		open := EventSummary{ID: 8, StartedAt: started, Zones: []string{}, Alerts: map[string]AlertResult{}}
		fields := jsonObjectOf(t, open)
		for _, key := range []string{"ended_at", "cover_frame_id"} {
			value, present := fields[key]
			if !present || value != nil {
				t.Errorf("%s = %#v (present %v), want JSON null", key, value, present)
			}
		}
		if got, ok := fields["zones"].([]interface{}); !ok || len(got) != 0 {
			t.Errorf("zones = %#v, want []", fields["zones"])
		}
		if got, ok := fields["alerts"].(map[string]interface{}); !ok || len(got) != 0 {
			t.Errorf("alerts = %#v, want {}", fields["alerts"])
		}
	})

	t.Run("camera status and page", func(t *testing.T) {
		lastID := int64(7)
		fields := jsonObjectOf(t, CameraStatus{ID: "front", LastEventID: &lastID, LastEventAt: &started})
		if got, ok := fields["last_event_id"].(json.Number); !ok || got.String() != "7" {
			t.Errorf("last_event_id = %#v, want the number 7", fields["last_event_id"])
		}
		if got := fields["last_event_at"]; got != "2026-10-11T12:00:00.123Z" {
			t.Errorf("last_event_at = %#v, want a millisecond UTC string", got)
		}
		fields = jsonObjectOf(t, CameraStatus{ID: "front"})
		for _, key := range []string{"last_event_id", "last_event_at"} {
			if value, present := fields[key]; !present || value != nil {
				t.Errorf("%s = %#v (present %v), want JSON null", key, value, present)
			}
		}
		fields = jsonObjectOf(t, EventPage{Events: []EventSummary{}})
		if value, present := fields["next_before"]; !present || value != nil {
			t.Errorf("next_before = %#v (present %v), want JSON null", value, present)
		}
		next := int64(99)
		fields = jsonObjectOf(t, EventPage{Events: []EventSummary{}, NextBefore: &next})
		if got, ok := fields["next_before"].(json.Number); !ok || got.String() != "99" {
			t.Errorf("next_before = %#v, want the number 99", fields["next_before"])
		}
	})
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

func TestSettingsMaxStorageBytesIsMiB(t *testing.T) {
	for _, tc := range []struct {
		mb   int
		want int64
	}{
		{0, 0},
		{1, 1 << 20},
		{DefaultMaxStorageMB, 2048 << 20},
		{1_000_000, 1_000_000 << 20}, // config's upper bound; exceeds 32 bits
	} {
		if got := (Settings{MaxStorageMB: tc.mb}).MaxStorageBytes(); got != tc.want {
			t.Errorf("MaxStorageBytes(%d MiB) = %d, want %d", tc.mb, got, tc.want)
		}
	}
	if got := (Settings{MaxStorageMB: DefaultMaxStorageMB}).MaxStorageBytes(); got != 2147483648 {
		t.Errorf("default limit = %d bytes, want 2147483648", got)
	}
}
