package server

import (
	"testing"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

func TestFlowTriggerSpecsAcceptPersonDetected(t *testing.T) {
	specs, err := flowTriggerSpecs([]flows.TriggerBinding{{
		NodeID: "n_aaaaaaaa", Kind: flows.BindingMission, MissionTrigger: "person_detected",
		Config: map[string]any{"person_camera_id": "garden", "person_zone": "Einfahrt", "person_include_disarmed": true, "min_interval_seconds": 60.0},
	}})
	if err != nil || len(specs) != 1 {
		t.Fatalf("specs = %+v, %v", specs, err)
	}
	cfg := specs[0].TriggerConfig
	if specs[0].TriggerType != tools.TriggerPersonDetected || cfg == nil || cfg.PersonCameraID != "garden" ||
		cfg.PersonZone != "Einfahrt" || !cfg.PersonIncludeDisarmed || cfg.MinIntervalSeconds != 60 {
		t.Fatalf("spec = %+v, config %+v", specs[0], cfg)
	}
}
