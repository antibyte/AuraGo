package persondetect

import (
	"fmt"
	"runtime"

	"aurago/internal/onnxrt"
)

// engine runs D-FINE on preprocessed pixels. The serve loop and the self-test
// only see this interface, so they are testable without ONNX Runtime.
type engine interface {
	Run(pixels []float32, size int) (logits, boxes onnxrt.Tensor, err error)
	Close() error
}

type ortEngine struct {
	session *onnxrt.Session
}

func (e *ortEngine) Run(pixels []float32, size int) (onnxrt.Tensor, onnxrt.Tensor, error) {
	outputs, err := e.session.Run(map[string]onnxrt.Tensor{
		dfineInputName: {Shape: []int64{1, 3, int64(size), int64(size)}, Float32: pixels},
	}, []string{dfineLogitsName, dfineBoxesName})
	if err != nil {
		return onnxrt.Tensor{}, onnxrt.Tensor{}, err
	}
	return outputs[dfineLogitsName], outputs[dfineBoxesName], nil
}

func (e *ortEngine) Close() error {
	return e.session.Close()
}

// detectPixels runs the engine and decodes person boxes with score ≥ minScore.
func detectPixels(eng engine, pixels []float32, size int, minScore float64) ([]Box, error) {
	logits, boxes, err := eng.Run(pixels, size)
	if err != nil {
		return nil, err
	}
	queries, classes, err := outputLayout(logits, boxes)
	if err != nil {
		return nil, err
	}
	return DecodePersons(logits.Float32, boxes.Float32, queries, classes, dfinePersonClass, minScore), nil
}

// outputLayout validates logits [1,Q,C] and boxes [1,Q,4] and returns Q and C.
func outputLayout(logits, boxes onnxrt.Tensor) (queries, classes int, err error) {
	if logits.Float32 == nil || boxes.Float32 == nil {
		return 0, 0, fmt.Errorf("D-FINE outputs must be float32")
	}
	if len(logits.Shape) != 3 || len(boxes.Shape) != 3 || logits.Shape[0] != 1 || boxes.Shape[0] != 1 ||
		boxes.Shape[2] != 4 || boxes.Shape[1] != logits.Shape[1] {
		return 0, 0, fmt.Errorf("unexpected D-FINE output shapes %v and %v", logits.Shape, boxes.Shape)
	}
	queries, classes = int(logits.Shape[1]), int(logits.Shape[2])
	if queries <= 0 || classes <= dfinePersonClass || len(logits.Float32) != queries*classes || len(boxes.Float32) != queries*4 {
		return 0, 0, fmt.Errorf("D-FINE outputs do not match shapes %v and %v", logits.Shape, boxes.Shape)
	}
	return queries, classes, nil
}

// cpuThreads is the CPU path's intra-op thread count: min(4, max(1, NumCPU/2)).
func cpuThreads() int {
	threads := runtime.NumCPU() / 2
	if threads < 1 {
		threads = 1
	}
	if threads > 4 {
		threads = 4
	}
	return threads
}
