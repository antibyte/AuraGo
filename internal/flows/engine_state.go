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

// nodeInfo is what a run needs from a node's definition. It is computed once
// at run start, which pins one definition per node for the whole run and runs
// definition hooks such as OutputsFunc only once, under a recover.
type nodeInfo struct {
	def *NodeDef // nil when the type is unknown
	// ports are the declared output ports, "error" included when the node uses an error port.
	ports []string
	// defaultPort is the first non-error port, "" when there is none, and "out"
	// for an unknown type.
	defaultPort string
	// continuePort is the port a failure is delivered on under on_error "continue":
	// the definition's ContinuePort when the node has it, else defaultPort.
	continuePort string
	// refusal is the failure the node gets instead of running, nil for a node that runs
	// (see secretTargetRefusals).
	refusal *NodeError
	// hasEffects records that the node declares an outward effect (declaresEffects).
	hasEffects bool
}

// runState is owned by the coordinator goroutine of one run.
type runState struct {
	e        *Engine
	req      RunRequest
	emit     EventSink
	seq      int
	g        *graph
	topo     []string // all node ids in topological order (document order breaks ties)
	info     map[string]nodeInfo
	started  time.Time
	launched map[string]time.Time
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
	// outputBytes is the encoded size of all accepted outputs (see MaxRunOutputBytes).
	outputBytes int
	// storedBytes is what the run log keeps of the run so far (MaxStoredRunBytes).
	storedBytes int
	// interrupted records that the end of the run's context cut a node short:
	// a node was cancelled or abandoned, or never got to run.
	interrupted bool
	// gate is the run's slot gate (A7); nil without a runner slot.
	gate *slotGate
}

// newRunState prepares the state of one run; run builds the graph after
// checking the flow.
func newRunState(e *Engine, req RunRequest, emit EventSink) *runState {
	return &runState{
		e: e, req: req, emit: emit,
		launched: map[string]time.Time{},
		edges:    map[*Edge]edgeState{},
		status:   map[string]StepStatus{},
		queued:   map[string]bool{},
		outputs:  map[string]map[string]any{},
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
	return s.info[n.ID].def
}

func (s *runState) isTrigger(n *Node) bool {
	def := s.def(n)
	return def != nil && def.Trigger
}

// checkFlow rejects a flow the engine cannot run: a missing one, one over the
// document limits, one with a loop (a loop is only a warning for drafts, and
// its nodes would never become ready) and one whose node definitions panic.
// On success it has built the graph, the topological order and the node infos,
// which also hold the nodes the run refuses to execute (unchecked data deciding
// where a secret goes; the lint runs once here, not per node or attempt).
func (s *runState) checkFlow() (code, msg, nodeID string) {
	f := s.req.Flow
	if f == nil {
		return "FLOW_INVALID", "the run has no flow", ""
	}
	if len(f.Nodes) > MaxNodes || len(f.Edges) > MaxEdges {
		return "FLOW_TOO_LARGE", fmt.Sprintf("the flow has %d nodes and %d connections; the limits are %d and %d",
			len(f.Nodes), len(f.Edges), MaxNodes, MaxEdges), ""
	}
	s.g = buildGraph(f)
	order, cyclic := s.g.topoOrder()
	if len(cyclic) > 0 {
		return "FLOW_CYCLE", "the flow contains a loop", ""
	}
	s.topo = order
	s.info = make(map[string]nodeInfo, len(order))
	for _, id := range order {
		info, ne := describeNode(s.e.reg, s.g.nodes[id])
		if ne != nil {
			return ne.Code, ne.Message, id
		}
		info.hasEffects = declaresEffects(info.def, s.g.nodes[id])
		s.info[id] = info
	}
	for id, ne := range secretTargetRefusals(f, s.e.reg) {
		if info, ok := s.info[id]; ok {
			info.refusal = ne
			s.info[id] = info
		}
	}
	return "", "", ""
}

// declaresEffects reports whether n's definition names an outward effect (NodeDef.EffectsOf). A
// hook that panics counts as one: "effect may have happened" is the safe side.
func declaresEffects(def *NodeDef, n *Node) bool {
	if def == nil {
		return false
	}
	var fx []Effect
	if catchPanic(func() { fx = def.EffectsOf(n) }) != nil {
		return true
	}
	return len(fx) > 0
}

// describeNode looks up n's definition and evaluates its port hooks. A panic in
// a hook becomes FLOW_NODE_PANIC naming the node.
func describeNode(reg *Registry, n *Node) (info nodeInfo, ne *NodeError) {
	defer func() {
		if r := recover(); r != nil {
			ne = &NodeError{Code: "FLOW_NODE_PANIC", Message: truncateRunes(
				fmt.Sprintf("node %s: its definition crashed: %v", quoteForError(n.Key), r), maxErrorMessageRunes)}
		}
	}()
	def, _ := reg.Lookup(n.Type)
	if def == nil {
		return nodeInfo{defaultPort: PortOut, continuePort: PortOut}, nil
	}
	info = nodeInfo{def: def, ports: def.OutputPorts(n)}
	for _, p := range info.ports {
		if p != PortError {
			info.defaultPort = p
			break
		}
	}
	info.continuePort = info.defaultPort
	if def.ContinuePort != "" && def.ContinuePort != PortError && containsString(info.ports, def.ContinuePort) {
		info.continuePort = def.ContinuePort
	}
	return info, nil
}

func (s *runState) run(parent context.Context) RunResult {
	s.started = s.e.services.Now()
	s.publish(RunEvent{Type: EventRunStarted, Run: &RunSummary{Status: RunRunning}})
	if code, msg, nodeID := s.checkFlow(); code != "" {
		return s.finish(RunError, code, msg, nodeID)
	}
	timeout := s.req.Timeout
	if limit := MaxRunSecondsLimit * time.Second; timeout > limit {
		timeout = limit
	}
	if timeout <= 0 {
		// Clamp before multiplying: a huge value would overflow the duration.
		timeout = time.Duration(clampInt(s.req.Flow.Settings.MaxRunSeconds, 0, MaxRunSecondsLimit)) * time.Second
	}
	if timeout <= 0 {
		timeout = DefaultMaxRunSeconds * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	s.gate = newSlotGate(s.req.Slot, ctx)

	trigger := s.g.nodes[s.req.TriggerNode]
	if trigger == nil || !s.isTrigger(trigger) {
		return s.finish(RunError, "FLOW_TRIGGER_INVALID", "the trigger node does not exist", "")
	}
	if trigger.Settings.Disabled {
		return s.finish(RunError, "FLOW_TRIGGER_DISABLED", "the trigger node is disabled", trigger.ID)
	}
	if s.req.OnlyNode != "" {
		if s.g.nodes[s.req.OnlyNode] == nil {
			return s.finish(RunError, IssueNodeNotFound, "the node to test does not exist", "")
		}
		s.inScope = s.g.ancestors(s.req.OnlyNode)
		s.inScope[s.req.OnlyNode] = true
	}
	s.fireTrigger(trigger)

	// Buffered for every worker that can be in flight, so a worker never blocks
	// on its send, even after the coordinator gave up waiting for it.
	done := make(chan nodeDone, s.e.parallel)
	ctxDone := ctx.Done()
	var grace *time.Timer
	var giveUp <-chan time.Time
	// stopCut ends the grace that steps still running get after a stop node (E3); then the rest
	// is cancelled. stopping records that the grace was started.
	var stopCut <-chan time.Time
	stopping := false
	var ready []string
	s.collectReady(&ready)
loop:
	for {
		for len(ready) > 0 && s.running < s.e.parallel && s.failure == nil && s.stop == nil && ctx.Err() == nil {
			id := ready[0]
			ready = ready[1:]
			s.launch(ctx, id, done)
		}
		if s.running == 0 {
			break
		}
		s.gate.idle()
		select {
		case d := <-done:
			s.running--
			s.gate.received()
			s.complete(ctx, d)
			switch {
			case s.failure != nil:
				cancel()
			case s.stop != nil && !stopping && s.running > 0:
				// A stop ends the run, but steps that are running may be sending something right
				// now: they get the abandon grace to finish before the rest is cancelled.
				stopping = true
				stopCut = s.e.services.clock().After(s.e.abandonAfter)
			}
			s.collectReady(&ready)
		case <-stopCut:
			stopCut = nil
			cancel()
		case <-ctxDone:
			// Running nodes get a real-time grace to return after the run ended.
			ctxDone = nil
			grace = time.NewTimer(s.e.abandonAfter)
			giveUp = grace.C
		case <-giveUp:
			s.abandonRunning(ctx, done)
			break loop
		}
	}
	if grace != nil {
		grace.Stop()
	}
	return s.conclude(parent, ctx)
}

// fireTrigger records the trigger's output and decides the nodes that cannot
// run in this run. Trigger data that cannot be encoded, or that makes the
// trigger output larger than MaxOutputBytes, is replaced with {} and logged.
// The step keeps a preview of a large output; templates see all of it.
func (s *runState) fireTrigger(trigger *Node) {
	now := s.e.services.Now()
	data := s.req.TriggerData
	if data == nil {
		data = map[string]any{}
	}
	raw := map[string]any{"type": s.req.TriggerType, "node": trigger.Key, "fired_at": now.Format(time.RFC3339), "data": data}
	out, encoded, err := normalizeOutput(raw)
	if err != nil || out == nil {
		s.e.logger.Warn("flows: trigger data replaced with an empty object", "flow", s.req.Flow.ID, "run", s.req.RunID,
			"bytes", len(encoded), "limit", MaxOutputBytes, "error", err)
		raw["data"] = map[string]any{}
		out, encoded, _ = normalizeOutput(raw)
	}
	s.trigger = out
	s.outputs[trigger.Key] = out
	s.outputBytes += len(encoded)
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
	var ports []string
	if p := s.info[trigger.ID].defaultPort; p != "" {
		ports = []string{p}
	}
	s.status[trigger.ID] = StepSuccess
	s.deliverPorts(trigger, ports)
	stored, truncated := storedOutput(out, encoded)
	s.storedBytes += storedSize(encoded, truncated)
	step := StepRecord{NodeID: trigger.ID, NodeKey: trigger.Key, Attempt: 1, Status: StepSuccess,
		StartedAt: now, FinishedAt: now, Output: stored, OutputTruncated: truncated, Ports: ports}
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
	started := s.e.services.Now()
	s.launched[id] = started
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
	s.gate.launched()
	if s.gate != nil {
		in.Slot = s.gate // never a typed nil
	}
	def, refusal := s.def(n), s.info[id].refusal
	// The worker touches only these locals. runNode recovers from any panic, and
	// the deferred send of the prefilled fallback also covers runtime.Goexit, so
	// exactly one result is always sent; done has room for it.
	e := s.e
	go func() {
		d := exitedNode(n, started)
		defer func() { done <- d }()
		d = e.runNode(ctx, def, n, in, refusal)
	}()
}

// abandonRunning gives up on the nodes that did not return within the grace
// after the run's context ended. Results that already arrived are recorded
// first; the remaining running nodes are closed as cancelled. Their workers
// keep running in the background; their late results land in the buffer of
// done and are never read.
func (s *runState) abandonRunning(ctx context.Context, done <-chan nodeDone) {
	for drained := false; !drained && s.running > 0; {
		select {
		case d := <-done:
			s.running--
			s.complete(ctx, d)
		default:
			drained = true
		}
	}
	for _, id := range s.topo {
		if s.status[id] != StepRunning {
			continue
		}
		n := s.g.nodes[id]
		s.e.logger.Warn("flows: node did not stop after the run ended; abandoned", "flow", s.req.Flow.ID,
			"run", s.req.RunID, "node", n.Key, "grace", s.e.abandonAfter)
		s.status[id] = StepCancelled
		s.skipOutgoing(id)
		s.interrupted = true
		now := s.e.services.Now()
		step := StepRecord{NodeID: id, NodeKey: n.Key, Status: StepCancelled,
			StartedAt: s.launched[id], FinishedAt: now, DurationMS: now.Sub(s.launched[id]).Milliseconds(),
			ErrorCode: "FLOW_NODE_ABANDONED", ErrorMessage: "the node did not stop after the run ended",
			EffectUncertain: s.info[id].hasEffects}
		s.steps = append(s.steps, step)
		s.publish(RunEvent{Type: EventStepFinished, NodeID: id, Step: &step})
	}
	s.running = 0
}

// rejectResult returns why a successful result cannot be accepted: a port the
// node does not declare (its branch would stay pending silently), the error
// port (reserved for failures) or outputs that would push the run over
// MaxRunOutputBytes.
func (s *runState) rejectResult(info nodeInfo, d nodeDone) *NodeError {
	for _, p := range d.result.Ports {
		if p == PortError {
			return &NodeError{Code: "FLOW_PORT_INVALID", Message: "node returned the error output for a successful result"}
		}
		if !containsString(info.ports, p) {
			return &NodeError{Code: "FLOW_PORT_INVALID", Message: "node returned unknown output " + quoteForError(p)}
		}
	}
	if s.outputBytes+d.size > MaxRunOutputBytes {
		return &NodeError{Code: "FLOW_RUN_OUTPUT_TOO_LARGE", Message: fmt.Sprintf("the run's outputs exceed %d bytes in total", MaxRunOutputBytes)}
	}
	return nil
}

// complete applies a node's result. ctx is the run's context.
func (s *runState) complete(ctx context.Context, d nodeDone) {
	n := s.g.nodes[d.nodeID]
	info := s.info[n.ID]
	if d.err == nil && !d.cancelled {
		if ne := s.rejectResult(info, d); ne != nil {
			d.err = ne
			d.step.Status = StepError
			d.step.ErrorCode, d.step.ErrorMessage = ne.Code, ne.Message
			d.step.Output, d.step.OutputTruncated, d.step.ItemCount = nil, false, 0
		}
	}
	step := d.step
	switch {
	case d.cancelled:
		s.status[n.ID] = StepCancelled
		s.skipOutgoing(n.ID)
		s.interrupted = true
		step.EffectUncertain = info.hasEffects
	case d.err != nil:
		s.status[n.ID] = StepError
		errOut := map[string]any{"error": map[string]any{"code": d.err.Code, "message": d.err.Message}}
		errorMode := n.Settings.OnError
		if info.def != nil && !info.def.AllowsErrorMode(errorMode) {
			// Imported or test-only drafts can bypass publish validation. Keep a
			// node's declared failure policy binding at runtime as well.
			errorMode = ErrorStop
		}
		switch errorMode {
		case ErrorContinue:
			// The continue port: the first output, except for a node that names a
			// fail-closed one ("false" for logic.if, "default" for logic.switch).
			s.outputs[n.Key] = errOut
			step.Output = errOut
			if info.continuePort != "" {
				step.Ports = []string{info.continuePort}
			}
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
		s.outputBytes += d.size
		ports := d.result.Ports
		if len(ports) == 0 && info.defaultPort != "" {
			ports = []string{info.defaultPort}
		}
		step.Ports = ports
		s.deliverPorts(n, ports)
		// The first terminal outcome wins: a stop after a failure, a cancel or
		// the timeout must not turn the run into a success.
		if d.result.Stop != nil && s.stop == nil && s.failure == nil && ctx.Err() == nil {
			s.stop = d.result.Stop
		}
	}
	step = s.chargeStored(step, d.stored)
	s.steps = append(s.steps, step)
	s.publish(RunEvent{Type: EventStepFinished, NodeID: n.ID, Step: &step})
}

// chargeStored counts what the run log keeps of a step against MaxStoredRunBytes. Once the run
// would pass it, the step keeps neither params nor output (storedOmitted, flags set); the step
// event carries the same, so the live view and the stored run agree.
func (s *runState) chargeStored(step StepRecord, size int) StepRecord {
	if size <= 0 {
		return step
	}
	if s.storedBytes+size <= MaxStoredRunBytes {
		s.storedBytes += size
		return step
	}
	if step.Params != nil {
		step.Params, step.ParamsTruncated = storedOmitted(), true
	}
	if step.Output != nil {
		step.Output, step.OutputTruncated = storedOmitted(), true
	}
	return step
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
		s.interrupted = true
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
	case s.stop != nil && s.stop.Status == RunError:
		msg := s.stop.Message
		if msg == "" {
			msg = "the flow was stopped with an error"
		}
		return s.finish(RunError, "FLOW_STOPPED", msg, "")
	case s.failure != nil:
		// A failure before any stop, or one of a step still running after a stop with success:
		// the stop does not hide it (E3).
		label := s.failedID
		if n := s.g.nodes[s.failedID]; n != nil {
			label = n.Label
			if label == "" {
				label = n.Key
			}
		}
		return s.finish(RunError, s.failure.Code, truncateRunes(label, maxErrorLabelRunes)+": "+s.failure.Message, s.failedID)
	case s.stop != nil:
		return s.finish(RunSuccess, "", "", "")
	case s.interrupted && parent.Err() != nil:
		return s.finish(RunCancelled, "FLOW_CANCELLED", "the run was cancelled", "")
	case s.interrupted && errors.Is(ctx.Err(), context.DeadlineExceeded):
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
