package server

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/flows"
)

// ff1FlowsChanged returns the payloads of the flows_changed events with the given reason.
func ff1FlowsChanged(events []desktop.Event, reason string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, ev := range events {
		payload, _ := ev.Payload.(map[string]interface{})
		if ev.Type == "flows_changed" && payload["reason"] == reason {
			out = append(out, payload)
		}
	}
	return out
}

// FF1: the flow-mission hooks broadcast flows_changed with the flow's id, so an editor
// that has the flow open reacts when Mission Control switches or deletes its mission.
func TestFF1MissionHooksBroadcastTheFlowID(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	ctx := context.Background()
	events := c17Events(t, s)
	rec := createTestFlow(t, s, greetFlowJSON)
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c17Drain(events)
	hooks := flowMissionHooks{s: s}

	hooks.FlowEnabledChanged(rec.MissionID, true)
	if got := ff1FlowsChanged(c17Drain(events), "enabled"); len(got) != 1 || got[0]["flow_id"] != rec.ID {
		t.Fatalf("enabled broadcasts = %+v", got)
	}

	hooks.FlowMissionDeleted(rec.MissionID)
	if got := ff1FlowsChanged(c17Drain(events), "deleted"); len(got) != 1 || got[0]["flow_id"] != rec.ID {
		t.Fatalf("deleted broadcasts = %+v", got)
	}
	if _, err := s.Flows.GetFlow(ctx, rec.ID); !errors.Is(err, flows.ErrNotFound) {
		t.Fatalf("the flow of the deleted mission is still there: %v", err)
	}
}

// FF1: POST run of a published flow that is switched off answers 409 FLOW_DISABLED, as
// Mission Control refuses a disabled mission; a test run is still allowed.
func TestFF1RunNowOfAPausedFlowIsRefused(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	rec := createTestFlow(t, s, greetFlowJSON)
	if _, _, err := s.Flows.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, "")
	body := flowsBody(t, w)
	if w.Code != http.StatusConflict || body["code"] != "FLOW_DISABLED" || body["error"] != "the flow is paused; switch it on first" {
		t.Fatalf("run of a paused flow = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/test", token, `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("test run of a paused flow = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, `{"enabled":true}`); w.Code != http.StatusOK {
		t.Fatalf("enable = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/run", token, ""); w.Code != http.StatusAccepted {
		t.Fatalf("run of an enabled flow = %d %s", w.Code, w.Body.String())
	}
}
