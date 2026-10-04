package flows

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestGraphAncestorsAndOrder(t *testing.T) {
	b := newFlow("Graph")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	bb := b.node("b", "test.echo", nil)
	c := b.node("c", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(tr, PortOut, bb)
	b.edge(a, PortOut, c)
	b.edge(bb, PortOut, c)
	g := buildGraph(b.build())
	anc := g.ancestors(c)
	if len(anc) != 3 || !anc[tr] || !anc[a] || !anc[bb] {
		t.Fatalf("ancestors(c) = %v", anc)
	}
	if len(g.ancestors(tr)) != 0 {
		t.Fatal("the trigger has no ancestors")
	}
	order, cyclic := g.topoOrder()
	if !reflect.DeepEqual(order, []string{tr, a, bb, c}) || cyclic != nil {
		t.Fatalf("topoOrder = %v, %v", order, cyclic)
	}
}

func TestGraphCycle(t *testing.T) {
	b := newFlow("Cycle")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	bb := b.node("b", "test.echo", nil)
	c := b.node("c", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, bb)
	b.edge(bb, PortOut, a)
	b.edge(bb, PortOut, c)
	g := buildGraph(b.build())
	order, cyclic := g.topoOrder()
	if !reflect.DeepEqual(order, []string{tr}) || !reflect.DeepEqual(cyclic, []string{a, bb, c}) {
		t.Fatalf("topoOrder = %v, %v", order, cyclic)
	}
	if !g.onCycle(a) || !g.onCycle(bb) || g.onCycle(c) || g.onCycle(tr) {
		t.Fatal("onCycle mismatch")
	}
}

func TestGraphIgnoresDanglingEdges(t *testing.T) {
	b := newFlow("Dangling")
	tr := b.node("start", "test.trigger", nil)
	f := b.build()
	f.Edges = append(f.Edges, Edge{ID: "e_x", Source: PortRef{Node: tr, Port: PortOut}, Target: PortRef{Node: "n_missingx", Port: PortIn}})
	g := buildGraph(f)
	if len(g.outgoing[tr]) != 0 {
		t.Fatal("edges to missing nodes must be ignored")
	}
	if !containsString([]string{"a", "b"}, "b") || containsString(nil, "a") {
		t.Fatal("containsString mismatch")
	}
}

// bulkGraphFlow returns a flow with n nodes n_0 .. n_<n-1>. The flowBuilder's id
// space is too small for the sizes the scale tests need.
func bulkGraphFlow(n int) *Flow {
	f := &Flow{Schema: SchemaVersion, ID: "flow_test", Kind: KindFlow, Name: "Bulk", Nodes: make([]Node, n)}
	for i := range f.Nodes {
		f.Nodes[i] = Node{ID: fmt.Sprintf("n_%d", i)}
	}
	return f
}

func bulkEdge(f *Flow, from, to int) {
	f.Edges = append(f.Edges, Edge{
		ID:     fmt.Sprintf("e_%d", len(f.Edges)),
		Source: PortRef{Node: f.Nodes[from].ID, Port: PortOut},
		Target: PortRef{Node: f.Nodes[to].ID, Port: PortIn},
	})
}

func TestGraphTopoOrderTieBreaksByDocumentOrder(t *testing.T) {
	// Document order is c, a, b, root; all three become ready at the same time.
	f := &Flow{Nodes: []Node{{ID: "c"}, {ID: "a"}, {ID: "b"}, {ID: "root"}}}
	for _, to := range []string{"a", "b", "c"} {
		f.Edges = append(f.Edges, Edge{ID: "e_" + to, Source: PortRef{Node: "root", Port: PortOut}, Target: PortRef{Node: to, Port: PortIn}})
	}
	order, cyclic := buildGraph(f).topoOrder()
	if !reflect.DeepEqual(order, []string{"root", "c", "a", "b"}) || cyclic != nil {
		t.Fatalf("topoOrder = %v, %v", order, cyclic)
	}
}

// A node that becomes ready later still goes before the ready nodes that come
// after it in the document: r1 frees "late" (document position 0), which must be
// picked before r2 and r3.
func TestGraphTopoOrderPicksNewlyReadyEarlierNode(t *testing.T) {
	f := &Flow{
		Nodes: []Node{{ID: "late"}, {ID: "r1"}, {ID: "r2"}, {ID: "r3"}},
		Edges: []Edge{{ID: "e_1", Source: PortRef{Node: "r1", Port: PortOut}, Target: PortRef{Node: "late", Port: PortIn}}},
	}
	order, cyclic := buildGraph(f).topoOrder()
	if !reflect.DeepEqual(order, []string{"r1", "late", "r2", "r3"}) || cyclic != nil {
		t.Fatalf("topoOrder = %v, %v", order, cyclic)
	}
}

func TestGraphSelfLoopsAndDuplicateEdges(t *testing.T) {
	b := newFlow("Loops")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	c := b.node("c", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, a) // self-loop
	b.edge(a, PortOut, c)
	g := buildGraph(b.build())
	order, cyclic := g.topoOrder()
	if !reflect.DeepEqual(order, []string{tr}) || !reflect.DeepEqual(cyclic, []string{a, c}) {
		t.Fatalf("self-loop: topoOrder = %v, %v", order, cyclic)
	}
	if !g.onCycle(a) || g.onCycle(c) || g.onCycle(tr) {
		t.Fatal("self-loop: onCycle mismatch")
	}
	if anc := g.ancestors(a); len(anc) != 1 || !anc[tr] || anc[a] {
		t.Fatalf("self-loop: ancestors(a) = %v, want only the trigger", anc)
	}

	d := newFlow("Duplicates")
	dt := d.node("start", "test.trigger", nil)
	da := d.node("a", "test.echo", nil)
	db := d.node("b", "test.echo", nil)
	d.edge(dt, PortOut, da)
	d.edge(dt, PortOut, da) // duplicate
	d.edge(da, PortOut, db)
	d.edge(da, PortOut, db) // duplicate
	dg := buildGraph(d.build())
	order, cyclic = dg.topoOrder()
	if !reflect.DeepEqual(order, []string{dt, da, db}) || cyclic != nil {
		t.Fatalf("duplicates: topoOrder = %v, %v", order, cyclic)
	}
	if anc := dg.ancestors(db); len(anc) != 2 || !anc[dt] || !anc[da] {
		t.Fatalf("duplicates: ancestors(b) = %v", anc)
	}
	if dg.onCycle(da) {
		t.Fatal("duplicate edges are not a cycle")
	}
}

func TestGraphAncestorsOnCycleExcludeSelf(t *testing.T) {
	b := newFlow("Ring")
	a := b.node("a", "test.echo", nil)
	c := b.node("c", "test.echo", nil)
	b.edge(a, PortOut, c)
	b.edge(c, PortOut, a)
	g := buildGraph(b.build())
	if anc := g.ancestors(a); len(anc) != 1 || !anc[c] {
		t.Fatalf("ancestors(a) = %v, want only c", anc)
	}
}

func TestGraphDuplicateNodeIDsAndUnknownEndpoints(t *testing.T) {
	b := newFlow("Odd")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", nil)
	f := b.build()
	f.Nodes = append(f.Nodes, Node{ID: tr, Key: "dup"})
	f.Edges = append(f.Edges,
		Edge{ID: "e_in", Source: PortRef{Node: "n_missingx", Port: PortOut}, Target: PortRef{Node: a, Port: PortIn}},
		Edge{ID: "e_out", Source: PortRef{Node: a, Port: PortOut}, Target: PortRef{Node: "n_missingy", Port: PortIn}},
	)
	g := buildGraph(f)
	if len(g.order) != 2 || g.nodes[tr].Key != "start" {
		t.Fatalf("the first of duplicate ids must win: order=%v key=%q", g.order, g.nodes[tr].Key)
	}
	if len(g.incoming[a]) != 0 || len(g.outgoing[a]) != 0 {
		t.Fatal("edges with an unknown endpoint must be ignored in both directions")
	}
	order, cyclic := g.topoOrder()
	if !reflect.DeepEqual(order, []string{tr, a}) || cyclic != nil {
		t.Fatalf("topoOrder = %v, %v", order, cyclic)
	}
	if len(g.ancestors("n_missingx")) != 0 || g.onCycle("n_missingx") {
		t.Fatal("unknown ids have no ancestors and are not on a cycle")
	}
}

func TestGraphHelpersDoNotMutateFlow(t *testing.T) {
	b := newFlow("Readonly")
	tr := b.node("start", "test.trigger", nil)
	a := b.node("a", "test.echo", map[string]any{"value": "x"})
	c := b.node("c", "test.echo", nil)
	b.edge(tr, PortOut, a)
	b.edge(a, PortOut, c)
	b.edge(c, PortOut, a)
	f := b.build()
	before, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	g := buildGraph(f)
	g.ancestors(c)
	g.topoOrder()
	g.onCycle(a)
	after, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("graph helpers changed the flow:\nbefore %s\nafter  %s", before, after)
	}
}

// ParseFlow only caps the document size, so graphs far beyond MaxNodes can reach
// these helpers before validation rejects them. They must stay near-linear.
func TestGraphScalesLinearly(t *testing.T) {
	const wide = 30000
	f := bulkGraphFlow(wide + 2) // node 0 fans out to 1..wide, which all join in the last node
	for i := 1; i <= wide; i++ {
		bulkEdge(f, 0, i)
		bulkEdge(f, i, wide+1)
	}
	start := time.Now()
	g := buildGraph(f)
	order, cyclic := g.topoOrder()
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("topoOrder of a %d-node fan-out took %v", wide+2, elapsed)
	}
	if cyclic != nil || len(order) != wide+2 {
		t.Fatalf("topoOrder: %d ordered, cyclic %d", len(order), len(cyclic))
	}
	for i, id := range order {
		if id != f.Nodes[i].ID {
			t.Fatalf("order[%d] = %s, want document order %s", i, id, f.Nodes[i].ID)
		}
	}
	if anc := g.ancestors(f.Nodes[wide+1].ID); len(anc) != wide+1 {
		t.Fatalf("ancestors of the join = %d, want %d", len(anc), wide+1)
	}

	const chain = 100000
	c := bulkGraphFlow(chain)
	for i := 0; i+1 < chain; i++ {
		bulkEdge(c, i, i+1)
	}
	start = time.Now()
	cg := buildGraph(c)
	if anc := cg.ancestors(c.Nodes[chain-1].ID); len(anc) != chain-1 {
		t.Fatalf("chain ancestors = %d, want %d", len(anc), chain-1)
	}
	if cg.onCycle(c.Nodes[0].ID) {
		t.Fatal("an open chain has no cycle")
	}
	bulkEdge(c, chain-1, 0)
	cg = buildGraph(c)
	if !cg.onCycle(c.Nodes[0].ID) {
		t.Fatal("the closed chain is a cycle")
	}
	if order, cyclic := cg.topoOrder(); len(order) != 0 || len(cyclic) != chain {
		t.Fatalf("closed chain: %d ordered, %d cyclic", len(order), len(cyclic))
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("chain traversals took %v", elapsed)
	}
}
