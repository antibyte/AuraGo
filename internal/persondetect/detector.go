package persondetect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
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
