package flows

import (
	"reflect"
	"testing"
	"time"
)

func TestPersonDetectedTriggerBindsMissionConfig(t *testing.T) {
	reg := triggerRegistry(t)
	b := newFlow("Person")
	full := b.node("cam", TypeTriggerPersonDetected, map[string]any{
		"camera": "garden", "zone": " Einfahrt ", "include_disarmed": true, "min_interval_seconds": 60.0,
	})
	bare := b.node("any", TypeTriggerPersonDetected, nil)
	got, err := BindTriggers(b.build(), reg, time.UTC, triggerNow)
	if err != nil || len(got) != 2 {
		t.Fatalf("BindTriggers = %+v, %v", got, err)
	}
	want := TriggerBinding{NodeID: full, Kind: BindingMission, MissionTrigger: "person_detected", Config: map[string]any{
		"person_camera_id": "garden", "person_zone": "Einfahrt", "person_include_disarmed": true, "min_interval_seconds": 60.0,
	}}
	if !reflect.DeepEqual(got[0], want) {
		t.Errorf("configured trigger = %+v, want %+v", got[0], want)
	}
	wantBare := TriggerBinding{NodeID: bare, Kind: BindingMission, MissionTrigger: "person_detected", Config: map[string]any{
		"person_camera_id": "", "person_zone": "", "person_include_disarmed": false,
	}}
	if !reflect.DeepEqual(got[1], wantBare) {
		t.Errorf("bare trigger = %+v, want %+v", got[1], wantBare)
	}
	def := lookupDef(t, reg, TypeTriggerPersonDetected)
	if def.UntrustedOutput || def.Icon != "camera" || def.LabelKey != "easydrag.node.trigger_person_detected.label" {
		t.Errorf("definition = untrusted %v, icon %q, label %q", def.UntrustedOutput, def.Icon, def.LabelKey)
	}
	sample := TriggerSample(&Node{ID: full, Type: TypeTriggerPersonDetected})
	for _, key := range []string{"event_id", "camera_id", "camera_name", "zones", "max_score", "persons", "armed", "started_at", "image_path"} {
		if _, ok := sample[key]; !ok {
			t.Errorf("sample lacks %s: %v", key, sample)
		}
	}
}
