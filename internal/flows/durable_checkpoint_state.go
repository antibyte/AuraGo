package flows

import (
	"encoding/json"
	"reflect"
	"time"

	"aurago/internal/security"
)

// durableCheckpointState is full coordinator data, not the truncated run log.
// Only a quiescent pure-graph coordinator may create one.
type durableCheckpointState struct {
	Version       int                       `json:"version"`
	RunID         string                    `json:"run_id"`
	Flow          *Flow                     `json:"flow"`
	Revision      int                       `json:"revision"`
	TriggerNode   string                    `json:"trigger_node"`
	Trigger       map[string]any            `json:"trigger"`
	StartedAt     time.Time                 `json:"started_at"`
	Deadline      time.Time                 `json:"deadline"`
	WakeAt        time.Time                 `json:"wake_at"`
	WaitStartedAt time.Time                 `json:"wait_started_at"`
	WaitingNode   string                    `json:"waiting_node"`
	Statuses      map[string]StepStatus     `json:"statuses"`
	Edges         map[string]edgeState      `json:"edges"`
	Outputs       map[string]map[string]any `json:"outputs"`
	Untrusted     map[string]bool           `json:"untrusted"`
	OutputBytes   int                       `json:"output_bytes"`
	StoredBytes   int                       `json:"stored_bytes"`
	EventSeq      int                       `json:"event_seq"`
}

// encodeDurableState rejects registered secret values instead of redacting
// executable data into an incorrect checkpoint.
func encodeDurableState(state durableCheckpointState) ([]byte, error) {
	if !validDurableState(state) {
		return nil, errInvalidCheckpoint
	}
	data, err := json.Marshal(state)
	if err != nil || len(data) > maxCheckpointBytes-64 {
		return nil, errInvalidCheckpoint
	}
	if checkpointHasRegisteredSecret(data) {
		return nil, errInvalidCheckpoint
	}
	return data, nil
}

func checkpointHasRegisteredSecret(data []byte) bool {
	if security.ScrubSecrets(string(data)) != string(data) {
		return true
	}
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return true
	}
	stack := []any{decoded}
	visited := 0
	for len(stack) > 0 {
		visited++
		if visited > 1_000_000 {
			return true
		}
		value := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch x := value.(type) {
		case string:
			if security.ScrubSecrets(x) != x {
				return true
			}
		case []any:
			if len(x) > 1_000_000-visited-len(stack) {
				return true
			}
			stack = append(stack, x...)
		case map[string]any:
			if len(x) > 1_000_000-visited-len(stack) {
				return true
			}
			for key, item := range x {
				if security.ScrubSecrets(key) != key {
					return true
				}
				stack = append(stack, item)
			}
		}
	}
	return false
}

func decodeDurableState(data []byte) (durableCheckpointState, error) {
	var state durableCheckpointState
	if len(data) > maxCheckpointBytes-64 || json.Unmarshal(data, &state) != nil || !validDurableState(state) {
		return durableCheckpointState{}, errInvalidCheckpoint
	}
	return state, nil
}

func validDurableState(s durableCheckpointState) bool {
	if s.Version != 1 || s.RunID == "" || len(s.RunID) > 128 || s.Revision <= 0 || !durableGraphAllowed(s.Flow) || s.Flow.Schema != SchemaVersion || s.Flow.ID == "" ||
		s.StartedAt.IsZero() || !s.Deadline.After(s.StartedAt) || s.Deadline.Sub(s.StartedAt) > MaxRunSecondsLimit*time.Second ||
		s.OutputBytes < 0 || s.OutputBytes > MaxRunOutputBytes ||
		s.StoredBytes < 0 || s.StoredBytes > MaxStoredRunBytes || s.EventSeq < 0 {
		return false
	}
	doc, err := json.Marshal(s.Flow)
	if err != nil || len(doc) > MaxDocumentBytes {
		return false
	}
	trigger := s.Flow.NodeByID(s.TriggerNode)
	if trigger == nil || trigger.Type != TypeTriggerManual || trigger.Settings.Disabled || len(s.Edges) != len(s.Flow.Edges) {
		return false
	}
	triggerType, validType := s.Trigger["type"].(string)
	triggerKey, validKey := s.Trigger["node"].(string)
	firedAt, validTime := s.Trigger["fired_at"].(string)
	_, validData := s.Trigger["data"].(map[string]any)
	if !validType || triggerType == "" || !validKey || triggerKey != trigger.Key || !validTime || !validData {
		return false
	}
	if _, err := time.Parse(time.RFC3339, firedAt); err != nil {
		return false
	}
	if s.WaitingNode != "" {
		wait := s.Flow.NodeByID(s.WaitingNode)
		if wait == nil || wait.Type != durableWaitType || wait.Settings.Disabled || s.Statuses[s.WaitingNode] != StepWaiting ||
			s.WaitStartedAt.Before(s.StartedAt) || s.WakeAt.Before(s.WaitStartedAt) || !s.WakeAt.Before(s.Deadline) {
			return false
		}
	} else if !s.WaitStartedAt.IsZero() || !s.WakeAt.IsZero() {
		return false
	}
	nodes, keys := map[string]bool{}, map[string]bool{}
	for _, n := range s.Flow.Nodes {
		if !ValidNodeID(n.ID) || nodes[n.ID] || !ValidKey(n.Key) || keys[n.Key] {
			return false
		}
		nodes[n.ID], keys[n.Key] = true, true
	}
	for id, status := range s.Statuses {
		if !nodes[id] {
			return false
		}
		switch status {
		case StepSuccess, StepSkipped, StepError:
		case StepWaiting:
			if id != s.WaitingNode {
				return false
			}
		default:
			return false
		}
	}
	if s.Statuses[s.TriggerNode] != StepSuccess {
		return false
	}
	outputBytes := 0
	for _, node := range s.Flow.Nodes {
		output, present := s.Outputs[node.Key]
		status := s.Statuses[node.ID]
		if status == StepSuccess || status == StepError {
			if !present || output == nil {
				return false
			}
			if status == StepError && node.Settings.OnError != ErrorContinue && node.Settings.OnError != ErrorPort {
				return false
			}
			encoded, err := json.Marshal(output)
			if err != nil || len(encoded) > MaxOutputBytes {
				return false
			}
			if status == StepSuccess {
				outputBytes += len(encoded)
			}
		} else if present {
			return false
		}
	}
	for key := range s.Outputs {
		if !keys[key] {
			return false
		}
	}
	if !reflect.DeepEqual(s.Trigger, s.Outputs[trigger.Key]) {
		return false
	}
	if outputBytes != s.OutputBytes {
		return false
	}
	edges := map[string]bool{}
	for _, edge := range s.Flow.Edges {
		if edge.ID == "" || edges[edge.ID] || !nodes[edge.Source.Node] || !nodes[edge.Target.Node] {
			return false
		}
		edges[edge.ID] = true
		value, ok := s.Edges[edge.ID]
		if !ok || value < edgePending || value > edgeSkipped {
			return false
		}
		switch s.Statuses[edge.Source.Node] {
		case StepSuccess, StepError:
			if value == edgePending {
				return false
			}
		case StepSkipped:
			if value != edgeSkipped {
				return false
			}
		default:
			if value != edgePending {
				return false
			}
		}
	}
	g := buildGraph(s.Flow)
	_, cyclic := g.topoOrder()
	if len(cyclic) != 0 {
		return false
	}
	for id, status := range s.Statuses {
		if id == s.TriggerNode || status == StepSkipped {
			continue
		}
		delivered := false
		for _, edge := range g.incoming[id] {
			value := s.Edges[edge.ID]
			if value == edgePending {
				return false
			}
			if value == edgeDelivered {
				delivered = true
			}
		}
		if !delivered {
			return false
		}
	}
	return true
}
