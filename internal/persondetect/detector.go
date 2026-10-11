package persondetect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/onnxrt"
)

// DetectResult is one analyzed frame.
type DetectResult struct {
	Boxes         []Box
	Width, Height int
	WallMS, CPUMS float64
}

// DetectorInfo describes the running detector.
type DetectorInfo struct {
	Model          ModelTier `json:"model"`
	ModelName      string    `json:"model_name"`
	Requested      Backend   `json:"requested_backend"`
	Active         Backend   `json:"backend"` // gpu | cpu
	Runtime        string    `json:"runtime"` // "onnxruntime 1.27.0"
	Providers      []string  `json:"providers"`
	FallbackReason string    `json:"fallback_reason,omitempty"`
	SelfTest       SelfTest  `json:"self_test"`
}

// SetupProgress reports NewDetector's phase and download progress.
type SetupProgress struct {
	Phase    string                `json:"phase"` // downloading_model | downloading_runtime | preparing_gpu_model | starting_worker | self_test
	Download onnxrt.DownloadStatus `json:"download"`
}

// Detector analyzes JPEG frames in a supervised worker process.
type Detector interface {
	Detect(ctx context.Context, jpegData []byte, minScore float64) (DetectResult, error)
	Info() DetectorInfo
	Close() error
}

// DetectorOptions configures NewDetector.
type DetectorOptions struct {
	ModelDir        string // <data>/persondetect/models
	RuntimeCacheDir string // <data>/embeddings (shared onnxrt.AssetCache root)
	Model           ModelTier
	Backend         Backend
	Logger          *slog.Logger
	OnProgress      func(SetupProgress)
	GPUProbe        func() (ok bool, reason string) // nil = default probe (linux: onnxrt.AccessibleRenderNode("/dev/dri"); windows/darwin: true)
	Executable      string                          // "" = os.Executable()
}

var (
	ErrDetectorClosed   = errors.New("person detector is closed")
	ErrWorkerRestarting = errors.New("person detector worker is restarting")
	ErrDetectorFailed   = errors.New("person detector failed")
)

const (
	phaseDownloadingModel   = "downloading_model"
	phaseDownloadingRuntime = "downloading_runtime"
	phasePreparingGPUModel  = "preparing_gpu_model"
	phaseStartingWorker     = "starting_worker"
	phaseSelfTest           = "self_test"

	userAgent = "AuraGo/persondetect"
)

type installer interface {
	Model(ctx context.Context, spec onnxrt.AssetSpec, destination string) error
	Runtime(ctx context.Context, spec onnxrt.AssetSpec) (string, error)
}

// cacheInstaller installs through onnxrt.AssetCache. The runtime cache root
// (<data>/embeddings) and its process lock are shared with embeddings; the
// cache honours ctx while it waits for that lock.
type cacheInstaller struct {
	models, runtimes *onnxrt.AssetCache
}

func (c cacheInstaller) Model(ctx context.Context, spec onnxrt.AssetSpec, destination string) error {
	return c.models.EnsureFile(ctx, spec, destination)
}

func (c cacheInstaller) Runtime(ctx context.Context, spec onnxrt.AssetSpec) (string, error) {
	return c.runtimes.EnsureRuntime(ctx, spec)
}

// detectorDeps are the seams tests replace; production uses
// defaultDetectorDeps.
type detectorDeps struct {
	install       installer // nil = onnxrt.AssetCache under ModelDir and RuntimeCacheDir
	goos, goarch  string
	webgpuAsset   func(goos, goarch string) (onnxrt.AssetSpec, bool)
	cpuAsset      func(goos, goarch string) (onnxrt.AssetSpec, bool)
	cpuThreads    int
	startTimeout  time.Duration // prepare, and worker start including the self-test
	detectTimeout time.Duration
	backoffBase   time.Duration
	backoffMax    time.Duration
	maxFailures   int
}

func defaultDetectorDeps() detectorDeps {
	return detectorDeps{
		goos:          goruntime.GOOS,
		goarch:        goruntime.GOARCH,
		webgpuAsset:   onnxrt.WebGPURuntimeAsset,
		cpuAsset:      onnxrt.CPURuntimeAsset,
		cpuThreads:    cpuThreads(),
		startTimeout:  5 * time.Minute,
		detectTimeout: 10 * time.Second,
		backoffBase:   time.Second,
		backoffMax:    time.Minute,
		maxFailures:   5,
	}
}

// NewDetector installs the pinned model and runtime, selects the backend
// (WebGPU first for auto/gpu when a WebGPU runtime exists and the GPU probe
// passes, else CPU), and returns a detector backed by a self-tested worker.
func NewDetector(ctx context.Context, opts DetectorOptions) (Detector, error) {
	return newDetector(ctx, opts, defaultDetectorDeps())
}

func newDetector(ctx context.Context, opts DetectorOptions, deps detectorDeps) (Detector, error) {
	if opts.Model == "" {
		opts.Model = ModelStandard
	}
	if opts.Backend == "" {
		opts.Backend = BackendAuto
	}
	spec, ok := Model(opts.Model)
	if !ok {
		return nil, fmt.Errorf("unknown person detection model %q", opts.Model)
	}
	if !ValidBackend(string(opts.Backend)) {
		return nil, fmt.Errorf("unknown person detection backend %q", opts.Backend)
	}
	if strings.TrimSpace(opts.ModelDir) == "" || strings.TrimSpace(opts.RuntimeCacheDir) == "" {
		return nil, fmt.Errorf("person detection model and runtime directories are required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve AuraGo executable: %w", err)
		}
		opts.Executable = executable
	}
	if opts.GPUProbe == nil {
		opts.GPUProbe = defaultGPUProbe(deps.goos)
	}
	s := &setup{opts: opts, deps: deps, spec: spec}
	if s.deps.install == nil {
		s.deps.install = cacheInstaller{
			models:   onnxrt.NewAssetCache(opts.ModelDir, userAgent, s.downloadProgress),
			runtimes: onnxrt.NewAssetCache(opts.RuntimeCacheDir, userAgent, s.downloadProgress),
		}
	}
	return s.run(ctx)
}

func defaultGPUProbe(goos string) func() (bool, string) {
	if goos != "linux" {
		return func() (bool, string) { return true, "" }
	}
	return func() (bool, string) {
		if _, err := onnxrt.AccessibleRenderNode("/dev/dri"); err != nil {
			return false, err.Error()
		}
		return true, ""
	}
}

// setup is one NewDetector run.
type setup struct {
	opts  DetectorOptions
	deps  detectorDeps
	spec  ModelSpec
	phase string
}

func (s *setup) report(phase string) {
	s.phase = phase
	if s.opts.OnProgress != nil {
		s.opts.OnProgress(SetupProgress{Phase: phase})
	}
}

func (s *setup) downloadProgress(status onnxrt.DownloadStatus) {
	if s.opts.OnProgress != nil {
		s.opts.OnProgress(SetupProgress{Phase: s.phase, Download: status})
	}
}

func (s *setup) run(ctx context.Context) (Detector, error) {
	modelPath := filepath.Join(s.opts.ModelDir, string(s.spec.Tier), s.spec.Asset.FileName)
	s.report(phaseDownloadingModel)
	if err := s.deps.install.Model(ctx, s.spec.Asset, modelPath); err != nil {
		return nil, fmt.Errorf("download %s model: %w", s.spec.Name, err)
	}
	info := DetectorInfo{Model: s.spec.Tier, ModelName: s.spec.Name, Requested: s.opts.Backend}
	if s.opts.Backend != BackendCPU {
		worker, launch, hello, reason := s.tryGPU(ctx, modelPath)
		if worker != nil {
			info.Active = BackendGPU
			return s.activate(launch, worker, hello, info), nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info.FallbackReason = reason
		s.opts.Logger.Warn("Person detection GPU path unavailable; using the CPU", "model", s.spec.Name, "reason", reason)
	}
	worker, launch, hello, err := s.startCPU(ctx, modelPath)
	if err != nil {
		return nil, err
	}
	info.Active = BackendCPU
	return s.activate(launch, worker, hello, info), nil
}

func (s *setup) tryGPU(ctx context.Context, modelPath string) (*workerProcess, workerLaunch, workerMessage, string) {
	rejected := func(reason string) (*workerProcess, workerLaunch, workerMessage, string) {
		return nil, workerLaunch{}, workerMessage{}, reason
	}
	asset, ok := s.deps.webgpuAsset(s.deps.goos, s.deps.goarch)
	if !ok {
		return rejected(fmt.Sprintf("no WebGPU runtime for %s/%s", s.deps.goos, s.deps.goarch))
	}
	if accessible, reason := s.opts.GPUProbe(); !accessible {
		return rejected("GPU not accessible: " + reason)
	}
	s.report(phaseDownloadingRuntime)
	runtimeRoot, err := s.deps.install.Runtime(ctx, asset)
	if err != nil {
		return rejected("WebGPU runtime: " + err.Error())
	}
	s.report(phasePreparingGPUModel)
	gpuModel, err := s.prepareGPUModel(ctx, runtimeRoot, modelPath)
	if err != nil {
		return rejected("GPU model preparation: " + err.Error())
	}
	launch := s.serveLaunch(runtimeRoot, gpuModel, "webgpu", 1)
	worker, hello, err := s.startServe(ctx, launch)
	if err != nil {
		if ctx.Err() == nil {
			s.discardGPUModel(ctx)
		}
		return rejected("GPU worker: " + err.Error())
	}
	if reason, modelSuspect := gpuRejection(hello.SelfTest); reason != "" {
		worker.stop()
		if modelSuspect {
			s.discardGPUModel(ctx)
		}
		return rejected(reason)
	}
	return worker, launch, hello, ""
}

func (s *setup) startCPU(ctx context.Context, modelPath string) (*workerProcess, workerLaunch, workerMessage, error) {
	asset, ok := s.deps.cpuAsset(s.deps.goos, s.deps.goarch)
	if !ok {
		return nil, workerLaunch{}, workerMessage{}, fmt.Errorf("no ONNX Runtime CPU bundle for %s/%s", s.deps.goos, s.deps.goarch)
	}
	s.report(phaseDownloadingRuntime)
	runtimeRoot, err := s.deps.install.Runtime(ctx, asset)
	if err != nil {
		return nil, workerLaunch{}, workerMessage{}, fmt.Errorf("download ONNX Runtime CPU bundle: %w", err)
	}
	launch := s.serveLaunch(runtimeRoot, modelPath, "cpu", s.deps.cpuThreads)
	worker, hello, err := s.startServe(ctx, launch)
	if err != nil {
		return nil, workerLaunch{}, workerMessage{}, fmt.Errorf("start CPU worker: %w", err)
	}
	if err := acceptHello("cpu", hello); err != nil {
		worker.stop()
		return nil, workerLaunch{}, workerMessage{}, fmt.Errorf("CPU worker: %w", err)
	}
	return worker, launch, hello, nil
}

func (s *setup) serveLaunch(runtimeRoot, model, provider string, threads int) workerLaunch {
	return workerLaunch{
		executable:  s.opts.Executable,
		runtimeRoot: runtimeRoot,
		provider:    provider,
		args: []string{
			"serve", "--runtime", runtimeRoot, "--model", model,
			"--provider", provider, "--threads", strconv.Itoa(threads),
		},
	}
}

// startServe starts a serve worker and waits for its hello with the start-up
// timeout (the self-test's first WebGPU run compiles shaders for seconds),
// bounded by ctx.
func (s *setup) startServe(ctx context.Context, launch workerLaunch) (*workerProcess, workerMessage, error) {
	s.report(phaseStartingWorker)
	return startServeWorker(ctx, launch, s.deps.startTimeout, func() { s.report(phaseSelfTest) })
}

// gpuModelManifest is the sidecar of a cached WebGPU model.
type gpuModelManifest struct {
	SourceSHA256 string `json:"source_sha256"`
	ORTVersion   string `json:"ort_version"`
	PatchedNodes int    `json:"patched_nodes"`
}

// gpuModelPaths are <ModelDir>/<tier>/<stem>.webgpu-<version>.onnx and its
// .json sidecar, <stem> being the pinned asset's file name without extension
// ("model").
func (s *setup) gpuModelPaths() (string, string) {
	name := s.spec.Asset.FileName
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	base := filepath.Join(s.opts.ModelDir, string(s.spec.Tier), stem+".webgpu-"+onnxrt.WebGPURuntimeVersion)
	return base + ".onnx", base + ".json"
}

// prepareGPUModel reuses the cached patched model when its sidecar names the
// pinned source SHA-256 and the WebGPU runtime version; otherwise it runs
// prepare in a worker subprocess and writes the sidecar. It holds the model
// file's lock throughout, so concurrent setups never prepare one output at the
// same time, and returns only after the prepare subprocess has exited.
func (s *setup) prepareGPUModel(ctx context.Context, runtimeRoot, modelPath string) (string, error) {
	gpuPath, manifestPath := s.gpuModelPaths()
	unlock, err := lockGPUModel(ctx, gpuPath)
	if err != nil {
		return "", err
	}
	defer unlock()
	if cachedGPUModelValid(gpuPath, manifestPath, s.spec.Asset.SHA256) {
		return gpuPath, nil
	}
	_ = os.Remove(manifestPath)
	count, err := runPrepare(ctx, s.opts.Executable, runtimeRoot, modelPath, gpuPath, s.deps.startTimeout)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(gpuModelManifest{
		SourceSHA256: s.spec.Asset.SHA256,
		ORTVersion:   onnxrt.WebGPURuntimeVersion,
		PatchedNodes: count,
	})
	if err != nil {
		return "", fmt.Errorf("marshal GPU model manifest: %w", err)
	}
	if err := writeFileAtomic(manifestPath, raw, 0o640); err != nil {
		return "", err
	}
	return gpuPath, nil
}

func cachedGPUModelValid(gpuPath, manifestPath, sourceSHA256 string) bool {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}
	var manifest gpuModelManifest
	if json.Unmarshal(raw, &manifest) != nil {
		return false
	}
	if !strings.EqualFold(manifest.SourceSHA256, sourceSHA256) ||
		manifest.ORTVersion != onnxrt.WebGPURuntimeVersion || manifest.PatchedNodes < 1 {
		return false
	}
	info, err := os.Stat(gpuPath)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

// discardGPUModel drops the cached WebGPU model so the next setup prepares it
// again; the sidecar goes first so a half-removed cache never looks valid. It
// takes the model file's lock and gives up when ctx ends first.
func (s *setup) discardGPUModel(ctx context.Context) {
	gpuPath, manifestPath := s.gpuModelPaths()
	unlock, err := lockGPUModel(ctx, gpuPath)
	if err != nil {
		return
	}
	defer unlock()
	_ = os.Remove(manifestPath)
	_ = os.Remove(gpuPath)
}

// gpuModelLocks serializes the prepares and discards of one WebGPU model file
// within the process: a prepare sweeps the temp files an overlapping prepare
// of the same output is still writing and makes it fail
// (sweepStalePrepareFiles). Each value is a chan struct{} of capacity 1, so
// waiting for it can honour a context.
var gpuModelLocks sync.Map

func lockGPUModel(ctx context.Context, path string) (unlock func(), err error) {
	value, _ := gpuModelLocks.LoadOrStore(gpuModelLockKey(path), make(chan struct{}, 1))
	slot := value.(chan struct{})
	select {
	case slot <- struct{}{}:
		return func() { <-slot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// gpuModelLockKey is the absolute path, lower-cased where file names are
// usually case-insensitive.
func gpuModelLockKey(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if goruntime.GOOS == "windows" || goruntime.GOOS == "darwin" {
		path = strings.ToLower(path)
	}
	return path
}

// gpuRejection applies the GPU acceptance rule: the self-test passed and the
// GPU path's CPU time is at most half of the CPU path's (rejects software
// Vulkan such as llvmpipe). A missing GPU CPU time reads as 0 (accepted); a
// missing CPU reference time rejects. modelSuspect marks failures that may
// come from a bad cached model.
func gpuRejection(selfTest *SelfTest) (reason string, modelSuspect bool) {
	switch {
	case selfTest == nil:
		return "self-test: worker reported no self-test", true
	case !selfTest.Passed:
		return "self-test: " + selfTest.Reason, true
	case selfTest.CPUCPUMS <= 0:
		return "self-test: CPU reference time is missing", false
	case selfTest.GPUCPUMS > 0.5*selfTest.CPUCPUMS:
		return fmt.Sprintf("GPU path used %.0f%% of the CPU path's CPU time", 100*selfTest.GPUCPUMS/selfTest.CPUCPUMS), false
	default:
		return "", false
	}
}

// acceptHello applies the acceptance rule of the worker's provider.
func acceptHello(provider string, hello workerMessage) error {
	if provider == "webgpu" {
		if reason, _ := gpuRejection(hello.SelfTest); reason != "" {
			return errors.New(reason)
		}
		return nil
	}
	if hello.SelfTest == nil {
		return errors.New("self-test: worker reported no self-test")
	}
	if !hello.SelfTest.Passed {
		return errors.New("self-test: " + hello.SelfTest.Reason)
	}
	return nil
}

func (s *setup) activate(launch workerLaunch, worker *workerProcess, hello workerMessage, info DetectorInfo) *workerDetector {
	info.Runtime = "onnxruntime " + hello.Version
	info.Providers = append([]string(nil), hello.Providers...)
	if hello.SelfTest != nil {
		info.SelfTest = *hello.SelfTest
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.opts.Logger.Info("Person detector ready",
		"model", info.ModelName,
		"backend", info.Active,
		"runtime", info.Runtime,
		"fallback_reason", info.FallbackReason,
		"gpu_cpu_ms", info.SelfTest.GPUCPUMS,
		"cpu_cpu_ms", info.SelfTest.CPUCPUMS)
	return &workerDetector{
		logger: s.opts.Logger,
		launch: launch,
		deps:   s.deps,
		info:   info,
		ctx:    ctx,
		cancel: cancel,
		worker: worker,
	}
}

// workerDetector supervises one serve worker.
type workerDetector struct {
	logger *slog.Logger
	launch workerLaunch
	deps   detectorDeps
	info   DetectorInfo
	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc

	mu       sync.Mutex
	worker   *workerProcess // nil while restarting or failed
	nextID   uint64
	failures int   // consecutive worker failures
	failed   error // set after maxFailures consecutive failures
	closed   bool
	timer    *time.Timer
	restarts sync.WaitGroup
}

func (d *workerDetector) Info() DetectorInfo {
	info := d.info
	info.Providers = append([]string(nil), d.info.Providers...)
	return info
}

// Detect sends one frame to the worker and waits for its result. Frames are
// analyzed one at a time. The detect timeout covers sending the frame and the
// answer; when it expires the worker is killed. A cancelled ctx abandons the
// frame without counting a failure (the worker still answers it, and that
// answer is skipped).
func (d *workerDetector) Detect(ctx context.Context, jpegData []byte, minScore float64) (DetectResult, error) {
	if len(jpegData) == 0 {
		return DetectResult{}, fmt.Errorf("frame is empty")
	}
	if len(jpegData) > maxFrameBytes {
		return DetectResult{}, fmt.Errorf("frame has %d bytes, limit %d", len(jpegData), maxFrameBytes)
	}
	if !(minScore > 0 && minScore <= 1) {
		return DetectResult{}, fmt.Errorf("min score must be in (0, 1]")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return DetectResult{}, ErrDetectorClosed
	}
	if d.failed != nil {
		return DetectResult{}, d.failed
	}
	worker := d.worker
	if worker == nil {
		return DetectResult{}, ErrWorkerRestarting
	}
	d.nextID++
	id := d.nextID
	timer := time.NewTimer(d.deps.detectTimeout)
	defer timer.Stop()
	sent := worker.sendAsync(workerRequest{ID: id, Command: "detect", JPEG: jpegData, MinScore: minScore})
	answer := func(message workerMessage) bool {
		return isWorkerError(message) || (message.Type == "result" && message.ID == id)
	}
	for {
		select {
		case err := <-sent:
			sent = nil
			if err != nil {
				return DetectResult{}, d.failLocked(fmt.Errorf("send frame: %w", err))
			}
		case message := <-worker.messages:
			if answer(message) {
				return d.finishLocked(message)
			}
		case <-worker.done:
			if message, ok := worker.buffered(answer); ok {
				return d.finishLocked(message)
			}
			return DetectResult{}, d.failLocked(fmt.Errorf("worker exited: %s", worker.exitSummary()))
		case <-timer.C:
			return DetectResult{}, d.failLocked(fmt.Errorf("detect timed out after %s", d.deps.detectTimeout))
		case <-ctx.Done():
			return DetectResult{}, ctx.Err()
		case <-d.ctx.Done():
			return DetectResult{}, ErrDetectorClosed
		}
	}
}

// finishLocked turns the worker's answer into a result. A frame-level error
// (an undecodable JPEG, for example) is returned without counting a worker
// failure; an id-less error line is a worker failure.
func (d *workerDetector) finishLocked(message workerMessage) (DetectResult, error) {
	if isWorkerError(message) {
		return DetectResult{}, d.failLocked(workerError(message))
	}
	if message.Error != "" {
		return DetectResult{}, fmt.Errorf("detect: %s", message.Error)
	}
	d.failures = 0
	boxes := message.Boxes
	if boxes == nil {
		boxes = []Box{}
	}
	return DetectResult{Boxes: boxes, Width: message.Width, Height: message.Height, WallMS: message.WallMS, CPUMS: message.CPUMS}, nil
}

// failLocked stops the current worker, counts a consecutive failure and
// either schedules a restart with backoff or marks the detector failed.
func (d *workerDetector) failLocked(cause error) error {
	if d.worker != nil {
		d.worker.stop()
		d.worker = nil
	}
	d.failures++
	d.logger.Warn("Person detection worker failed", "backend", d.info.Active, "failures", d.failures, "error", cause)
	if d.failures >= d.deps.maxFailures {
		d.failed = fmt.Errorf("%w after %d consecutive failures: %v", ErrDetectorFailed, d.failures, cause)
		return d.failed
	}
	delay := backoffDelay(d.deps.backoffBase, d.deps.backoffMax, d.failures)
	d.timer = time.AfterFunc(delay, d.restart)
	return fmt.Errorf("person detection worker failed (restart in %s): %w", delay, cause)
}

// backoffDelay is base·2^(failures−1), capped at limit.
func backoffDelay(base, limit time.Duration, failures int) time.Duration {
	delay := base
	for i := 1; i < failures && delay < limit; i++ {
		delay *= 2
	}
	if delay > limit {
		delay = limit
	}
	return delay
}

func (d *workerDetector) restart() {
	d.mu.Lock()
	if d.closed || d.failed != nil || d.worker != nil {
		d.mu.Unlock()
		return
	}
	d.restarts.Add(1)
	d.mu.Unlock()
	defer d.restarts.Done()

	worker, hello, err := startServeWorker(d.ctx, d.launch, d.deps.startTimeout, nil)
	if err == nil {
		if err = acceptHello(d.launch.provider, hello); err != nil {
			worker.stop()
			worker = nil
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		if worker != nil {
			worker.stop()
		}
		return
	}
	if err != nil {
		_ = d.failLocked(fmt.Errorf("restart: %w", err))
		return
	}
	d.worker = worker
	d.logger.Info("Person detection worker restarted", "backend", d.info.Active)
}

// Close stops the worker; afterwards every Detect returns ErrDetectorClosed.
func (d *workerDetector) Close() error {
	d.cancel()
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil
	}
	d.closed = true
	if d.timer != nil {
		d.timer.Stop()
	}
	worker := d.worker
	d.worker = nil
	d.mu.Unlock()
	d.restarts.Wait()
	if worker != nil {
		worker.shutdown()
	}
	return nil
}
