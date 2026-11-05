package flows

// durableWaitType is deliberately unregistered until persistence and recovery ship together.
const durableWaitType = "logic.durable_wait"
const durableApprovalType = "logic.durable_approval"

func containsDurableApproval(f *Flow) bool {
	if f == nil {
		return false
	}
	for _, node := range f.Nodes {
		if node.Type == durableApprovalType {
			return true
		}
	}
	return false
}

// durableGraphAllowed checks the entire immutable graph, including disabled and
// unreachable nodes. Callers must validate the document and execute with a
// trusted built-in registry, never arbitrary production registry callbacks.
func durableGraphAllowed(f *Flow) bool {
	if f == nil || len(f.Nodes) > MaxNodes || len(f.Edges) > MaxEdges {
		return false
	}
	triggers, waits := 0, 0
	for _, n := range f.Nodes {
		if n.TypeVersion != 1 {
			return false
		}
		switch n.Type {
		case TypeTriggerManual:
			triggers++
		case durableWaitType, durableApprovalType:
			waits++
		case TypeIf, TypeSwitch, TypeMerge, TypeSet, TypeStop:
		default:
			return false
		}
	}
	return triggers == 1 && waits > 0
}
