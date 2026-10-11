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
	original := parseTestModel(t, model)
	if len(original.fields) != len(result.fields) {
		t.Fatalf("model field count %d -> %d", len(original.fields), len(result.fields))
	}
	for i, field := range original.fields {
		if field.number == modelGraphField {
			continue
		}
		if !bytes.Equal(field.raw, result.fields[i].raw) {
			t.Fatalf("model field %d changed", field.number)
		}
	}
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
	model = testModel(nodes, append(sharedWeight, testShapeTensor), nil, []string{"integral_y", "integral_1_y", "integral_2_y"},
		[]string{"integral_m", "integral_1_m", "integral_2_m"})
	patched, count, err := PatchVectorMatMuls(model)
	if err != nil || count != 3 {
		t.Fatalf("shared weight: PatchVectorMatMuls = %d, %v; want 3", count, err)
	}
	result := parseTestModel(t, patched)
	if dims := result.initializers["shared_w"].dims; !slices.Equal(dims, []int64{33, 1}) || len(result.valueInfos) != 0 {
		t.Fatalf("shared weight dims = %v, value_info = %v", dims, result.valueInfos)
	}
}

func TestPatchVectorMatMulsRefusesUnsafeRewrites(t *testing.T) {
	cases := map[string]func() []byte{
		"consumer is not Reshape": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Add", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		},
		"weight is a graph input": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), []string{"w"}, []string{"integral_y"}, nil)
		},
		"weight shared with another op": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes = append(nodes, testNode("scale", "Mul", []string{"w", "w"}, []string{"scaled"}))
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "scaled"}, nil)
		},
		"MatMul output is a graph output": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "integral_m"}, nil)
		},
		"weight is not float": func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", 7, []int64{33}, false, make([]byte, 33*8)), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		},
		"weight is already 2-D": func() []byte {
			nodes, _ := integralChain("integral", "w", "Reshape", false)
			initializers := [][]byte{testTensor("w", tensorFloat, []int64{33, 1}, false, testWeightBytes), testShapeTensor}
			return testModel(nodes, initializers, nil, []string{"integral_y"}, nil)
		},
		"consumer is a Reshape of another domain": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			nodes[1] = pbString(nodes[1], 7, "com.example")
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		},
		"weight is a graph output": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y", "w"}, nil)
		},
		"weight has value_info": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, []string{"w"})
		},
		"weight name is not unique": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			initializers = append([][]byte{testTensor("w", tensorFloat, []int64{3, 11}, false, testWeightBytes)}, initializers...)
			return testModel(nodes, append(initializers, testShapeTensor), nil, []string{"integral_y"}, nil)
		},
		"MatMul output is used in a subgraph": func() []byte {
			nodes, initializers := integralChain("integral", "w", "Reshape", false)
			branch := pbMessage(nil, 1, testNode("branch/Add", "Add", []string{"integral_m", "integral_m"}, []string{"branch_y"}))
			branch = pbMessage(branch, 12, pbString(nil, 1, "branch_y"))
			attribute := pbMessage(pbString(nil, 1, "then_branch"), 6, branch)
			nodes = append(nodes, pbMessage(testNode("if", "If", []string{"cond"}, []string{"if_y"}), 5, attribute))
			return testModel(nodes, append(initializers, testShapeTensor), []string{"cond"}, []string{"integral_y", "if_y"}, nil)
		},
	}
	for name, build := range cases {
		model := build()
		patched, count, err := PatchVectorMatMuls(model)
		if err != nil || count != 0 || !bytes.Equal(patched, model) {
			t.Errorf("%s: count %d, err %v, unchanged %v", name, count, err, bytes.Equal(patched, model))
		}
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
