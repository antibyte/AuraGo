package pixelmatte

import (
	"errors"
	"fmt"
)

// Protocol buffer field numbers of onnx.proto that ParseIONames reads.
const (
	modelGraphField       = 7  // ModelProto.graph
	graphInitializerField = 5  // GraphProto.initializer
	graphInputField       = 11 // GraphProto.input
	graphOutputField      = 12 // GraphProto.output
	valueInfoNameField    = 1  // ValueInfoProto.name
	tensorNameField       = 8  // TensorProto.name
)

// Protocol buffer wire types.
const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

var errMalformedModel = errors.New("malformed ONNX model")

// ParseIONames returns the name of the first graph input that is not an
// initializer (a weight) and the name of the first graph output of an ONNX
// model. Session.Run needs both names and the ONNX Runtime layer deliberately
// has no C API for reading them, so this walks the protocol buffer wire format
// of ModelProto.graph directly and never copies tensor data.
func ParseIONames(model []byte) (input, output string, err error) {
	graph, found, err := firstBytesField(model, modelGraphField)
	if err != nil {
		return "", "", err
	}
	if !found {
		return "", "", fmt.Errorf("%w: no graph", errMalformedModel)
	}
	initializers := map[string]struct{}{}
	var inputs []string
	var firstOutput string
	haveOutput := false
	err = walkFields(graph, func(field uint64, wire int, data []byte, _ uint64) error {
		if wire != wireBytes {
			return nil
		}
		switch field {
		case graphInitializerField:
			name, ok, err := firstBytesField(data, tensorNameField)
			if err != nil {
				return err
			}
			if ok {
				initializers[string(name)] = struct{}{}
			}
		case graphInputField:
			name, ok, err := firstBytesField(data, valueInfoNameField)
			if err != nil {
				return err
			}
			if ok {
				inputs = append(inputs, string(name))
			}
		case graphOutputField:
			if haveOutput {
				return nil
			}
			name, ok, err := firstBytesField(data, valueInfoNameField)
			if err != nil {
				return err
			}
			if ok {
				firstOutput, haveOutput = string(name), true
			}
		}
		return nil
	})
	if err != nil {
		return "", "", err
	}
	for _, name := range inputs {
		if _, weight := initializers[name]; !weight && name != "" {
			input = name
			break
		}
	}
	if input == "" {
		return "", "", fmt.Errorf("%w: no graph input besides the initializers", errMalformedModel)
	}
	if firstOutput == "" {
		return "", "", fmt.Errorf("%w: no graph output", errMalformedModel)
	}
	return input, firstOutput, nil
}

// firstBytesField returns the payload of the first length-delimited field
// with the given number.
func firstBytesField(message []byte, number uint64) ([]byte, bool, error) {
	var result []byte
	found := false
	err := walkFields(message, func(field uint64, wire int, data []byte, _ uint64) error {
		if field == number && wire == wireBytes && !found {
			result, found = data, true
		}
		return nil
	})
	return result, found, err
}

// walkFields calls visit for every field of a message in wire order: data is
// the payload of a length-delimited field, value the number of a varint,
// fixed32 or fixed64 field.
func walkFields(message []byte, visit func(field uint64, wire int, data []byte, value uint64) error) error {
	for len(message) > 0 {
		tag, n := readVarint(message)
		if n <= 0 {
			return fmt.Errorf("%w: bad field tag", errMalformedModel)
		}
		message = message[n:]
		field, wire := tag>>3, int(tag&7)
		if field == 0 {
			return fmt.Errorf("%w: field number 0", errMalformedModel)
		}
		var data []byte
		var value uint64
		switch wire {
		case wireVarint:
			value, n = readVarint(message)
			if n <= 0 {
				return fmt.Errorf("%w: bad varint", errMalformedModel)
			}
			message = message[n:]
		case wireFixed64:
			if len(message) < 8 {
				return fmt.Errorf("%w: truncated fixed64", errMalformedModel)
			}
			message = message[8:]
		case wireFixed32:
			if len(message) < 4 {
				return fmt.Errorf("%w: truncated fixed32", errMalformedModel)
			}
			message = message[4:]
		case wireBytes:
			length, n := readVarint(message)
			if n <= 0 || length > uint64(len(message)-n) {
				return fmt.Errorf("%w: truncated field %d", errMalformedModel, field)
			}
			data = message[n : n+int(length)]
			message = message[n+int(length):]
		default:
			return fmt.Errorf("%w: unsupported wire type %d", errMalformedModel, wire)
		}
		if err := visit(field, wire, data, value); err != nil {
			return err
		}
	}
	return nil
}

// readVarint decodes a base-128 varint; n is the byte count, or <= 0 for a
// truncated or over-long (> 10 bytes) value.
func readVarint(b []byte) (value uint64, n int) {
	for i := 0; i < len(b) && i < 10; i++ {
		value |= uint64(b[i]&0x7f) << (7 * uint(i))
		if b[i] < 0x80 {
			return value, i + 1
		}
	}
	return 0, 0
}
