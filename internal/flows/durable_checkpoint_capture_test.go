package flows

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestCaptureDurableStateRequiresQuiescentCoordinator(t *testing.T) {
	f := simpleFlow("Local")
	wait := Node{ID: testNodeID(3), Key: "wait", Type: durableWaitType, TypeVersion: 1}
	f.Nodes = append(f.Nodes, wait)
	f.Edges = append(f.Edges, Edge{ID: "e_2", Source: PortRef{Node: f.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: wait.ID, Port: PortIn}})
	s := newRunState(nil, RunRequest{RunID: "run_local", Flow: f, Revision: 1, Mode: ModeLive, TriggerNode: f.Nodes[0].ID}, nil)
	s.g, s.started = buildGraph(f), storeNow
	s.status[f.Nodes[0].ID], s.status[f.Nodes[1].ID] = StepSuccess, StepSuccess
	s.trigger = map[string]any{"type": "manual", "node": "start", "fired_at": storeNow.Format(time.RFC3339), "data": map[string]any{}}
	s.outputs["start"], s.outputs["greet"] = s.trigger, map[string]any{"greeting": "hello"}
	triggerJSON, _ := json.Marshal(s.trigger)
	s.outputBytes = len(triggerJSON) + 20
	for i := range f.Edges {
		s.edges[&f.Edges[i]] = edgeDelivered
	}
	wake, deadline := storeNow.Add(time.Minute), storeNow.Add(time.Hour)
	state, err := captureDurableState(s, wait.ID, storeNow, wake, deadline)
	if err != nil || state.Statuses[wait.ID] != StepWaiting {
		t.Fatalf("capture: %+v %v", state, err)
	}
	restored, err := restoreDurableCoordinator(state, &Services{Tools: &fakeTools{}}, discardLogger(), nil)
	if err != nil || restored.status[wait.ID] != StepWaiting || restored.edges[&restored.req.Flow.Edges[1]] != edgeDelivered || restored.e.services.Tools != nil {
		t.Fatalf("restore: %+v %v", restored, err)
	}
	s.running = 1
	if _, err := captureDurableState(s, wait.ID, storeNow, wake, deadline); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("running capture: %v", err)
	}
}
