package persondetect

import (
	"fmt"
	"runtime"
	"slices"

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

// outputLayout requires exactly the pinned D-FINE outputs, logits
// [1,300,80] and boxes [1,300,4] with matching float32 data, and returns the
// query and class counts.
func outputLayout(logits, boxes onnxrt.Tensor) (queries, classes int, err error) {
	if logits.Float32 == nil || boxes.Float32 == nil {
		return 0, 0, fmt.Errorf("D-FINE outputs must be float32")
	}
	if !slices.Equal(logits.Shape, []int64{1, dfineQueries, dfineClasses}) || !slices.Equal(boxes.Shape, []int64{1, dfineQueries, 4}) {
		return 0, 0, fmt.Errorf("unexpected D-FINE output shapes %v and %v (want [1 %d %d] and [1 %d 4])",
			logits.Shape, boxes.Shape, dfineQueries, dfineClasses, dfineQueries)
	}
	if len(logits.Float32) != dfineQueries*dfineClasses || len(boxes.Float32) != dfineQueries*4 {
		return 0, 0, fmt.Errorf("D-FINE outputs hold %d and %d values for shapes %v and %v",
			len(logits.Float32), len(boxes.Float32), logits.Shape, boxes.Shape)
	}
	return dfineQueries, dfineClasses, nil
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
