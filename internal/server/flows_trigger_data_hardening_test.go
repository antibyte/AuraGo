package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
)

// FF1: POST /api/missions/v2/{id}/trigger (like the daemon wake-up, both through
// TriggerMissionWithOptions) starts a flow mission without the caller's data: the manual
// trigger runs with its sample, so caller data never reaches a step through a trigger the
// lint treats as trusted.
func TestFF1MissionTriggerAPIDropsCallerDataForFlows(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := s.Flows.SetEnabled(ctx, rec.ID, true); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/missions/v2/"+rec.MissionID+"/trigger",
		strings.NewReader(`{"trigger_data":"{\"name\":\"Angreifer\"}"}`))
	handleMissionTriggerV2(s, w, r, rec.MissionID)
	if w.Code >= 300 {
		t.Fatalf("trigger = %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		runs, err := s.Flows.Runs(ctx, rec.ID, flows.RunFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) == 1 && runs[0].Status.Terminal() {
			detail, err := s.Flows.Run(ctx, runs[0].ID, false)
			if err != nil || len(detail.Steps) != 2 {
				t.Fatalf("run = %+v, %v", detail, err)
			}
			if got := detail.Steps[1].Output["greeting"]; got != "Hallo Welt" || runs[0].TriggerType != "api" {
				t.Fatalf("greeting %v (trigger type %q); want the manual trigger's sample", got, runs[0].TriggerType)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no finished run: %+v", runs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
