package flows

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"aurago/internal/security"
)

func TestDurableStateRejectsIncompleteOrUnsafeSnapshots(t *testing.T) {
	f := simpleFlow("Local")
	wait := Node{ID: testNodeID(3), Key: "wait", Type: durableWaitType, TypeVersion: 1}
	f.Nodes = append(f.Nodes, wait)
	f.Edges = append(f.Edges, Edge{ID: "e_2", Source: PortRef{Node: f.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: wait.ID, Port: PortIn}})
	trigger := map[string]any{"type": "manual", "node": "start", "fired_at": storeNow.Format(time.RFC3339), "data": map[string]any{}}
	triggerJSON, _ := json.Marshal(trigger)
	s := durableCheckpointState{Version: 1, RunID: "run_local", Flow: f, Revision: 1, TriggerNode: f.Nodes[0].ID,
		StartedAt: storeNow, WaitStartedAt: storeNow, WakeAt: storeNow.Add(time.Minute), Deadline: storeNow.Add(time.Hour), WaitingNode: wait.ID, Trigger: trigger,
		Statuses: map[string]StepStatus{f.Nodes[0].ID: StepSuccess, f.Nodes[1].ID: StepSuccess, wait.ID: StepWaiting},
		Edges:    map[string]edgeState{"e_1": edgeDelivered, "e_2": edgeDelivered}, Outputs: map[string]map[string]any{"start": trigger, "greet": {"greeting": "hello"}}, OutputBytes: len(triggerJSON) + 20}
	encoded, err := encodeDurableState(s)
	if err != nil {
		t.Fatalf("valid state: %v", err)
	}
	if _, err := decodeDurableState(encoded); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	s.Edges["e_2"] = edgeState(99)
	if _, err := encodeDurableState(s); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("invalid edge accepted: %v", err)
	}
	s.Edges["e_2"] = edgeDelivered
	s.Statuses[f.Nodes[1].ID] = StepRunning
	if _, err := encodeDurableState(s); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("running node accepted: %v", err)
	}
	s.Statuses[f.Nodes[1].ID] = StepSuccess
	secret := "test-durable-\\\"<checkpoint>-secret-8492"
	security.RegisterSensitive(secret)
	s.Outputs["greet"]["greeting"] = secret
	output, _ := json.Marshal(s.Outputs["greet"])
	s.OutputBytes = len(output) + len(triggerJSON)
	if _, err := encodeDurableState(s); !errors.Is(err, errInvalidCheckpoint) {
		t.Fatalf("registered secret accepted: %v", err)
	}
}
