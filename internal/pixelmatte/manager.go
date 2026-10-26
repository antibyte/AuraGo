package pixelmatte

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/onnxrt"
)

// Errors the HTTP layer maps to status codes.
var (
	ErrUnknownModel = errors.New("unknown matting model")
	ErrUnavailable  = errors.New("AI cutout is not available on this platform")
	ErrNotReady     = errors.New("the matting model or runtime is not downloaded yet")
	ErrBusy         = errors.New("another cutout is running")
	ErrTimeout      = errors.New("the cutout timed out")
)

const (
	userAgent = "AuraGo/pixelmatte"
	// DefaultRunTimeout bounds one worker run.
	DefaultRunTimeout = 180 * time.Second
	// maxStatusError bounds the error text status reports.
	maxStatusError = 300
)

// Installer downloads and probes the pinned assets. Production uses
// onnxrt.AssetCache; tests replace it. The *Ready probes are cheap (no
// hashing): status is polled while a download runs.
type Installer interface {
	ModelReady(spec onnxrt.AssetSpec, destination string) bool
	RuntimeReady(spec onnxrt.AssetSpec) (dir string, ok bool)
	EnsureModel(ctx context.Context, spec onnxrt.AssetSpec, destination string) error
	EnsureRuntime(ctx context.Context, spec onnxrt.AssetSpec) (dir string, err error)
}

// Options configures NewManager.
type Options struct {
	// DataDir is the AuraGo data directory. Models live in
	// <DataDir>/pixelmatte/models, the ONNX Runtime in the cache shared with
	// embeddings and person detection, <DataDir>/embeddings.
	DataDir    string
	Executable string // "" = os.Executable()
	Installer  Installer
	Runner     Runner
	Logger     *slog.Logger
	GOOS       string // "" = runtime.GOOS
	GOARCH     string // "" = runtime.GOARCH
	Threads    int    // 0 = DefaultThreads()
	Timeout    time.Duration
}

// ModelStatus is one model in Status.
type ModelStatus struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	License     string  `json:"license"`
	SizeBytes   int64   `json:"size_bytes"`
	Ready       bool    `json:"ready"`
	Downloading bool    `json:"downloading"`
	Progress    float64 `json:"progress"`
	Error       string  `json:"error"`
}

// Status is the GET /api/pixel/cutout/status payload.
type Status struct {
	Available    bool          `json:"available"`
	RuntimeReady bool          `json:"runtime_ready"`
	Busy         bool          `json:"busy"`
	Models       []ModelStatus `json:"models"`
}

// PrepareState is the outcome of Prepare.
type PrepareState string

const (
	PrepareReady       PrepareState = "ready"
	PrepareDownloading PrepareState = "downloading"
)

// Result is a finished cutout.
type Result struct {
	PNG      []byte // 8-bit grayscale matte, the size of the input
	Duration time.Duration
}

type prepareJob struct {
	runtimeNeeded bool
	phase         string // "runtime" | "model"
	cancel        context.CancelFunc
}

// Manager owns the downloads and the single worker slot of the feature.
type Manager struct {
	installer    Installer
	runner       Runner
	logger       *slog.Logger
	executable   string
	modelDir     string
	threads      int
	timeout      time.Duration
	available    bool
	runtimeAsset onnxrt.AssetSpec

	mu       sync.Mutex
	jobs     map[Kind]*prepareJob
	lastErr  map[Kind]string
	progress map[string]onnxrt.DownloadStatus
	running  bool
}

// NewManager builds a manager. Nothing is downloaded or started until
// Prepare is called.
func NewManager(opts Options) *Manager {
	goos, goarch := opts.GOOS, opts.GOARCH
	if goos == "" {
		goos = goruntime.GOOS
	}
	if goarch == "" {
		goarch = goruntime.GOARCH
	}
	m := &Manager{
		logger:     opts.Logger,
		executable: opts.Executable,
		modelDir:   filepath.Join(opts.DataDir, "pixelmatte", "models"),
		threads:    opts.Threads,
		timeout:    opts.Timeout,
		jobs:       map[Kind]*prepareJob{},
		lastErr:    map[Kind]string{},
		progress:   map[string]onnxrt.DownloadStatus{},
	}
	if m.logger == nil {
		m.logger = slog.Default()
	}
	if m.threads < 1 {
		m.threads = DefaultThreads()
	}
	if m.timeout <= 0 {
		m.timeout = DefaultRunTimeout
	}
	m.runtimeAsset, m.available = runtimeAsset(goos, goarch)
	m.available = m.available && strings.TrimSpace(opts.DataDir) != ""
	m.installer = opts.Installer
	if m.installer == nil {
		m.installer = &cacheInstaller{
			models:      onnxrt.NewAssetCache(m.modelDir, userAgent, m.onProgress),
			runtimes:    onnxrt.NewAssetCache(filepath.Join(opts.DataDir, "embeddings"), userAgent, m.onProgress),
			runtimeRoot: filepath.Join(opts.DataDir, "embeddings"),
		}
	}
	m.runner = opts.Runner
	if m.runner == nil {
		m.runner = processRunner{}
	}
	return m
}

func (m *Manager) modelPath(spec ModelSpec) string {
	return filepath.Join(m.modelDir, spec.Asset.FileName)
}

func (m *Manager) onProgress(status onnxrt.DownloadStatus) {
	m.mu.Lock()
	m.progress[status.Asset] = status
	m.mu.Unlock()
}

// Available reports whether the feature can run on this platform.
func (m *Manager) Available() bool { return m.available }

// Busy reports whether a cutout holds the worker slot right now. It is a cheap
// hint for callers that want to refuse a request before doing any work for it;
// Cutout still decides authoritatively (ErrBusy) when it takes the slot.
func (m *Manager) Busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// Status reports the platform, the runtime, the worker slot and every model.
// It never touches the network and hashes no files.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{Available: m.available, Busy: m.running, Models: make([]ModelStatus, 0, len(modelOrder))}
	runtimeReady := false
	if m.available {
		_, runtimeReady = m.installer.RuntimeReady(m.runtimeAsset)
	}
	status.RuntimeReady = runtimeReady
	for _, kind := range modelOrder {
		spec := models[kind]
		job := m.jobs[kind]
		entry := ModelStatus{
			ID:          string(kind),
			Name:        spec.Name,
			License:     spec.License,
			SizeBytes:   spec.Asset.Size,
			Downloading: job != nil,
			Error:       m.lastErr[kind],
		}
		entry.Ready = job == nil && runtimeReady && m.installer.ModelReady(spec.Asset, m.modelPath(spec))
		switch {
		case entry.Ready:
			entry.Progress = 1
		case job != nil:
			entry.Progress = m.jobProgress(spec, job)
		}
		status.Models = append(status.Models, entry)
	}
	return status
}

// jobProgress is the share of the bytes of the runtime (when the job has to
// fetch it) and the model that have arrived, below 1 until the job ends.
// m.mu must be held.
func (m *Manager) jobProgress(spec ModelSpec, job *prepareJob) float64 {
	received := func(asset onnxrt.AssetSpec) float64 {
		return float64(min(max(m.progress[asset.ID].Downloaded, 0), asset.Size))
	}
	total := float64(spec.Asset.Size)
	done := received(spec.Asset)
	if job.runtimeNeeded {
		total += float64(m.runtimeAsset.Size)
		if job.phase == "model" {
			done += float64(m.runtimeAsset.Size)
		} else {
			done += received(m.runtimeAsset)
		}
	}
	return min(done/total, 0.99)
}

// Prepare starts the download of the runtime and the model in the background
// (one job per model, further calls join it) and returns PrepareDownloading,
// or PrepareReady when both are present. ctx is the lifetime of the download,
// not of a request: pass the server context.
func (m *Manager) Prepare(ctx context.Context, kind Kind) (PrepareState, error) {
	spec, ok := models[kind]
	if !ok {
		return "", ErrUnknownModel
	}
	if !m.available {
		return "", ErrUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs[kind] != nil {
		return PrepareDownloading, nil
	}
	_, runtimeReady := m.installer.RuntimeReady(m.runtimeAsset)
	if runtimeReady && m.installer.ModelReady(spec.Asset, m.modelPath(spec)) {
		delete(m.lastErr, kind)
		return PrepareReady, nil
	}
	jobCtx, cancel := context.WithCancel(ctx)
	job := &prepareJob{runtimeNeeded: !runtimeReady, phase: "runtime", cancel: cancel}
	if !runtimeReady {
		delete(m.progress, m.runtimeAsset.ID)
	}
	delete(m.progress, spec.Asset.ID)
	delete(m.lastErr, kind)
	m.jobs[kind] = job
	go m.runPrepare(jobCtx, spec, job)
	return PrepareDownloading, nil
}

func (m *Manager) runPrepare(ctx context.Context, spec ModelSpec, job *prepareJob) {
	var err error
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("download panicked: %v", value)
		}
		interrupted := ctx.Err() != nil // server shutdown
		job.cancel()
		m.mu.Lock()
		delete(m.jobs, spec.Kind)
		if err != nil && !interrupted {
			m.lastErr[spec.Kind] = statusError(err)
		}
		m.mu.Unlock()
		if err != nil {
			m.logger.Warn("Pixel AI cutout download failed", "model", spec.Kind, "error", err)
		} else {
			m.logger.Info("Pixel AI cutout model ready", "model", spec.Kind)
		}
	}()
	m.logger.Info("Pixel AI cutout download started", "model", spec.Kind, "runtime", job.runtimeNeeded)
	if job.runtimeNeeded {
		if _, err = m.installer.EnsureRuntime(ctx, m.runtimeAsset); err != nil {
			err = fmt.Errorf("ONNX Runtime: %w", err)
			return
		}
	}
	m.mu.Lock()
	job.phase = "model"
	m.mu.Unlock()
	if err = m.installer.EnsureModel(ctx, spec.Asset, m.modelPath(spec)); err != nil {
		err = fmt.Errorf("%s model: %w", spec.Name, err)
	}
}

func statusError(err error) string {
	text := strings.Join(strings.Fields(err.Error()), " ")
	if len(text) > maxStatusError {
		text = strings.ToValidUTF8(text[:maxStatusError], "") + "..."
	}
	return text
}

// Cutout runs the model on a PNG or JPEG and returns the matte. It returns
// ErrNotReady when the runtime or the model is missing (nothing is downloaded
// here), ErrBusy while another cutout runs, and ErrTimeout after the run
// timeout. ctx ends the worker process when the caller goes away.
func (m *Manager) Cutout(ctx context.Context, kind Kind, image []byte) (Result, error) {
	spec, ok := models[kind]
	if !ok {
		return Result{}, ErrUnknownModel
	}
	if !m.available {
		return Result{}, ErrUnavailable
	}
	m.mu.Lock()
	runtimeDir, runtimeReady := m.installer.RuntimeReady(m.runtimeAsset)
	if !runtimeReady || !m.installer.ModelReady(spec.Asset, m.modelPath(spec)) {
		m.mu.Unlock()
		return Result{}, ErrNotReady
	}
	if m.running {
		m.mu.Unlock()
		return Result{}, ErrBusy
	}
	m.running = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	executable := m.executable
	if executable == "" {
		var err error
		if executable, err = os.Executable(); err != nil {
			return Result{}, fmt.Errorf("resolve AuraGo executable: %w", err)
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()
	started := time.Now()
	matte, err := m.runner.Run(runCtx, RunRequest{
		Executable:  executable,
		RuntimeRoot: runtimeDir,
		ModelPath:   m.modelPath(spec),
		Kind:        kind,
		Threads:     m.threads,
		Image:       image,
	})
	if err != nil {
		if ctx.Err() == nil && errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return Result{}, ErrTimeout
		}
		return Result{}, err
	}
	return Result{PNG: matte, Duration: time.Since(started)}, nil
}

// cacheInstaller installs through onnxrt.AssetCache. The runtime cache root
// (<data>/embeddings) and its process lock are shared with embeddings and
// person detection.
type cacheInstaller struct {
	models, runtimes *onnxrt.AssetCache
	runtimeRoot      string
}

func (c *cacheInstaller) ModelReady(spec onnxrt.AssetSpec, destination string) bool {
	info, err := os.Lstat(destination)
	return err == nil && info.Mode().IsRegular() && info.Size() == spec.Size
}

func (c *cacheInstaller) RuntimeReady(spec onnxrt.AssetSpec) (string, bool) {
	dir := onnxrt.RuntimeDir(c.runtimeRoot, spec)
	if info, err := os.Lstat(filepath.Join(dir, ".complete.json")); err != nil || !info.Mode().IsRegular() {
		return dir, false
	}
	_, err := onnxrt.FindLibrary(dir)
	return dir, err == nil
}

func (c *cacheInstaller) EnsureModel(ctx context.Context, spec onnxrt.AssetSpec, destination string) error {
	return c.models.EnsureFile(ctx, spec, destination)
}

func (c *cacheInstaller) EnsureRuntime(ctx context.Context, spec onnxrt.AssetSpec) (string, error) {
	return c.runtimes.EnsureRuntime(ctx, spec)
}
