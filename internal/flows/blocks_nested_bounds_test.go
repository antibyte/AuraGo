package flows

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestBlockNestedExpandedEdgesAndBytesCountRepeatedInvocations(t *testing.T) {
	for _, dimension := range []string{"edges", "bytes"} {
		t.Run(dimension, func(t *testing.T) {
			ctx := context.Background()
			s, _ := newServiceFixture(t, nil)
			leaf := publishedGreetingBlock(t, s)
			doc, err := leaf.Draft.Clone()
			if err != nil {
				t.Fatal(err)
			}
			if dimension == "edges" {
				input, output := doc.Nodes[0], doc.Nodes[1]
				doc.Nodes = []Node{input}
				for i := 0; i < 61; i++ {
					doc.Nodes = append(doc.Nodes, Node{ID: NewNodeID(), Key: fmt.Sprintf("value_%d", i), Type: TypeSet, TypeVersion: 1,
						Params: map[string]any{"keep_input": false, "fields": []any{map[string]any{"name": "value", "type": "text", "value": "ok"}}}})
				}
				doc.Nodes = append(doc.Nodes, output)
				doc.Edges = nil
				for i := range doc.Nodes {
					for j := i + 1; j < len(doc.Nodes); j++ {
						doc.Edges = append(doc.Edges, Edge{ID: NewEdgeID(), Source: PortRef{Node: doc.Nodes[i].ID, Port: PortOut}, Target: PortRef{Node: doc.Nodes[j].ID, Port: PortIn}})
					}
				}
			} else {
				doc.Nodes[1].Params["values"] = map[string]any{"greeting": strings.Repeat("x", MaxDocumentBytes/2+4096)}
			}
			revision, issues, err := s.SaveBlockDraft(ctx, leaf.ID, doc, leaf.DraftRevision)
			if err != nil {
				t.Fatalf("save valid %s fixture: %v %+v", dimension, err, issues)
			}
			leaf, issues, err = s.PublishBlock(ctx, leaf.ID, revision)
			if err != nil {
				t.Fatalf("publish valid %s fixture: %v %+v", dimension, err, issues)
			}
			middle := nestedGreeting(t, s, leaf)
			parent, callID := blockParent(t, s, middle, "Expanded "+dimension)
			if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); err != nil {
				t.Fatalf("one nested invocation must fit: %v", err)
			}
			second := *parent.Draft.NodeByID(callID)
			second.ID, second.Key = NewNodeID(), "second_call"
			parent.Draft.Nodes = append(parent.Draft.Nodes, second)
			if len(parent.Draft.Nodes)+2*(len(middle.Draft.Nodes)+len(leaf.Draft.Nodes)) >= MaxNodes {
				t.Fatal("fixture accidentally exercises the node limit")
			}
			expandedEdges := len(parent.Draft.Edges) + 2*(len(middle.Draft.Edges)+len(leaf.Draft.Edges))
			expandedBytes := 0
			for i, frame := range []*Flow{parent.Draft, middle.Draft, leaf.Draft} {
				encoded, err := frame.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				if i == 0 {
					expandedBytes += len(encoded)
				} else {
					expandedBytes += 2 * len(encoded)
				}
			}
			if (expandedEdges > MaxEdges) != (dimension == "edges") || (expandedBytes > MaxDocumentBytes) != (dimension == "bytes") {
				t.Fatalf("fixture does not isolate %s: edges=%d bytes=%d", dimension, expandedEdges, expandedBytes)
			}
			if _, err := prepareBlockPlan(ctx, s.store, parent.Draft, s.services); !errors.Is(err, ErrBlockInvalid) {
				t.Fatalf("repeated nested %s accepted despite expanded limit: %v", dimension, err)
			}
		})
	}
}
