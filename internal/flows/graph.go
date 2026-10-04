package flows

import "container/heap"

// positionHeap is a min-heap of node positions in document order. topoOrder
// uses it to pick the earliest ready node in O(log n) instead of scanning the
// whole ready list, which made the order quadratic on wide flows.
type positionHeap []int

func (h positionHeap) Len() int           { return len(h) }
func (h positionHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h positionHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *positionHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *positionHeap) Pop() any {
	old := *h
	last := len(old) - 1
	x := old[last]
	*h = old[:last]
	return x
}

// graph is an adjacency view of a flow. Edges to missing nodes are ignored.
// It only reads the flow it was built from; the maps hold pointers into the
// flow's Nodes and Edges slices.
type graph struct {
	nodes    map[string]*Node
	order    []string
	incoming map[string][]*Edge
	outgoing map[string][]*Edge
}

func buildGraph(f *Flow) *graph {
	g := &graph{
		nodes:    make(map[string]*Node, len(f.Nodes)),
		incoming: map[string][]*Edge{},
		outgoing: map[string][]*Edge{},
	}
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if _, dup := g.nodes[n.ID]; dup {
			continue
		}
		g.nodes[n.ID] = n
		g.order = append(g.order, n.ID)
	}
	for i := range f.Edges {
		e := &f.Edges[i]
		if g.nodes[e.Source.Node] == nil || g.nodes[e.Target.Node] == nil {
			continue
		}
		g.outgoing[e.Source.Node] = append(g.outgoing[e.Source.Node], e)
		g.incoming[e.Target.Node] = append(g.incoming[e.Target.Node], e)
	}
	return g
}

// ancestors returns all nodes with a path to id (id itself excluded).
func (g *graph) ancestors(id string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{id}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, e := range g.incoming[cur] {
			if !seen[e.Source.Node] {
				seen[e.Source.Node] = true
				stack = append(stack, e.Source.Node)
			}
		}
	}
	delete(seen, id)
	return seen
}

// topoOrder returns a topological order (document order breaks ties) and the
// nodes that could not be ordered because they are on or behind a cycle.
func (g *graph) topoOrder() ([]string, []string) {
	indeg := make(map[string]int, len(g.order))
	pos := make(map[string]int, len(g.order))
	// ready holds the document positions of the nodes without pending inputs.
	// Positions are pushed in ascending order here, which is a valid min-heap.
	ready := &positionHeap{}
	for i, id := range g.order {
		pos[id] = i
		indeg[id] = len(g.incoming[id])
		if indeg[id] == 0 {
			*ready = append(*ready, i)
		}
	}
	order := make([]string, 0, len(g.order))
	for ready.Len() > 0 {
		id := g.order[heap.Pop(ready).(int)]
		order = append(order, id)
		for _, e := range g.outgoing[id] {
			indeg[e.Target.Node]--
			if indeg[e.Target.Node] == 0 {
				heap.Push(ready, pos[e.Target.Node])
			}
		}
	}
	if len(order) == len(g.order) {
		return order, nil
	}
	done := make(map[string]bool, len(order))
	for _, id := range order {
		done[id] = true
	}
	var cyclic []string
	for _, id := range g.order {
		if !done[id] {
			cyclic = append(cyclic, id)
		}
	}
	return order, cyclic
}

// onCycle reports whether id can reach itself.
func (g *graph) onCycle(id string) bool {
	seen := map[string]bool{}
	var stack []string
	for _, e := range g.outgoing[id] {
		stack = append(stack, e.Target.Node)
	}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == id {
			return true
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		for _, e := range g.outgoing[cur] {
			stack = append(stack, e.Target.Node)
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
