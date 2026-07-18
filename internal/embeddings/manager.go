package embeddings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type LocalOptions struct {
	CacheDir        string
	ResetMarkerPath string
	Backend         string
	ContextSize     int
	BatchSize       int
	Logger          *slog.Logger
}

type selectionState struct {
	Version              int               `json:"version"`
	SelectionFingerprint string            `json:"selection_fingerprint"`
	HardwareFingerprint  string            `json:"hardware_fingerprint"`
	CandidateID          string            `json:"candidate_id"`
	EmbeddingFingerprint string            `json:"embedding_fingerprint"`
	Benchmark            []BenchmarkResult `json:"benchmark"`
	SelectedAt           time.Time         `json:"selected_at"`
}

type resetMarker struct {
	RequestedAt time.Time `json:"requested_at"`
	Reason      string    `json:"reason"`
}

type Manager struct {
	options LocalOptions
	cache   *assetCache
	logger  *slog.Logger

	operationMu sync.Mutex
	mu          sync.RWMutex
	active      Embedder
	fallback    Embedder
	status      Status
	failures    int
	closed      bool

	cancel    context.CancelFunc
	initWG    sync.WaitGroup
	ready     chan struct{}
	readyOnce sync.Once
}

func NewLocalGranite(options LocalOptions) (*Manager, error) {
	if strings.TrimSpace(options.CacheDir) == "" {
		return nil, fmt.Errorf("local Granite cache directory is required")
	}
	if options.ContextSize <= 0 {
		options.ContextSize = 2048
	}
	if options.ContextSize > 32_768 {
		return nil, fmt.Errorf("local Granite context size %d exceeds model limit 32768", options.ContextSize)
	}
	if options.BatchSize <= 0 {
		options.BatchSize = 2048
	}
	options.Backend = strings.ToLower(strings.TrimSpace(options.Backend))
	if options.Backend == "" {
		options.Backend = "auto"
	}
	if !validBackend(options.Backend) {
		return nil, fmt.Errorf("unsupported local Granite backend %q", options.Backend)
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}

	manager := &Manager{
		options: options,
		logger:  options.Logger,
		status:  initialStatus(),
		ready:   make(chan struct{}),
	}
	manager.status.Fingerprint = onnxFingerprint()
	manager.cache = newAssetCache(options.CacheDir, manager.updateDownload)
	ctx, cancel := context.WithCancel(context.Background())
	manager.cancel = cancel
	manager.initWG.Add(1)
	go func() {
		defer manager.initWG.Done()
		err := manager.setup(ctx, false)
		manager.mu.Lock()
		if err != nil && !manager.closed {
			manager.status.State = "error"
			manager.status.Error = err.Error()
			manager.status.UpdatedAt = time.Now().UTC()
		}
		manager.mu.Unlock()
		manager.readyOnce.Do(func() { close(manager.ready) })
		if err != nil && !errors.Is(err, context.Canceled) {
			manager.logger.Warn("Local Granite embeddings setup failed; AuraGo will continue without local embeddings", "error", err)
		}
	}()
	return manager, nil
}

func (manager *Manager) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-manager.ready:
	}

	manager.mu.RLock()
	if manager.closed {
		manager.mu.RUnlock()
		return nil, fmt.Errorf("local Granite embedder is closed")
	}
	active := manager.active
	fallback := manager.fallback
	restartRequired := manager.status.RestartRequired
	manager.mu.RUnlock()
	if restartRequired {
		return nil, fmt.Errorf("local Granite runtime changed; restart AuraGo to apply the scheduled embedding reindex")
	}
	if active == nil {
		status := manager.Status()
		if status.RestartRequired {
			return nil, fmt.Errorf("local Granite runtime changed; restart AuraGo to apply the scheduled embedding reindex")
		}
		if status.Error != "" {
			return nil, fmt.Errorf("local Granite embeddings are unavailable: %s", status.Error)
		}
		return nil, fmt.Errorf("local Granite embeddings are still being set up")
	}

	vectors, err := active.Embed(ctx, texts)
	if err == nil {
		manager.mu.Lock()
		manager.failures = 0
		manager.mu.Unlock()
		return vectors, nil
	}
	manager.logger.Warn("Selected local embedding backend failed", "backend", active.Status().Backend, "runtime", active.Status().Runtime, "error", err)
	if fallback != nil && fallback != active {
		fallbackVectors, fallbackErr := fallback.Embed(ctx, texts)
		if fallbackErr == nil {
			if fallback.Fingerprint() != active.Fingerprint() {
				if scheduleErr := manager.scheduleCrossFormatFallback(ctx, active, fallback); scheduleErr != nil {
					return nil, fmt.Errorf("selected backend failed: %v; safe CPU fallback could not be scheduled: %w", err, scheduleErr)
				}
				return nil, fmt.Errorf("selected backend failed and the ONNX CPU fallback uses a different vector fingerprint; restart AuraGo to apply the scheduled controlled reindex")
			}
			manager.mu.Lock()
			manager.failures++
			manager.status.State = "degraded"
			manager.status.FallbackReason = fmt.Sprintf("%s/%s failed: %v", active.Status().Runtime, active.Status().Backend, err)
			manager.status.Error = ""
			manager.status.UpdatedAt = time.Now().UTC()
			failures := manager.failures
			manager.mu.Unlock()
			if failures >= 2 {
				manager.triggerBackgroundBenchmark()
			}
			return fallbackVectors, nil
		}
		return nil, fmt.Errorf("selected backend failed: %v; ONNX CPU fallback failed: %w", err, fallbackErr)
	}
	return nil, err
}

func (manager *Manager) scheduleCrossFormatFallback(ctx context.Context, active, fallback Embedder) error {
	state, err := manager.loadSelection()
	if err != nil {
		hardware := detectHardware(ctx)
		state = selectionState{
			Version:              1,
			SelectionFingerprint: manager.selectionFingerprint(hardware.Fingerprint),
			HardwareFingerprint:  hardware.Fingerprint,
		}
	}
	state.CandidateID = "onnx-cpu"
	state.EmbeddingFingerprint = fallback.Fingerprint()
	state.SelectedAt = time.Now().UTC()
	if err := manager.scheduleReset("local_granite_runtime_failure_to_onnx_cpu"); err != nil {
		return err
	}
	if err := manager.saveSelection(state); err != nil {
		return fmt.Errorf("persist ONNX CPU fallback selection: %w", err)
	}
	manager.mu.RLock()
	results := append([]BenchmarkResult(nil), manager.status.Benchmark...)
	manager.mu.RUnlock()
	manager.requireRestart(results, active.Fingerprint(), fallback.Fingerprint())
	return nil
}

func (manager *Manager) Dimensions() int {
	return GraniteDimensions
}

func (manager *Manager) ModelID() string {
	return GraniteModelID
}

func (manager *Manager) Fingerprint() string {
	manager.mu.RLock()
	active := manager.active
	fingerprint := manager.status.Fingerprint
	manager.mu.RUnlock()
	if active != nil {
		return active.Fingerprint()
	}
	if fingerprint != "" {
		return fingerprint
	}
	return onnxFingerprint()
}

func (manager *Manager) Status() Status {
	manager.mu.RLock()
	status := cloneStatus(manager.status)
	active := manager.active
	manager.mu.RUnlock()
	if active == nil {
		return status
	}
	activeStatus := active.Status()
	status.Backend = activeStatus.Backend
	status.Runtime = activeStatus.Runtime
	status.RuntimeBuild = activeStatus.RuntimeBuild
	status.GPU = activeStatus.GPU
	status.GPUVerified = activeStatus.GPUVerified
	status.Fingerprint = active.Fingerprint()
	if status.State == "setting_up" || status.State == "benchmarking" {
		return status
	}
	if status.State != "degraded" && status.State != "error" && status.State != "restart_required" {
		status.State = activeStatus.State
	}
	return status
}

func (manager *Manager) Close() error {
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil
	}
	manager.closed = true
	if manager.cancel != nil {
		manager.cancel()
	}
	manager.mu.Unlock()
	manager.initWG.Wait()

	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	manager.mu.Lock()
	active := manager.active
	fallback := manager.fallback
	manager.active = nil
	manager.fallback = nil
	manager.status.State = "closed"
	manager.status.UpdatedAt = time.Now().UTC()
	manager.mu.Unlock()
	var closeErrors []error
	if active != nil {
		if err := active.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if fallback != nil && fallback != active {
		if err := fallback.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func (manager *Manager) Rebenchmark(ctx context.Context) error {
	manager.mu.RLock()
	closed := manager.closed
	manager.mu.RUnlock()
	if closed {
		return fmt.Errorf("local Granite embedder is closed")
	}
	return manager.setup(ctx, true)
}

func (manager *Manager) setup(ctx context.Context, forceBenchmark bool) error {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	manager.mu.RLock()
	if manager.closed {
		manager.mu.RUnlock()
		return context.Canceled
	}
	manager.mu.RUnlock()

	manager.setState("setting_up", "")
	hardware := detectHardware(ctx)
	candidates := candidateMatrix(runtime.GOOS, runtime.GOARCH, manager.options.Backend, hardware)
	if len(candidates) == 0 {
		return fmt.Errorf("no local embedding runtime is available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	manager.mu.Lock()
	manager.status.HardwareFingerprint = hardware.Fingerprint
	manager.mu.Unlock()

	selectionFingerprint := manager.selectionFingerprint(hardware.Fingerprint)
	previousState, previousStateErr := manager.loadSelection()
	if !forceBenchmark {
		if previousStateErr == nil &&
			previousState.SelectionFingerprint == selectionFingerprint &&
			previousState.HardwareFingerprint == hardware.Fingerprint {
			if selected, ok := findCandidate(candidates, previousState.CandidateID); ok {
				manager.logger.Info("Using cached local embedding benchmark selection", "candidate", selected.ID)
				activationErr := manager.activateCandidate(ctx, selected, previousState.Benchmark, true)
				if activationErr == nil {
					return nil
				}
				manager.logger.Warn("Cached local embedding backend could not be started; benchmarking again", "candidate", selected.ID, "error", activationErr)
			}
		}
	}

	manager.setState("benchmarking", "")
	selected, results, err := manager.runBenchmark(ctx, candidates)
	if err != nil {
		return err
	}
	state := selectionState{
		Version:              1,
		SelectionFingerprint: selectionFingerprint,
		HardwareFingerprint:  hardware.Fingerprint,
		CandidateID:          selected.ID,
		EmbeddingFingerprint: candidateFingerprint(selected),
		Benchmark:            results,
		SelectedAt:           time.Now().UTC(),
	}
	formatChanged := previousStateErr == nil &&
		previousState.EmbeddingFingerprint != "" &&
		previousState.EmbeddingFingerprint != state.EmbeddingFingerprint
	if formatChanged {
		reason := fmt.Sprintf(
			"local_granite_runtime_change:%s_to_%s",
			previousState.CandidateID,
			state.CandidateID,
		)
		if err := manager.scheduleReset(reason); err != nil {
			return fmt.Errorf("schedule embedding reindex after runtime change: %w", err)
		}
		if err := manager.saveSelection(state); err != nil {
			return fmt.Errorf("persist embedding selection after scheduling reindex: %w", err)
		}
		manager.requireRestart(results, previousState.EmbeddingFingerprint, state.EmbeddingFingerprint)
		return nil
	}
	if err := manager.saveSelection(state); err != nil {
		manager.logger.Warn("Could not persist local embedding benchmark result", "error", err)
	}
	if err := manager.activateCandidate(ctx, selected, results, false); err != nil {
		return err
	}
	manager.cleanupUnusedGPURuntimes(selected, candidates)
	manager.cleanupUnusedDockerGPURuntimes(ctx, selected)
	return nil
}

func (manager *Manager) runBenchmark(ctx context.Context, candidates []candidate) (candidate, []BenchmarkResult, error) {
	referenceCandidate, ok := findCandidate(candidates, "onnx-cpu")
	if !ok || !referenceCandidate.HasAsset {
		// macOS amd64 has no official ONNX Runtime 1.26 archive. It remains
		// supported through llama.cpp CPU, but cannot run a cross-runtime
		// cosine comparison.
		for _, current := range candidates {
			if current.Runtime == "llama.cpp" && current.Backend == "cpu" && current.HasAsset {
				result, _, err := manager.probeCandidate(ctx, current, nil)
				if err != nil {
					return candidate{}, []BenchmarkResult{result}, err
				}
				return current, []BenchmarkResult{result}, nil
			}
		}
		return candidate{}, nil, fmt.Errorf("ONNX CPU reference runtime is unavailable for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	referenceResult, referenceVectors, err := manager.probeCandidate(ctx, referenceCandidate, nil)
	results := []BenchmarkResult{referenceResult}
	if err != nil {
		return candidate{}, results, fmt.Errorf("ONNX CPU reference failed: %w", err)
	}
	validCandidates := []struct {
		candidate candidate
		result    BenchmarkResult
	}{{candidate: referenceCandidate, result: referenceResult}}

	for _, current := range candidates {
		if current.ID == referenceCandidate.ID {
			continue
		}
		result, _, probeErr := manager.probeCandidate(ctx, current, referenceVectors)
		results = append(results, result)
		if probeErr == nil && result.Valid {
			validCandidates = append(validCandidates, struct {
				candidate candidate
				result    BenchmarkResult
			}{candidate: current, result: result})
		}
	}
	selected, ok := selectBenchmarkWinner(validCandidates)
	if !ok {
		return candidate{}, results, fmt.Errorf("no local embedding backend passed validation")
	}
	return selected, results, nil
}

func selectBenchmarkWinner(validCandidates []struct {
	candidate candidate
	result    BenchmarkResult
}) (candidate, bool) {
	if len(validCandidates) == 0 {
		return candidate{}, false
	}
	sort.SliceStable(validCandidates, func(i, j int) bool {
		if validCandidates[i].candidate.GPU != validCandidates[j].candidate.GPU {
			return validCandidates[i].candidate.GPU
		}
		return validCandidates[i].result.LatencyMS < validCandidates[j].result.LatencyMS
	})
	return validCandidates[0].candidate, true
}

func (manager *Manager) probeCandidate(ctx context.Context, current candidate, reference [][]float32) (BenchmarkResult, [][]float32, error) {
	result := BenchmarkResult{
		Candidate: current.ID,
		Runtime:   current.Runtime,
		Backend:   current.Backend,
		GPU:       current.GPU,
	}
	if !current.HasAsset {
		result.Skipped = true
		result.Error = "no pinned runtime archive for this platform"
		return result, nil, fmt.Errorf("%s", result.Error)
	}
	probe, err := manager.startCandidate(ctx, current)
	if err != nil {
		result.Error = err.Error()
		return result, nil, err
	}
	defer probe.Close()

	inputs := benchmarkInputs()
	if _, err := probe.Embed(ctx, inputs); err != nil {
		result.Error = fmt.Sprintf("warmup: %v", err)
		return result, nil, err
	}
	const runs = 3
	var vectors [][]float32
	start := time.Now()
	for run := 0; run < runs; run++ {
		vectors, err = probe.Embed(ctx, inputs)
		if err != nil {
			result.Error = err.Error()
			return result, nil, err
		}
	}
	result.LatencyMS = float64(time.Since(start).Microseconds()) / 1000 / runs
	for i := range vectors {
		if err := validateGraniteVector(vectors[i]); err != nil {
			result.Error = err.Error()
			return result, nil, err
		}
	}
	probeStatus := probe.Status()
	result.GPUVerified = probeStatus.GPUVerified
	if current.GPU && !result.GPUVerified {
		result.Error = "runtime did not confirm actual GPU execution"
		return result, nil, fmt.Errorf("%s", result.Error)
	}
	if reference != nil {
		if len(reference) != len(vectors) {
			result.Error = "reference vector count mismatch"
			return result, nil, fmt.Errorf("%s", result.Error)
		}
		var cosine float64
		for i := range vectors {
			cosine += cosineSimilarity(reference[i], vectors[i])
		}
		result.CosineReference = cosine / float64(len(vectors))
		if result.CosineReference < 0.93 {
			result.Error = fmt.Sprintf("cosine similarity %.4f is below 0.93", result.CosineReference)
			return result, nil, fmt.Errorf("%s", result.Error)
		}
	} else {
		result.CosineReference = 1
	}
	result.Valid = true
	return result, vectors, nil
}

func (manager *Manager) startCandidate(ctx context.Context, current candidate) (Embedder, error) {
	if !current.HasAsset {
		return nil, fmt.Errorf("candidate %s has no pinned runtime asset", current.ID)
	}
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	switch current.Runtime {
	case "onnxruntime":
		runtimeRoot, err := manager.cache.ensureRuntimeAsset(ctx, current.Asset)
		if err != nil {
			return nil, fmt.Errorf("prepare %s runtime: %w", current.ID, err)
		}
		onnxPath := modelPath(manager.options.CacheDir, "onnx")
		if err := manager.cache.ensureDirectAsset(ctx, graniteONNXModelAsset, onnxPath); err != nil {
			return nil, err
		}
		tokenPath := tokenizerPath(manager.options.CacheDir)
		if err := manager.cache.ensureDirectAsset(ctx, graniteTokenizerAsset, tokenPath); err != nil {
			return nil, err
		}
		return newONNXWorkerEmbedder(startupCtx, onnxWorkerOptions{
			RuntimeRoot:   runtimeRoot,
			ModelPath:     onnxPath,
			TokenizerPath: tokenPath,
			Backend:       current.Backend,
			ContextSize:   manager.options.ContextSize,
		})
	case "llama.cpp":
		ggufPath := modelPath(manager.options.CacheDir, "gguf")
		if err := manager.cache.ensureDirectAsset(ctx, graniteGGUFModelAsset, ggufPath); err != nil {
			return nil, err
		}
		if dockerEmbeddingSidecarsAvailable() {
			sidecar, sidecarErr := newDockerLlamaEmbedder(
				startupCtx,
				os.Getenv("DOCKER_HOST"),
				ggufPath,
				current.Backend,
				manager.options.ContextSize,
				manager.options.BatchSize,
			)
			if sidecarErr == nil {
				return sidecar, nil
			}
			manager.logger.Warn("Managed llama.cpp Docker sidecar unavailable; trying the isolated native runtime",
				"backend", current.Backend,
				"error", sidecarErr)
		}
		runtimeRoot, err := manager.cache.ensureRuntimeAsset(ctx, current.Asset)
		if err != nil {
			return nil, fmt.Errorf("prepare %s runtime: %w", current.ID, err)
		}
		var supplementalRoots []string
		if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" && current.Backend == "cuda" {
			supplemental, ok := runtimeManifest["llama-windows-amd64-cuda-runtime"]
			if !ok {
				return nil, fmt.Errorf("pinned Windows CUDA runtime archive is missing")
			}
			supplementalRoot, supplementalErr := manager.cache.ensureRuntimeAsset(ctx, supplemental)
			if supplementalErr != nil {
				return nil, fmt.Errorf("prepare Windows CUDA runtime libraries: %w", supplementalErr)
			}
			supplementalRoots = append(supplementalRoots, supplementalRoot)
		}
		return newLlamaEmbedder(
			startupCtx,
			runtimeRoot,
			ggufPath,
			current.Backend,
			manager.options.ContextSize,
			manager.options.BatchSize,
			supplementalRoots...,
		)
	default:
		return nil, fmt.Errorf("unknown candidate runtime %q", current.Runtime)
	}
}

func (manager *Manager) activateCandidate(ctx context.Context, selected candidate, results []BenchmarkResult, cached bool) error {
	active, err := manager.startCandidate(ctx, selected)
	if err != nil {
		return fmt.Errorf("start selected backend %s: %w", selected.ID, err)
	}
	var fallback Embedder
	if selected.ID == "onnx-cpu" {
		fallback = active
	} else if fallbackCandidate, ok := findCandidate(
		candidateMatrix(runtime.GOOS, runtime.GOARCH, "auto", detectHardware(ctx)),
		"onnx-cpu",
	); ok && fallbackCandidate.HasAsset {
		fallback, err = manager.startCandidate(ctx, fallbackCandidate)
		if err != nil {
			manager.logger.Warn("ONNX CPU fallback could not be started", "error", err)
		}
	}

	manager.mu.Lock()
	oldActive := manager.active
	oldFallback := manager.fallback
	manager.active = active
	manager.fallback = fallback
	manager.failures = 0
	activeStatus := active.Status()
	manager.status.State = "ready"
	manager.status.Backend = activeStatus.Backend
	manager.status.Runtime = activeStatus.Runtime
	manager.status.RuntimeBuild = activeStatus.RuntimeBuild
	manager.status.GPU = activeStatus.GPU
	manager.status.GPUVerified = activeStatus.GPUVerified
	manager.status.Benchmark = append([]BenchmarkResult(nil), results...)
	manager.status.FallbackReason = ""
	manager.status.Error = ""
	manager.status.RestartRequired = false
	manager.status.Fingerprint = active.Fingerprint()
	manager.status.Cached = cached
	manager.status.UpdatedAt = time.Now().UTC()
	manager.mu.Unlock()
	if oldActive != nil && oldActive != active {
		_ = oldActive.Close()
	}
	if oldFallback != nil && oldFallback != oldActive && oldFallback != fallback {
		_ = oldFallback.Close()
	}
	manager.readyOnce.Do(func() { close(manager.ready) })
	manager.logger.Info("Local Granite embedding backend selected",
		"runtime", activeStatus.Runtime,
		"backend", activeStatus.Backend,
		"gpu", activeStatus.GPU,
		"gpu_verified", activeStatus.GPUVerified,
		"cached", cached)
	return nil
}

func (manager *Manager) selectionFingerprint(hardwareFingerprint string) string {
	return strings.Join([]string{
		"v1",
		hardwareFingerprint,
		"backend=" + manager.options.Backend,
		"context=" + fmt.Sprintf("%d", manager.options.ContextSize),
		"batch=" + fmt.Sprintf("%d", manager.options.BatchSize),
		"onnx=" + onnxFingerprint(),
		"gguf=" + ggufFingerprint(),
	}, "|")
}

func (manager *Manager) selectionPath() string {
	return filepath.Join(manager.options.CacheDir, "selection.json")
}

func (manager *Manager) loadSelection() (selectionState, error) {
	raw, err := os.ReadFile(manager.selectionPath())
	if err != nil {
		return selectionState{}, err
	}
	var state selectionState
	if err := json.Unmarshal(raw, &state); err != nil {
		return selectionState{}, fmt.Errorf("parse embedding selection: %w", err)
	}
	if state.Version != 1 || state.CandidateID == "" {
		return selectionState{}, fmt.Errorf("unsupported embedding selection state")
	}
	if state.EmbeddingFingerprint == "" {
		state.EmbeddingFingerprint = candidateFingerprint(candidate{
			ID:      state.CandidateID,
			Runtime: candidateRuntimeFromID(state.CandidateID),
		})
	}
	return state, nil
}

func (manager *Manager) saveSelection(state selectionState) error {
	if err := os.MkdirAll(manager.options.CacheDir, 0o750); err != nil {
		return fmt.Errorf("create embedding cache: %w", err)
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal embedding selection: %w", err)
	}
	tempPath := manager.selectionPath() + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0o600); err != nil {
		return fmt.Errorf("write embedding selection: %w", err)
	}
	if err := os.Remove(manager.selectionPath()); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace embedding selection: %w", err)
	}
	if err := os.Rename(tempPath, manager.selectionPath()); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("activate embedding selection: %w", err)
	}
	return nil
}

func (manager *Manager) updateDownload(download DownloadStatus) {
	manager.mu.Lock()
	manager.status.Download = download
	manager.status.UpdatedAt = time.Now().UTC()
	manager.mu.Unlock()
}

func (manager *Manager) setState(state, errorMessage string) {
	manager.mu.Lock()
	manager.status.State = state
	manager.status.Error = errorMessage
	manager.status.UpdatedAt = time.Now().UTC()
	manager.mu.Unlock()
}

func (manager *Manager) requireRestart(results []BenchmarkResult, oldFingerprint, newFingerprint string) {
	message := "The selected local embedding runtime changed between ONNX and GGUF. A controlled reindex is scheduled for the next restart."
	manager.mu.Lock()
	manager.status.State = "restart_required"
	manager.status.Benchmark = append([]BenchmarkResult(nil), results...)
	manager.status.FallbackReason = message
	manager.status.Error = ""
	manager.status.RestartRequired = true
	manager.status.Fingerprint = oldFingerprint
	manager.status.UpdatedAt = time.Now().UTC()
	active := manager.active
	manager.mu.Unlock()
	if active == nil {
		manager.readyOnce.Do(func() { close(manager.ready) })
	}
	manager.logger.Warn("Local embedding runtime format changed; restart required before activation",
		"old_fingerprint", oldFingerprint,
		"new_fingerprint", newFingerprint,
		"marker", manager.options.ResetMarkerPath)
}

func (manager *Manager) scheduleReset(reason string) error {
	path := strings.TrimSpace(manager.options.ResetMarkerPath)
	if path == "" {
		return fmt.Errorf("embedding reset marker path is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create reset marker directory: %w", err)
	}
	raw, err := json.MarshalIndent(resetMarker{
		RequestedAt: time.Now().UTC(),
		Reason:      reason,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal embedding reset marker: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0o600); err != nil {
		return fmt.Errorf("write embedding reset marker: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace embedding reset marker: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("activate embedding reset marker: %w", err)
	}
	return nil
}

func candidateFingerprint(current candidate) string {
	switch current.Runtime {
	case "llama.cpp":
		return ggufFingerprint()
	case "onnxruntime":
		return onnxFingerprint()
	default:
		return ""
	}
}

func candidateRuntimeFromID(candidateID string) string {
	if strings.HasPrefix(candidateID, "llama-") {
		return "llama.cpp"
	}
	if strings.HasPrefix(candidateID, "onnx-") {
		return "onnxruntime"
	}
	return ""
}

func (manager *Manager) triggerBackgroundBenchmark() {
	manager.initWG.Add(1)
	go func() {
		defer manager.initWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if err := manager.setup(ctx, true); err != nil && !errors.Is(err, context.Canceled) {
			manager.logger.Warn("Automatic local embedding re-benchmark failed", "error", err)
		}
	}()
}

func (manager *Manager) cleanupUnusedGPURuntimes(selected candidate, candidates []candidate) {
	keep := map[string]bool{selected.Asset.ID: true}
	if selected.ID == "llama-cuda" && runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		keep["llama-windows-amd64-cuda-runtime"] = true
	}
	if fallback, ok := onnxRuntimeAsset(runtime.GOOS, runtime.GOARCH, "cpu"); ok {
		keep[fallback.ID] = true
	}
	if supplemental, ok := runtimeManifest["llama-windows-amd64-cuda-runtime"]; ok &&
		runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		candidates = append(candidates, candidate{GPU: true, HasAsset: true, Asset: supplemental})
	}
	for _, current := range candidates {
		if !current.GPU || !current.HasAsset || keep[current.Asset.ID] {
			continue
		}
		target := runtimeAssetPath(manager.options.CacheDir, current.Asset)
		if err := safeRemoveAll(filepath.Join(manager.options.CacheDir, "runtimes"), target); err != nil {
			manager.logger.Debug("Could not remove unused GPU runtime", "asset", current.Asset.ID, "error", err)
		}
		archive := filepath.Join(manager.options.CacheDir, "downloads", current.Asset.ID+archiveSuffix(current.Asset.Kind))
		if pathWithinRoot(archive, filepath.Join(manager.options.CacheDir, "downloads")) {
			_ = os.Remove(archive)
		}
	}
}

func (manager *Manager) cleanupUnusedDockerGPURuntimes(ctx context.Context, selected candidate) {
	if !dockerEmbeddingSidecarsAvailable() {
		return
	}
	keepBackend := ""
	if selected.Runtime == "llama.cpp" && selected.GPU {
		keepBackend = selected.Backend
	}
	for _, backend := range []string{"cuda", "vulkan"} {
		if backend == keepBackend {
			continue
		}
		if err := removeDockerLlamaImage(ctx, os.Getenv("DOCKER_HOST"), backend); err != nil {
			manager.logger.Debug("Could not remove unused llama.cpp GPU image", "backend", backend, "error", err)
		}
	}
}

func findCandidate(candidates []candidate, id string) (candidate, bool) {
	for _, current := range candidates {
		if current.ID == id {
			return current, true
		}
	}
	return candidate{}, false
}

func validBackend(backend string) bool {
	switch backend {
	case "auto", "cpu", "cuda", "directml", "coreml", "metal", "vulkan":
		return true
	default:
		return false
	}
}

func benchmarkInputs() []string {
	return []string{
		"Kurzer deutscher Satz über lokale semantische Suche.",
		"An English query about running multilingual embeddings without a cloud dependency.",
		"ローカル環境で意味検索用の埋め込みを生成します。",
		"func cosine(a, b []float32) float64 { /* vector similarity */ return 0 }",
		"Mehrsprachiger Dokumentabschnitt: AuraGo indiziert Dateien, Notizen und Wissensgraphen. " +
			"The benchmark includes both short queries and longer retrieval passages so batching and context handling are exercised.",
		"GPU acceleration should be used only after the runtime confirms the selected execution provider.",
	}
}
