package flows

import (
	"context"
	"log/slog"
	"math"
	"time"
)

func newDurableEngine(services *Services, logger *slog.Logger) (*Engine, error) {
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		return nil, err
	}
	if err := RegisterTriggerNodes(reg); err != nil {
		return nil, err
	}
	if err := reg.Register(&NodeDef{Type: durableWaitType, Version: 1, Category: "logic",
		Params: []ParamSpec{{Name: "seconds", Kind: ParamNumber, Required: true, Templatable: true}}}); err != nil {
		return nil, err
	}
	local := &Services{}
	if services != nil {
		local.Clock, local.Location = services.Clock, services.Location
	}
	engine := NewEngine(reg, local, logger, 1)
	engine.durableSafe = true
	return engine, nil
}

type durableRunControl struct {
	deadline     time.Time
	save         func(context.Context, durableCheckpointState, StepRecord) error
	checkpoint   func(context.Context, durableCheckpointState, StepRecord) error
	finish       func(context.Context, RunResult, []RunEvent) error
	active       bool
	ctx          context.Context
	pending      []RunEvent
	lost         bool
	persistedSeq int
}

// suspendDurableNode runs only after the coordinator has drained every worker.
func (s *runState) suspendDurableNode(ctx context.Context, id string) (RunResult, *NodeError) {
	control := s.req.durable
	if control == nil || control.save == nil || s.running != 0 || !durableGraphAllowed(s.req.Flow) {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable execution is unavailable")
	}
	n := s.g.nodes[id]
	if n == nil {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the durable wait is unavailable")
	}
	def := s.def(n)
	if def == nil || n.Type != durableWaitType {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the durable wait is unavailable")
	}
	params, err := resolveParams(withDefaults(def, n.Params), s.env(), def.fencedParams())
	if err != nil {
		return RunResult{}, NewNodeError("FLOW_TEMPLATE_ERROR", "%v", err)
	}
	seconds, ok := toNumber(params["seconds"])
	if !ok || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 1 || seconds > MaxRunSecondsLimit {
		return RunResult{}, NewNodeError("FLOW_PARAM_INVALID", "wait seconds must be between 1 and 86400")
	}
	now := s.e.services.Now()
	wake := now.Add(time.Duration(seconds * float64(time.Second)))
	if !wake.Before(control.deadline) {
		return RunResult{}, NewNodeError("FLOW_WAIT_TOO_LONG", "the wait exceeds the run deadline")
	}
	stored, truncated, size := storedParams(redactCredentialHeaders(params, false))
	step := s.chargeStored(StepRecord{NodeID: id, NodeKey: n.Key, Attempt: 1, Status: StepWaiting,
		StartedAt: now, Params: stored, ParamsTruncated: truncated}, size)
	state, err := captureDurableState(s, id, now, wake, control.deadline)
	if err != nil {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the durable run cannot be checkpointed")
	}
	// Reserve the waiting event's sequence in the committed checkpoint.
	state.EventSeq++
	if err := control.save(ctx, state, step); err != nil {
		control.lost = true
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the durable run could not be saved")
	}
	s.status[id] = StepWaiting
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: id, Step: &step})
	return RunResult{Status: RunWaiting, StartedAt: s.started, Steps: s.steps, Outputs: s.outputs, suspended: true}, nil
}
