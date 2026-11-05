package flows

import "time"

// captureDurableState runs on the coordinator after all workers have drained.
// The coordinator must keep its flow and outputs read-only until encoded.
func captureDurableState(s *runState, waitingNode string, waitStartedAt, wakeAt, deadline time.Time, approvals ...*durableApprovalCheckpoint) (durableCheckpointState, error) {
	if s == nil || s.g == nil || s.req.Flow == nil || s.running != 0 || s.failure != nil || s.stop != nil || s.req.Mode != ModeLive || s.req.OnlyNode != "" {
		return durableCheckpointState{}, errInvalidCheckpoint
	}
	if waitingNode != "" {
		if _, decided := s.status[waitingNode]; decided {
			return durableCheckpointState{}, errInvalidCheckpoint
		}
	}
	for i := range s.req.Flow.Nodes {
		node := &s.req.Flow.Nodes[i]
		if s.g.nodes[node.ID] != node {
			return durableCheckpointState{}, errInvalidCheckpoint
		}
	}
	statuses := make(map[string]StepStatus, len(s.status)+1)
	for id, status := range s.status {
		statuses[id] = status
	}
	if waitingNode != "" {
		statuses[waitingNode] = StepWaiting
	}
	edges := make(map[string]edgeState, len(s.req.Flow.Edges))
	for i := range s.req.Flow.Edges {
		edge := &s.req.Flow.Edges[i]
		edges[edge.ID] = s.edges[edge]
	}
	loc := time.UTC
	if s.e != nil && s.e.services != nil {
		loc = s.e.services.Loc()
	}
	location, err := durableLocation(s.req.Flow, loc)
	if err != nil {
		return durableCheckpointState{}, err
	}
	state := durableCheckpointState{
		Version: 3, Location: location, RunID: s.req.RunID, Flow: s.req.Flow, Revision: s.req.Revision,
		TriggerNode: s.req.TriggerNode, Trigger: s.trigger, StartedAt: s.started, Deadline: deadline,
		WakeAt: wakeAt, WaitStartedAt: waitStartedAt, WaitingNode: waitingNode, Statuses: statuses, Edges: edges,
		Outputs: s.outputs, Untrusted: s.untrusted, OutputBytes: s.outputBytes,
		StoredBytes: s.storedBytes, EventSeq: s.seq,
	}
	if len(approvals) > 0 {
		state.PendingApproval = approvals[0]
	}
	if !validDurableState(state) {
		return durableCheckpointState{}, errInvalidCheckpoint
	}
	return state, nil
}
