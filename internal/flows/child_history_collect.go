package flows

import (
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync"
)

// childHistoryBudget is shared by all outer calls of one run or simulation.
// Its separate limits never consume the engine's output or stored-value quota.
type childHistoryBudget struct {
	mu                             sync.Mutex
	bytes, metadata, frames, steps int
}

func (b *childHistoryBudget) reserve(bytes, metadata, frames, steps int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if bytes < 0 || metadata < 0 || bytes > maxChildHistoryBytes-b.bytes ||
		metadata > maxChildHistoryMetadataBytes-b.metadata || frames > maxChildHistoryFrames-b.frames || steps > maxChildHistorySteps-b.steps {
		return false
	}
	b.bytes += bytes
	b.metadata += metadata
	b.frames += frames
	b.steps += steps
	return true
}

// childCollector is private to one outer node. It remains reachable by late
// workers, which cannot append or reserve anything after the parent freezes it.
type childCollector struct {
	mu         sync.Mutex
	budget     *childHistoryBudget
	scrub      Scrubber
	frames     []ChildFrame
	bytes      int
	truncated  bool
	closed     bool
	afterError bool
}

func newChildCollector(scope *blockRunScope) *childCollector {
	if scope == nil {
		return nil
	}
	scope.mu.Lock()
	if scope.history == nil {
		scope.history = &childHistoryBudget{}
	}
	budget, scrub := scope.history, scope.historyScrubber
	scope.mu.Unlock()
	collector := &childCollector{budget: budget, scrub: scrub, frames: []ChildFrame{}, bytes: 1024}
	if !budget.reserve(1024, 1024, 0, 0) {
		collector.truncated = true
		collector.bytes = MaxChildTraceBytes
	}
	return collector
}

func (c *childCollector) open(parent, node string, attempt int, block *resolvedBlock, item *int) string {
	if c == nil || block == nil {
		return ""
	}
	if !childTextSafe(node, c.scrub) || !childTextSafe(block.Ref.FlowID, c.scrub) || !childTextSafe(hex.EncodeToString(block.Hash[:]), c.scrub) {
		c.mu.Lock()
		if !c.closed {
			c.truncated = true
		}
		c.mu.Unlock()
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ""
	}
	id := "i_" + strconv.Itoa(len(c.frames)+1)
	frame := ChildFrame{InvocationID: id, ParentInvocationID: parent, CallNodeID: node, CallAttempt: attempt,
		BlockID: block.Ref.FlowID, Revision: block.Ref.Revision, Hash: hex.EncodeToString(block.Hash[:]), ItemIndex: item, Steps: []ChildStep{}}
	data, err := json.Marshal(frame)
	// Reserve status/error growth up front so a later outcome cannot overflow.
	const outcomeReserve = 128
	if err != nil || len(data)+outcomeReserve+c.bytes > MaxChildTraceBytes || !c.budget.reserve(len(data)+1+outcomeReserve, len(data)+1+outcomeReserve, 1, 0) {
		c.truncated = true
		return ""
	}
	c.bytes += len(data) + 1 + outcomeReserve
	c.frames = append(c.frames, frame)
	return id
}

func (c *childCollector) update(id string, entered bool, status RunStatus, code string) {
	if c == nil || id == "" {
		return
	}
	if code != "" {
		code = safeBatchErrorCode(code)
		if !childTextSafe(code, c.scrub) {
			code = ""
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	for i := range c.frames {
		if c.frames[i].InvocationID == id {
			c.frames[i].Entered = c.frames[i].Entered || entered
			c.frames[i].Status = status
			if code != "" {
				c.frames[i].ErrorCode = code
			}
			return
		}
	}
}

func (c *childCollector) step(id string, step StepRecord) {
	if c == nil || id == "" {
		return
	}
	// Error-continue/port forwards the engine's already-shortened error text as
	// data. Once that occurs, subsequent values may derive from the shortened
	// text and cannot be safely re-scrubbed after later secret registration.
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	afterError := c.afterError
	if step.Status == StepError {
		c.afterError = true
	}
	c.mu.Unlock()
	if afterError {
		if step.Params != nil {
			step.Params, step.ParamsTruncated = childOmitted(), true
		}
		if step.Output != nil {
			step.Output, step.OutputTruncated = childOmitted(), true
		}
	} else if step.Status == StepError && step.Output != nil {
		step.Output, step.OutputTruncated = childOmitted(), true
	}
	if !childTextSafe(step.NodeID, c.scrub) || !childTextSafe(step.NodeKey, c.scrub) {
		c.mu.Lock()
		if !c.closed {
			c.truncated = true
		}
		c.mu.Unlock()
		return
	}
	for _, port := range step.Ports {
		if !childTextSafe(port, c.scrub) {
			step.Ports = nil
			break
		}
	}
	// Scrub/copy outside the collector lock, including any injected scrubber.
	if !childKeysSafe(step.Params, c.scrub) {
		step.Params, step.ParamsTruncated = childOmitted(), true
	} else {
		step.Params, step.ParamsTruncated = childBoundValue(step.Params, step.ParamsTruncated, c.scrub)
	}
	if !childKeysSafe(step.Output, c.scrub) {
		step.Output, step.OutputTruncated = childOmitted(), true
	} else {
		step.Output, step.OutputTruncated = childBoundValue(step.Output, step.OutputTruncated, c.scrub)
	}
	// The ordinary engine may already have shortened free text before this hook.
	step.ErrorMessage = ""
	if step.ErrorCode != "" {
		step.ErrorCode = safeBatchErrorCode(step.ErrorCode)
		if !childTextSafe(step.ErrorCode, c.scrub) {
			step.ErrorCode = ""
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	for i := range c.frames {
		if c.frames[i].InvocationID != id {
			continue
		}
		record := ChildStep{Ordinal: len(c.frames[i].Steps) + 1, Step: step}
		data, err := json.Marshal(record)
		metadata := record
		metadata.Step.Params, metadata.Step.Output = nil, nil
		metadataBytes, metadataErr := json.Marshal(metadata)
		if err != nil || metadataErr != nil || len(data)+c.bytes > MaxChildTraceBytes || !c.budget.reserve(len(data)+1, len(metadataBytes)+1, 0, 1) {
			c.truncated = true
			return
		}
		c.bytes += len(data) + 1
		c.frames[i].Steps = append(c.frames[i].Steps, record)
		return
	}
}

func (c *childCollector) freeze(incomplete bool) *ChildTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	state := ChildHistoryComplete
	if c.truncated {
		state = ChildHistoryTruncated
	}
	if incomplete {
		state = ChildHistoryIncomplete
	}
	steps := 0
	for _, frame := range c.frames {
		steps += len(frame.Steps)
	}
	summary := ChildHistorySummary{State: state, Truncated: c.truncated, RetainedInvocations: len(c.frames), RetainedSteps: steps}
	if state == ChildHistoryComplete {
		frames := len(c.frames)
		summary.TotalInvocations, summary.TotalSteps = &frames, &steps
	}
	return &ChildTrace{Schema: 1, History: summary, Frames: c.frames}
}

func (c *childCollector) hasFrames() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames) > 0
}

func (c *childCollector) hasShortError() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.afterError
}

func (c *childCollector) hasInvocation(parent, node string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, frame := range c.frames {
		if frame.ParentInvocationID == parent && frame.CallNodeID == node {
			return true
		}
	}
	return false
}
