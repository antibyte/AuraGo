package flows

import (
	"context"
	"log/slog"
	"reflect"
)

// restoreDurableCoordinator rebuilds the graph against trusted built-in logic.
// The caller owns the decoded state and must hold the run's current lease.
// It does not start execution or complete the pending wait.
func restoreDurableCoordinator(state durableCheckpointState, services *Services, logger *slog.Logger, emit EventSink, missionIDs ...string) (*runState, error) {
	encoded, err := encodeDurableState(state)
	if err != nil {
		return nil, err
	}
	defer clear(encoded)
	state, err = decodeDurableState(encoded)
	if err != nil {
		return nil, err
	}
	if state.Version < 2 || state.Version > 3 {
		return nil, errInvalidCheckpoint
	}
	location, err := restoreDurableLocation(state.Flow, state.Location)
	if err != nil {
		return nil, err
	}
	local := &Services{Location: location}
	if services != nil {
		local.Clock = services.Clock
	}
	engine, err := newDurableEngine(local, logger)
	if err != nil {
		return nil, err
	}
	triggerType, _ := state.Trigger["type"].(string)
	triggerData, _ := state.Trigger["data"].(map[string]any)
	req := RunRequest{RunID: state.RunID, Flow: state.Flow, Revision: state.Revision, Mode: ModeLive,
		TriggerNode: state.TriggerNode, TriggerType: triggerType, TriggerData: triggerData}
	if len(missionIDs) > 0 {
		req.missionID = missionIDs[0]
	}
	if emit == nil {
		emit = func(RunEvent) {}
	}
	s := newRunState(engine, req, emit)
	if code, _, _ := s.checkFlow(); code != "" || !reflect.DeepEqual(s.untrusted, state.Untrusted) {
		return nil, errInvalidCheckpoint
	}
	s.started, s.trigger, s.outputs = state.StartedAt, state.Trigger, state.Outputs
	s.status, s.outputBytes, s.storedBytes, s.seq = state.Statuses, state.OutputBytes, state.StoredBytes, state.EventSeq
	for i := range state.Flow.Edges {
		edge := &state.Flow.Edges[i]
		s.edges[edge] = state.Edges[edge.ID]
	}
	return s, nil
}

// resumeDurableCoordinator may be called only after the store claims a due
// checkpoint and rechecks the current authorization. It never fires the trigger.
func resumeDurableCoordinator(parent context.Context, state durableCheckpointState, services *Services, logger *slog.Logger,
	control *durableRunControl, slot RunSlot, emit EventSink, missionIDs ...string) RunResult {
	if control == nil || control.save == nil || control.checkpoint == nil || control.finish == nil {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "durable recovery is unavailable", durabilityLost: true}
	}
	s, err := restoreDurableCoordinator(state, services, logger, emit, missionIDs...)
	if err != nil {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "the durable checkpoint is invalid", durabilityLost: true}
	}
	if control.testWrapSet != nil {
		def, ok := s.e.reg.Lookup(TypeSet)
		if !ok || def.Execute == nil {
			return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
		}
		def.Execute = control.testWrapSet(def.Execute)
		if def.Execute == nil {
			return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
		}
	}
	now := s.e.services.Now()
	if state.WaitingNode != "" && now.Before(state.WakeAt) {
		return RunResult{Status: RunWaiting, StartedAt: state.StartedAt, suspended: true}
	}
	ctx, cancel := context.WithDeadline(parent, state.Deadline)
	defer cancel()
	control.deadline, control.ctx, control.active = state.Deadline, ctx, true
	s.req.durable, s.req.Slot = control, slot
	s.gate = newSlotGate(slot, ctx)
	if !now.Before(state.Deadline) || ctx.Err() != nil {
		if parent.Err() != nil {
			return s.finish(RunCancelled, "FLOW_CANCELLED", "the run was cancelled", "")
		}
		return s.finish(RunError, "FLOW_RUN_TIMEOUT", "the run took longer than its time limit", "")
	}
	if state.PendingApproval != nil {
		return resumeDurableApproval(parent, ctx, cancel, s, state, control)
	}
	if state.WaitingNode == "" {
		return s.drive(parent, ctx, cancel)
	}
	output, encoded, err := normalizeOutput(map[string]any{"waited_seconds": state.WakeAt.Sub(state.WaitStartedAt).Seconds()})
	if err != nil {
		return RunResult{Status: RunError, ErrorCode: "FLOW_OUTPUT_INVALID", durabilityLost: true}
	}
	stored, truncated := storedOutput(output, encoded)
	n := s.g.nodes[state.WaitingNode]
	step := StepRecord{NodeID: n.ID, NodeKey: n.Key, Attempt: 1, Status: StepSuccess, StartedAt: state.WaitStartedAt,
		FinishedAt: now, DurationMS: now.Sub(state.WaitStartedAt).Milliseconds(), Output: stored, OutputTruncated: truncated}
	s.complete(ctx, nodeDone{nodeID: n.ID, step: step, result: ExecResult{Output: output}, size: len(encoded), stored: storedSize(encoded, truncated)})
	return s.drive(parent, ctx, cancel)
}

func resumeDurableApproval(parent, ctx context.Context, cancel context.CancelFunc, s *runState, state durableCheckpointState, control *durableRunControl) RunResult {
	approval := state.PendingApproval
	if approval == nil || approval.Decision != DurableApprovalDecisionApprove || approval.GateID == "" || approval.GateGeneration <= 0 {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "the approval checkpoint is invalid", durabilityLost: true}
	}
	digest, err := durableApprovalDigest(state, approval)
	if err != nil || digest != approval.ContentDigest {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "the approval checkpoint is invalid", durabilityLost: true}
	}
	n := s.g.nodes[approval.NodeID]
	if n == nil || n.Type != durableApprovalType || n.Key != approval.NodeKey {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", ErrorMessage: "the approval node is unavailable", durabilityLost: true}
	}
	output, encoded, err := normalizeOutput(map[string]any{"approved": true})
	if err != nil {
		return RunResult{Status: RunError, ErrorCode: "FLOW_OUTPUT_INVALID", durabilityLost: true}
	}
	stored, truncated := storedOutput(output, encoded)
	now := s.e.services.Now()
	step := StepRecord{NodeID: n.ID, NodeKey: n.Key, Attempt: 1, Status: StepSuccess, StartedAt: state.WaitStartedAt,
		FinishedAt: now, DurationMS: now.Sub(state.WaitStartedAt).Milliseconds(), Output: stored, OutputTruncated: truncated}
	s.complete(ctx, nodeDone{nodeID: n.ID, step: step, result: ExecResult{Output: output}, size: len(encoded), stored: storedSize(encoded, truncated)})
	if control.lost {
		return RunResult{Status: RunError, ErrorCode: "FLOW_NODE_UNAVAILABLE", durabilityLost: true}
	}
	return s.drive(parent, ctx, cancel)
}
