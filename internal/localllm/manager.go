package localllm

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

type vault interface {
	ReadSecret(string) (string, error)
	WriteSecret(string, string) error
}

type managerOptions struct {
	manifest   Manifest
	docker     dockerEngine
	httpClient *http.Client
}

// Option customizes Manager dependencies for tests and controlled release builds.
type Option func(*managerOptions)

func WithManifest(manifest Manifest) Option {
	return func(options *managerOptions) { options.manifest = manifest }
}

func withDockerEngine(engine dockerEngine) Option {
	return func(options *managerOptions) { options.docker = engine }
}

func withDownloadClient(client *http.Client) Option {
	return func(options *managerOptions) { options.httpClient = client }
}

// Manager owns AuraGo-Qwen artifacts, the hardened sidecar, and request/idle lifetime.
type Manager struct {
	mu        sync.Mutex
	cond      *sync.Cond
	cfg       config.LocalLLMConfig
	vault     vault
	logger    *slog.Logger
	manifest  Manifest
	docker    dockerEngine
	downloads *http.Client

	runningInDocker bool
	modelDir        string
	runtimeDir      string

	status             Status
	profile            HardwareProfile
	starting           bool
	startDone          chan struct{}
	startErr           error
	activeRequests     int
	lastRelease        time.Time
	pendingCfg         *config.LocalLLMConfig
	desiredFingerprint string
	generation         uint64
	desiredCtx         context.Context
	desiredCancel      context.CancelFunc

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	shuttingDown    bool
	operationWG     sync.WaitGroup

	installing  bool
	installDone chan struct{}
	installErr  error

	idleStop chan struct{}
	control  chan struct{}

	promptSlot           chan struct{}
	promptSeed           *promptCacheSeed
	promptCacheQualified bool
	promptWarmSequence   uint64
	promptWarmCancel     context.CancelFunc
	promptWarmWG         sync.WaitGroup
	promptPersistRunning bool
	promptPersistPending *promptCacheDecisionEntry
	promptPersistLast    time.Time
	promptPersistWG      sync.WaitGroup
	promptPersistCommit  sync.Mutex
	promptDecisionWrite  func(string, []byte, os.FileMode) error
	appliedPlan          *runtimePlan
}

type runtimePlan struct {
	Generation         uint64
	Config             config.LocalLLMConfig
	Profile            HardwareProfile
	Fingerprint        string
	Model              Artifact
	DownloadDraft      *Artifact
	Draft              *Artifact
	Image              Image
	ResolvedParameters []string
}

// NewManager creates a passive manager. It never downloads, pulls, or starts a container.
func NewManager(cfg *config.Config, secrets vault, logger *slog.Logger, options ...Option) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	opts := managerOptions{manifest: DefaultManifest(), httpClient: &http.Client{Timeout: 0}}
	for _, option := range options {
		option(&opts)
	}
	host := ""
	runningInDocker := false
	localCfg := config.LocalLLMConfig{}
	dataDir := "data"
	if cfg != nil {
		host = cfg.Docker.Host
		runningInDocker = cfg.Runtime.IsDocker
		localCfg = cfg.LocalLLM
		if strings.TrimSpace(cfg.Directories.DataDir) != "" {
			dataDir = cfg.Directories.DataDir
		}
	}
	if opts.docker == nil {
		opts.docker = dockerutil.NewClient(host, 30*time.Second)
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	desiredCtx, desiredCancel := context.WithCancel(lifecycleCtx)
	manager := &Manager{
		cfg: localCfg, vault: secrets, logger: logger, manifest: opts.manifest,
		docker: opts.docker, downloads: opts.httpClient, runningInDocker: runningInDocker,
		modelDir:   filepath.Join(dataDir, "models", "aurago-qwen35"),
		runtimeDir: filepath.Join(dataDir, "runtime", "aurago-local-llm"),
		idleStop:   make(chan struct{}), lifecycleCtx: lifecycleCtx, lifecycleCancel: lifecycleCancel,
		desiredCtx: desiredCtx, desiredCancel: desiredCancel, generation: 1,
		control: make(chan struct{}, 1), promptSlot: make(chan struct{}, 1),
		promptDecisionWrite: config.WriteFileAtomic,
	}
	manager.cond = sync.NewCond(&manager.mu)
	manager.status = Status{
		State: "disabled", ContextSize: localCfg.ContextSize,
		ReleaseManifestReady: opts.manifest.ReleaseReady,
		PromptCache:          PromptCacheStatus{State: "disabled"},
	}
	manager.cleanupRuntimeKey()
	manager.desiredFingerprint = manager.computeDesiredFingerprint(localCfg, HardwareProfile{})
	manager.status.DesiredFingerprint = manager.desiredFingerprint
	go manager.idleLoop()
	return manager
}

func (m *Manager) acquireControl(ctx context.Context) (func(), error) {
	m.mu.Lock()
	control := m.control
	lifecycle := m.lifecycleCtx
	shuttingDown := m.shuttingDown
	m.mu.Unlock()
	if shuttingDown {
		return nil, &UnavailableError{Code: "local_llm_shutting_down"}
	}
	if control == nil {
		return func() {}, nil
	}
	if lifecycle == nil {
		lifecycle = context.Background()
	}
	select {
	case control <- struct{}{}:
		m.mu.Lock()
		if m.shuttingDown {
			m.mu.Unlock()
			<-control
			return nil, &UnavailableError{Code: "local_llm_shutting_down"}
		}
		m.operationWG.Add(1)
		m.mu.Unlock()
		return func() {
			m.operationWG.Done()
			<-control
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-lifecycle.Done():
		return nil, &UnavailableError{Code: "local_llm_shutting_down", Err: lifecycle.Err()}
	}
}

func (m *Manager) beginOperation(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.Operation != "" {
		return false
	}
	m.status.Operation = name
	m.status.OperationInProgress = true
	return true
}

func (m *Manager) finishOperation(name string, owned bool) {
	if !owned {
		return
	}
	m.mu.Lock()
	if m.status.Operation == name {
		m.status.Operation = ""
		m.status.OperationInProgress = false
	}
	m.mu.Unlock()
}

// Close stops background management without performing Docker I/O. Server
// shutdown uses Shutdown so ephemeral runtime resources are also removed.
func (m *Manager) Close() {
	m.lifecycleCancel()
	m.mu.Lock()
	if m.promptWarmCancel != nil {
		m.promptWarmCancel()
		m.promptWarmCancel = nil
	}
	m.promptWarmSequence++
	m.promptPersistPending = nil
	m.promptSeed = nil
	m.promptCacheQualified = false
	m.appliedPlan = nil
	m.mu.Unlock()
	select {
	case <-m.idleStop:
	default:
		close(m.idleStop)
	}
	m.cleanupRuntimeKey()
}

// Shutdown cancels managed operations, waits for live response bodies up to the
// caller deadline, and removes only ephemeral runtime resources.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.shuttingDown {
		m.shuttingDown = true
		m.lifecycleCancel()
		if m.promptWarmCancel != nil {
			m.promptWarmCancel()
			m.promptWarmCancel = nil
		}
		m.promptWarmSequence++
	}
	m.mu.Unlock()
	select {
	case <-m.idleStop:
	default:
		close(m.idleStop)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
activeWait:
	for {
		m.mu.Lock()
		active := m.activeRequests
		m.mu.Unlock()
		if active == 0 {
			break
		}
		select {
		case <-ctx.Done():
			break activeWait
		case <-ticker.C:
		}
	}

	if ctx.Err() == nil {
		waitDone := make(chan struct{})
		go func() {
			m.operationWG.Wait()
			m.promptWarmWG.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-ctx.Done():
		}
	}
	m.queuePromptCacheSnapshot(true)
	if err := m.waitPromptCachePersistence(ctx); err != nil && ctx.Err() == nil {
		m.mu.Lock()
		m.status.PromptCache.DecisionPersisted = false
		m.status.PromptCache.ErrorCode = "prompt_cache_decision_write_failed"
		m.mu.Unlock()
	}

	cleanupCtx := ctx
	cleanupCancel := func() {}
	if ctx.Err() != nil {
		cleanupCtx, cleanupCancel = context.WithTimeout(context.Background(), 10*time.Second)
	}
	defer cleanupCancel()
	if err := m.CleanupStaleRuntime(cleanupCtx); err != nil && ctx.Err() == nil {
		return err
	}
	m.cleanupRuntimeKey()
	m.mu.Lock()
	m.promptSeed = nil
	m.promptCacheQualified = false
	m.appliedPlan = nil
	m.status.PromptCache.State = "disabled"
	m.mu.Unlock()
	return ctx.Err()
}

// Configure publishes a new desired state. Recreation waits until active streams close.
func (m *Manager) Configure(cfg config.LocalLLMConfig) {
	m.promptPersistCommit.Lock()
	defer m.promptPersistCommit.Unlock()
	m.mu.Lock()
	fingerprint := m.computeDesiredFingerprint(cfg, m.profile)
	if fingerprint == m.desiredFingerprint {
		m.cfg = cfg
		m.mu.Unlock()
		return
	}
	m.generation++
	m.desiredCancel()
	m.desiredCtx, m.desiredCancel = context.WithCancel(m.lifecycleCtx)
	m.desiredFingerprint = fingerprint
	m.status.DesiredFingerprint = fingerprint
	m.status.ContextSize = cfg.ContextSize
	m.invalidateVerificationLocked()
	m.cfg = cfg
	if m.activeRequests > 0 {
		copy := cfg
		m.pendingCfg = &copy
		m.status.PendingRestart = true
		m.mu.Unlock()
		return
	}
	m.status.PendingRestart = m.status.AppliedFingerprint != "" && m.status.AppliedFingerprint != fingerprint
	reconcile := m.status.State == "running"
	m.mu.Unlock()
	if reconcile {
		go func() {
			if cfg.Enabled {
				_ = m.Recreate(context.Background())
			} else {
				_ = m.Stop(context.Background(), false)
			}
		}()
	}
}

func (m *Manager) invalidateVerificationLocked() {
	if m.promptWarmCancel != nil {
		m.promptWarmCancel()
		m.promptWarmCancel = nil
	}
	m.promptWarmSequence++
	m.status.ToolCallVerified = false
	m.status.GPUOffloadVerified = false
	m.status.MemoryProfileVerified = false
	m.status.VerifiedFingerprint = ""
	m.status.VerifiedContextSize = 0
	m.status.ActualDevice = ""
	m.status.PromptCache = PromptCacheStatus{State: "cold"}
	m.promptSeed = nil
	m.promptCacheQualified = false
	m.promptPersistPending = nil
	m.appliedPlan = nil
}

func (m *Manager) desiredSnapshot() (config.LocalLLMConfig, uint64, context.Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.shuttingDown {
		return config.LocalLLMConfig{}, 0, nil, fmt.Errorf("local_llm_shutting_down")
	}
	desired := m.desiredCtx
	if desired == nil {
		desired = context.Background()
	}
	return m.cfg, m.generation, desired, nil
}

func operationContext(parent, desired context.Context) (context.Context, context.CancelFunc) {
	if desired == nil {
		desired = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(desired, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

func (m *Manager) generationCurrent(generation uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return generation == m.generation && !m.shuttingDown
}

// Probe performs passive Docker and hardware inspection only.
func (m *Manager) Probe(ctx context.Context) HardwareProfile {
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return HardwareProfile{Compatibility: "unsupported", Warnings: []string{"local_llm_shutting_down"}}
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	dockerOnline, nvidiaToolkit := m.probeDocker(ctx)
	profile := m.probeHardwareForBackend(ctx, cfg.Backend, dockerOnline, nvidiaToolkit)
	profile = m.applyManifestCompatibility(profile)
	fingerprint := m.computeDesiredFingerprint(cfg, profile)
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return profile
	}
	m.profile = profile
	if fingerprint != m.desiredFingerprint {
		m.desiredFingerprint = fingerprint
		m.invalidateVerificationLocked()
	}
	m.status.Compatibility = profile.Compatibility
	m.status.Warnings = append([]string(nil), profile.Warnings...)
	m.status.Backend = profile.SelectedBackend
	m.status.PhysicalDevice = profile.SelectedDevice
	m.status.ResolvedProfile = m.manifest.profileFor(profile)
	m.status.HardwareFingerprint = profile.Fingerprint
	m.status.AcknowledgementDue = profile.AcknowledgementDue && !m.acknowledged(profile.Fingerprint)
	m.status.DesiredFingerprint = fingerprint
	perf := performanceProfileFor(profile)
	m.status.PerformanceProfile = perf.Name
	m.status.PromptCache.CacheRAMMiB = perf.CacheRAMMiB
	m.status.PromptCache.CheckpointProfile = "32x2048"
	if m.status.PromptCache.State == "" {
		m.status.PromptCache.State = "cold"
	}
	if gpu, err := profile.selectedGPU(); err == nil {
		m.status.VRAMBytes = gpu.VRAMBytes
	}
	if !cfg.Enabled {
		m.status.State = "disabled"
		m.status.PromptCache.State = "disabled"
	} else if !m.manifest.ReleaseReady {
		m.setErrorLocked("release_artifacts_unavailable")
	} else if profile.Compatibility == "unsupported" {
		m.setErrorLocked("hardware_unsupported")
	} else {
		m.status.State = "ready_to_install"
		m.clearErrorLocked()
	}
	m.mu.Unlock()
	return profile
}

// ProbeBackend performs the same passive probe for a not-yet-saved setup
// backend without publishing or mutating the manager's desired state.
func (m *Manager) ProbeBackend(ctx context.Context, backend string) HardwareProfile {
	dockerOnline, nvidiaToolkit := m.probeDocker(ctx)
	return m.applyManifestCompatibility(m.probeHardwareForBackend(ctx, backend, dockerOnline, nvidiaToolkit))
}

func (m *Manager) probeHardwareForBackend(ctx context.Context, backend string, dockerOnline, nvidiaToolkit bool) HardwareProfile {
	if !strings.EqualFold(strings.TrimSpace(backend), "auto") && strings.TrimSpace(backend) != "" {
		return probeHardware(ctx, backend, dockerOnline, nvidiaToolkit)
	}
	allowed := make(map[string]bool)
	if m.manifest.ReleaseReady {
		for name, image := range m.manifest.Images {
			if !image.Supported {
				continue
			}
			for _, profile := range m.manifest.HardwareProfiles {
				if profile.Backend == name && profile.Status == "validated-linux" {
					allowed[name] = true
					break
				}
			}
		}
	}
	profile := probeHardwareAllowed(ctx, "auto", dockerOnline, nvidiaToolkit, allowed)
	if len(allowed) == 0 {
		profile.Warnings = appendWarning(profile.Warnings, "no_release_validated_gpu_backend")
	}
	return profile
}

func (m *Manager) applyManifestCompatibility(profile HardwareProfile) HardwareProfile {
	if profile.Compatibility == "unsupported" || profile.SelectedBackend == "cpu" {
		return profile
	}
	image, supported := m.manifest.Images[profile.SelectedBackend]
	resolved := m.manifest.profileFor(profile)
	validated := strings.HasSuffix(resolved, ":validated-linux")
	gpu, gpuErr := profile.selectedGPU()
	recommended := profile.DockerAvailable && supported && image.Supported && validated && gpuErr == nil &&
		gpu.Discrete && gpu.VRAMBytes >= 8<<30
	if profile.SelectedBackend == "vulkan" {
		recommended = recommended && profile.Vulkan12Verified
	}
	if recommended {
		profile.Compatibility = "recommended"
		profile.AcknowledgementDue = false
		return profile
	}
	profile.Compatibility = "experimental"
	profile.AcknowledgementDue = true
	switch {
	case strings.Contains(resolved, ":candidate-linux"):
		profile.Warnings = appendWarning(profile.Warnings, "hardware_profile_candidate_linux")
	case resolved == "unvalidated-hardware":
		profile.Warnings = appendWarning(profile.Warnings, "hardware_profile_unvalidated")
	case !supported || !image.Supported:
		profile.Warnings = appendWarning(profile.Warnings, "backend_not_release_validated")
	}
	return profile
}

func appendWarning(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// Acknowledge records the experimental-hardware warning for exactly one fingerprint.
func (m *Manager) Acknowledge(fingerprint string) error {
	m.mu.Lock()
	current := m.profile.Fingerprint
	m.mu.Unlock()
	if fingerprint == "" || fingerprint != current {
		return fmt.Errorf("hardware_fingerprint_changed")
	}
	if m.vault == nil {
		return fmt.Errorf("vault_unavailable")
	}
	if err := m.vault.WriteSecret("local_llm_ack_"+fingerprint, "acknowledged"); err != nil {
		return fmt.Errorf("save_acknowledgement: %w", err)
	}
	m.mu.Lock()
	m.status.AcknowledgementDue = false
	m.mu.Unlock()
	return nil
}

// AcknowledgeSavedHardware passively revalidates a setup-wizard fingerprint
// against the now-saved backend before persisting the experimental warning
// acknowledgement. It must only be called after the regular setup config save.
func (m *Manager) AcknowledgeSavedHardware(ctx context.Context, fingerprint string) error {
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	dockerOnline, nvidiaToolkit := m.probeDocker(ctx)
	profile := m.applyManifestCompatibility(m.probeHardwareForBackend(ctx, cfg.Backend, dockerOnline, nvidiaToolkit))
	if fingerprint == "" || profile.Fingerprint != fingerprint {
		return fmt.Errorf("hardware_fingerprint_changed")
	}
	if m.vault == nil {
		return fmt.Errorf("vault_unavailable")
	}
	if err := m.vault.WriteSecret("local_llm_ack_"+fingerprint, "acknowledged"); err != nil {
		return fmt.Errorf("save_acknowledgement: %w", err)
	}
	m.mu.Lock()
	if generation == m.generation {
		m.profile = profile
		m.status.HardwareFingerprint = profile.Fingerprint
		m.status.AcknowledgementDue = false
	}
	m.mu.Unlock()
	return nil
}

// Install downloads only the pinned artifacts selected by the saved config, then validates the runtime.
func (m *Manager) Install(ctx context.Context) error {
	m.mu.Lock()
	if m.shuttingDown {
		m.mu.Unlock()
		return &UnavailableError{Code: "local_llm_shutting_down"}
	}
	if m.installing {
		done := m.installDone
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return &UnavailableError{Code: "local_install_cancelled", Err: ctx.Err()}
		case <-done:
			m.mu.Lock()
			err := m.installErr
			m.mu.Unlock()
			return err
		}
	}
	m.installing = true
	m.installDone = make(chan struct{})
	m.operationWG.Add(1)
	m.mu.Unlock()

	release, gateErr := m.acquireControl(ctx)
	err := gateErr
	if gateErr == nil {
		m.mu.Lock()
		m.status.Operation = "install"
		m.status.OperationInProgress = true
		m.mu.Unlock()
		err = m.install(ctx)
		release()
	}
	m.mu.Lock()
	m.installing = false
	m.installErr = err
	close(m.installDone)
	if m.status.Operation == "install" {
		m.status.Operation = ""
		m.status.OperationInProgress = false
	}
	m.mu.Unlock()
	m.operationWG.Done()
	return err
}

func (m *Manager) install(parent context.Context) error {
	if err := m.manifest.validate(); err != nil {
		return m.fail(errorCode(err), err)
	}
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return m.fail(errorCode(err), err)
	}
	ctx, cancel := operationContext(parent, desiredCtx)
	defer cancel()
	plan, err := m.resolveRuntimePlan(ctx, cfg, generation)
	if err != nil {
		return m.failGeneration(generation, errorCode(err), err)
	}
	if plan.Profile.Compatibility == "unsupported" || !plan.Profile.DockerAvailable {
		return m.failGeneration(generation, "hardware_or_docker_unavailable", nil)
	}
	if plan.Profile.AcknowledgementDue && !m.acknowledged(plan.Profile.Fingerprint) {
		return m.failGeneration(generation, "experimental_hardware_acknowledgement_required", nil)
	}
	artifacts := []Artifact{plan.Model}
	if plan.DownloadDraft != nil {
		artifacts = append(artifacts, *plan.DownloadDraft)
	}
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return &UnavailableError{Code: "desired_state_changed"}
	}
	m.status.State = "downloading"
	m.status.Progress = 0
	m.clearErrorLocked()
	m.mu.Unlock()
	for index, artifact := range artifacts {
		destination := filepath.Join(m.modelDir, artifact.Name)
		err := downloadArtifactGuarded(ctx, m.downloads, artifact.URL(), destination, artifact, func(done, total int64) {
			m.mu.Lock()
			if generation == m.generation {
				m.status.Progress = (float64(index) + float64(done)/float64(total)) / float64(len(artifacts))
			}
			m.mu.Unlock()
		}, func(publish func() error) error {
			m.mu.Lock()
			defer m.mu.Unlock()
			if generation != m.generation || m.shuttingDown {
				return fmt.Errorf("desired_state_changed")
			}
			return publish()
		})
		if err != nil {
			return m.failGeneration(generation, errorCode(err), err)
		}
	}
	if !m.generationCurrent(generation) {
		return &UnavailableError{Code: "desired_state_changed"}
	}
	if !isDigestPinned(plan.Image.Reference) {
		return m.failGeneration(generation, "backend_not_supported", nil)
	}
	m.mu.Lock()
	if generation == m.generation {
		m.status.State = "pulling"
	}
	m.mu.Unlock()
	if err := m.pullImage(ctx, plan.Image.Reference); err != nil {
		return m.failGeneration(generation, errorCode(err), err)
	}
	if err := m.startWithPlan(ctx, plan); err != nil {
		return err
	}
	plan, err = m.ensureAutoMTPPlan(ctx, plan)
	if err != nil {
		if m.generationCurrent(generation) {
			_ = m.stop(context.Background(), true)
		}
		return m.failGeneration(generation, errorCode(err), err)
	}
	if err := m.smokeTestPlan(ctx, plan); err != nil {
		if m.generationCurrent(generation) {
			_ = m.stop(context.Background(), true)
		}
		return m.failGeneration(generation, errorCode(err), err)
	}
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return &UnavailableError{Code: "desired_state_changed"}
	}
	m.status.State = "running"
	m.status.Progress = 1
	m.mu.Unlock()
	return nil
}

// Start is singleflight and waits up to 180 seconds for the private endpoint.
func (m *Manager) Start(ctx context.Context) error {
	release, err := m.acquireControl(ctx)
	if err != nil {
		return err
	}
	defer release()
	return m.start(ctx)
}

func (m *Manager) start(ctx context.Context) error {
	m.mu.Lock()
	if m.starting {
		done := m.startDone
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return &UnavailableError{Code: "local_start_cancelled", Err: ctx.Err()}
		case <-done:
		}
		m.mu.Lock()
		err := m.startErr
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	plan, err := m.resolveRuntimePlan(ctx, cfg, generation)
	if err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	return m.startWithPlan(ctx, plan)
}

func (m *Manager) startWithPlan(ctx context.Context, plan runtimePlan) error {
	for {
		m.mu.Lock()
		if m.shuttingDown {
			m.mu.Unlock()
			return &UnavailableError{Code: "local_llm_shutting_down"}
		}
		if !m.starting {
			break
		}
		done := m.startDone
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return &UnavailableError{Code: "local_start_cancelled", Err: ctx.Err()}
		case <-done:
		}
		m.mu.Lock()
		err := m.startErr
		applied := m.status.AppliedFingerprint
		m.mu.Unlock()
		if err != nil {
			return err
		}
		if applied == plan.Fingerprint {
			return m.verifyRunningHealth(ctx, plan)
		}
	}
	if m.status.State == "running" && m.status.AppliedFingerprint == plan.Fingerprint {
		lastHealth := m.status.LastHealthCheck
		m.mu.Unlock()
		if lastHealth != nil && time.Since(*lastHealth) < 5*time.Second {
			return nil
		}
		return m.verifyRunningHealth(ctx, plan)
	}
	ownsOperation := m.status.Operation == ""
	m.starting = true
	m.startDone = make(chan struct{})
	m.status.State = "starting"
	if ownsOperation {
		m.status.Operation = "start"
	}
	m.status.OperationInProgress = true
	m.operationWG.Add(1)
	m.mu.Unlock()

	err := m.startPlan(ctx, plan)
	if err != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = m.CleanupStaleRuntime(cleanupCtx)
		cleanupCancel()
	}
	m.mu.Lock()
	m.starting = false
	m.startErr = err
	close(m.startDone)
	currentGeneration := m.generationCurrentLocked(plan.Generation)
	if !currentGeneration {
		err = &UnavailableError{Code: "desired_state_changed"}
		m.startErr = err
	} else if err != nil {
		m.setErrorLocked(errorCode(err))
	} else {
		m.status.State = "running"
		m.status.AppliedFingerprint = plan.Fingerprint
		m.status.PendingRestart = false
		m.lastRelease = time.Now()
		deadline := m.lastRelease.Add(time.Duration(plan.Config.IdleTimeoutMinutes) * time.Minute)
		m.status.IdleDeadline = &deadline
	}
	if ownsOperation && m.status.Operation == "start" {
		m.status.Operation = ""
		m.status.OperationInProgress = false
	}
	m.cond.Broadcast()
	m.mu.Unlock()
	m.operationWG.Done()
	return err
}

func (m *Manager) startPlan(parent context.Context, plan runtimePlan) error {
	if err := m.manifest.validate(); err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	if !plan.Config.Enabled {
		return &UnavailableError{Code: "local_llm_disabled"}
	}
	if plan.Profile.Compatibility == "unsupported" || !plan.Profile.DockerAvailable {
		return &UnavailableError{Code: "hardware_or_docker_unavailable"}
	}
	if plan.Profile.AcknowledgementDue && !m.acknowledged(plan.Profile.Fingerprint) {
		return &UnavailableError{Code: "experimental_hardware_acknowledgement_required"}
	}
	if ok, _ := verifyArtifact(filepath.Join(m.modelDir, plan.Model.Name), plan.Model); !ok {
		return &UnavailableError{Code: "model_not_installed"}
	}
	if plan.DownloadDraft != nil {
		if ok, _ := verifyArtifact(filepath.Join(m.modelDir, plan.DownloadDraft.Name), *plan.DownloadDraft); !ok {
			return &UnavailableError{Code: "draft_not_installed"}
		}
	}
	if !isDigestPinned(plan.Image.Reference) {
		return &UnavailableError{Code: "backend_not_supported"}
	}
	key, err := m.ensureRuntimeKey()
	if err != nil {
		return &UnavailableError{Code: "runtime_key_failed", Err: err}
	}
	spec, err := m.containerSpecFor(plan)
	if err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	startCtx, cancel := context.WithTimeout(parent, 180*time.Second)
	defer cancel()
	if err := m.ensureImageAvailable(startCtx, plan.Image.Reference); err != nil {
		return &UnavailableError{Code: "image_not_installed", Err: err}
	}
	if err := m.prepareRuntimeKeyVolume(startCtx, plan.Image.Reference, key, plan.Fingerprint); err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	if err := m.recreateContainer(startCtx, spec); err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	if err := m.waitForHealth(startCtx, key, plan.Config); err != nil {
		return &UnavailableError{Code: "health_check_failed", Err: err}
	}
	if err := m.attestStartupPlan(startCtx, plan, key); err != nil {
		return &UnavailableError{Code: errorCode(err), Err: err}
	}
	m.mu.Lock()
	if plan.Generation != m.generation {
		m.mu.Unlock()
		return &UnavailableError{Code: "desired_state_changed"}
	}
	m.status.ModelSHA256 = plan.Model.SHA256
	if plan.Draft != nil {
		m.status.DraftSHA256 = plan.Draft.SHA256
	} else {
		m.status.DraftSHA256 = ""
	}
	m.status.ImageDigest = plan.Image.Reference[strings.LastIndex(plan.Image.Reference, "@sha256:")+1:]
	m.status.ResolvedParameters = append([]string(nil), plan.ResolvedParameters...)
	if plan.Config.MTP == "auto" && m.status.MTP.Reason == "" {
		m.status.MTP = MTPDecision{Reason: "mtp_benchmark_required"}
	}
	now := time.Now()
	m.status.LastHealthCheck = &now
	applied := plan
	m.appliedPlan = &applied
	m.mu.Unlock()
	return nil
}

// Stop refuses to interrupt a live stream unless force is true.
func (m *Manager) Stop(ctx context.Context, force bool) error {
	release, err := m.acquireControl(ctx)
	if err != nil {
		return err
	}
	defer release()
	owned := m.beginOperation("stop")
	defer m.finishOperation("stop", owned)
	return m.stop(ctx, force)
}

func (m *Manager) stop(ctx context.Context, force bool) error {
	m.cancelPromptCacheWarmForRequest()
	m.mu.Lock()
	if m.activeRequests > 0 && !force {
		m.mu.Unlock()
		return fmt.Errorf("active_requests")
	}
	m.mu.Unlock()
	_, stopErr := m.docker.DoJSON(ctx, http.MethodPost, "containers/"+managedContainerName+"/stop?t=15", nil, nil)
	deleteErr := m.deleteContainer(ctx, managedContainerName)
	seedErr := m.deleteContainer(ctx, runtimeKeySeedName)
	volumeErr := m.deleteRuntimeKeyVolume(ctx)
	m.cleanupRuntimeKey()
	m.mu.Lock()
	m.status.State = "stopped"
	m.status.IdleDeadline = nil
	if m.promptSeed != nil {
		m.status.PromptCache.State = "cold"
	}
	m.appliedPlan = nil
	m.mu.Unlock()
	if stopErr != nil && !strings.Contains(stopErr.Error(), "404") && !strings.Contains(stopErr.Error(), "304") {
		return fmt.Errorf("container_stop_failed: %w", stopErr)
	}
	if deleteErr != nil || seedErr != nil || volumeErr != nil {
		return fmt.Errorf("runtime_cleanup_failed")
	}
	return nil
}

// CleanupStaleRuntime removes the ephemeral key volume and any sidecar left by
// a previous AuraGo process. Downloaded model artifacts are never removed.
func (m *Manager) CleanupStaleRuntime(ctx context.Context) error {
	_, stopErr := m.docker.DoJSON(ctx, http.MethodPost, "containers/"+managedContainerName+"/stop?t=5", nil, nil)
	containerErr := m.deleteContainer(ctx, managedContainerName)
	seedErr := m.deleteContainer(ctx, runtimeKeySeedName)
	volumeErr := m.deleteRuntimeKeyVolume(ctx)
	m.cleanupRuntimeKey()
	if stopErr != nil && !strings.Contains(stopErr.Error(), "404") && !strings.Contains(stopErr.Error(), "304") {
		return fmt.Errorf("stale_runtime_stop_failed")
	}
	if containerErr != nil || seedErr != nil || volumeErr != nil {
		return fmt.Errorf("stale_runtime_cleanup_failed")
	}
	return nil
}

// Recreate safely replaces the container while retaining verified model files.
func (m *Manager) Recreate(ctx context.Context) error {
	release, err := m.acquireControl(ctx)
	if err != nil {
		return err
	}
	defer release()
	owned := m.beginOperation("recreate")
	defer m.finishOperation("recreate", owned)
	return m.recreate(ctx)
}

func (m *Manager) recreate(ctx context.Context) error {
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	plan, err := m.resolveRuntimePlan(ctx, cfg, generation)
	if err != nil {
		return err
	}
	if err := m.stop(ctx, false); err != nil {
		return err
	}
	if err := m.startWithPlan(ctx, plan); err != nil {
		return err
	}
	plan, err = m.ensureAutoMTPPlan(ctx, plan)
	if err != nil {
		return err
	}
	return m.smokeTestPlan(ctx, plan)
}

// SmokeTest requires a native tool call and the image's sanitized startup manifest.
func (m *Manager) SmokeTest(ctx context.Context) error {
	release, err := m.acquireControl(ctx)
	if err != nil {
		return err
	}
	defer release()
	owned := m.beginOperation("smoke_test")
	defer m.finishOperation("smoke_test", owned)
	return m.smokeTest(ctx)
}

func (m *Manager) smokeTest(ctx context.Context) error {
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return err
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	plan, err := m.resolveRuntimePlan(ctx, cfg, generation)
	if err != nil {
		return err
	}
	if err := m.startWithPlan(ctx, plan); err != nil {
		return err
	}
	return m.smokeTestPlan(ctx, plan)
}

func (m *Manager) smokeTestPlan(ctx context.Context, plan runtimePlan) error {
	key, err := m.runtimeKey()
	if err != nil {
		return err
	}
	request := map[string]any{
		"model":    config.LocalLLMModelAlias,
		"messages": []map[string]string{{"role": "user", "content": "Call report_status with status ok."}},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": "report_status",
				"parameters": map[string]any{
					"type": "object", "properties": map[string]any{"status": map[string]string{"type": "string"}},
					"required": []string{"status"}, "additionalProperties": false,
				},
			},
		}},
		"tool_choice":         "required",
		"parallel_tool_calls": false,
		"max_tokens":          128,
	}
	var response struct {
		Choices []struct {
			Message struct {
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := m.apiJSONFor(ctx, plan.Config, http.MethodPost, "/v1/chat/completions", key, request, &response); err != nil {
		return fmt.Errorf("tool_call_smoke_failed: %w", err)
	}
	if len(response.Choices) != 1 || len(response.Choices[0].Message.ToolCalls) != 1 ||
		!validReportStatusToolCall(response.Choices[0].Message.ToolCalls[0]) {
		return fmt.Errorf("tool_call_smoke_failed")
	}
	if err := m.attestStartupPlan(ctx, plan, key); err != nil {
		return err
	}
	m.mu.Lock()
	if plan.Generation != m.generation || plan.Fingerprint != m.desiredFingerprint {
		m.mu.Unlock()
		return fmt.Errorf("desired_state_changed")
	}
	m.status.ToolCallVerified = true
	m.status.VerifiedFingerprint = plan.Fingerprint
	if m.status.MemoryProfileVerified {
		verifiedContext := plan.Config.ContextSize
		if cached, ok := m.loadContextCapability(plan); ok && cached > verifiedContext {
			verifiedContext = cached
		}
		if verifiedContext > m.status.VerifiedContextSize {
			m.status.VerifiedContextSize = verifiedContext
		}
	}
	m.mu.Unlock()
	return nil
}

type startupManifest struct {
	GPUOffload            bool     `json:"gpu_offload"`
	KVOffload             bool     `json:"kv_offload"`
	MemoryProfileVerified bool     `json:"memory_profile_verified"`
	ImageDigest           string   `json:"image_digest"`
	TargetSHA256          string   `json:"target_sha256"`
	DraftSHA256           string   `json:"draft_sha256"`
	PhysicalDevice        string   `json:"physical_device"`
	ActualDevice          string   `json:"actual_device"`
	ResolvedParameters    []string `json:"resolved_parameters"`
	LlamaCPPCommit        string   `json:"llama_cpp_commit"`
	PerformanceProfile    string   `json:"performance_profile"`
	Threads               int      `json:"threads"`
	ThreadsBatch          int      `json:"threads_batch"`
	BatchSize             int      `json:"batch_size"`
	UBatchSize            int      `json:"ubatch_size"`
	CacheTypeK            string   `json:"cache_type_k"`
	CacheTypeV            string   `json:"cache_type_v"`
	FlashAttention        string   `json:"flash_attention"`
	CacheRAMMiB           int      `json:"cache_ram_mib"`
	ContextCheckpoints    int      `json:"context_checkpoints"`
	CheckpointMinStep     int      `json:"checkpoint_min_step"`
	CacheReuse            int      `json:"cache_reuse"`
	CacheIdleSlots        string   `json:"cache_idle_slots"`
	SlotsEndpoint         string   `json:"slots_endpoint"`
	SplitMode             string   `json:"split_mode"`
	Poll                  string   `json:"poll"`
	Priority              string   `json:"priority"`
	RADVPerfTest          string   `json:"radv_perftest"`
}

func (m *Manager) attestStartupPlan(ctx context.Context, plan runtimePlan, key string) error {
	var startup startupManifest
	if err := m.apiJSONFor(ctx, plan.Config, http.MethodGet, "/startup-manifest", key, nil, &startup); err != nil {
		return fmt.Errorf("startup_manifest_unavailable: %w", err)
	}
	if err := validateStartupManifest(plan, startup); err != nil {
		return err
	}
	m.mu.Lock()
	if plan.Generation != m.generation {
		m.mu.Unlock()
		return fmt.Errorf("desired_state_changed")
	}
	m.status.GPUOffloadVerified = plan.Config.Backend != "cpu" && startup.GPUOffload && startup.KVOffload
	m.status.MemoryProfileVerified = startup.MemoryProfileVerified
	m.status.ActualDevice = startup.ActualDevice
	now := time.Now()
	m.status.LastHealthCheck = &now
	m.mu.Unlock()
	return nil
}

func validateStartupManifest(plan runtimePlan, startup startupManifest) error {
	if plan.Config.Backend == "cpu" && (startup.GPUOffload || startup.KVOffload) {
		return fmt.Errorf("startup_manifest_mismatch")
	}
	if plan.Config.Backend != "cpu" && (!startup.GPUOffload || !startup.KVOffload) {
		return fmt.Errorf("gpu_offload_not_verified")
	}
	if !startup.MemoryProfileVerified {
		return fmt.Errorf("memory_profile_not_verified")
	}
	expectedDraft := ""
	if plan.Draft != nil {
		expectedDraft = plan.Draft.SHA256
	}
	expectedDigest := plan.Image.Reference[strings.LastIndex(plan.Image.Reference, "@sha256:")+1:]
	perf := performanceProfileFor(plan.Profile)
	if startup.ImageDigest != expectedDigest ||
		startup.LlamaCPPCommit != LlamaCPPCommit ||
		startup.TargetSHA256 != plan.Model.SHA256 ||
		startup.DraftSHA256 != expectedDraft ||
		startup.PhysicalDevice != plan.Profile.SelectedDevice ||
		!equalStrings(startup.ResolvedParameters, plan.ResolvedParameters) ||
		!actualDeviceMatches(plan, startup.ActualDevice) {
		return fmt.Errorf("startup_manifest_mismatch")
	}
	expectedPoll, expectedPriority := "", ""
	if perf.Poll != nil {
		expectedPoll = strconv.Itoa(*perf.Poll)
	}
	if perf.Priority != nil {
		expectedPriority = strconv.Itoa(*perf.Priority)
	}
	if startup.PerformanceProfile != perf.Name ||
		startup.BatchSize != perf.BatchSize ||
		startup.UBatchSize != perf.UBatchSize ||
		startup.CacheTypeK != perf.CacheTypeK ||
		startup.CacheTypeV != perf.CacheTypeV ||
		startup.FlashAttention != perf.FlashAttention ||
		startup.CacheRAMMiB != perf.CacheRAMMiB ||
		startup.ContextCheckpoints != perf.ContextCheckpoints ||
		startup.CheckpointMinStep != perf.CheckpointMinStep ||
		startup.CacheReuse != perf.CacheReuse ||
		startup.CacheIdleSlots != "on" ||
		startup.SlotsEndpoint != "off" ||
		startup.SplitMode != perf.SplitMode ||
		startup.Poll != expectedPoll ||
		startup.Priority != expectedPriority ||
		startup.RADVPerfTest != perf.RADVPerfTest {
		return fmt.Errorf("startup_manifest_performance_mismatch")
	}
	if perf.Threads > 0 && (startup.Threads != perf.Threads || startup.ThreadsBatch != perf.ThreadsBatch) {
		return fmt.Errorf("startup_manifest_performance_mismatch")
	}
	return nil
}

func actualDeviceMatches(plan runtimePlan, actual string) bool {
	actual = strings.TrimSpace(actual)
	if plan.Config.Backend == "cpu" {
		return actual == "cpu"
	}
	return actual != "" && strings.EqualFold(actual, resolvedRuntimeDevice(plan.Profile))
}

func validReportStatusToolCall(raw json.RawMessage) bool {
	var call struct {
		Type     string `json:"type"`
		Function struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	}
	if json.Unmarshal(raw, &call) != nil || call.Function.Name != "report_status" {
		return false
	}
	arguments := call.Function.Arguments
	var encoded string
	if json.Unmarshal(arguments, &encoded) == nil {
		arguments = json.RawMessage(encoded)
	}
	var value struct {
		Status string `json:"status"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(arguments)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || value.Status != "ok" {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

// Benchmark evaluates MTP with one warm-up and three measured tool-call runs
// per profile, then performs the controlled 32K memory/offload qualification.
func (m *Manager) Benchmark(ctx context.Context) (MTPDecision, error) {
	release, err := m.acquireControl(ctx)
	if err != nil {
		return MTPDecision{}, err
	}
	defer release()
	decision, err := m.benchmark(ctx)
	if err != nil {
		return decision, err
	}
	m.mu.Lock()
	cacheStatus := m.status.PromptCache
	if decision.Runtime == nil {
		decision.Runtime = &RuntimeBenchmark{
			PerformanceProfile: m.status.PerformanceProfile,
			ContextSize:        m.status.ContextSize,
		}
	}
	decision.PromptCache = &cacheStatus
	m.mu.Unlock()
	return decision, nil
}

func (m *Manager) benchmark(ctx context.Context) (MTPDecision, error) {
	cfg, generation, desiredCtx, err := m.desiredSnapshot()
	if err != nil {
		return MTPDecision{}, err
	}
	ctx, cancel := operationContext(ctx, desiredCtx)
	defer cancel()
	plan, err := m.resolveRuntimePlan(ctx, cfg, generation)
	if err != nil {
		return MTPDecision{}, err
	}
	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return MTPDecision{}, fmt.Errorf("desired_state_changed")
	}
	if m.activeRequests > 0 {
		m.mu.Unlock()
		return MTPDecision{}, fmt.Errorf("active_requests")
	}
	m.status.Operation = "benchmark"
	m.status.OperationInProgress = true
	m.status.State = "benchmarking"
	m.operationWG.Add(1)
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		if m.status.Operation == "benchmark" {
			m.status.Operation = ""
			m.status.OperationInProgress = false
			if m.status.State == "benchmarking" {
				m.status.State = "running"
			}
		}
		m.mu.Unlock()
		m.operationWG.Done()
	}()

	// Benchmark is an explicit managed action and may be invoked while the
	// sidecar is idle-stopped. Re-establish and attest the immutable desired
	// plan before taking any samples.
	if err := m.startWithPlan(ctx, plan); err != nil {
		return MTPDecision{}, err
	}

	var decision MTPDecision
	switch cfg.MTP {
	case "off":
		if err := m.smokeTestPlan(ctx, plan); err != nil {
			return MTPDecision{}, err
		}
		decision = MTPDecision{Reason: "mtp_disabled"}
	case "mtp2":
		if err := m.smokeTestPlan(ctx, plan); err != nil {
			return MTPDecision{}, err
		}
		decision = MTPDecision{Selected: true, Reason: "mtp_forced"}
		m.mu.Lock()
		if generation == m.generation {
			m.status.MTP = decision
		}
		m.mu.Unlock()
	case "auto":
		decision, err = m.benchmarkMTPAuto(ctx, plan)
		if err != nil {
			return MTPDecision{}, err
		}
		if decision.Selected {
			plan.Draft = plan.DownloadDraft
		} else {
			plan.Draft = nil
		}
		plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, plan.Draft != nil, plan.Profile)
	default:
		return MTPDecision{}, fmt.Errorf("invalid_mtp_mode")
	}
	if err := m.verify32KCapability(ctx, plan); err != nil {
		return decision, err
	}
	return decision, nil
}

func (m *Manager) benchmarkMTPAuto(ctx context.Context, plan runtimePlan) (MTPDecision, error) {
	// A new auto-benchmark supersedes any older cached success for this
	// fingerprint. A failed re-test must remain visibly on the target-only profile.
	m.mu.Lock()
	_ = os.Remove(filepath.Join(m.modelDir, "mtp-cache.json"))
	if plan.Generation == m.generation {
		m.status.MTP = MTPDecision{Reason: "mtp_benchmark_running"}
	}
	m.mu.Unlock()
	benchmarkPlan := mtpMeasurementPlan(plan)
	target, err := m.benchmarkProfile(ctx, benchmarkPlan, false)
	if err != nil {
		decision := MTPDecision{Reason: "mtp_target_profile_failed"}
		m.mu.Lock()
		if plan.Generation == m.generation {
			m.status.MTP = decision
			m.saveMTPDecisionLocked(decision)
		}
		m.mu.Unlock()
		if restoreCtx, restoreCancel, restoreErr := m.benchmarkRestoreContext(plan); restoreErr == nil {
			plan.Draft = nil
			plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, false, plan.Profile)
			_ = m.restoreBenchmarkPlan(restoreCtx, plan, false)
			restoreCancel()
		}
		return MTPDecision{}, err
	}
	benchmarkPlan.ResolvedParameters = resolvedParametersForPlan(
		benchmarkPlan.Config,
		true,
		benchmarkPlan.Profile,
	)
	speculative, err := m.benchmarkProfile(ctx, benchmarkPlan, true)
	if err != nil {
		reason := "mtp_profile_failed"
		if strings.Contains(err.Error(), "oom") || strings.Contains(err.Error(), "offload") {
			reason = "mtp_oom_or_offload_error"
		}
		decision := MTPDecision{Reason: reason}
		m.mu.Lock()
		if plan.Generation == m.generation {
			m.status.MTP = decision
			m.saveMTPDecisionLocked(decision)
		}
		m.mu.Unlock()
		restoreCtx, restoreCancel, restoreErr := m.benchmarkRestoreContext(plan)
		if restoreErr != nil {
			return decision, restoreErr
		}
		defer restoreCancel()
		if restoreErr := m.restoreBenchmarkPlan(restoreCtx, plan, false); restoreErr != nil {
			return decision, restoreErr
		}
		return decision, nil
	}
	decision := EvaluateMTP(target, speculative)
	selectedSamples := target
	if decision.Selected {
		selectedSamples = speculative
	}
	decision.Runtime = &RuntimeBenchmark{
		PerformanceProfile: performanceProfileFor(plan.Profile).Name,
		ContextSize:        benchmarkPlan.Config.ContextSize,
		GenerationTPS:      median(selectedSamples, func(v BenchmarkSample) float64 { return v.GenerationTokensS }),
		TTFTMilliseconds:   median(selectedSamples, func(v BenchmarkSample) float64 { return v.TTFTMilliseconds }),
		DraftAcceptance:    median(speculative, func(v BenchmarkSample) float64 { return v.DraftAcceptance }),
	}
	m.mu.Lock()
	if plan.Generation == m.generation {
		m.status.MTP = decision
		m.saveMTPDecisionLocked(decision)
	}
	m.mu.Unlock()
	restoreCtx, restoreCancel, err := m.benchmarkRestoreContext(plan)
	if err != nil {
		return decision, err
	}
	defer restoreCancel()
	if err := m.restoreBenchmarkPlan(restoreCtx, plan, decision.Selected); err != nil {
		return decision, err
	}
	return decision, nil
}

func (m *Manager) ensureAutoMTPPlan(ctx context.Context, plan runtimePlan) (runtimePlan, error) {
	if plan.Config.MTP != "auto" {
		return plan, nil
	}
	if _, cached := m.loadMTPDecision(plan.Fingerprint); cached {
		return plan, nil
	}
	decision, err := m.benchmarkMTPAuto(ctx, plan)
	if err != nil {
		return runtimePlan{}, err
	}
	if decision.Selected {
		plan.Draft = plan.DownloadDraft
	} else {
		plan.Draft = nil
	}
	plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, plan.Draft != nil, plan.Profile)
	return plan, nil
}

func mtpMeasurementPlan(plan runtimePlan) runtimePlan {
	plan.Config.ContextSize = 8192
	plan.Draft = nil
	plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, false, plan.Profile)
	return plan
}

// benchmarkRestoreContext deliberately does not inherit cancellation from the
// caller. Once a temporary profile has replaced the saved desired state, a
// cancelled benchmark still has to restore it. Desired-state changes and
// manager shutdown continue to cancel restoration immediately.
func (m *Manager) benchmarkRestoreContext(plan runtimePlan) (context.Context, context.CancelFunc, error) {
	m.mu.Lock()
	if !m.generationCurrentLocked(plan.Generation) {
		m.mu.Unlock()
		return nil, nil, fmt.Errorf("desired_state_changed")
	}
	lifecycleCtx := m.lifecycleCtx
	desiredCtx := m.desiredCtx
	m.mu.Unlock()
	ctx, timeoutCancel := context.WithTimeout(lifecycleCtx, 180*time.Second)
	stopDesired := context.AfterFunc(desiredCtx, timeoutCancel)
	return ctx, func() {
		stopDesired()
		timeoutCancel()
	}, nil
}

func (m *Manager) restoreBenchmarkPlan(ctx context.Context, plan runtimePlan, speculative bool) error {
	if speculative {
		plan.Draft = plan.DownloadDraft
	} else {
		plan.Draft = nil
	}
	plan.ResolvedParameters = resolvedParametersForPlan(plan.Config, plan.Draft != nil, plan.Profile)
	if err := m.stop(ctx, false); err != nil {
		return err
	}
	if err := m.startWithPlan(ctx, plan); err != nil {
		return err
	}
	return m.smokeTestPlan(ctx, plan)
}

func (m *Manager) benchmarkProfile(ctx context.Context, plan runtimePlan, speculative bool) ([]BenchmarkSample, error) {
	m.mu.Lock()
	if plan.Generation == m.generation {
		m.status.MTP = MTPDecision{Selected: speculative, Reason: "mtp_benchmark_running"}
	}
	m.mu.Unlock()
	if err := m.restoreBenchmarkPlan(ctx, plan, speculative); err != nil {
		return nil, err
	}
	if warmup, err := m.benchmarkSample(ctx, plan.Config); err != nil {
		return nil, err
	} else if warmup.OOM || warmup.OffloadError {
		return nil, fmt.Errorf("mtp_warmup_oom_or_offload_error")
	}
	samples := make([]BenchmarkSample, 0, 3)
	for range 3 {
		sample, err := m.benchmarkSample(ctx, plan.Config)
		if err != nil {
			return nil, err
		}
		samples = append(samples, sample)
	}
	return samples, nil
}

func (m *Manager) benchmarkSample(ctx context.Context, cfg config.LocalLLMConfig) (BenchmarkSample, error) {
	key, err := m.runtimeKey()
	if err != nil {
		return BenchmarkSample{}, err
	}
	request := map[string]any{
		"model": config.LocalLLMModelAlias,
		"messages": []map[string]string{{
			"role": "user",
			"content": strings.Repeat("Use the report_status tool after checking this fixed test context. ", 180) +
				"Return status ready and code benchmark.",
		}},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": "report_status",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status": map[string]string{"type": "string"},
						"code":   map[string]string{"type": "string"},
					},
					"required": []string{"status", "code"},
				},
			},
		}},
		"tool_choice": "required", "parallel_tool_calls": false, "max_tokens": 128,
		"stream": true, "cache_prompt": false,
		"stream_options": map[string]bool{"include_usage": true},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return BenchmarkSample{}, err
	}
	endpoint := strings.TrimSuffix(cfg.Endpoint(m.runningInDocker), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return BenchmarkSample{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	started := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return BenchmarkSample{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		sample := benchmarkFailureSample(string(message))
		if sample.OOM || sample.OffloadError {
			return sample, nil
		}
		return BenchmarkSample{}, fmt.Errorf("benchmark_http_%d", resp.StatusCode)
	}

	type toolDelta struct {
		Index    int `json:"index"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	var name, arguments string
	var ttft time.Duration
	var predictedPerSecond float64
	var draftN, draftAccepted int
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if failed := benchmarkFailureSample(data); failed.OOM || failed.OffloadError {
			return failed, nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string      `json:"content"`
					ToolCalls []toolDelta `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Timings struct {
				PredictedPerSecond float64 `json:"predicted_per_second"`
				DraftN             int     `json:"draft_n"`
				DraftNAccepted     int     `json:"draft_n_accepted"`
			} `json:"timings"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		for _, choice := range chunk.Choices {
			if ttft == 0 && (choice.Delta.Content != "" || len(choice.Delta.ToolCalls) > 0) {
				ttft = time.Since(started)
			}
			for _, call := range choice.Delta.ToolCalls {
				if call.Index != 0 {
					continue
				}
				name += call.Function.Name
				arguments += call.Function.Arguments
			}
		}
		if chunk.Timings.PredictedPerSecond > 0 {
			predictedPerSecond = chunk.Timings.PredictedPerSecond
		}
		if chunk.Timings.DraftN > 0 {
			draftN = chunk.Timings.DraftN
			draftAccepted = chunk.Timings.DraftNAccepted
		}
	}
	if err := scanner.Err(); err != nil {
		if failed := benchmarkFailureSample(err.Error()); failed.OOM || failed.OffloadError {
			return failed, nil
		}
		return BenchmarkSample{}, err
	}
	if name == "" || arguments == "" || ttft == 0 {
		return BenchmarkSample{}, fmt.Errorf("benchmark_tool_call_missing")
	}
	toolCall, _ := json.Marshal(map[string]any{
		"function": map[string]string{"name": name, "arguments": arguments},
	})
	acceptance := float64(0)
	if draftN > 0 {
		acceptance = float64(draftAccepted) / float64(draftN)
	}
	return BenchmarkSample{
		ToolCall:          string(toolCall),
		GenerationTokensS: predictedPerSecond,
		TTFTMilliseconds:  float64(ttft.Microseconds()) / 1000,
		DraftAcceptance:   acceptance,
	}, nil
}

func benchmarkFailureSample(message string) BenchmarkSample {
	lower := strings.ToLower(message)
	return BenchmarkSample{
		OOM: strings.Contains(lower, "out of memory") ||
			strings.Contains(lower, `"oom"`) ||
			strings.Contains(lower, "oom_error"),
		OffloadError: strings.Contains(lower, "offload") ||
			strings.Contains(lower, "device lost") ||
			strings.Contains(lower, "device_lost"),
	}
}

type contextCapabilityCacheEntry struct {
	Fingerprint string `json:"fingerprint"`
	MaxContext  int    `json:"max_context"`
}

func (m *Manager) contextCapabilityFingerprint(plan runtimePlan) string {
	draftSHA := ""
	if plan.Draft != nil {
		draftSHA = plan.Draft.SHA256
	}
	payload, _ := json.Marshal(struct {
		Hardware string
		Backend  string
		Image    string
		Model    string
		Draft    string
		Params   []string
	}{
		Hardware: plan.Profile.Fingerprint,
		Backend:  plan.Profile.SelectedBackend,
		Image:    plan.Image.Reference,
		Model:    plan.Model.SHA256,
		Draft:    draftSHA,
		Params: func() []string {
			cfg := plan.Config
			cfg.ContextSize = 32768
			return resolvedParametersForPlan(cfg, plan.Draft != nil, plan.Profile)
		}(),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) loadContextCapability(plan runtimePlan) (int, bool) {
	payload, err := os.ReadFile(filepath.Join(m.modelDir, "context-cache.json"))
	if err != nil {
		return 0, false
	}
	var entry contextCapabilityCacheEntry
	if json.Unmarshal(payload, &entry) != nil ||
		entry.Fingerprint != m.contextCapabilityFingerprint(plan) ||
		entry.MaxContext < 32768 {
		return 0, false
	}
	return entry.MaxContext, true
}

func (m *Manager) saveContextCapability(plan runtimePlan, maxContext int) error {
	if maxContext < 32768 {
		return fmt.Errorf("context_capability_not_verified")
	}
	if err := os.MkdirAll(m.modelDir, 0o700); err != nil {
		return err
	}
	payload, err := json.Marshal(contextCapabilityCacheEntry{
		Fingerprint: m.contextCapabilityFingerprint(plan),
		MaxContext:  maxContext,
	})
	if err != nil {
		return err
	}
	part := filepath.Join(m.modelDir, "context-cache.json.part")
	if err := os.WriteFile(part, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(part, filepath.Join(m.modelDir, "context-cache.json"))
}

func (m *Manager) verify32KCapability(ctx context.Context, original runtimePlan) error {
	if original.Profile.SelectedBackend == "cpu" {
		return fmt.Errorf("context_32k_requires_gpu_offload")
	}
	if original.Config.ContextSize == 32768 {
		if err := m.smokeTestPlan(ctx, original); err != nil {
			return err
		}
		m.mu.Lock()
		verified := original.Generation == m.generation &&
			m.status.GPUOffloadVerified && m.status.MemoryProfileVerified
		m.mu.Unlock()
		if !verified {
			return fmt.Errorf("context_32k_memory_or_offload_not_verified")
		}
		return m.saveContextCapability(original, 32768)
	}

	temporary := original
	temporary.Config.ContextSize = 32768
	temporary.ResolvedParameters = resolvedParametersForPlan(temporary.Config, temporary.Draft != nil, temporary.Profile)
	if err := m.stop(ctx, false); err != nil {
		return err
	}
	qualificationErr := m.startWithPlan(ctx, temporary)
	if qualificationErr == nil {
		qualificationErr = m.smokeTestPlan(ctx, temporary)
	}
	if qualificationErr == nil {
		m.mu.Lock()
		qualified := temporary.Generation == m.generation &&
			m.status.GPUOffloadVerified && m.status.MemoryProfileVerified &&
			m.status.VerifiedContextSize >= 32768
		m.mu.Unlock()
		if !qualified {
			qualificationErr = fmt.Errorf("context_32k_memory_or_offload_not_verified")
		}
	}
	if qualificationErr == nil {
		qualificationErr = m.saveContextCapability(temporary, 32768)
	}

	restoreCtx, restoreCancel, restoreContextErr := m.benchmarkRestoreContext(original)
	var restoreErr error
	if restoreContextErr != nil {
		restoreErr = restoreContextErr
	} else {
		restoreErr = m.restoreBenchmarkPlan(restoreCtx, original, original.Draft != nil)
		restoreCancel()
	}
	if qualificationErr != nil {
		if restoreErr != nil {
			return fmt.Errorf("context_32k_failed: %v; restore_failed: %w", qualificationErr, restoreErr)
		}
		return fmt.Errorf("context_32k_failed: %w", qualificationErr)
	}
	return restoreErr
}

// Status returns a copy with no paths, raw logs, or credentials.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.status
	status.Warnings = append([]string(nil), status.Warnings...)
	status.ResolvedParameters = append([]string(nil), status.ResolvedParameters...)
	status.ActiveRequests = m.activeRequests
	return status
}

func (m *Manager) selectedArtifacts() (Artifact, *Artifact, error) {
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	return m.selectedArtifactsFor(cfg)
}

func (m *Manager) selectedArtifactsFor(cfg config.LocalLLMConfig) (Artifact, *Artifact, error) {
	suffix := cfg.ModelVariant
	if cfg.MTP == "off" {
		artifact, ok := m.manifest.Artifacts["normal_"+suffix]
		if !ok {
			return Artifact{}, nil, fmt.Errorf("model_variant_unavailable")
		}
		return artifact, nil, nil
	}
	target, targetOK := m.manifest.Artifacts["mtp_target_"+suffix]
	sidecar, sidecarOK := m.manifest.Artifacts["mtp_sidecar_"+suffix]
	if !targetOK || !sidecarOK {
		return Artifact{}, nil, fmt.Errorf("mtp_pair_unavailable")
	}
	// The pair is selected by the same suffix; a normal artifact can never be combined with a draft.
	return target, &sidecar, nil
}

func (m *Manager) resolveRuntimePlan(ctx context.Context, cfg config.LocalLLMConfig, generation uint64) (runtimePlan, error) {
	dockerOnline, nvidiaToolkit := m.probeDocker(ctx)
	profile := m.applyManifestCompatibility(m.probeHardwareForBackend(ctx, cfg.Backend, dockerOnline, nvidiaToolkit))
	fingerprint := m.computeDesiredFingerprint(cfg, profile)
	model, downloadDraft, err := m.selectedArtifactsFor(cfg)
	if err != nil {
		return runtimePlan{}, err
	}
	imageBackend := profile.SelectedBackend
	if imageBackend == "cpu" {
		// The Vulkan runtime is the CPU-capable llama.cpp build in v1. No GPU
		// device is mounted and startup attestation must report actual_device=cpu.
		imageBackend = "vulkan"
	}
	image, ok := m.manifest.Images[imageBackend]
	if !ok {
		return runtimePlan{}, fmt.Errorf("backend_not_supported")
	}

	var draft *Artifact
	var decision MTPDecision
	if cfg.MTP == "auto" {
		if cached, ok := m.loadMTPDecision(fingerprint); ok {
			decision = cached
		}
	}
	if cfg.MTP == "mtp2" || cfg.MTP == "auto" && decision.Selected {
		draft = downloadDraft
	}
	plan := runtimePlan{
		Generation: generation, Config: cfg, Profile: profile, Fingerprint: fingerprint,
		Model: model, DownloadDraft: downloadDraft, Draft: draft, Image: image,
		ResolvedParameters: resolvedParametersForPlan(cfg, draft != nil, profile),
	}

	m.mu.Lock()
	if generation != m.generation {
		m.mu.Unlock()
		return runtimePlan{}, fmt.Errorf("desired_state_changed")
	}
	m.profile = profile
	if fingerprint != m.desiredFingerprint {
		m.desiredFingerprint = fingerprint
		m.invalidateVerificationLocked()
	}
	m.status.DesiredFingerprint = fingerprint
	m.status.Compatibility = profile.Compatibility
	m.status.Warnings = append([]string(nil), profile.Warnings...)
	m.status.Backend = profile.SelectedBackend
	m.status.PhysicalDevice = profile.SelectedDevice
	m.status.ResolvedProfile = m.manifest.profileFor(profile)
	m.status.HardwareFingerprint = profile.Fingerprint
	m.status.AcknowledgementDue = profile.AcknowledgementDue && !m.acknowledged(profile.Fingerprint)
	m.status.ContextSize = cfg.ContextSize
	perf := performanceProfileFor(profile)
	m.status.PerformanceProfile = perf.Name
	m.status.PromptCache.CacheRAMMiB = perf.CacheRAMMiB
	m.status.PromptCache.CheckpointProfile = "32x2048"
	if m.status.PromptCache.State == "" || m.status.PromptCache.State == "disabled" {
		m.status.PromptCache.State = "cold"
	}
	if gpu, gpuErr := profile.selectedGPU(); gpuErr == nil {
		m.status.VRAMBytes = gpu.VRAMBytes
	}
	if cfg.MTP == "auto" && decision.Reason != "" {
		m.status.MTP = decision
	}
	m.mu.Unlock()
	return plan, nil
}

func (m *Manager) generationCurrentLocked(generation uint64) bool {
	return generation == m.generation && !m.shuttingDown
}

func (m *Manager) probeDocker(ctx context.Context) (bool, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerutil.Endpoint("_ping"), nil)
	if err != nil {
		return false, false
	}
	resp, err := m.docker.HTTPClient().Do(req)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, false
	}
	var info struct {
		Runtimes map[string]json.RawMessage `json:"Runtimes"`
	}
	_, infoErr := m.docker.DoJSON(ctx, http.MethodGet, "info", nil, &info)
	_, nvidiaToolkit := info.Runtimes["nvidia"]
	return true, infoErr == nil && nvidiaToolkit
}

func (m *Manager) verifyRunningHealth(ctx context.Context, plan runtimePlan) error {
	key, err := m.runtimeKey()
	if err != nil {
		return &UnavailableError{Code: "runtime_key_unavailable", Err: err}
	}
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.checkHealth(healthCtx, key, plan.Config); err != nil {
		m.mu.Lock()
		m.setErrorLocked("health_check_failed")
		m.mu.Unlock()
		return &UnavailableError{Code: "health_check_failed", Err: err}
	}
	m.mu.Lock()
	if plan.Generation == m.generation {
		now := time.Now()
		m.status.LastHealthCheck = &now
	}
	m.mu.Unlock()
	return nil
}

func (m *Manager) checkHealth(ctx context.Context, key string, cfg config.LocalLLMConfig) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(cfg.Endpoint(m.runningInDocker), "/v1")+"/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("local_health_returned_%d", resp.StatusCode)
	}
	return nil
}

func (m *Manager) waitForHealth(ctx context.Context, key string, cfg config.LocalLLMConfig) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := m.checkHealth(ctx, key, cfg); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) apiJSON(ctx context.Context, method, path, key string, body, response any) error {
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	return m.apiJSONFor(ctx, cfg, method, path, key, body, response)
}

func (m *Manager) apiJSONFor(ctx context.Context, cfg config.LocalLLMConfig, method, path, key string, body, response any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = strings.NewReader(string(payload))
	}
	base := strings.TrimSuffix(cfg.Endpoint(m.runningInDocker), "/v1")
	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("local API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(response)
}

func (m *Manager) ensureRuntimeKey() (string, error) {
	key, err := m.runtimeKey()
	if err != nil {
		buffer := make([]byte, 32)
		if _, err := rand.Read(buffer); err != nil {
			return "", err
		}
		key = hex.EncodeToString(buffer)
		if m.vault == nil {
			return "", fmt.Errorf("vault_unavailable")
		}
		if err := m.vault.WriteSecret(config.LocalLLMRuntimeAPIKeyVaultKey, key); err != nil {
			return "", err
		}
	}
	return key, nil
}

func (m *Manager) runtimeKey() (string, error) {
	if m.vault == nil {
		return "", fmt.Errorf("vault_unavailable")
	}
	key, err := m.vault.ReadSecret(config.LocalLLMRuntimeAPIKeyVaultKey)
	if err != nil || strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("runtime_key_unavailable")
	}
	return strings.TrimSpace(key), nil
}

func (m *Manager) cleanupRuntimeKey() {
	_ = os.Remove(filepath.Join(m.runtimeDir, "api-key"))
}

func (m *Manager) acknowledged(fingerprint string) bool {
	if m.vault == nil || fingerprint == "" {
		return false
	}
	value, err := m.vault.ReadSecret("local_llm_ack_" + fingerprint)
	return err == nil && value == "acknowledged"
}

func (m *Manager) computeDesiredFingerprint(cfg config.LocalLLMConfig, profile HardwareProfile) string {
	payload, _ := json.Marshal(struct {
		Config      config.LocalLLMConfig
		Profile     string
		Performance runtimePerformanceProfile
		Manifest    Manifest
	}{cfg, profile.Fingerprint, performanceProfileFor(profile), m.manifest})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

type mtpCacheEntry struct {
	Fingerprint string      `json:"fingerprint"`
	Decision    MTPDecision `json:"decision"`
}

func (m *Manager) loadMTPDecisionLocked() (MTPDecision, bool) {
	return m.loadMTPDecision(m.desiredFingerprint)
}

func (m *Manager) loadMTPDecision(fingerprint string) (MTPDecision, bool) {
	payload, err := os.ReadFile(filepath.Join(m.modelDir, "mtp-cache.json"))
	if err != nil {
		return MTPDecision{}, false
	}
	var entry mtpCacheEntry
	if json.Unmarshal(payload, &entry) != nil || entry.Fingerprint != fingerprint {
		return MTPDecision{}, false
	}
	return entry.Decision, true
}

func (m *Manager) saveMTPDecisionLocked(decision MTPDecision) {
	if err := os.MkdirAll(m.modelDir, 0o700); err != nil {
		return
	}
	payload, err := json.Marshal(mtpCacheEntry{Fingerprint: m.desiredFingerprint, Decision: decision})
	if err != nil {
		return
	}
	tmp := filepath.Join(m.modelDir, "mtp-cache.json.part")
	if os.WriteFile(tmp, payload, 0o600) == nil {
		_ = os.Rename(tmp, filepath.Join(m.modelDir, "mtp-cache.json"))
	}
}

func (m *Manager) idleLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.idleStop:
			return
		case now := <-ticker.C:
			m.mu.Lock()
			reconcile := false
			stopForDisable := false
			if m.pendingCfg != nil && m.activeRequests == 0 {
				m.cfg = *m.pendingCfg
				m.pendingCfg = nil
				m.status.PendingRestart = true
				reconcile = m.status.State == "running"
				stopForDisable = !m.cfg.Enabled
			}
			idle := time.Duration(m.cfg.IdleTimeoutMinutes) * time.Minute
			shouldStop := m.status.State == "running" && m.activeRequests == 0 && idle > 0 && now.Sub(m.lastRelease) >= idle
			m.mu.Unlock()
			if reconcile && !stopForDisable {
				_ = m.Recreate(context.Background())
			} else if reconcile || shouldStop {
				_ = m.Stop(context.Background(), false)
			}
		}
	}
}

func (m *Manager) fail(code string, err error) error {
	m.mu.Lock()
	m.setErrorLocked(code)
	m.mu.Unlock()
	return &UnavailableError{Code: code, Err: err}
}

func (m *Manager) failGeneration(generation uint64, code string, err error) error {
	m.mu.Lock()
	if generation != m.generation || m.shuttingDown {
		m.mu.Unlock()
		return &UnavailableError{Code: "desired_state_changed", Err: err}
	}
	m.setErrorLocked(code)
	m.mu.Unlock()
	return &UnavailableError{Code: code, Err: err}
}

func (m *Manager) setErrorLocked(code string) {
	m.status.State = "error"
	m.status.ErrorCode = code
	m.status.Recommendation = recommendation(code)
}

func (m *Manager) clearErrorLocked() {
	m.status.ErrorCode = ""
	m.status.Recommendation = ""
}

func errorCode(err error) string {
	if err == nil {
		return "local_llm_error"
	}
	var unavailable *UnavailableError
	if errors.As(err, &unavailable) {
		return unavailable.Code
	}
	message := err.Error()
	if pos := strings.IndexAny(message, ": "); pos > 0 {
		message = message[:pos]
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return "local_llm_error"
	}
	return message
}

func recommendation(code string) string {
	switch code {
	case "release_artifacts_unavailable":
		return "Wait for a release manifest with public, commit-pinned model files and smoke-tested image digests."
	case "experimental_hardware_acknowledgement_required":
		return "Review and acknowledge that iGPU or CPU performance may be unsatisfactory."
	case "model_not_installed", "draft_not_installed":
		return "Run the explicit install action before selecting AuraGo-Qwen for chat routing."
	case "gpu_offload_not_verified":
		return "Select a supported GPU backend and verify device access, VRAM, and container runtime support."
	case "listen_port_unavailable":
		return "Choose an unused loopback port, save the Local LLM configuration, and retry the operation."
	default:
		return "Run the passive probe and review Docker, GPU, and storage compatibility."
	}
}

func resolvedParameters(cfg config.LocalLLMConfig, mtp bool) []string {
	values := []string{
		"--alias=aurago-qwen",
		"--fit=off",
		"--kv-offload=on",
		"--reasoning=off",
		fmt.Sprintf("--ctx-size=%d", cfg.ContextSize),
	}
	if mtp {
		values = append(values,
			"--spec-draft-n-max=2",
			"--spec-draft-n-min=0",
			"--spec-draft-p-min=0.80",
			"--spec-draft-ngl=all",
		)
	}
	return values
}

// resolvedRuntimeDevice is safe only because the container receives exactly
// one physical GPU. The pinned image must still derive this alias with
// llama-server --list-devices and attest the matching physical PCI/DRM identity
// in its startup manifest before AuraGo accepts it.
func resolvedRuntimeDevice(profile HardwareProfile) string {
	switch profile.SelectedBackend {
	case "cuda":
		return "CUDA0"
	case "sycl":
		return "SYCL0"
	case "vulkan":
		return "Vulkan0"
	case "cpu":
		return "cpu"
	default:
		return ""
	}
}

func resolvedParametersForPlan(cfg config.LocalLLMConfig, mtp bool, profile HardwareProfile) []string {
	values := resolvedParameters(cfg, mtp)
	values = append(values, "--parallel=1")
	values = append(values, performanceParameters(cfg, profile)...)
	if profile.SelectedBackend == "cpu" {
		if mtp {
			for index := range values {
				if values[index] == "--spec-draft-ngl=all" {
					values[index] = "--spec-draft-ngl=0"
				}
			}
			return append(values,
				"--spec-type=draft-mtp",
				"--spec-draft-device=cpu",
			)
		}
		return append(values, "--spec-type=none")
	}
	device := resolvedRuntimeDevice(profile)
	values = append(values,
		"--device="+device,
		"--n-gpu-layers=all",
	)
	if mtp {
		values = append(values,
			"--spec-type=draft-mtp",
			"--spec-draft-device="+device,
		)
	} else {
		values = append(values, "--spec-type=none")
	}
	return values
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
