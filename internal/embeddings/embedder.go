package embeddings

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	LocalGraniteProvider = "local-granite"
	GraniteModelID       = "ibm-granite/granite-embedding-97m-multilingual-r2"
	GraniteDimensions    = 384
)

// Embedder is the runtime-neutral embedding contract used by memory, the
// knowledge graph, connection tests, and local runtime supervision.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	ModelID() string
	Fingerprint() string
	Status() Status
	Close() error
}

// BenchmarkResult records one isolated backend probe.
type BenchmarkResult struct {
	Candidate       string  `json:"candidate"`
	Runtime         string  `json:"runtime"`
	Backend         string  `json:"backend"`
	GPU             bool    `json:"gpu"`
	GPUVerified     bool    `json:"gpu_verified"`
	Valid           bool    `json:"valid"`
	Skipped         bool    `json:"skipped"`
	LatencyMS       float64 `json:"latency_ms,omitempty"`
	CosineReference float64 `json:"cosine_reference,omitempty"`
	Error           string  `json:"error,omitempty"`
}

// DownloadStatus is safe to expose through the local status API.
type DownloadStatus struct {
	Asset      string  `json:"asset,omitempty"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
}

// Status describes setup, selection, health, and fallback state.
type Status struct {
	State               string            `json:"state"`
	Provider            string            `json:"provider"`
	ModelID             string            `json:"model_id"`
	Dimensions          int               `json:"dimensions"`
	Backend             string            `json:"backend,omitempty"`
	Runtime             string            `json:"runtime,omitempty"`
	RuntimeBuild        string            `json:"runtime_build,omitempty"`
	GPU                 bool              `json:"gpu"`
	GPUVerified         bool              `json:"gpu_verified"`
	Download            DownloadStatus    `json:"download"`
	Benchmark           []BenchmarkResult `json:"benchmark,omitempty"`
	FallbackReason      string            `json:"fallback_reason,omitempty"`
	Error               string            `json:"error,omitempty"`
	RestartRequired     bool              `json:"restart_required"`
	Fingerprint         string            `json:"fingerprint,omitempty"`
	HardwareFingerprint string            `json:"hardware_fingerprint,omitempty"`
	Cached              bool              `json:"cached"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

func initialStatus() Status {
	return Status{
		State:      "setting_up",
		Provider:   LocalGraniteProvider,
		ModelID:    GraniteModelID,
		Dimensions: GraniteDimensions,
		UpdatedAt:  time.Now().UTC(),
	}
}

func cloneStatus(status Status) Status {
	status.Benchmark = append([]BenchmarkResult(nil), status.Benchmark...)
	return status
}

// FuncEmbedder adapts an existing single-text embedding function to Embedder.
// It keeps all existing OpenAI-compatible and Ollama providers supported.
type FuncEmbedder struct {
	mu          sync.RWMutex
	embed       func(context.Context, string) ([]float32, error)
	modelID     string
	fingerprint string
	dimensions  int
	closed      bool
}

func NewFuncEmbedder(
	embed func(context.Context, string) ([]float32, error),
	modelID string,
	fingerprint string,
) *FuncEmbedder {
	return &FuncEmbedder{
		embed:       embed,
		modelID:     modelID,
		fingerprint: fingerprint,
	}
}

func (e *FuncEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return nil, fmt.Errorf("embedder is closed")
	}
	embed := e.embed
	e.mu.RUnlock()

	if embed == nil {
		return nil, fmt.Errorf("embedding function is not configured")
	}
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vector, err := embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		if err := validateFiniteVector(vector); err != nil {
			return nil, fmt.Errorf("validate text %d: %w", i, err)
		}
		result[i] = vector
		e.mu.Lock()
		if e.dimensions == 0 {
			e.dimensions = len(vector)
		}
		e.mu.Unlock()
	}
	return result, nil
}

func (e *FuncEmbedder) Dimensions() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.dimensions
}

func (e *FuncEmbedder) ModelID() string {
	return e.modelID
}

func (e *FuncEmbedder) Fingerprint() string {
	return e.fingerprint
}

func (e *FuncEmbedder) Status() Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	state := "ready"
	if e.closed {
		state = "closed"
	}
	return Status{
		State:       state,
		Provider:    "configured",
		ModelID:     e.modelID,
		Dimensions:  e.dimensions,
		Fingerprint: e.fingerprint,
		UpdatedAt:   time.Now().UTC(),
	}
}

func (e *FuncEmbedder) Close() error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	return nil
}

func validateFiniteVector(vector []float32) error {
	if len(vector) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	for i, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding contains a non-finite value at index %d", i)
		}
	}
	return nil
}

func validateGraniteVector(vector []float32) error {
	if len(vector) != GraniteDimensions {
		return fmt.Errorf("embedding dimensions = %d, want %d", len(vector), GraniteDimensions)
	}
	if err := validateFiniteVector(vector); err != nil {
		return err
	}
	var normSquared float64
	for _, value := range vector {
		normSquared += float64(value) * float64(value)
	}
	norm := math.Sqrt(normSquared)
	if math.Abs(norm-1) > 0.001 {
		return fmt.Errorf("embedding norm = %.6f, want 1 ± 0.001", norm)
	}
	return nil
}

func l2Normalize(vector []float32) error {
	var normSquared float64
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding contains a non-finite value")
		}
		normSquared += float64(value) * float64(value)
	}
	if normSquared == 0 {
		return fmt.Errorf("embedding has zero norm")
	}
	inverse := float32(1 / math.Sqrt(normSquared))
	for i := range vector {
		vector[i] *= inverse
	}
	return nil
}

func cosineSimilarity(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	var dot, leftNorm, rightNorm float64
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
