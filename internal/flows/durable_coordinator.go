package flows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

func newDurableEngine(services *Services, logger *slog.Logger) (*Engine, error) {
	reg := NewRegistry()
	if err := RegisterLogicNodes(reg); err != nil {
		return nil, err
	}
	if err := RegisterTriggerNodes(reg); err != nil {
		return nil, err
	}
	if err := reg.Register(durableWaitDef(true)); err != nil {
		return nil, err
	}
	if err := reg.Register(durableApprovalDef(true, func() bool { return true })); err != nil {
		return nil, err
	}
	local := &Services{}
	if services != nil {
		local.Clock, local.Location, local.DurableApprovalAuthority = services.Clock, services.Location, services.DurableApprovalAuthority
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
	renew        func(context.Context) error
	active       bool
	ctx          context.Context
	ownerCtx     context.Context
	pending      []RunEvent
	lost         bool
	persistedSeq int
	testWrapSet  func(ExecuteFunc) ExecuteFunc
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

func (s *runState) suspendDurableApproval(ctx context.Context, id string) (RunResult, *NodeError) {
	control := s.req.durable
	if control == nil || control.save == nil || s.running != 0 || !durableGraphAllowed(s.req.Flow) {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable approval is unavailable")
	}
	n := s.g.nodes[id]
	if n == nil || n.Type != durableApprovalType || n.Settings.Disabled {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable approval is unavailable")
	}
	authority, ok := s.e.services.DurableApprovalAuthority.(DurableApprovalAuthority)
	if !ok || authority.DurableApprovalAllowed(ctx) != nil {
		return RunResult{}, NewNodeError("FLOW_APPROVAL_AUTH_UNAVAILABLE", "durable approval requires the configured owner login")
	}
	def := s.def(n)
	if def == nil {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "durable approval is unavailable")
	}
	params, err := resolveParams(withDefaults(def, n.Params), s.env(), def.fencedParams())
	if err != nil {
		return RunResult{}, NewNodeError("FLOW_TEMPLATE_ERROR", "%v", err)
	}
	summary, ok := params["summary"].(string)
	if !ok || !utf8.ValidString(summary) || len(summary) == 0 || len(summary) > 2<<10 || strings.TrimSpace(summary) == "" {
		return RunResult{}, NewNodeError("FLOW_PARAM_INVALID", "approval summary must contain at most 2 KiB of UTF-8 text")
	}
	reviewData, err := json.Marshal(params["review_data"])
	if err != nil || len(reviewData) == 0 || len(reviewData) > 16<<10 || !json.Valid(reviewData) || jsonDepth(reviewData) > 32 || !jsSafeReviewNumbers(reviewData) {
		return RunResult{}, NewNodeError("FLOW_PARAM_INVALID", "approval review data must be valid JSON up to 16 KiB and depth 32")
	}
	now := s.e.services.Now()
	deadline := control.deadline
	if deadline.IsZero() || !deadline.After(now) {
		return RunResult{}, NewNodeError("FLOW_RUN_TIMEOUT", "the run deadline elapsed before approval")
	}
	pending := &durableApprovalCheckpoint{GateID: NewRunID(), NodeID: n.ID, NodeKey: n.Key,
		AdmissionOwner: s.req.missionID, GateGeneration: 1, ContentDigest: strings.Repeat("0", 64), Summary: summary, ReviewData: reviewData}
	// Pending gates cannot wake; using now avoids rounding a deadline-adjacent time into the deadline.
	state, err := captureDurableState(s, id, now, now, deadline, pending)
	if err != nil {
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the approval checkpoint cannot be prepared")
	}
	state.EventSeq++
	stored, truncated, size := storedParams(map[string]any{"summary": "[review stored securely]", "review_data": "[review stored securely]"})
	step := s.chargeStored(StepRecord{NodeID: id, NodeKey: n.Key, Attempt: 1, Status: StepWaiting, StartedAt: now,
		Params: stored, ParamsTruncated: truncated}, size)
	if err := control.save(ctx, state, step); err != nil {
		control.lost = true
		s.e.logger.Error("flows: approval checkpoint could not be saved", "run", s.req.RunID, "error", err)
		return RunResult{}, NewNodeError("FLOW_NODE_UNAVAILABLE", "the approval checkpoint could not be saved")
	}
	s.status[id] = StepWaiting
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: id, Step: &step})
	return RunResult{Status: RunWaiting, StartedAt: s.started, Steps: s.steps, Outputs: s.outputs, suspended: true}, nil
}

func jsonDepth(raw []byte) int {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return 33
	}
	var depth func(any, int) int
	depth = func(v any, current int) int {
		if current > 33 {
			return current
		}
		switch x := v.(type) {
		case []any:
			for _, child := range x {
				current = max(current, depth(child, current+1))
			}
		case map[string]any:
			for _, child := range x {
				current = max(current, depth(child, current+1))
			}
		}
		return current
	}
	return depth(value, 1)
}

func jsSafeReviewNumbers(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	var safe func(any) bool
	safe = func(v any) bool {
		switch x := v.(type) {
		case json.Number:
			if integer, err := x.Int64(); err == nil {
				return integer >= -9007199254740991 && integer <= 9007199254740991
			}
			f, err := strconv.ParseFloat(string(x), 64)
			if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
				return false
			}
			return math.Trunc(f) != f || (f >= -9007199254740991 && f <= 9007199254740991)
		case []any:
			for _, item := range x {
				if !safe(item) {
					return false
				}
			}
		case map[string]any:
			for _, item := range x {
				if !safe(item) {
					return false
				}
			}
		}
		return true
	}
	return safe(value)
}

func durableApprovalDigest(state durableCheckpointState, pending *durableApprovalCheckpoint) (string, error) {
	if pending == nil || pending.GateID == "" || pending.GateGeneration <= 0 {
		return "", errInvalidCheckpoint
	}
	value := struct {
		RunID      string          `json:"run_id"`
		FlowID     string          `json:"flow_id"`
		Revision   int             `json:"revision"`
		Owner      string          `json:"owner"`
		NodeID     string          `json:"node_id"`
		NodeKey    string          `json:"node_key"`
		GateID     string          `json:"gate_id"`
		Generation int64           `json:"generation"`
		Summary    string          `json:"summary"`
		Review     json.RawMessage `json:"review_data"`
	}{state.RunID, state.Flow.ID, state.Revision, pending.AdmissionOwner, pending.NodeID, pending.NodeKey, pending.GateID, pending.GateGeneration, pending.Summary, pending.ReviewData}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
