package flows

// ChangeCount is what a draft differs from the live version by, in the two numbers the
// editor's primary button names: steps added, changed or removed, and connections added or
// removed. It is DiffFlows' own counting (countDiff), without the per-step details, so the
// button and the publish dialog always agree.
type ChangeCount struct {
	Steps       int `json:"steps"`
	Connections int `json:"connections"`
}

// CountChanges compares the live revision with the draft. ok is false for a flow that was never
// published (no live revision): there is nothing to count against.
func CountChanges(live, draft *Flow) (ChangeCount, bool) {
	if live == nil {
		return ChangeCount{}, false
	}
	d := countDiff(live, draft)
	return ChangeCount{Steps: d.AddedNodes + d.ChangedNodes + d.RemovedNodes, Connections: d.ChangedEdges}, true
}
