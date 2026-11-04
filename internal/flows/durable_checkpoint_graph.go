package flows

// durableWaitType is deliberately unregistered until persistence and recovery ship together.
const durableWaitType = "logic.durable_wait"

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
		case durableWaitType:
			waits++
		case TypeIf, TypeSwitch, TypeMerge, TypeSet, TypeStop:
		default:
			return false
		}
	}
	return triggers == 1 && waits > 0
}
