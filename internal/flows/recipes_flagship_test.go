package flows

import (
	"context"
	"reflect"
	"testing"
)

func TestFlagshipRecipesSimulateWithoutEffects(t *testing.T) {
	s, bridge := newServiceFixture(t, nil)
	spy := &fakeTools{}
	s.services.Tools = spy
	ctx := context.Background()
	for _, tc := range []struct {
		id, trigger, action string
		param, want         string
		data, output        map[string]any
	}{
		{"email_push", "email", "push", "message", "easydrag.template.email_push.text_1: Invoice", map[string]any{"subject": "Invoice"}, map[string]any{"sent": true}},
		{"mqtt_temperature", "sensor", "push", "message", "easydrag.template.mqtt_temperature.text_1: 31", map[string]any{"json": map[string]any{"temperature": 31.0}}, map[string]any{"sent": true}},
		{"device_offline", "device", "push", "message", "easydrag.template.device_offline.text_1: Laptop", map[string]any{"device_name": "Laptop"}, map[string]any{"sent": true}},
		{"camera_person", "camera", "push", "message", "easydrag.template.camera_person.text_1: Driveway", map[string]any{"camera_name": "Driveway", "persons": 1.0}, map[string]any{"sent": true}},
		{"manual_pdf", "start", "pdf", "content", "A note", map[string]any{"text": "A note"}, map[string]any{"file": map[string]any{"$type": "file", "path": "documents/note.pdf", "name": "note.pdf", "mime": "application/pdf"}}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			rec, err := s.CreateFlow(ctx, CreateRequest{Template: tc.id})
			if err != nil {
				t.Fatal(err)
			}
			trigger, action := rec.Draft.NodeByKey(tc.trigger), rec.Draft.NodeByKey(tc.action)
			if trigger == nil || action == nil {
				t.Fatal("recipe nodes missing")
			}
			req := SimulationRequest{BaseRevision: rec.DraftRevision, TriggerNode: trigger.ID, TriggerData: tc.data,
				Fixtures: map[string]SimulationFixture{action.ID: {Output: tc.output}}}
			result, err := s.Simulate(ctx, rec.ID, req)
			if err != nil || result.Status != RunSuccess {
				t.Fatalf("sample: %+v %v", result, err)
			}
			found := false
			for _, step := range result.Steps {
				if step.NodeID == action.ID && step.Status == StepSuccess {
					if step.Params[tc.param] != tc.want || !reflect.DeepEqual(step.Output, tc.output) {
						t.Fatalf("sample mapping/result: params=%+v output=%+v", step.Params, step.Output)
					}
					if tc.action == "push" && step.Params["channel"] != "push" {
						t.Fatalf("notification channel = %v, want Web Push", step.Params["channel"])
					}
					found = true
				}
			}
			if !found {
				t.Fatal("fixture result not reached")
			}
			req.Fixtures[action.ID] = SimulationFixture{ErrorCode: "FLOW_NODE_UNAVAILABLE"}
			failed, err := s.Simulate(ctx, rec.ID, req)
			if err != nil || failed.Status != RunError || failed.ErrorCode != "FLOW_NODE_UNAVAILABLE" {
				t.Fatalf("fixture failure: %+v %v", failed, err)
			}
			if spy.count() != 0 || len(bridge.startedRuns()) != 0 {
				t.Fatal("simulation dispatched a real effect")
			}
		})
	}
}

func TestFlagshipTemperatureRecipeRejectsMissingOrInvalidInput(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec, err := s.CreateFlow(ctx, CreateRequest{Template: "mqtt_temperature"})
	if err != nil {
		t.Fatal(err)
	}
	trigger, action := rec.Draft.NodeByKey("sensor"), rec.Draft.NodeByKey("push")
	for _, tc := range []struct {
		name   string
		data   map[string]any
		status RunStatus
		code   string
	}{
		{"below threshold", map[string]any{"json": map[string]any{"temperature": 22.0}}, RunSuccess, ""},
		{"missing", map[string]any{"json": map[string]any{}}, RunError, "FLOW_CONDITION_FAILED"},
		{"text", map[string]any{"json": map[string]any{"temperature": "hot"}}, RunError, "FLOW_CONDITION_FAILED"},
		{"object", map[string]any{"json": map[string]any{"temperature": map[string]any{}}}, RunError, "FLOW_CONDITION_FAILED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.Simulate(ctx, rec.ID, SimulationRequest{BaseRevision: rec.DraftRevision, TriggerNode: trigger.ID,
				TriggerData: tc.data, Fixtures: map[string]SimulationFixture{action.ID: {Output: map[string]any{"sent": true}}}})
			if err != nil || res.Status != tc.status || res.ErrorCode != tc.code {
				t.Fatalf("temperature result: %+v %v", res, err)
			}
			for _, step := range res.Steps {
				if step.NodeID == action.ID && step.Status == StepSuccess {
					t.Fatal("unexpected notification")
				}
			}
		})
	}
}
