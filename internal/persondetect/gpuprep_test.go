package persondetect

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func pbString(b []byte, number protowire.Number, value string) []byte {
	b = protowire.AppendTag(b, number, protowire.BytesType)
	return protowire.AppendString(b, value)
}

func pbMessage(b []byte, number protowire.Number, value []byte) []byte {
	b = protowire.AppendTag(b, number, protowire.BytesType)
	return protowire.AppendBytes(b, value)
}

func pbVarint(b []byte, number protowire.Number, value uint64) []byte {
	b = protowire.AppendTag(b, number, protowire.VarintType)
	return protowire.AppendVarint(b, value)
}

func testNode(name, opType string, inputs, outputs []string) []byte {
	var node []byte
	for _, input := range inputs {
		node = pbString(node, 1, input)
	}
	for _, output := range outputs {
		node = pbString(node, 2, output)
	}
	node = pbString(node, 3, name)
	return pbString(node, 4, opType)
}

func testTensor(name string, dataType uint64, dims []int64, packed bool, raw []byte) []byte {
	var tensor []byte
	if packed {
		var values []byte
		for _, dim := range dims {
			values = protowire.AppendVarint(values, uint64(dim))
		}
		tensor = pbMessage(tensor, 1, values)
	} else {
		for _, dim := range dims {
			tensor = pbVarint(tensor, 1, uint64(dim))
		}
	}
	tensor = pbVarint(tensor, 2, dataType)
	tensor = pbString(tensor, 8, name)
	return pbMessage(tensor, 9, raw)
}

func testModel(nodes, initializers [][]byte, inputs, outputs, valueInfos []string) []byte {
	var graph []byte
	for _, node := range nodes {
		graph = pbMessage(graph, 1, node)
	}
	graph = pbString(graph, 2, "dfine-test")
	for _, initializer := range initializers {
		graph = pbMessage(graph, 5, initializer)
	}
	for _, name := range inputs {
		graph = pbMessage(graph, 11, pbString(nil, 1, name))
	}
	for _, name := range outputs {
		graph = pbMessage(graph, 12, pbString(nil, 1, name))
	}
	for _, name := range valueInfos {
		graph = pbMessage(graph, 13, pbString(nil, 1, name))
	}
	var model []byte
	model = pbVarint(model, 1, 8)
	model = pbString(model, 2, "dfine-export")
	model = pbMessage(model, 7, graph)
	return pbMessage(model, 8, pbVarint(pbString(nil, 1, ""), 2, 17))
}

var testWeightBytes = bytes.Repeat([]byte{1, 2, 3, 4}, 33)

var testShapeTensor = testTensor("shape", 7, []int64{2}, false, make([]byte, 16))

// integralChain is D-FINE's /model/decoder/integral pattern:
// m = MatMul(x, weight[33]); y = consumer(m, shape).
func integralChain(prefix, weight, consumer string, packed bool) ([][]byte, [][]byte) {
	nodes := [][]byte{
		testNode(prefix+"/MatMul", "MatMul", []string{prefix + "_x", weight}, []string{prefix + "_m"}),
		testNode(prefix+"/Reshape", consumer, []string{prefix + "_m", "shape"}, []string{prefix + "_y"}),
	}
	initializers := [][]byte{testTensor(weight, tensorFloat, []int64{33}, packed, testWeightBytes)}
	return nodes, initializers
}

type parsedTestModel struct {
	fields          []wireField
	initializers    map[string]tensorInfo
	rawInitializers map[string][]byte
	valueInfos      []string
}

func parseTestModel(t *testing.T, model []byte) parsedTestModel {
	t.Helper()
	fields, err := parseWireFields(model)
	if err != nil {
		t.Fatal(err)
	}
	result := parsedTestModel{fields: fields, initializers: map[string]tensorInfo{}, rawInitializers: map[string][]byte{}}
	for _, field := range fields {
		if field.number != modelGraphField {
			continue
		}
		graphFields, err := parseWireFields(field.bytes)
		if err != nil {
			t.Fatal(err)
		}
		for _, graphField := range graphFields {
			switch graphField.number {
			case graphInitializerField:
				info, err := parseTensorInfo(graphField.bytes)
				if err != nil {
					t.Fatal(err)
				}
				result.initializers[info.name] = info
				tensorFields, err := parseWireFields(graphField.bytes)
				if err != nil {
					t.Fatal(err)
				}
				for _, tensorField := range tensorFields {
					if tensorField.number == 9 {
						result.rawInitializers[info.name] = tensorField.bytes
					}
				}
			case graphValueInfoField:
				name, err := parseValueInfoName(graphField.bytes)
				if err != nil {
					t.Fatal(err)
				}
				result.valueInfos = append(result.valueInfos, name)
			}
		}
	}
	return result
}

func mustParseWireFields(t *testing.T, data []byte) []wireField {
	t.Helper()
	fields, err := parseWireFields(data)
	if err != nil {
		t.Fatal(err)
	}
	return fields
}

// checkOnlyPatchChanged verifies that patched equals original except for the
// dims of the weights in wantDims and the value_info entries in removed:
// every other model, graph and tensor field keeps its bytes and its order.
func checkOnlyPatchChanged(t *testing.T, original, patched []byte, wantDims map[string][]int64, removed []string) {
	t.Helper()
	originalFields, patchedFields := mustParseWireFields(t, original), mustParseWireFields(t, patched)
	if len(originalFields) != len(patchedFields) {
		t.Fatalf("model field count %d -> %d", len(originalFields), len(patchedFields))
	}
	var originalGraph, patchedGraph []wireField
	for i, field := range originalFields {
		if field.number == modelGraphField && patchedFields[i].number == modelGraphField {
			originalGraph = mustParseWireFields(t, field.bytes)
			patchedGraph = mustParseWireFields(t, patchedFields[i].bytes)
			continue
		}
		if !bytes.Equal(field.raw, patchedFields[i].raw) {
			t.Fatalf("model field %d (#%d) changed", field.number, i)
		}
	}

	toRemove := map[string]bool{}
	for _, name := range removed {
		toRemove[name] = true
	}
	patchedWeights := map[string]bool{}
	next := 0
	for i, field := range originalGraph {
		if field.number == graphValueInfoField {
			name, err := parseValueInfoName(field.bytes)
			if err != nil {
				t.Fatal(err)
			}
			if toRemove[name] {
				delete(toRemove, name)
				continue
			}
		}
		if next >= len(patchedGraph) {
			t.Fatalf("patched graph ends before original graph field #%d", i)
		}
		got := patchedGraph[next]
		next++
		if field.number == graphInitializerField {
			info, err := parseTensorInfo(field.bytes)
			if err != nil {
				t.Fatal(err)
			}
			if want, ok := wantDims[info.name]; ok {
				if got.number != graphInitializerField {
					t.Fatalf("initializer %q replaced by field %d", info.name, got.number)
				}
				checkOnlyDimsChanged(t, info.name, field.bytes, got.bytes, want)
				patchedWeights[info.name] = true
				continue
			}
		}
		if !bytes.Equal(field.raw, got.raw) {
			t.Fatalf("graph field %d (#%d) changed or moved", field.number, i)
		}
	}
	if next != len(patchedGraph) {
		t.Fatalf("patched graph has %d extra fields", len(patchedGraph)-next)
	}
	if len(toRemove) != 0 {
		t.Fatalf("value_info not removed: %v", toRemove)
	}
	if len(patchedWeights) != len(wantDims) {
		t.Fatalf("patched weights %v, want %v", patchedWeights, wantDims)
	}
}

// checkOnlyDimsChanged verifies that every TensorProto field except dims keeps
// its bytes and order and that the dims are exactly want.
func checkOnlyDimsChanged(t *testing.T, name string, original, patched []byte, want []int64) {
	t.Helper()
	withoutDims := func(data []byte) []wireField {
		var kept []wireField
		for _, field := range mustParseWireFields(t, data) {
			if field.number != tensorDimsField {
				kept = append(kept, field)
			}
		}
		return kept
	}
	originalFields, patchedFields := withoutDims(original), withoutDims(patched)
	if len(originalFields) != len(patchedFields) {
		t.Fatalf("initializer %q: non-dims field count %d -> %d", name, len(originalFields), len(patchedFields))
	}
	for i := range originalFields {
		if !bytes.Equal(originalFields[i].raw, patchedFields[i].raw) {
			t.Fatalf("initializer %q: field %d (#%d) changed or moved", name, originalFields[i].number, i)
		}
	}
	info, err := parseTensorInfo(patched)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(info.dims, want) {
		t.Fatalf("initializer %q dims = %v, want %v", name, info.dims, want)
	}
}

// withGraphFields wraps the graph of model in extra leading and trailing
// graph fields.
func withGraphFields(t *testing.T, model, before, after []byte) []byte {
	t.Helper()
	var out []byte
	for _, field := range mustParseWireFields(t, model) {
		if field.number == modelGraphField {
			graph := append(append(append([]byte(nil), before...), field.bytes...), after...)
			out = pbMessage(out, modelGraphField, graph)
			continue
		}
		out = append(out, field.raw...)
	}
	return out
}

func TestPatchVectorMatMulsRewritesWeightAndDropsStaleValueInfo(t *testing.T) {
	nodes, initializers := integralChain("integral", "w", "Reshape", false)
	model := testModel(nodes, append(initializers, testShapeTensor), []string{"integral_x"}, []string{"integral_y"}, []string{"integral_m", "keep"})
	patched, count, err := PatchVectorMatMuls(model)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	result := parseTestModel(t, patched)
	if dims := result.initializers["w"].dims; !slices.Equal(dims, []int64{33, 1}) {
		t.Fatalf("w dims = %v", dims)
	}
	if dims := result.initializers["shape"].dims; !slices.Equal(dims, []int64{2}) {
		t.Fatalf("shape dims = %v", dims)
	}
	if !bytes.Equal(result.rawInitializers["w"], testWeightBytes) {
		t.Fatal("weight raw_data changed")
	}
	if !slices.Equal(result.valueInfos, []string{"keep"}) {
		t.Fatalf("value_info = %v", result.valueInfos)
	}
	checkOnlyPatchChanged(t, model, patched, map[string][]int64{"w": {33, 1}}, []string{"integral_m"})
}

func TestPatchVectorMatMulsAcceptsPackedDims(t *testing.T) {
	nodes, initializers := integralChain("integral", "w", "Reshape", true)
	model := testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
	patched, count, err := PatchVectorMatMuls(model)
	if err != nil || count != 1 {
		t.Fatalf("PatchVectorMatMuls = %d, %v", count, err)
	}
	if dims := parseTestModel(t, patched).initializers["w"].dims; !slices.Equal(dims, []int64{33, 1}) {
		t.Fatalf("w dims = %v", dims)
	}
}

func TestPatchVectorMatMulsCountsEveryPatchedNode(t *testing.T) {
	var nodes, initializers [][]byte
	for _, name := range []string{"integral", "integral_1", "integral_2"} {
		chainNodes, chainInitializers := integralChain(name, name+"_w", "Reshape", false)
		nodes = append(nodes, chainNodes...)
		initializers = append(initializers, chainInitializers...)
	}
	model := testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "integral_1_y", "integral_2_y"}, nil)
	_, count, err := PatchVectorMatMuls(model)
	if err != nil || count != 3 {
		t.Fatalf("PatchVectorMatMuls = %d, %v; want 3 (D-FINE-S)", count, err)
	}

	// The folded D-FINE graphs feed one shared weight to all three MatMuls.
	nodes = nil
	for _, name := range []string{"integral", "integral_1", "integral_2"} {
		chainNodes, _ := integralChain(name, "shared_w", "Reshape", false)
		nodes = append(nodes, chainNodes...)
	}
	_, sharedWeight := integralChain("integral", "shared_w", "Reshape", false)
	stale := []string{"integral_m", "integral_1_m", "integral_2_m"}
	model = testModel(nodes, append(sharedWeight, testShapeTensor), nil, []string{"integral_y", "integral_1_y", "integral_2_y"}, stale)
	patched, count, err := PatchVectorMatMuls(model)
	if err != nil || count != 3 {
		t.Fatalf("shared weight: PatchVectorMatMuls = %d, %v; want 3", count, err)
	}
	checkOnlyPatchChanged(t, model, patched, map[string][]int64{"shared_w": {33, 1}}, stale)
}

func TestPatchVectorMatMulsChangesOnlyWeightDims(t *testing.T) {
	cases := []struct {
		name    string
		build   func(t *testing.T) []byte
		removed []string
	}{
		{"dims split over an empty packed and an unpacked field, unknown fields kept", func(t *testing.T) []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			var tensor []byte
			tensor = pbVarint(tensor, 2, tensorFloat)
			tensor = protowire.AppendTag(tensor, 99, protowire.StartGroupType)
			tensor = pbVarint(tensor, 1, 5)
			tensor = protowire.AppendTag(tensor, 99, protowire.EndGroupType)
			tensor = pbMessage(tensor, 1, nil) // empty packed dims
			tensor = pbString(tensor, 8, "w")
			tensor = pbVarint(tensor, 1, 33)
			tensor = pbMessage(tensor, 9, testWeightBytes)
			model := testModel(nodes, [][]byte{tensor, testShapeTensor}, nil, []string{"integral_y"}, []string{"integral_m", "keep"})
			before := pbString(nil, 10, "doc")
			before = protowire.AppendTag(before, 98, protowire.StartGroupType)
			before = protowire.AppendTag(before, 98, protowire.EndGroupType)
			model = withGraphFields(t, model, before, pbVarint(nil, 97, 7))
			return pbVarint(model, 99, 1)
		}, []string{"integral_m"}},
		{"explicit ai.onnx domain", func(*testing.T) []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[0] = pbString(nodes[0], 7, "ai.onnx")
			nodes[1] = pbString(nodes[1], 7, "ai.onnx")
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}, nil},
		{"two Reshape consumers", func(*testing.T) []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes = append(nodes, testNode("integral/Reshape_2", "Reshape", []string{"integral_m", "shape"}, []string{"integral_y2"}))
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "integral_y2"}, []string{"integral_m"})
		}, []string{"integral_m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := tc.build(t)
			patched, count, err := PatchVectorMatMuls(model)
			if err != nil || count != 1 {
				t.Fatalf("PatchVectorMatMuls = %d, %v; want 1", count, err)
			}
			checkOnlyPatchChanged(t, model, patched, map[string][]int64{"w": {33, 1}}, tc.removed)
		})
	}
}

func TestPatchVectorMatMulsRefusesUnsafeRewrites(t *testing.T) {
	cases := []struct {
		name  string
		build func() []byte
	}{
		{"consumer is not Reshape", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Add", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"MatMul output is the Reshape shape input", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[1] = testNode("integral/Reshape", "Reshape", []string{"data", "integral_m"}, []string{"integral_y"})
			return testModel(nodes, initializers, []string{"data"}, []string{"integral_y"}, nil)
		}},
		{"Reshape uses the MatMul output as data and shape", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[1] = testNode("integral/Reshape", "Reshape", []string{"integral_m", "integral_m"}, []string{"integral_y"})
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"MatMul output is unused", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes[:1], initializers, nil, nil, nil)
		}},
		{"consumer is a Reshape of another domain", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[1] = pbString(nodes[1], 7, "com.example")
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"MatMul of another domain", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[0] = pbString(nodes[0], 7, "com.microsoft")
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"MatMul has one input", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[0] = testNode("integral/MatMul", "MatMul", []string{"w"}, []string{"integral_m"})
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"MatMul has two outputs", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[0] = testNode("integral/MatMul", "MatMul", []string{"integral_x", "w"}, []string{"integral_m", "integral_extra"})
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"MatMul output is a graph output", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "integral_m"}, nil)
		}},
		{"weight is a graph input", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), []string{"w"}, []string{"integral_y"}, nil)
		}},
		{"weight is a graph output", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "w"}, nil)
		}},
		{"weight has value_info", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, []string{"w"})
		}},
		{"weight shared with another op", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes = append(nodes, testNode("scale", "Mul", []string{"w", "w"}, []string{"scaled"}))
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "scaled"}, nil)
		}},
		{"weight is also the first MatMul input", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[0] = testNode("integral/MatMul", "MatMul", []string{"w", "w"}, []string{"integral_m"})
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"weight shared with a MatMul that feeds Add", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			other, _ := integralChain("other", "w", "Add", false)
			return testModel(append(nodes, other...), append(initializers, testShapeTensor), nil, []string{"integral_y", "other_y"}, nil)
		}},
		{"weight name is empty", func() []byte {
			nodes, _ := integralChain("integral", "", "Reshape", false)
			initializers := [][]byte{testTensor("", tensorFloat, []int64{33}, false, testWeightBytes), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"weight name is not unique", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			initializers = append([][]byte{testTensor("w", tensorFloat, []int64{3, 11}, false, testWeightBytes)}, initializers...)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		}},
		{"weight is not float", func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", 7, []int64{33}, false, make([]byte, 33*8)), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"weight is already 2-D", func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", tensorFloat, []int64{33, 1}, false, testWeightBytes), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"weight dim is zero", func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", tensorFloat, []int64{0}, false, nil), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"weight dim is negative", func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", tensorFloat, []int64{-1}, false, testWeightBytes), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		}},
		{"MatMul output is used in a subgraph", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			branch := pbMessage(nil, 1, testNode("branch/Add", "Add", []string{"integral_m", "integral_m"}, []string{"branch_y"}))
			branch = pbMessage(branch, 12, pbString(nil, 1, "branch_y"))
			attribute := pbMessage(pbString(nil, 1, "then_branch"), 6, branch)
			nodes = append(nodes, pbMessage(testNode("if", "If", []string{"cond"}, []string{"if_y"}), 5, attribute))
			return testModel(nodes, append(initializers, testShapeTensor), []string{"cond"}, []string{"integral_y", "if_y"}, nil)
		}},
		{"graph has a graphs attribute", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			attribute := pbMessage(pbString(nil, 1, "bodies"), 11, pbString(nil, 2, "body"))
			nodes = append(nodes, pbMessage(testNode("scan", "Scan", []string{"state"}, []string{"scan_y"}), 5, attribute))
			return testModel(nodes, append(initializers, testShapeTensor), []string{"state"}, []string{"integral_y", "scan_y"}, nil)
		}},
		{"training_info uses the weight", func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			model := testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
			algorithm := pbMessage(nil, 1, testNode("update", "Add", []string{"w", "w"}, []string{"w_next"}))
			return pbMessage(model, 20, pbMessage(nil, 2, algorithm)) // TrainingInfoProto.algorithm
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := tc.build()
			patched, count, err := PatchVectorMatMuls(model)
			if err != nil || count != 0 || !bytes.Equal(patched, model) {
				t.Fatalf("count %d, err %v, unchanged %v", count, err, bytes.Equal(patched, model))
			}
		})
	}
}

func TestPatchVectorMatMulsRejectsMalformedModels(t *testing.T) {
	cases := map[string][]byte{
		"truncated": {0x0a, 0x05, 0x01},
		"no graph":  pbVarint(nil, 1, 8),
		"bad graph": pbMessage(pbVarint(nil, 1, 8), 7, []byte{0x0a, 0x09}),
	}
	for name, model := range cases {
		if _, _, err := PatchVectorMatMuls(model); err == nil {
			t.Errorf("%s model accepted", name)
		}
	}
	if _, _, err := PatchVectorMatMuls(pbVarint(nil, 1, 8)); err == nil || !strings.Contains(err.Error(), "no graph") {
		t.Fatalf("missing graph error = %v", err)
	}
}
