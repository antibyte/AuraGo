package flows

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// changeCountFlow is a manual trigger followed by a chain of n set steps.
func changeCountFlow(n int) *Flow {
	b := newFlow("Zaehlen")
	prev := b.node("start", TypeTriggerManual, nil)
	for i := 0; i < n; i++ {
		id := b.node(fmt.Sprintf("s%d", i), TypeSet, map[string]any{"fields": []any{map[string]any{"name": "x", "value": fmt.Sprint(i)}}})
		b.edge(prev, PortOut, id)
		prev = id
	}
	return b.build()
}

func TestCountChangesNeverPublished(t *testing.T) {
	if got, ok := CountChanges(nil, changeCountFlow(3)); ok {
		t.Fatalf("CountChanges without a live version = %+v, want none", got)
	}
}

// CountChanges and DiffFlows come from one counting: the button and the publish dialog agree.
func TestCountChangesEqualsTheDiffCounts(t *testing.T) {
	live := changeCountFlow(4)
	draft, err := live.Clone()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		edit  func(f *Flow)
		steps int
		conns int
	}{
		{"unchanged", func(f *Flow) {}, 0, 0},
		{"a node moved is no change", func(f *Flow) { f.Nodes[2].Position = Point{X: 900, Y: 40} }, 0, 0},
		{"a step changed", func(f *Flow) {
			f.Nodes[2].Params = map[string]any{"fields": []any{map[string]any{"name": "x", "value": "neu"}}}
		}, 1, 0},
		{"a step removed with its connection", func(f *Flow) {
			f.Nodes = f.Nodes[:len(f.Nodes)-1]
			f.Edges = f.Edges[:len(f.Edges)-1]
		}, 1, 1},
		{"a step added and connected", func(f *Flow) {
			id := testNodeID(90)
			f.Nodes = append(f.Nodes, Node{ID: id, Key: "neu", Type: TypeSet, TypeVersion: 1, Label: "neu", Params: map[string]any{}})
			f.Edges = append(f.Edges, Edge{ID: "e_90", Source: PortRef{Node: f.Nodes[1].ID, Port: PortOut}, Target: PortRef{Node: id, Port: PortIn}})
		}, 1, 1},
	}
	for _, c := range cases {
		doc, err := draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		c.edit(doc)
		got, ok := CountChanges(live, doc)
		diff := DiffFlows(live, doc)
		if !ok || got.Steps != diff.AddedNodes+diff.ChangedNodes+diff.RemovedNodes || got.Connections != diff.ChangedEdges {
			t.Fatalf("%s: CountChanges = %+v %v, DiffFlows = %+v", c.name, got, ok, diff)
		}
		if got.Steps != c.steps || got.Connections != c.conns {
			t.Fatalf("%s: CountChanges = %+v, want %d steps and %d connections", c.name, got, c.steps, c.conns)
		}
	}
}

// The count of a stored flow equals the counts of the publish preview of the same draft.
func TestCountChangesEqualsThePublishPreview(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	rec := mustCreate(t, s, changeCountFlow(3))
	if _, _, err := s.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	live, err := s.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited := changeCountFlow(5) // two more steps and two more connections
	edited.Nodes[1].Label = "Anders"
	if _, _, err := s.SaveDraft(ctx, rec.ID, edited, live.DraftRevision); err != nil {
		t.Fatal(err)
	}
	cur, err := s.GetFlow(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PublishPreview(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := CountChanges(cur.Live, cur.Draft)
	want := preview.Diff.AddedNodes + preview.Diff.ChangedNodes + preview.Diff.RemovedNodes
	if !ok || got.Steps != want || got.Connections != preview.Diff.ChangedEdges || got.Steps == 0 || got.Connections == 0 {
		t.Fatalf("CountChanges = %+v %v, preview diff = %+v", got, ok, preview.Diff)
	}
}

// BenchmarkCountChanges measures one count on the largest flow the service accepts
// (MaxNodes steps and MaxEdges connections, every step changed), which is what each save
// of such a flow pays. Run: go test ./internal/flows -run '^$' -bench CountChanges -benchmem
func BenchmarkCountChanges(b *testing.B) {
	live := changeCountFlow(MaxNodes - 1)
	for i := 0; len(live.Edges) < MaxEdges; i++ {
		live.Edges = append(live.Edges, Edge{ID: fmt.Sprintf("x_%d", i), Source: PortRef{Node: live.Nodes[1+i%(MaxNodes-2)].ID, Port: "alt"}, Target: PortRef{Node: live.Nodes[2+i%(MaxNodes-2)].ID, Port: PortIn}})
	}
	draft, err := live.Clone()
	if err != nil {
		b.Fatal(err)
	}
	for i := range draft.Nodes {
		draft.Nodes[i].Label += "!"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := CountChanges(live, draft); !ok {
			b.Fatal("no count")
		}
	}
}

// TestSaveCostOfTheChangeCount measures what each save pays for the count: reading the stored
// flow (draft and live document) and counting. It logs a typical flow (25 steps) and the longest
// chain the service accepts (MaxNodes steps). The ceiling is far above what either takes, so
// only a real regression fails.
func TestSaveCostOfTheChangeCount(t *testing.T) {
	s, _ := newServiceFixture(t, nil)
	ctx := context.Background()
	for _, steps := range []int{25, MaxNodes - 1} {
		rec, err := s.CreateFlow(ctx, CreateRequest{Import: changeCountFlow(steps)})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.Publish(ctx, rec.ID, rec.DraftRevision); err != nil {
			t.Fatalf("Publish of %d steps: %v", steps, err)
		}
		const rounds = 20
		begin := time.Now()
		for i := 0; i < rounds; i++ {
			cur, err := s.GetFlow(ctx, rec.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := CountChanges(cur.Live, cur.Draft); !ok {
				t.Fatal("no count")
			}
		}
		each := time.Since(begin) / rounds
		t.Logf("GetFlow + CountChanges on %d steps: %s per save", steps+1, each)
		if each > time.Second {
			t.Fatalf("one count costs %s per save on %d steps, want well below 1s", each, steps+1)
		}
	}
}
