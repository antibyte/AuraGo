package flows

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type edgeState int

const (
	edgePending edgeState = iota
	edgeDelivered
	edgeSkipped
)

// runState is owned by the coordinator goroutine of one run.
type runState struct {
	e        *Engine
	req      RunRequest
	emit     EventSink
	seq      int
	g        *graph
	topo     []string // all node ids in topological order (document order breaks ties)
	started  time.Time
	edges    map[*Edge]edgeState
	status   map[string]StepStatus
	queued   map[string]bool
	outputs  map[string]map[string]any
	trigger  map[string]any
	steps    []StepRecord
	inScope  map[string]bool
	running  int
	failure  *NodeError
	failedID string
	stop     *StopSignal
}

// newRunState prepares the state of one run; run builds the graph after
// checking the flow.
func newRunState(e *Engine, req RunRequest, emit EventSink) *runState {
	return &runState{
		e: e, req: req, emit: emit,
		edges:   map[*Edge]edgeState{},
		status:  map[string]StepStatus{},
		queued:  map[string]bool{},
		outputs: map[string]map[string]any{},
	}
}

func (s *runState) publish(ev RunEvent) {
	s.seq++
	ev.Seq = s.seq
	ev.RunID = s.req.RunID
	ev.Time = s.e.services.Now()
	s.emit(ev)
}

func (s *runState) def(n *Node) *NodeDef {
	def, _ := s.e.reg.Lookup(n.Type)
	return def
}

func (s *runState) isTrigger(n *Node) bool {
	def := s.def(n)
	return def != nil && def.Trigger
}

// checkFlow rejects a flow the engine cannot run: a missing one, one over the
// document limits and one with a loop (a loop is only a warning for drafts,
// and its nodes would never become ready). On success it has built the graph
// and the topological order.
func (s *runState) checkFlow() (code, msg string) {
	f := s.req.Flow
	if f == nil {
		return "FLOW_INVALID", "the run has no flow"
	}
	if len(f.Nodes) > MaxNodes || len(f.Edges) > MaxEdges {
		return "FLOW_TOO_LARGE", fmt.Sprintf("the flow has %d nodes and %d connections; the limits are %d and %d",
			len(f.Nodes), len(f.Edges), MaxNodes, MaxEdges)
	}
	s.g = buildGraph(f)
	order, cyclic := s.g.topoOrder()
	if len(cyclic) > 0 {
		return "FLOW_CYCLE", "the flow contains a loop"
	}
	s.topo = order
	return "", ""
}

func (s *runState) run(parent context.Context) RunResult {
	s.started = s.e.services.Now()
	s.publish(RunEvent{Type: EventRunStarted, Run: &RunSummary{Status: RunRunning}})
	if code, msg := s.checkFlow(); code != "" {
		return s.finish(RunError, code, msg, "")
	}
	timeout := s.req.Timeout
	if timeout <= 0 {
		// Clamp before multiplying: a huge value would overflow the duration.
		timeout = time.Duration(clampInt(s.req.Flow.Settings.MaxRunSeconds, 0, MaxRunSecondsLimit)) * time.Second
	}
	if timeout <= 0 {
		timeout = DefaultMaxRunSeconds * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	trigger := s.g.nodes[s.req.TriggerNode]
	if trigger == nil || !s.isTrigger(trigger) {
		return s.finish(RunError, "FLOW_TRIGGER_INVALID", "the trigger node does not exist", "")
	}
	if trigger.Settings.Disabled {
		return s.finish(RunError, "FLOW_TRIGGER_DISABLED", "the trigger node is disabled", trigger.ID)
	}
	if s.req.OnlyNode != "" {
		if s.g.nodes[s.req.OnlyNode] == nil {
			return s.finish(RunError, "FLOW_NODE_NOT_FOUND", "the node to test does not exist", "")
		}
		s.inScope = s.g.ancestors(s.req.OnlyNode)
		s.inScope[s.req.OnlyNode] = true
	}
	s.fireTrigger(trigger)

	done := make(chan nodeDone)
	var ready []string
	s.collectReady(&ready)
	for {
		for len(ready) > 0 && s.running < s.e.parallel && s.failure == nil && s.stop == nil && ctx.Err() == nil {
			id := ready[0]
			ready = ready[1:]
			s.launch(ctx, id, done)
		}
		if s.running == 0 {
			break
		}
		d := <-done
		s.running--
		s.complete(d)
		if s.failure != nil || s.stop != nil {
			cancel()
		}
		s.collectReady(&ready)
	}
	return s.conclude(parent, ctx)
}

// fireTrigger records the trigger's output and decides the nodes that cannot
// run in this run. Trigger data that cannot be encoded, or that makes the
// trigger output larger than MaxOutputBytes, is replaced with {} and logged.
func (s *runState) fireTrigger(trigger *Node) {
	now := s.e.services.Now()
	data := s.req.TriggerData
	if data == nil {
		data = map[string]any{}
	}
	raw := map[string]any{"type": s.req.TriggerType, "node": trigger.Key, "fired_at": now.Format(time.RFC3339), "data": data}
	out, size, err := normalizeOutput(raw)
	if err != nil || out == nil {
		s.e.logger.Warn("flows: trigger data replaced with an empty object", "flow", s.req.Flow.ID, "run", s.req.RunID,
			"bytes", size, "limit", MaxOutputBytes, "error", err)
		raw["data"] = map[string]any{}
		out, _, _ = normalizeOutput(raw)
	}
	s.trigger = out
	s.outputs[trigger.Key] = out
	for _, id := range s.g.order {
		n := s.g.nodes[id]
		if id != trigger.ID && s.isTrigger(n) {
			s.status[id] = StepSkipped
			s.skipOutgoing(id)
		}
	}
	if s.inScope != nil {
		for _, id := range s.g.order {
			if _, decided := s.status[id]; !decided && id != trigger.ID && !s.inScope[id] {
				s.status[id] = StepSkipped
				s.skipOutgoing(id)
			}
		}
	}
	port := s.def(trigger).DefaultPort(trigger)
	s.status[trigger.ID] = StepSuccess
	s.deliverPorts(trigger, []string{port})
	step := StepRecord{NodeID: trigger.ID, NodeKey: trigger.Key, Attempt: 1, Status: StepSuccess,
		StartedAt: now, FinishedAt: now, Output: out, Ports: []string{port}}
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: trigger.ID, Step: &step})
}

// collectReady appends runnable nodes to ready and marks skipped nodes until nothing changes.
// It walks the topological order, so a skip reaches all downstream nodes in one
// pass; the changed loop is only a safety net.
func (s *runState) collectReady(ready *[]string) {
	for changed := true; changed; {
		changed = false
		for _, id := range s.topo {
			if _, decided := s.status[id]; decided || s.queued[id] {
				continue
			}
			n := s.g.nodes[id]
			if s.isTrigger(n) {
				continue
			}
			incoming := s.g.incoming[id]
			if len(incoming) == 0 {
				s.markSkipped(n)
				changed = true
				continue
			}
			resolved, delivered := true, false
			for _, e := range incoming {
				switch s.edges[e] {
				case edgePending:
					resolved = false
				case edgeDelivered:
					delivered = true
				}
			}
			if !resolved {
				continue
			}
			if !delivered || n.Settings.Disabled {
				s.markSkipped(n)
				changed = true
				continue
			}
			s.queued[id] = true
			*ready = append(*ready, id)
		}
	}
}

func (s *runState) markSkipped(n *Node) {
	s.status[n.ID] = StepSkipped
	s.skipOutgoing(n.ID)
	now := s.e.services.Now()
	step := StepRecord{NodeID: n.ID, NodeKey: n.Key, Status: StepSkipped, StartedAt: now, FinishedAt: now}
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: n.ID, Step: &step})
}

func (s *runState) launch(ctx context.Context, id string, done chan<- nodeDone) {
	n := s.g.nodes[id]
	s.status[id] = StepRunning
	s.running++
	s.publish(RunEvent{Type: EventStepStarted, NodeID: id})
	in := ExecInput{
		Flow:   s.req.Flow,
		Node:   n,
		Inputs: s.inputsFor(id),
		Env:    s.env(),
		Run: RunInfo{ID: s.req.RunID, FlowID: s.req.Flow.ID, Mode: s.req.Mode,
			Revision: s.req.Revision, StartedAt: s.started},
		Services: s.e.services,
		Logger:   s.e.logger.With("flow", s.req.Flow.ID, "run", s.req.RunID, "node", n.Key),
	}
	def := s.def(n)
	// The worker touches only these locals; runNode recovers from any panic, so
	// a result is always sent.
	e := s.e
	go func() { done <- e.runNode(ctx, def, n, in) }()
}

func (s *runState) complete(d nodeDone) {
	n := s.g.nodes[d.nodeID]
	def := s.def(n)
	if d.err == nil && !d.cancelled && def != nil {
		// A port the node does not declare would leave its branch pending
		// silently, so it fails the node like any other error.
		if port, bad := undeclaredPort(def, n, d.result.Ports); bad {
			d.err = &NodeError{Code: "FLOW_PORT_INVALID", Message: "node returned unknown output " + quoteForError(port)}
			d.step.Status = StepError
			d.step.ErrorCode, d.step.ErrorMessage = d.err.Code, d.err.Message
			d.step.Output, d.step.OutputTruncated, d.step.ItemCount = nil, false, 0
		}
	}
	step := d.step
	switch {
	case d.cancelled:
		s.status[n.ID] = StepCancelled
		s.skipOutgoing(n.ID)
	case d.err != nil:
		s.status[n.ID] = StepError
		errOut := map[string]any{"error": map[string]any{"code": d.err.Code, "message": d.err.Message}}
		switch n.Settings.OnError {
		case ErrorContinue:
			// The default port: "true" for logic.if, "case_1" for logic.switch.
			s.outputs[n.Key] = errOut
			step.Output = errOut
			step.Ports = []string{defaultPortOf(def, n)}
			s.deliverPorts(n, step.Ports)
		case ErrorPort:
			s.outputs[n.Key] = errOut
			step.Output = errOut
			step.Ports = []string{PortError}
			s.deliverPorts(n, step.Ports)
		default:
			s.skipOutgoing(n.ID)
			if s.failure == nil {
				s.failure = d.err
				s.failedID = n.ID
			}
		}
	default:
		s.status[n.ID] = StepSuccess
		s.outputs[n.Key] = d.result.Output
		ports := d.result.Ports
		if len(ports) == 0 {
			if p := defaultPortOf(def, n); p != "" {
				ports = []string{p}
			}
		}
		step.Ports = ports
		s.deliverPorts(n, ports)
		if d.result.Stop != nil && s.stop == nil {
			s.stop = d.result.Stop
		}
	}
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: n.ID, Step: &step})
}

// undeclaredPort returns the first of ports that def does not declare for n.
func undeclaredPort(def *NodeDef, n *Node, ports []string) (string, bool) {
	if len(ports) == 0 {
		return "", false
	}
	declared := def.OutputPorts(n)
	for _, p := range ports {
		if !containsString(declared, p) {
			return p, true
		}
	}
	return "", false
}

func defaultPortOf(def *NodeDef, n *Node) string {
	if def == nil {
		return PortOut
	}
	return def.DefaultPort(n)
}

func (s *runState) deliverPorts(n *Node, ports []string) {
	for _, e := range s.g.outgoing[n.ID] {
		if containsString(ports, e.Source.Port) {
			s.edges[e] = edgeDelivered
		} else {
			s.edges[e] = edgeSkipped
		}
	}
}

func (s *runState) skipOutgoing(id string) {
	for _, e := range s.g.outgoing[id] {
		s.edges[e] = edgeSkipped
	}
}

func (s *runState) inputsFor(id string) []NodeInput {
	var inputs []NodeInput
	for _, e := range s.g.incoming[id] {
		if s.edges[e] != edgeDelivered {
			continue
		}
		src := s.g.nodes[e.Source.Node]
		inputs = append(inputs, NodeInput{NodeID: src.ID, Key: src.Key, Port: e.Source.Port, Output: s.outputs[src.Key]})
	}
	return inputs
}

func (s *runState) env() *Env {
	roots := make(map[string]any, len(s.outputs)+3)
	for k, v := range s.outputs {
		roots[k] = v
	}
	roots["trigger"] = s.trigger
	roots["run"] = map[string]any{
		"id": s.req.RunID, "started_at": s.started.Format(time.RFC3339),
		"mode": string(s.req.Mode), "revision": float64(s.req.Revision),
	}
	roots["flow"] = map[string]any{"id": s.req.Flow.ID, "name": s.req.Flow.Name}
	return &Env{Roots: roots, Location: s.e.services.Loc()}
}

// closeOpenNodes gives every node that has no final status yet, queued or
// waiting for its inputs, a step: skipped after a stop node, cancelled
// otherwise (failure, cancel or timeout). It walks the topological order, so the
// steps and events are deterministic.
func (s *runState) closeOpenNodes() {
	status := StepCancelled
	if s.stop != nil {
		status = StepSkipped
	}
	for _, id := range s.topo {
		if _, decided := s.status[id]; decided {
			continue
		}
		n := s.g.nodes[id]
		s.status[id] = status
		now := s.e.services.Now()
		step := StepRecord{NodeID: id, NodeKey: n.Key, Status: status, StartedAt: now, FinishedAt: now}
		if status == StepCancelled {
			step.ErrorCode, step.ErrorMessage = "FLOW_CANCELLED", "the run was stopped"
		}
		s.steps = append(s.steps, step)
		s.publish(RunEvent{Type: EventStepFinished, NodeID: id, Step: &step})
	}
}

func (s *runState) conclude(parent, ctx context.Context) RunResult {
	s.closeOpenNodes()
	switch {
	case s.stop != nil:
		if s.stop.Status == RunError {
			msg := s.stop.Message
			if msg == "" {
				msg = "the flow was stopped with an error"
			}
			return s.finish(RunError, "FLOW_STOPPED", msg, "")
		}
		return s.finish(RunSuccess, "", "", "")
	case s.failure != nil:
		label := s.failedID
		if n := s.g.nodes[s.failedID]; n != nil {
			label = n.Label
			if label == "" {
				label = n.Key
			}
		}
		return s.finish(RunError, s.failure.Code, truncateRunes(label, maxErrorLabelRunes)+": "+s.failure.Message, s.failedID)
	case parent.Err() != nil:
		return s.finish(RunCancelled, "FLOW_CANCELLED", "the run was cancelled", "")
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return s.finish(RunError, "FLOW_RUN_TIMEOUT", "the run took longer than its time limit", "")
	}
	return s.finish(RunSuccess, "", "", "")
}

// finish publishes run_finished and returns the result. The message is capped
// here too: it may carry a label prefix or a stop message from a custom node.
func (s *runState) finish(status RunStatus, code, msg, nodeID string) RunResult {
	msg = truncateRunes(msg, maxErrorMessageRunes)
	end := s.e.services.Now()
	res := RunResult{
		Status: status, ErrorCode: code, ErrorMessage: msg, ErrorNodeID: nodeID,
		StartedAt: s.started, FinishedAt: end, DurationMS: end.Sub(s.started).Milliseconds(),
		Steps: s.steps, Outputs: s.outputs,
	}
	s.publish(RunEvent{Type: EventRunFinished, Run: &RunSummary{Status: status, ErrorCode: code, ErrorMessage: msg, DurationMS: res.DurationMS}})
	return res
}
