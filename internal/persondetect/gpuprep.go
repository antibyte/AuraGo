package persondetect

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// ONNX protobuf field numbers (onnx/onnx.proto v1.17.0, proto2).
const (
	modelGraphField        = 7
	modelTrainingInfoField = 20

	graphNodeField        = 1
	graphInitializerField = 5
	graphInputField       = 11
	graphOutputField      = 12
	graphValueInfoField   = 13

	nodeInputField     = 1
	nodeOutputField    = 2
	nodeOpTypeField    = 4
	nodeAttributeField = 5
	nodeDomainField    = 7

	attributeGraphField  = 6  // AttributeProto.g
	attributeGraphsField = 11 // AttributeProto.graphs

	tensorDimsField     = 1
	tensorDataTypeField = 2
	tensorNameField     = 8

	valueInfoNameField = 1

	tensorFloat = 1 // TensorProto.DataType FLOAT
)

// wireField is one protobuf field exactly as read.
type wireField struct {
	number protowire.Number
	typ    protowire.Type
	raw    []byte // tag and value
	bytes  []byte // value of a length-delimited field
	varint uint64 // value of a varint field
}

func parseWireFields(data []byte) ([]wireField, error) {
	var fields []wireField
	for len(data) > 0 {
		number, typ, tagLength := protowire.ConsumeTag(data)
		if tagLength < 0 {
			return nil, protowire.ParseError(tagLength)
		}
		valueLength := protowire.ConsumeFieldValue(number, typ, data[tagLength:])
		if valueLength < 0 {
			return nil, protowire.ParseError(valueLength)
		}
		field := wireField{number: number, typ: typ, raw: data[:tagLength+valueLength]}
		switch typ {
		case protowire.BytesType:
			value, n := protowire.ConsumeBytes(data[tagLength:])
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			field.bytes = value
		case protowire.VarintType:
			value, n := protowire.ConsumeVarint(data[tagLength:])
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			field.varint = value
		}
		fields = append(fields, field)
		data = data[tagLength+valueLength:]
	}
	return fields, nil
}

type onnxNode struct {
	opType, domain  string
	inputs, outputs []string
	hasSubgraph     bool // a graph-valued attribute (If, Loop, Scan …)
}

func parseNode(data []byte) (onnxNode, error) {
	fields, err := parseWireFields(data)
	if err != nil {
		return onnxNode{}, err
	}
	var node onnxNode
	for _, field := range fields {
		if field.typ != protowire.BytesType {
			continue
		}
		switch field.number {
		case nodeInputField:
			node.inputs = append(node.inputs, string(field.bytes))
		case nodeOutputField:
			node.outputs = append(node.outputs, string(field.bytes))
		case nodeOpTypeField:
			node.opType = string(field.bytes)
		case nodeDomainField:
			node.domain = string(field.bytes)
		case nodeAttributeField:
			attributeFields, attributeErr := parseWireFields(field.bytes)
			if attributeErr != nil {
				return onnxNode{}, fmt.Errorf("attribute: %w", attributeErr)
			}
			for _, attributeField := range attributeFields {
				if attributeField.number == attributeGraphField || attributeField.number == attributeGraphsField {
					node.hasSubgraph = true
				}
			}
		}
	}
	return node, nil
}

// isONNXOp reports whether node is opType of the default ONNX domain.
func isONNXOp(node onnxNode, opType string) bool {
	return node.opType == opType && (node.domain == "" || node.domain == "ai.onnx")
}

type tensorInfo struct {
	name     string
	dataType uint64
	dims     []int64
}

// parseTensorInfo reads name, data type and dims; dims may be packed or not.
func parseTensorInfo(data []byte) (tensorInfo, error) {
	fields, err := parseWireFields(data)
	if err != nil {
		return tensorInfo{}, err
	}
	var info tensorInfo
	for _, field := range fields {
		switch field.number {
		case tensorDimsField:
			switch field.typ {
			case protowire.VarintType:
				info.dims = append(info.dims, int64(field.varint))
			case protowire.BytesType:
				packed := field.bytes
				for len(packed) > 0 {
					value, n := protowire.ConsumeVarint(packed)
					if n < 0 {
						return tensorInfo{}, protowire.ParseError(n)
					}
					info.dims = append(info.dims, int64(value))
					packed = packed[n:]
				}
			}
		case tensorDataTypeField:
			if field.typ == protowire.VarintType {
				info.dataType = field.varint
			}
		case tensorNameField:
			if field.typ == protowire.BytesType {
				info.name = string(field.bytes)
			}
		}
	}
	return info, nil
}

func parseValueInfoName(data []byte) (string, error) {
	fields, err := parseWireFields(data)
	if err != nil {
		return "", err
	}
	for _, field := range fields {
		if field.number == valueInfoNameField && field.typ == protowire.BytesType {
			return string(field.bytes), nil
		}
	}
	return "", nil
}

// setTensorDims replaces every dims field with unpacked dims at the position
// of the first one; all other fields keep their bytes and order.
func setTensorDims(data []byte, dims []int64) ([]byte, error) {
	fields, err := parseWireFields(data)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(data)+8)
	written := false
	for _, field := range fields {
		if field.number == tensorDimsField {
			if !written {
				for _, dim := range dims {
					out = protowire.AppendTag(out, tensorDimsField, protowire.VarintType)
					out = protowire.AppendVarint(out, uint64(dim))
				}
				written = true
			}
			continue
		}
		out = append(out, field.raw...)
	}
	if !written {
		return nil, fmt.Errorf("tensor has no dims")
	}
	return out, nil
}

func appendBytesField(out []byte, number protowire.Number, value []byte) []byte {
	out = protowire.AppendTag(out, number, protowire.BytesType)
	return protowire.AppendBytes(out, value)
}

// PatchVectorMatMuls makes a constant-folded D-FINE graph WebGPU-compatible.
// The WebGPU EP rejects MatMul with a 1-D second operand ("Invalid dimension
// … for SizeToDimension"); every MatMul whose second input is a 1-D FLOAT
// initializer used by nothing else, and whose output feeds only Reshape nodes
// (which absorb the extra axis), gets that initializer reshaped to [K,1]. Stale
// value_info entries of the patched MatMul outputs are removed. Everything
// else is copied byte for byte. count is the number of patched MatMul nodes
// (several MatMuls may share one initializer); with count 0 the input is
// returned unchanged.
//
// Only operators of the default ONNX domain count, the initializer name must
// be unique and must not be declared as a graph input, output or value_info,
// and a model with subgraphs (If, Loop, Scan …) or training_info is left
// unchanged because names used inside those graphs are consumers this rewrite
// cannot see.
func PatchVectorMatMuls(model []byte) (patched []byte, count int, err error) {
	modelFields, err := parseWireFields(model)
	if err != nil {
		return nil, 0, fmt.Errorf("parse ONNX model: %w", err)
	}
	graphIndex := -1
	hasTrainingInfo := false
	for i, field := range modelFields {
		if field.number == modelTrainingInfoField {
			hasTrainingInfo = true
		}
		if field.number != modelGraphField {
			continue
		}
		if field.typ != protowire.BytesType {
			return nil, 0, fmt.Errorf("ONNX model graph has wire type %d", field.typ)
		}
		if graphIndex >= 0 {
			return nil, 0, fmt.Errorf("ONNX model has more than one graph field")
		}
		graphIndex = i
	}
	if graphIndex < 0 {
		return nil, 0, fmt.Errorf("ONNX model has no graph")
	}
	graphFields, err := parseWireFields(modelFields[graphIndex].bytes)
	if err != nil {
		return nil, 0, fmt.Errorf("parse ONNX graph: %w", err)
	}

	var nodes []onnxNode
	initializers := map[string]tensorInfo{}
	initializerCount := map[string]int{}
	initializerAt := map[int]string{}
	valueInfoAt := map[int]string{}
	declared := map[string]bool{} // graph inputs, outputs and value_info
	graphOutputs := map[string]bool{}
	hasSubgraph := false
	for i, field := range graphFields {
		if field.typ != protowire.BytesType {
			continue
		}
		switch field.number {
		case graphNodeField:
			node, nodeErr := parseNode(field.bytes)
			if nodeErr != nil {
				return nil, 0, fmt.Errorf("parse ONNX node %d: %w", len(nodes), nodeErr)
			}
			hasSubgraph = hasSubgraph || node.hasSubgraph
			nodes = append(nodes, node)
		case graphInitializerField:
			info, tensorErr := parseTensorInfo(field.bytes)
			if tensorErr != nil {
				return nil, 0, fmt.Errorf("parse ONNX initializer: %w", tensorErr)
			}
			initializers[info.name] = info
			initializerCount[info.name]++
			initializerAt[i] = info.name
		case graphInputField, graphOutputField, graphValueInfoField:
			name, infoErr := parseValueInfoName(field.bytes)
			if infoErr != nil {
				return nil, 0, fmt.Errorf("parse ONNX value info: %w", infoErr)
			}
			declared[name] = true
			switch field.number {
			case graphOutputField:
				graphOutputs[name] = true
			case graphValueInfoField:
				valueInfoAt[i] = name
			}
		}
	}
	if hasSubgraph || hasTrainingInfo {
		return model, 0, nil
	}

	type use struct{ node, input int }
	consumers := map[string][]use{}
	for i, node := range nodes {
		for j, input := range node.inputs {
			if input != "" {
				consumers[input] = append(consumers[input], use{node: i, input: j})
			}
		}
	}
	qualifies := func(index int) bool {
		node := nodes[index]
		if !isONNXOp(node, "MatMul") || len(node.inputs) != 2 || len(node.outputs) != 1 {
			return false
		}
		name := node.inputs[1]
		weight, ok := initializers[name]
		if !ok || name == "" || initializerCount[name] != 1 || declared[name] ||
			len(weight.dims) != 1 || weight.dims[0] <= 0 || weight.dataType != tensorFloat {
			return false
		}
		output := node.outputs[0]
		users := consumers[output]
		if len(users) == 0 || graphOutputs[output] {
			return false
		}
		for _, user := range users {
			if !isONNXOp(nodes[user.node], "Reshape") || user.input != 0 {
				return false
			}
		}
		return true
	}

	newDims := map[string][]int64{}
	staleValueInfo := map[string]bool{}
	for index := range nodes {
		if !qualifies(index) {
			continue
		}
		weight := nodes[index].inputs[1]
		exclusive := true
		for _, user := range consumers[weight] {
			if user.input != 1 || !qualifies(user.node) {
				exclusive = false
				break
			}
		}
		if !exclusive {
			continue
		}
		newDims[weight] = []int64{initializers[weight].dims[0], 1}
		staleValueInfo[nodes[index].outputs[0]] = true
		count++
	}
	if count == 0 {
		return model, 0, nil
	}

	graph := make([]byte, 0, len(modelFields[graphIndex].bytes)+16)
	for i, field := range graphFields {
		if name, ok := initializerAt[i]; ok {
			if dims, patch := newDims[name]; patch {
				tensor, dimsErr := setTensorDims(field.bytes, dims)
				if dimsErr != nil {
					return nil, 0, fmt.Errorf("patch initializer %s: %w", name, dimsErr)
				}
				graph = appendBytesField(graph, graphInitializerField, tensor)
				continue
			}
		}
		if name, ok := valueInfoAt[i]; ok && staleValueInfo[name] {
			continue
		}
		graph = append(graph, field.raw...)
	}
	patched = make([]byte, 0, len(model)+16)
	for i, field := range modelFields {
		if i == graphIndex {
			patched = appendBytesField(patched, modelGraphField, graph)
			continue
		}
		patched = append(patched, field.raw...)
	}
	return patched, count, nil
}
