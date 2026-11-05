package flows

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func blockTestDoc(id string, children ...blockRef) *Flow {
	doc := &Flow{ID: id, Kind: KindBlock, Schema: SchemaVersion, Nodes: []Node{
		{Type: blockInputType, TypeVersion: 1, Params: map[string]any{"fields": []any{
			map[string]any{"name": "value", "type": "string", "required": true},
		}}},
		{Type: blockOutputType, TypeVersion: 1, Params: map[string]any{"fields": []any{}}},
	}}
	for _, child := range children {
		doc.Nodes = append(doc.Nodes, Node{Type: blockCallType, TypeVersion: 1, Params: map[string]any{
			"flow_id": child.FlowID, "revision": float64(child.Revision), "inputs": map[string]any{"value": "sample"},
		}})
	}
	return doc
}

func TestBlockContractValidatesValues(t *testing.T) {
	doc := blockTestDoc("flow_block")
	contract, err := readBlockContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		values map[string]any
		valid  bool
	}{
		{map[string]any{"value": "hello"}, true},
		{map[string]any{}, false},
		{map[string]any{"value": nil}, false},
		{map[string]any{"value": 1.0}, false},
		{map[string]any{"value": "hello", "extra": true}, false},
	} {
		if got := checkBlockValues(contract.Inputs, tc.values) == nil; got != tc.valid {
			t.Errorf("values %#v: valid=%v, want %v", tc.values, got, tc.valid)
		}
	}
	doc.Nodes[0].Params["fields"] = []any{
		map[string]any{"name": "x", "type": "number", "nullable": true},
	}
	contract, err = readBlockContract(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []map[string]any{{}, {"x": nil}, {"x": 2.0}} {
		if err := checkBlockValues(contract.Inputs, v); err != nil {
			t.Errorf("valid %#v: %v", v, err)
		}
	}
	doc.Nodes = append(doc.Nodes, doc.Nodes[0])
	if _, err := readBlockContract(doc); !errors.Is(err, errBlockContract) {
		t.Fatalf("duplicate entry: %v", err)
	}
	doc = blockTestDoc("flow_block")
	doc.Nodes[0].Params["fields"] = []any{map[string]any{"name": "x", "type": "string", "typo": true}}
	if _, err := readBlockContract(doc); !errors.Is(err, errBlockContract) {
		t.Fatalf("unknown schema field: %v", err)
	}
	doc = blockTestDoc("flow_block")
	doc.Nodes[1].Params["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true}}
	if _, err := readBlockContract(doc); !errors.Is(err, errBlockContract) {
		t.Fatalf("missing required output binding: %v", err)
	}
	doc.Nodes[1].Params["values"] = map[string]any{"result": "{{steps.finish.value}}"}
	if _, err := readBlockContract(doc); err != nil {
		t.Fatalf("valid output binding: %v", err)
	}
}

func TestBlockDependenciesPinExactVersionsAndRejectCycles(t *testing.T) {
	ctx := context.Background()
	a, b := blockRef{"flow_a", 3}, blockRef{"flow_b", 2}
	docs := map[blockRef]*Flow{a: blockTestDoc(a.FlowID, b), b: blockTestDoc(b.FlowID)}
	load := func(_ context.Context, id string, revision int) (*Flow, error) {
		if doc := docs[blockRef{id, revision}]; doc != nil {
			return doc, nil
		}
		return nil, ErrNotFound
	}
	root := &Flow{ID: "flow_root", Nodes: []Node{{Type: blockCallType, TypeVersion: 1,
		Params: map[string]any{"flow_id": a.FlowID, "revision": float64(a.Revision), "inputs": map[string]any{"value": "sample"}}}}}
	resolved, err := resolveBlockDependencies(ctx, root, load)
	if err != nil || len(resolved) != 2 || resolved[0].Ref != b || resolved[1].Ref != a {
		t.Fatalf("dependencies = %+v, %v", resolved, err)
	}
	if resolved[1].Hash == ([32]byte{}) {
		t.Fatal("snapshot has no hash")
	}
	docs[a].Name = "changed after resolution"
	if resolved[1].Document.Name == docs[a].Name {
		t.Fatal("resolved snapshot aliases the loader's document")
	}
	docs[b] = blockTestDoc(b.FlowID, blockRef{a.FlowID, 1})
	if _, err := resolveBlockDependencies(ctx, root, load); !errors.Is(err, errBlockContract) {
		t.Fatalf("cross-version recursion = %v", err)
	}
	docs[b] = blockTestDoc(b.FlowID)
	root.Nodes[0].Params["revision"] = 4.0
	if _, err := resolveBlockDependencies(ctx, root, load); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing exact version = %v", err)
	}
	root.Nodes[0].Params["revision"] = "{{trigger.data.version}}"
	if _, err := resolveBlockDependencies(ctx, root, load); !errors.Is(err, errBlockContract) {
		t.Fatalf("dynamic revision = %v", err)
	}
	root.Nodes[0].Params["revision"] = 3.0
	root.Nodes[0].Params["flow_id"] = "{{trigger.data.target}}"
	if _, err := resolveBlockDependencies(ctx, root, load); !errors.Is(err, errBlockContract) {
		t.Fatalf("dynamic flow ID = %v", err)
	}
}

func TestBlockDependencyAndBatchBounds(t *testing.T) {
	root := &Flow{ID: "flow_root"}
	docs := map[blockRef]*Flow{}
	for i := 0; i <= maxBlockVersions; i++ {
		ref := blockRef{fmt.Sprintf("flow_%d", i), 1}
		docs[ref] = blockTestDoc(ref.FlowID)
		root.Nodes = append(root.Nodes, Node{Type: blockCallType, TypeVersion: 1,
			Params: map[string]any{"flow_id": ref.FlowID, "revision": 1.0, "inputs": map[string]any{"value": "sample"}}})
	}
	load := func(_ context.Context, id string, rev int) (*Flow, error) { return docs[blockRef{id, rev}], nil }
	if _, err := resolveBlockDependencies(context.Background(), root, load); !errors.Is(err, errBlockContract) {
		t.Fatalf("too many dependencies = %v", err)
	}
	node := Node{Type: blockEachType, TypeVersion: 1, Params: map[string]any{
		"flow_id": "flow_a", "revision": 1.0, "max_items": 1000.0, "concurrency": 1.0, "on_item_error": "collect",
	}}
	if _, err := blockReference(node); err != nil {
		t.Fatal(err)
	}
	node.Params["concurrency"] = 5.0
	if _, err := blockReference(node); !errors.Is(err, errBlockContract) {
		t.Fatalf("unbounded batch = %v", err)
	}
}

func TestBlockCachedDependencyStillRejectsCrossVersionRecursion(t *testing.T) {
	a1, a2, b, c := blockRef{"flow_a", 1}, blockRef{"flow_a", 2}, blockRef{"flow_b", 1}, blockRef{"flow_c", 1}
	docs := map[blockRef]*Flow{
		a1: blockTestDoc(a1.FlowID, c), a2: blockTestDoc(a2.FlowID),
		b: blockTestDoc(b.FlowID, c), c: blockTestDoc(c.FlowID, a2),
	}
	root := blockTestDoc("flow_root", b, a1)
	load := func(_ context.Context, id string, revision int) (*Flow, error) {
		return docs[blockRef{id, revision}], nil
	}
	if _, err := resolveBlockDependencies(context.Background(), root, load); !errors.Is(err, errBlockContract) {
		t.Fatalf("cached dependency hid recursion: %v", err)
	}
}

func TestBlockBindingsRejectMissingUnknownAndDuplicateInputs(t *testing.T) {
	contract := blockContract{Inputs: []blockField{
		{Name: "value", Type: "string", Required: true},
		{Name: "position", Type: "number"},
	}}
	call := Node{Type: blockCallType, Params: map[string]any{"inputs": map[string]any{"value": "{{trigger.data.value}}"}}}
	if err := checkBlockBindings(call, contract); err != nil {
		t.Fatal(err)
	}
	call.Params["inputs"] = map[string]any{"extra": "x"}
	if err := checkBlockBindings(call, contract); !errors.Is(err, errBlockContract) {
		t.Fatalf("unknown input = %v", err)
	}
	call.Params["inputs"] = map[string]any{}
	if err := checkBlockBindings(call, contract); !errors.Is(err, errBlockContract) {
		t.Fatalf("missing input = %v", err)
	}
	batch := Node{Type: blockEachType, Params: map[string]any{"items": "{{trigger.data.items}}", "item_field": "value"}}
	if err := checkBlockBindings(batch, contract); err != nil {
		t.Fatal(err)
	}
	batch.Params["inputs"] = map[string]any{"value": "duplicate"}
	if err := checkBlockBindings(batch, contract); !errors.Is(err, errBlockContract) {
		t.Fatalf("duplicate item = %v", err)
	}
	batch.Params["inputs"] = map[string]any{}
	batch.Params["index_field"] = "value"
	if err := checkBlockBindings(batch, contract); !errors.Is(err, errBlockContract) {
		t.Fatalf("duplicate index = %v", err)
	}
}
