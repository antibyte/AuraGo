package embeddings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"aurago/internal/onnxrt"
)

// ortSession adapts the shared ONNX Runtime binding to the Granite embedding
// model: int64 token tensors in, L2-normalized CLS rows of last_hidden_state
// out. Hardware backends are verified through ORT execution profiling.
type ortSession struct {
	runtime        *onnxrt.Runtime
	session        *onnxrt.Session
	providers      []string
	backend        string
	verified       bool
	version        string
	profileEnabled bool
	profileEnded   bool
}

func newORTSession(runtimeRoot, modelFile, backend string) (_ *ortSession, resultErr error) {
	rt, err := onnxrt.Load(runtimeRoot, "aurago-embeddings")
	if err != nil {
		return nil, err
	}
	session := &ortSession{runtime: rt, backend: backend, version: rt.Version()}
	defer func() {
		if resultErr != nil {
			_ = session.Close()
		}
	}()
	providers, err := rt.Providers()
	if err != nil {
		return nil, err
	}
	session.providers = providers

	threads := goruntime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	options := onnxrt.SessionOptions{Threads: threads, Optimization: onnxrt.OptimizationAll}
	if backend != "cpu" {
		provider := onnxProviderName(backend)
		if provider == "" {
			return nil, fmt.Errorf("unsupported ONNX backend %q", backend)
		}
		options.Provider = provider
		options.ProfilePrefix = filepath.Join(
			os.TempDir(),
			fmt.Sprintf("aurago-ort-%d-%d", os.Getpid(), time.Now().UnixNano()),
		)
	}
	modelData, err := os.ReadFile(modelFile)
	if err != nil {
		return nil, fmt.Errorf("read ONNX model: %w", err)
	}
	if len(modelData) == 0 {
		return nil, fmt.Errorf("ONNX model is empty")
	}
	created, err := rt.NewSession(modelData, options)
	if err != nil {
		return nil, err
	}
	session.session = created
	session.profileEnabled = options.ProfilePrefix != ""
	return session, nil
}

func (session *ortSession) Embed(batch tokenBatch) ([][]float32, error) {
	if session == nil || session.session == nil {
		return nil, fmt.Errorf("ONNX session is closed")
	}
	shape := []int64{int64(batch.BatchSize), int64(batch.SequenceSize)}
	outputs, err := session.session.Run(map[string]onnxrt.Tensor{
		"input_ids":      {Shape: shape, Int64: batch.InputIDs},
		"attention_mask": {Shape: shape, Int64: batch.AttentionMask},
	}, []string{"last_hidden_state"})
	if err != nil {
		return nil, fmt.Errorf("run ONNX inference: %w", err)
	}
	vectors, err := readCLSOutput(outputs["last_hidden_state"], batch.BatchSize, batch.SequenceSize)
	if err != nil {
		return nil, err
	}
	if session.profileEnabled && !session.profileEnded {
		session.verified = session.verifyGPUExecution()
	}
	return vectors, nil
}

func (session *ortSession) verifyGPUExecution() bool {
	session.profileEnded = true
	path, err := session.session.EndProfiling()
	if err != nil || path == "" {
		return false
	}
	defer os.Remove(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var events []struct {
		Category string         `json:"cat"`
		Args     map[string]any `json:"args"`
	}
	if json.Unmarshal(raw, &events) != nil {
		return false
	}
	expectedProvider := onnxProviderName(session.backend)
	for _, event := range events {
		provider, _ := event.Args["provider"].(string)
		if strings.EqualFold(provider, expectedProvider) &&
			(strings.EqualFold(event.Category, "Node") || event.Category == "") {
			return true
		}
	}
	return false
}

func readCLSOutput(output onnxrt.Tensor, batchSize, sequenceSize int) ([][]float32, error) {
	if output.Float32 == nil {
		return nil, fmt.Errorf("ONNX output element type is not float32")
	}
	dimensions := output.Shape
	data := output.Float32
	result := make([][]float32, batchSize)
	switch {
	case len(dimensions) == 3 &&
		int(dimensions[0]) == batchSize &&
		int(dimensions[1]) == sequenceSize &&
		int(dimensions[2]) == GraniteDimensions:
		rowSize := sequenceSize * GraniteDimensions
		for batchIndex := 0; batchIndex < batchSize; batchIndex++ {
			start := batchIndex * rowSize
			result[batchIndex] = append([]float32(nil), data[start:start+GraniteDimensions]...)
		}
	case len(dimensions) == 2 &&
		int(dimensions[0]) == batchSize &&
		int(dimensions[1]) == GraniteDimensions:
		for batchIndex := 0; batchIndex < batchSize; batchIndex++ {
			start := batchIndex * GraniteDimensions
			result[batchIndex] = append([]float32(nil), data[start:start+GraniteDimensions]...)
		}
	default:
		return nil, fmt.Errorf("unexpected ONNX output shape %v", dimensions)
	}
	for i := range result {
		if err := l2Normalize(result[i]); err != nil {
			return nil, fmt.Errorf("normalize output %d: %w", i, err)
		}
	}
	return result, nil
}

func (session *ortSession) Close() error {
	if session == nil {
		return nil
	}
	if session.session != nil && session.profileEnabled && !session.profileEnded {
		_ = session.verifyGPUExecution()
	}
	var closeErr error
	if session.session != nil {
		closeErr = session.session.Close()
		session.session = nil
	}
	if session.runtime != nil {
		if err := session.runtime.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
		session.runtime = nil
	}
	return closeErr
}

func onnxProviderName(backend string) string {
	switch backend {
	case "cuda":
		return onnxrt.ProviderCUDA
	case "coreml":
		return onnxrt.ProviderCoreML
	default:
		return ""
	}
}
