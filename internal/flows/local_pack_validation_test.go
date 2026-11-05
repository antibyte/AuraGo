package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func cloneLocalPackForValidation(t *testing.T, pack LocalPack) LocalPack {
	t.Helper()
	raw, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	var cloned LocalPack
	if err := json.Unmarshal(raw, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func rehashLocalPackParentForValidation(t *testing.T, pack *LocalPack) {
	t.Helper()
	hash, err := localPackHash(pack.Parent.Flow)
	if err != nil {
		t.Fatal(err)
	}
	pack.Parent.Hash = hash
}

func rehashLocalPackBlockForValidation(t *testing.T, pack *LocalPack, index int) {
	t.Helper()
	hash, err := localPackHash(pack.Blocks[index].Flow)
	if err != nil {
		t.Fatal(err)
	}
	pack.Blocks[index].Hash = hash
}

func TestLocalPackValidationFailuresAreReadOnly(t *testing.T) {
	ctx := context.Background()
	s, bridge := newServiceFixture(t, nil)
	plainSource := localPackPlainFlow(t, s)
	plainPack, err := s.ExportLocalPack(ctx, plainSource.ID)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _, middle, _ := localPackNestedRevisionSource(t, s)
	nestedParent, _ := blockParent(t, s, middle, "Validation source")
	nestedPack, err := s.ExportLocalPack(ctx, nestedParent.ID)
	if err != nil {
		t.Fatal(err)
	}

	mutations := []struct {
		name string
		pack func() LocalPack
	}{
		{
			name: "parent document over 2 MiB",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, plainPack)
				bad.Parent.Flow.Description = strings.Repeat("p", MaxDocumentBytes)
				return bad
			},
		},
		{
			name: "block document over 2 MiB",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, nestedPack)
				bad.Blocks[0].Flow.Description = strings.Repeat("b", MaxDocumentBytes)
				return bad
			},
		},
		{
			name: "missing dependency",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, nestedPack)
				for i := range bad.Parent.Flow.Nodes {
					if bad.Parent.Flow.Nodes[i].Type == blockCallType {
						bad.Parent.Flow.Nodes[i].Params["flow_id"] = NewFlowID()
						bad.Parent.Flow.Nodes[i].Params["revision"] = float64(1)
					}
				}
				rehashLocalPackParentForValidation(t, &bad)
				return bad
			},
		},
		{
			name: "extra unreachable block",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, nestedPack)
				copy := bad.Blocks[0]
				copy.Flow, err = copy.Flow.Clone()
				if err != nil {
					t.Fatal(err)
				}
				copy.SourceID = NewFlowID()
				copy.Flow.ID = copy.SourceID
				copy.SourceRevision = 1
				copy.Hash, err = localPackHash(copy.Flow)
				if err != nil {
					t.Fatal(err)
				}
				bad.Blocks = append(bad.Blocks, copy)
				return bad
			},
		},
		{
			name: "cyclic block closure",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, nestedPack)
				leafIndex := -1
				for i := range bad.Blocks {
					if bad.Blocks[i].SourceID == leaf.ID {
						leafIndex = i
						break
					}
				}
				if leafIndex < 0 {
					t.Fatal("nested pack omitted leaf")
				}
				doc := bad.Blocks[leafIndex].Flow
				inputID, outputID := doc.Nodes[0].ID, doc.Nodes[1].ID
				call := Node{ID: NewNodeID(), Key: "cycle", Type: blockCallType, TypeVersion: 1,
					Params: map[string]any{"flow_id": middle.ID, "revision": float64(middle.LiveRevision), "inputs": map[string]any{"name": "{{block_input.data.name}}"}}}
				doc.Nodes = append(doc.Nodes, call)
				doc.Edges = []Edge{
					{ID: NewEdgeID(), Source: PortRef{Node: inputID, Port: PortOut}, Target: PortRef{Node: call.ID, Port: PortIn}},
					{ID: NewEdgeID(), Source: PortRef{Node: call.ID, Port: PortOut}, Target: PortRef{Node: outputID, Port: PortIn}},
				}
				doc.Normalize()
				rehashLocalPackBlockForValidation(t, &bad, leafIndex)
				return bad
			},
		},
		{
			name: "malformed typed block reference",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, nestedPack)
				for i := range bad.Parent.Flow.Nodes {
					if bad.Parent.Flow.Nodes[i].Type == blockCallType {
						bad.Parent.Flow.Nodes[i].Params["revision"] = "1"
					}
				}
				rehashLocalPackParentForValidation(t, &bad)
				return bad
			},
		},
		{
			name: "secret expression",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, plainPack)
				fields := bad.Parent.Flow.Nodes[1].Params["fields"].([]any)
				fields[0].(map[string]any)["value"] = "{{secrets.password}}"
				rehashLocalPackParentForValidation(t, &bad)
				return bad
			},
		},
		{
			name: "control character",
			pack: func() LocalPack {
				bad := cloneLocalPackForValidation(t, plainPack)
				bad.Parent.Flow.Description = "invalid\u0007text"
				rehashLocalPackParentForValidation(t, &bad)
				return bad
			},
		},
	}

	countsBefore := localPackDBCounts(t, s.store.db)
	missionsBefore := localPackMissionCount(bridge)
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			bad := tc.pack()
			if _, err := prepareLocalPack(ctx, bad); err == nil {
				t.Fatal("invalid pack passed service validation")
			}
			if _, err := s.PreviewLocalPack(ctx, bad); err == nil {
				t.Fatal("preview accepted invalid pack")
			}
			if _, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: bad}); err == nil {
				t.Fatal("import accepted invalid pack")
			}
			if got := localPackDBCounts(t, s.store.db); got != countsBefore {
				t.Fatalf("validation wrote SQL state: before=%v after=%v", countsBefore, got)
			}
			if got := localPackMissionCount(bridge); got != missionsBefore {
				t.Fatalf("validation created a mission: before=%d after=%d", missionsBefore, got)
			}
		})
	}
}

func TestLocalPackValidationRejectsAggregateOver18MiBBeforeWrites(t *testing.T) {
	ctx := context.Background()
	s, bridge := newServiceFixture(t, nil)
	source := localPackPlainFlow(t, s)
	pack, err := s.ExportLocalPack(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	blockSource := publishedGreetingBlock(t, s)
	const blocks = 12
	padding := strings.Repeat("x", MaxDocumentBytes*3/4)
	pack.Blocks = make([]LocalPackBlock, 0, blocks)
	for i := 0; i < blocks; i++ {
		doc, err := blockSource.Draft.Clone()
		if err != nil {
			t.Fatal(err)
		}
		doc.ID = fmt.Sprintf("flow_aggregate_%02d", i)
		doc.Nodes[1].Params["aggregate_padding"] = padding
		raw, err := doc.Marshal()
		if err != nil || len(raw) > MaxDocumentBytes {
			t.Fatalf("aggregate fixture document exceeds per-document cap: %d %v", len(raw), err)
		}
		hash, err := localPackHash(doc)
		if err != nil {
			t.Fatal(err)
		}
		pack.Blocks = append(pack.Blocks, LocalPackBlock{SourceID: doc.ID, SourceRevision: 1, Hash: hash, Flow: doc})
	}
	rawPack, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	if len(rawPack) <= maxLocalPackTotalBytes || len(rawPack) > MaxLocalPackBytes {
		t.Fatalf("aggregate fixture does not isolate the 18 MiB limit: pack=%d aggregate=%d raw_cap=%d", len(rawPack), maxLocalPackTotalBytes, MaxLocalPackBytes)
	}

	countsBefore := localPackDBCounts(t, s.store.db)
	missionsBefore := localPackMissionCount(bridge)
	if _, err := prepareLocalPack(ctx, pack); !errors.Is(err, ErrLocalPackTooLarge) {
		t.Fatalf("aggregate cap error = %v, want ErrLocalPackTooLarge", err)
	}
	if _, err := s.PreviewLocalPack(ctx, pack); !errors.Is(err, ErrLocalPackTooLarge) {
		t.Fatalf("preview aggregate cap error = %v, want ErrLocalPackTooLarge", err)
	}
	if _, err := s.ImportLocalPack(ctx, LocalPackImportRequest{Pack: pack}); !errors.Is(err, ErrLocalPackTooLarge) {
		t.Fatalf("import aggregate cap error = %v, want ErrLocalPackTooLarge", err)
	}
	if got := localPackDBCounts(t, s.store.db); got != countsBefore {
		t.Fatalf("aggregate refusal wrote SQL state: before=%v after=%v", countsBefore, got)
	}
	if got := localPackMissionCount(bridge); got != missionsBefore {
		t.Fatalf("aggregate refusal created a mission: before=%d after=%d", missionsBefore, got)
	}
}
