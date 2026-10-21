package pixelmatte

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func pbVarint(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

func pbTag(field, wire int) []byte { return pbVarint(uint64(field<<3 | wire)) }

func pbBytes(field int, payload []byte) []byte {
	out := append(pbTag(field, wireBytes), pbVarint(uint64(len(payload)))...)
	return append(out, payload...)
}

func pbString(field int, text string) []byte { return pbBytes(field, []byte(text)) }

func pbInt(field int, value uint64) []byte {
	return append(pbTag(field, wireVarint), pbVarint(value)...)
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// handBuiltModel is a ModelProto with an initializer that is also listed as a
// graph input (IR version 3 style), a real input, and two outputs.
func handBuiltModel() []byte {
	weight := concat(pbInt(1, 64), pbInt(2, 1), pbString(8, "weight"), pbBytes(9, make([]byte, 300)))
	graph := concat(
		pbBytes(1, concat(pbString(1, "x"), pbString(2, "conv"), pbString(4, "Conv"))), // node
		pbString(2, "main"),
		pbBytes(5, weight),
		pbBytes(11, pbString(1, "weight")),
		pbBytes(11, concat(pbString(1, "img"), pbBytes(2, []byte{0x0a, 0x00}))),
		pbBytes(12, pbString(1, "out0")),
		pbBytes(12, pbString(1, "out1")),
	)
	return concat(
		pbInt(1, 8),
		pbString(2, "test"),
		append(pbTag(20, wireFixed32), 1, 2, 3, 4),
		append(pbTag(21, wireFixed64), 1, 2, 3, 4, 5, 6, 7, 8),
		pbBytes(modelGraphField, graph),
	)
}

func TestParseIONamesSkipsInitializersAndTakesFirstOutput(t *testing.T) {
	input, output, err := ParseIONames(handBuiltModel())
	if err != nil {
		t.Fatalf("ParseIONames: %v", err)
	}
	if input != "img" || output != "out0" {
		t.Fatalf("names = %q, %q; want img, out0", input, output)
	}
}

func TestParseIONamesRejectsMalformedModels(t *testing.T) {
	valid := handBuiltModel()
	onlyWeights := concat(pbBytes(modelGraphField, concat(
		pbBytes(5, pbString(8, "w")), pbBytes(11, pbString(1, "w")), pbBytes(12, pbString(1, "y")))))
	noOutput := concat(pbBytes(modelGraphField, pbBytes(11, pbString(1, "x"))))
	cases := map[string][]byte{
		"empty":            nil,
		"no graph":         pbInt(1, 8),
		"truncated":        valid[:len(valid)-3],
		"group wire type":  append(pbTag(3, 3), valid...),
		"field zero":       {0x00, 0x01},
		"over-long varint": {0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
		"only initializer": onlyWeights,
		"no output":        noOutput,
	}
	for name, model := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := ParseIONames(model); !errors.Is(err, errMalformedModel) {
				t.Fatalf("ParseIONames error = %v, want errMalformedModel", err)
			}
		})
	}
}

// TestPinnedModelTensorNames reads the real model files when
// AURAGO_PIXELMATTE_MODELS points at a directory holding them (the download
// names, not the cache names) and logs what ONNX Runtime has to be fed.
func TestPinnedModelTensorNames(t *testing.T) {
	dir := os.Getenv("AURAGO_PIXELMATTE_MODELS")
	if dir == "" {
		t.Skip("set AURAGO_PIXELMATTE_MODELS to a directory with the pinned .onnx files")
	}
	files := map[Kind]string{
		KindStandard: "isnet-general-use.onnx",
		KindPrecise:  "BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx",
	}
	for kind, name := range files {
		t.Run(string(kind), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Skipf("model not available: %v", err)
			}
			input, output, err := ParseIONames(raw)
			if err != nil || input == "" || output == "" {
				t.Fatalf("ParseIONames = %q, %q, %v", input, output, err)
			}
			t.Logf("%s: input %q, first output %q", kind, input, output)
		})
	}
}
