package decisions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

const maxEvaluationTokens = 8192

// Manager owns the optional, single-generation decision runtime. Construction
// and configuration are passive: no files, devices, processes, or containers
// are acquired until Install or Start is called.
type Manager struct {
	mu                sync.RWMutex
	lifecycle         sync.Mutex
	evalGate          chan struct{}
	evalSlots         chan struct{}
	activeEvalID      uint64
	activeEvalCancel  context.CancelFunc
	cfg               config.DecisionsConfig
	dataDir           string
	options           managerOptions
	factory           runtimeFactory
	resolveModelPaths func(string) (modelPaths, error)
	session           runtimeSession
	sessionStop       context.CancelFunc
	operationCancel   context.CancelFunc
	operationID       uint64
	activeDeployment  string
	activeDockerHost  string
	activeIsDocker    bool
	activeDevice      runtimeDevice
	operationAccess   RuntimeAccess
	lastAccess        RuntimeAccess
	policyGeneration  uint64
	generation        uint64
	cleanupPending    int
	stopPending       int
	cleanupChanged    chan struct{}
	closed            bool
	status            Status
}

// NewManager creates a passive local decisions manager.
func NewManager(cfg config.DecisionsConfig, dataDir string, options ...ManagerOption) *Manager {
	cfg = cfg.Normalize()
	managerOpts := managerOptions{}
	for _, option := range options {
		if option != nil {
			option(&managerOpts)
		}
	}
	manager := &Manager{
		cfg:     cfg,
		dataDir: dataDir,
		options: managerOpts,
		resolveModelPaths: func(dataDir string) (modelPaths, error) {
			return validateInstalledModelPaths(filepath.Join(dataDir, "decisions", "models"), DefaultManifest())
		},
		evalGate:       make(chan struct{}, 1),
		evalSlots:      make(chan struct{}, 5),
		cleanupChanged: make(chan struct{}),
	}
	manager.lastAccess = managerOpts.access()
	if managerOpts.runtimeFactory != nil {
		manager.factory = managerOpts.runtimeFactory()
	} else {
		manager.factory = newManagedRuntimeFactory(managerOpts)
	}
	manager.status = Status{
		Enabled: cfg.Enabled, State: "disabled", Deployment: cfg.Deployment,
		CPUFallbackAllowed: cfg.CPUFallbackAllowed,
		Runtime:            RuntimeInfo{Deployment: cfg.Deployment},
	}
	if cfg.Enabled {
		manager.status.State = "not_installed"
		manager.status.Code = CodeModelNotInstalled
	}
	return manager
}

// Status returns a detached snapshot suitable for an admin API.
func (manager *Manager) Status() Status {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.status
}

// CacheGeneration invalidates derived classifications on every lifecycle change.
func (manager *Manager) CacheGeneration() uint64 {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.generation
}

// PrepareReconfigure immediately publishes a new policy and cancels any
// operation or runtime generation that is no longer authorized. The returned
// cleanup must be run after releasing caller-owned locks; it waits for any
// in-flight lifecycle operation before closing the detached session. Cleanup
// errors never restore old permissions.
func (manager *Manager) PrepareReconfigure(cfg config.DecisionsConfig) (func(context.Context) error, error) {
	if err := cfg.Validate(); err != nil {
		return nil, NewError(CodeInvalidRequest, err)
	}
	cfg = cfg.Normalize()
	manager.mu.Lock()
	if manager.closed {
		manager.mu.Unlock()
		return nil, NewError(CodeServiceDisabled, nil)
	}
	previous := manager.cfg
	previousAccess := manager.lastAccess
	manager.cfg = cfg
	access := manager.options.access()
	policyChanged := previous != cfg || previousAccess != access
	if policyChanged {
		manager.policyGeneration++
	}
	manager.lastAccess = access
	manager.status.Enabled = cfg.Enabled
	manager.status.CPUFallbackAllowed = cfg.CPUFallbackAllowed
	manager.status.Deployment = cfg.Deployment
	oldSession := manager.session
	oldCancel := manager.sessionStop
	opCancel := manager.operationCancel
	revoke := oldSession != nil && (!cfg.Enabled ||
		(manager.status.CPUFallbackActive && !cfg.CPUFallbackAllowed) ||
		(manager.activeDeployment == config.DecisionsDeploymentDocker &&
			(!dockerMayMutate(access) || access.DockerHost != manager.activeDockerHost || access.IsDocker != manager.activeIsDocker)))
	if previous.Deployment != cfg.Deployment && oldSession != nil {
		revoke = true
	}
	if policyChanged {
		// Generation also fences status/result writes from work that raced the
		// policy publication, even when an active accelerator session remains
		// permitted by the new policy.
		manager.generation++
	}
	if policyChanged && opCancel != nil {
		manager.operationCancel = nil
	}
	if revoke {
		manager.session = nil
		manager.sessionStop = nil
		manager.status.Ready = false
		manager.status.State = "stopping"
		manager.status.Code = ""
		manager.status.ErrorMessage = ""
		manager.status.CPUFallbackActive = false
	}
	needsCleanup := revoke || (policyChanged && opCancel != nil)
	if needsCleanup {
		manager.cleanupPending++
		manager.signalCleanupChangedLocked()
		if !revoke {
			manager.status.Ready = false
			manager.status.CPUFallbackActive = false
			manager.status.GPUOffloadVerified = false
			manager.status.State = "stopping"
			manager.status.Code = ""
			manager.status.ErrorMessage = ""
			manager.status.Progress = Progress{}
		}
	}
	if oldSession != nil && !revoke {
		// `auto` is a selection policy; the status should identify the concrete
		// deployment currently serving requests.
		manager.status.Deployment = manager.activeDeployment
		manager.status.Runtime.Deployment = manager.activeDeployment
	}
	if !cfg.Enabled {
		manager.status.Ready = false
		manager.status.State = "disabled"
		manager.status.Code = CodeServiceDisabled
	}
	activeEvalCancel := manager.activeEvalCancel
	manager.mu.Unlock()

	if opCancel != nil && policyChanged {
		opCancel()
	}
	if activeEvalCancel != nil && policyChanged {
		activeEvalCancel()
	}
	if oldCancel != nil && revoke {
		oldCancel()
	}
	if !needsCleanup {
		return func(context.Context) error { return nil }, nil
	}
	var cleanupOnce sync.Once
	var cleanupErr error
	return func(ctx context.Context) error {
		cleanupOnce.Do(func() {
			if ctx == nil {
				ctx = context.Background()
			}
			manager.lifecycle.Lock()
			defer manager.lifecycle.Unlock()
			var closeErr error
			if oldSession != nil {
				closeErr = oldSession.Close(ctx)
			}
			manager.mu.Lock()
			// Only the last outstanding cleanup may publish a settled state.
			// A rapid second policy update can advance generation while this
			// cleanup waits; the current policy still owns the eventual status.
			if manager.cleanupPending == 1 && manager.session == nil && manager.operationCancel == nil {
				if closeErr != nil {
					if manager.cfg.Enabled {
						manager.status.State = "stopped_with_error"
						manager.status.Code = CodeRuntimeUnavailable
					} else {
						manager.status.State = "disabled"
						manager.status.Code = CodeServiceDisabled
					}
					manager.status.ErrorMessage = manager.status.Code
				} else if manager.cfg.Enabled {
					manager.status.State = "stopped"
					manager.status.Code = ""
					manager.status.ErrorMessage = ""
				} else {
					manager.status.State = "disabled"
					manager.status.Code = CodeServiceDisabled
					manager.status.ErrorMessage = ""
				}
				manager.status.Deployment = manager.cfg.Deployment
				manager.status.Runtime.Deployment = manager.cfg.Deployment
				manager.activeDeployment = ""
				manager.activeDockerHost = ""
				manager.activeIsDocker = false
				manager.activeDevice = runtimeDevice{}
			}
			manager.cleanupPending--
			manager.signalCleanupChangedLocked()
			manager.mu.Unlock()
			if closeErr != nil {
				cleanupErr = NewError(CodeRuntimeUnavailable, closeErr)
			}
		})
		return cleanupErr
	}, nil
}

func (manager *Manager) signalCleanupChangedLocked() {
	close(manager.cleanupChanged)
	manager.cleanupChanged = make(chan struct{})
}

// Reconfigure is the synchronous compatibility API. Callers that hold broad
// locks should use PrepareReconfigure and run its cleanup after unlocking.
func (manager *Manager) Reconfigure(cfg config.DecisionsConfig) error {
	cleanup, err := manager.PrepareReconfigure(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return cleanup(ctx)
}

// Install verifies or provisions the local runtime and downloads/verifies the
// immutable model and projector pins. The runtime is provisioned first so a
// missing bundle never triggers a large model download.
func (manager *Manager) Install(ctx context.Context) error {
	if !manager.lifecycle.TryLock() {
		return NewError(CodeLifecycleBusy, nil)
	}
	defer manager.lifecycle.Unlock()
	if manager.cleanupIsPending() {
		return NewError(CodeLifecycleBusy, nil)
	}
	if err := manager.authorizeEnabled(); err != nil {
		return err
	}
	// Provisioning may replace the executable currently mapped by the native
	// runtime (which Windows locks). Install is an explicit lifecycle action,
	// so stop the serving generation before touching managed runtime files.
	if err := manager.stopLocked(ctx); err != nil {
		return err
	}
	callerCtx := ctx
	ctx, operationCancel, operationID, stopParent, err := manager.beginOperation(ctx)
	if err != nil {
		return err
	}
	defer manager.finishOperation(operationCancel, operationID, stopParent, false)
	if err := ctx.Err(); err != nil {
		return NewError(CodeCancelled, cancellationCause(callerCtx))
	}
	manager.mu.RLock()
	cfg := manager.cfg
	manager.mu.RUnlock()
	access := manager.options.access()
	deployment, err := deploymentForInstall(cfg, access)
	if err != nil {
		manager.setInstallError(err)
		return err
	}
	// Verify or provision the pinned runtime first. A missing local native
	// bundle or Docker image should fail before downloading the 2 GB model.
	manager.setState("installing", "", Progress{Phase: "verifying_runtime"})
	err = manager.factory.install(ctx, deployment, access, manager.dataDir)
	if err != nil {
		if ErrorCode(err) == CodeInferenceFailed {
			err = NewError(CodeRuntimeArtifactMissing, err)
		}
		manager.setInstallError(err)
		return err
	}
	manager.setState("installing", "", Progress{Phase: "verifying_model"})
	cache := newArtifactCache(filepath.Join(manager.dataDir, "decisions", "models"), manager.options.httpClient, manager.updateProgress)
	paths, err := cache.ensure(ctx, DefaultManifest())
	if err != nil {
		manager.setInstallError(err)
		return err
	}
	if _, err := validateInstalledModelPaths(filepath.Dir(paths.Model), DefaultManifest()); err != nil {
		wrapped := WrapError(CodeModelIntegrityFailed, err)
		manager.setInstallError(wrapped)
		return wrapped
	}
	manager.mu.Lock()
	if ctx.Err() != nil || manager.closed || !manager.cfg.Enabled || manager.operationID != operationID || manager.operationCancel == nil {
		manager.mu.Unlock()
		return NewError(CodeCancelled, cancellationCause(callerCtx))
	}
	manager.status.Installed = true
	manager.status.State = "installed"
	manager.status.Code = ""
	manager.status.ErrorMessage = ""
	manager.status.Progress = Progress{Phase: "complete", Current: 1, Total: 1, Percent: 100}
	manager.mu.Unlock()
	return nil
}

// AutoStart starts only a previously installed model; startup never downloads.
func (manager *Manager) AutoStart(ctx context.Context) error {
	manager.mu.RLock()
	enabled := manager.cfg.Enabled && !manager.closed
	policyGeneration, generation, operationID := manager.policyGeneration, manager.generation, manager.operationID
	manager.mu.RUnlock()
	if !enabled {
		return nil
	}
	_, err := manager.resolveModelPaths(manager.dataDir)
	manager.mu.Lock()
	current := manager.cfg.Enabled && !manager.closed && manager.policyGeneration == policyGeneration &&
		manager.generation == generation && manager.operationID == operationID
	if err != nil && current {
		manager.status.Installed = false
		manager.status.Ready = false
		manager.status.State = "not_installed"
		manager.status.Code = CodeModelNotInstalled
	}
	manager.mu.Unlock()
	if !current {
		return NewError(CodeCancelled, context.Canceled)
	}
	if err != nil {
		return NewError(CodeModelNotInstalled, err)
	}
	if err := manager.waitForCleanup(ctx); err != nil {
		return NewError(CodeCancelled, err)
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		manager.mu.RLock()
		stillCurrent := manager.cfg.Enabled && !manager.closed && manager.policyGeneration == policyGeneration
		ready := stillCurrent && manager.status.Ready && manager.session != nil
		manager.mu.RUnlock()
		if !stillCurrent {
			return NewError(CodeCancelled, context.Canceled)
		}
		if ready {
			return nil
		}
		err = manager.Start(ctx)
		if ErrorCode(err) != CodeLifecycleBusy {
			return err
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return NewError(CodeCancelled, ctx.Err())
		case <-deadline.C:
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

// Start discovers only real accelerator devices first. CPU is selected only
// when the current configuration contains persistent explicit consent.
func (manager *Manager) Start(ctx context.Context) error {
	if !manager.lifecycle.TryLock() {
		return NewError(CodeLifecycleBusy, nil)
	}
	defer manager.lifecycle.Unlock()
	if manager.cleanupIsPending() {
		return NewError(CodeLifecycleBusy, nil)
	}
	if err := manager.authorizeEnabled(); err != nil {
		return err
	}
	if err := manager.stopLocked(ctx); err != nil {
		return err
	}
	callerCtx := ctx
	ctx, operationCancel, operationID, stopParent, err := manager.beginOperation(ctx)
	if err != nil {
		return err
	}
	keepSession := false
	defer func() { manager.finishOperation(operationCancel, operationID, stopParent, keepSession) }()
	if err := ctx.Err(); err != nil {
		return NewError(CodeCancelled, cancellationCause(callerCtx))
	}
	manager.mu.RLock()
	startPolicyGeneration := manager.policyGeneration
	manager.mu.RUnlock()
	paths, err := manager.resolveModelPaths(manager.dataDir)
	if err != nil {
		wrapped := NewError(CodeModelNotInstalled, err)
		manager.setInstallError(wrapped)
		return wrapped
	}
	manager.mu.RLock()
	cfg := manager.cfg
	manager.mu.RUnlock()
	access := manager.options.access()
	deployment, err := resolveDeployment(cfg.Deployment, access)
	if err != nil {
		manager.setRuntimeError(err)
		return err
	}
	if deployment == config.DecisionsDeploymentDocker && !dockerMayMutate(access) {
		err = NewError(CodeDeploymentDenied, nil)
		manager.setRuntimeError(err)
		return err
	}
	manager.setState("discovering_devices", "", Progress{})
	devices, err := manager.factory.discover(ctx, deployment, access, paths, manager.dataDir)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return NewError(CodeCancelled, cancellationCause(callerCtx))
	}
	if err != nil {
		manager.setRuntimeError(err)
		return err
	}
	ordered, err := orderedDevices(devices, runtime.GOOS, true)
	if err != nil {
		manager.setRuntimeError(err)
		return err
	}
	var accelerators []runtimeDevice
	var cpuDevices []runtimeDevice
	for _, device := range ordered {
		if strings.EqualFold(device.Type, "CPU") {
			cpuDevices = append(cpuDevices, device)
		} else {
			accelerators = append(accelerators, device)
		}
	}
	manager.setState("starting", "", Progress{})
	var lastErr error
	var acceleratorErr error
	var session runtimeSession
	var metadata runtimeMetadata
	var chosen runtimeDevice
	var cpuActive bool
	startCandidate := func(device runtimeDevice) bool {
		if err := ctx.Err(); err != nil {
			lastErr = NewError(CodeCancelled, cancellationCause(callerCtx))
			return false
		}
		candidateSession, startErr := manager.factory.start(ctx, deployment, device, paths, cfg, access, manager.dataDir)
		closeCandidate := func() {
			if candidateSession != nil {
				closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = candidateSession.Close(closeCtx)
				closeCancel()
			}
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			closeCandidate()
			lastErr = NewError(CodeCancelled, cancellationCause(callerCtx))
			return false
		}
		if startErr == nil {
			candidateMetadata, metadataErr := candidateSession.metadata(ctx)
			if ctxErr := ctx.Err(); ctxErr != nil {
				closeCandidate()
				lastErr = NewError(CodeCancelled, cancellationCause(callerCtx))
				return false
			}
			if metadataErr == nil {
				metadataErr = validateRuntimeMetadata(candidateMetadata, device, deployment, cfg.CPUFallbackAllowed)
			}
			if metadataErr == nil {
				session, metadata, chosen, cpuActive = candidateSession, candidateMetadata, device, metadataUsesCPU(candidateMetadata)
				return true
			}
			startErr = NewError(CodeRuntimeAttestationFailed, metadataErr)
			closeCandidate()
		}
		lastErr = startErr
		return false
	}
	// Try full GPU placement on every discrete/integrated accelerator before
	// considering any CPU participation.
	for _, device := range accelerators {
		device.GPULayers = 0
		if startCandidate(device) {
			break
		}
		acceleratorErr = lastErr
		if ErrorCode(lastErr) == CodeCancelled {
			return lastErr
		}
	}
	// Partial layer placement and CPU projector use are a separate, bounded
	// retry tier that is only enabled by persistent user consent.
	if session == nil && cfg.CPUFallbackAllowed {
		for _, discovered := range accelerators {
			for _, layers := range hybridLayerAttempts(discovered.ModelLayers) {
				device := discovered
				device.GPULayers = layers
				if startCandidate(device) {
					break
				}
				acceleratorErr = lastErr
				if ErrorCode(lastErr) == CodeCancelled {
					return lastErr
				}
			}
			if session != nil {
				break
			}
		}
	}
	// Pure CPU is the final option and remains unavailable until explicit
	// persistent consent is stored in config.
	if session == nil && cfg.CPUFallbackAllowed {
		for _, device := range cpuDevices {
			device.GPULayers = 0
			if startCandidate(device) {
				break
			}
			if ErrorCode(lastErr) == CodeCancelled {
				return lastErr
			}
		}
	}
	if session == nil {
		if lastErr == nil {
			if len(accelerators) == 0 && len(cpuDevices) > 0 && !cfg.CPUFallbackAllowed {
				lastErr = NewError(CodeCPUConsentRequired, nil)
			} else {
				lastErr = NewError(CodeHardwareUnavailable, nil)
			}
		}
		manager.setRuntimeError(lastErr)
		return lastErr
	}
	manager.mu.Lock()
	if ctxErr := ctx.Err(); ctxErr != nil {
		manager.mu.Unlock()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = session.Close(closeCtx)
		closeCancel()
		return NewError(CodeCancelled, cancellationCause(callerCtx))
	}
	if manager.closed || !manager.cfg.Enabled || manager.policyGeneration != startPolicyGeneration || (cpuActive && !manager.cfg.CPUFallbackAllowed) {
		manager.mu.Unlock()
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = session.Close(closeCtx)
		closeCancel()
		return NewError(CodeServiceDisabled, nil)
	}
	manager.generation++
	manager.session = session
	manager.sessionStop = operationCancel
	manager.operationCancel = nil
	manager.activeDeployment = deployment
	manager.activeDockerHost = access.DockerHost
	manager.activeIsDocker = access.IsDocker
	manager.activeDevice = chosen
	manager.status = statusFromMetadata(manager.cfg, deployment, metadata, cpuActive)
	manager.status.Enabled = manager.cfg.Enabled
	manager.status.Installed = true
	manager.status.Ready = true
	manager.status.State = "ready"
	manager.status.Progress = Progress{Phase: "ready", Percent: 100}
	if cpuActive && !strings.EqualFold(chosen.Type, "CPU") {
		manager.status.ExecutionMode = "mixed"
		manager.status.Code = "cpu_fallback_consent_used"
		manager.status.CPUFallbackReason = runtimeFailureReason(acceleratorErr)
		if manager.status.CPUFallbackReason == "" {
			manager.status.CPUFallbackReason = "gpu_accelerator_partial"
		}
	}
	if cpuActive && strings.EqualFold(chosen.Type, "CPU") {
		manager.status.Code = "cpu_fallback_consent_used"
		if acceleratorErr != nil {
			manager.status.CPUFallbackReason = runtimeFailureReason(acceleratorErr)
		} else {
			manager.status.CPUFallbackReason = "accelerator_unavailable"
		}
	}
	keepSession = true
	manager.mu.Unlock()
	return nil
}

func hybridLayerAttempts(total int) []int {
	if total < 2 {
		return nil
	}
	var attempts []int
	seen := make(map[int]struct{}, 4)
	for _, numerator := range []int{3, 2, 1} {
		layers := total * numerator / 4
		if layers < 1 {
			layers = 1
		}
		if layers >= total {
			layers = total - 1
		}
		if _, ok := seen[layers]; !ok {
			seen[layers] = struct{}{}
			attempts = append(attempts, layers)
		}
	}
	return attempts
}

// Stop revokes the active generation before waiting for the process/container.
func (manager *Manager) Stop(ctx context.Context) error {
	manager.mu.Lock()
	manager.stopPending++
	opCancel := manager.operationCancel
	manager.operationCancel = nil
	evalCancel := manager.activeEvalCancel
	session, cancel := manager.session, manager.sessionStop
	manager.session = nil
	manager.sessionStop = nil
	manager.generation++
	manager.policyGeneration++
	manager.status.Ready = false
	manager.status.CPUFallbackActive = false
	if manager.cfg.Enabled {
		manager.status.State = "stopping"
	} else {
		manager.status.State = "disabled"
	}
	manager.mu.Unlock()
	defer func() {
		manager.mu.Lock()
		manager.stopPending--
		manager.mu.Unlock()
	}()
	if opCancel != nil {
		opCancel()
	}
	if evalCancel != nil {
		evalCancel()
	}
	if cancel != nil {
		cancel()
	}
	if err := manager.waitForCleanup(ctx); err != nil {
		return NewError(CodeCancelled, err)
	}
	manager.lifecycle.Lock()
	defer manager.lifecycle.Unlock()
	if session != nil {
		if err := session.Close(ctx); err != nil {
			manager.mu.Lock()
			manager.status.State = "stopped_with_error"
			manager.status.Code = CodeRuntimeUnavailable
			manager.mu.Unlock()
			return NewError(CodeRuntimeUnavailable, err)
		}
	}
	manager.mu.Lock()
	if manager.cfg.Enabled {
		manager.status.State = "stopped"
		manager.status.Code = ""
	} else {
		manager.status.State = "disabled"
		manager.status.Code = CodeServiceDisabled
	}
	manager.activeDeployment = ""
	manager.activeDockerHost = ""
	manager.activeIsDocker = false
	manager.activeDevice = runtimeDevice{}
	manager.mu.Unlock()
	return nil
}

func (manager *Manager) stopLocked(ctx context.Context) error {
	manager.mu.Lock()
	session, cancel := manager.session, manager.sessionStop
	manager.session = nil
	manager.sessionStop = nil
	manager.generation++
	manager.status.Ready = false
	manager.status.CPUFallbackActive = false
	if manager.cfg.Enabled {
		manager.status.State = "stopping"
	} else {
		manager.status.State = "disabled"
	}
	manager.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if session != nil {
		if err := session.Close(ctx); err != nil {
			manager.mu.Lock()
			manager.status.State = "stopped_with_error"
			manager.status.Code = CodeRuntimeUnavailable
			manager.mu.Unlock()
			return NewError(CodeRuntimeUnavailable, err)
		}
	}
	manager.mu.Lock()
	if manager.cfg.Enabled {
		manager.status.State = "stopped"
		manager.status.Code = ""
	} else {
		manager.status.State = "disabled"
		manager.status.Code = CodeServiceDisabled
	}
	manager.activeDeployment = ""
	manager.activeDockerHost = ""
	manager.activeIsDocker = false
	manager.activeDevice = runtimeDevice{}
	manager.mu.Unlock()
	return nil
}

// Shutdown prevents future lifecycle actions and stops the current generation.
func (manager *Manager) Shutdown(ctx context.Context) error {
	manager.mu.Lock()
	manager.closed = true
	opCancel := manager.operationCancel
	manager.operationCancel = nil
	evalCancel := manager.activeEvalCancel
	session, cancel := manager.session, manager.sessionStop
	manager.session = nil
	manager.sessionStop = nil
	manager.generation++
	manager.policyGeneration++
	manager.status.Ready = false
	manager.status.State = "shutdown"
	manager.mu.Unlock()
	if opCancel != nil {
		opCancel()
	}
	if evalCancel != nil {
		evalCancel()
	}
	if cancel != nil {
		cancel()
	}
	if err := manager.waitForCleanup(ctx); err != nil {
		return NewError(CodeCancelled, err)
	}
	manager.lifecycle.Lock()
	defer manager.lifecycle.Unlock()
	var err error
	if session != nil {
		err = session.Close(ctx)
	}
	manager.mu.Lock()
	manager.status.State = "shutdown"
	manager.status.Ready = false
	manager.activeDeployment = ""
	manager.activeDockerHost = ""
	manager.activeIsDocker = false
	manager.activeDevice = runtimeDevice{}
	manager.mu.Unlock()
	if err != nil {
		return NewError(CodeRuntimeUnavailable, err)
	}
	return nil
}

// Evaluate scores all questions against the same verified runtime generation.
// Questions are sequential so image/context tokens count against one shared
// input budget; any failure discards the complete response.
func (manager *Manager) Evaluate(ctx context.Context, request Request) (Response, error) {
	if err := ValidateRequest(request); err != nil {
		return Response{}, NewError(CodeInvalidRequest, err)
	}
	manager.mu.RLock()
	if !manager.cfg.Enabled || manager.closed {
		manager.mu.RUnlock()
		return Response{}, NewError(CodeServiceDisabled, nil)
	}
	session := manager.session
	generation := manager.generation
	selectedDevice := manager.activeDevice
	deployment := manager.activeDeployment
	cpuAllowed := manager.cfg.CPUFallbackAllowed
	if session == nil || !manager.status.Ready {
		code := manager.status.Code
		manager.mu.RUnlock()
		if code == CodeCPUConsentRequired || code == CodeRuntimeAttestationFailed {
			return Response{}, NewError(code, nil)
		}
		return Response{}, NewError(CodeServiceNotReady, nil)
	}
	manager.mu.RUnlock()
	select {
	case manager.evalSlots <- struct{}{}:
		defer func() { <-manager.evalSlots }()
	default:
		return Response{}, NewError(CodeQueueFull, nil)
	}
	select {
	case manager.evalGate <- struct{}{}:
		defer func() { <-manager.evalGate }()
	case <-ctx.Done():
		return Response{}, NewError(CodeCancelled, ctx.Err())
	}
	evaluationCtx, cancelEvaluation := context.WithCancel(ctx)
	manager.mu.Lock()
	if manager.session != session || manager.generation != generation || !manager.cfg.Enabled || manager.closed || manager.cfg.CPUFallbackAllowed != cpuAllowed {
		manager.mu.Unlock()
		cancelEvaluation()
		return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
	}
	manager.activeEvalID++
	evaluationID := manager.activeEvalID
	manager.activeEvalCancel = cancelEvaluation
	manager.mu.Unlock()
	defer func() {
		cancelEvaluation()
		manager.mu.Lock()
		if manager.activeEvalID == evaluationID {
			manager.activeEvalCancel = nil
		}
		manager.mu.Unlock()
	}()
	ctx = evaluationCtx
	metadata, err := session.metadata(ctx)
	if err != nil {
		if ctx.Err() != nil || !manager.evaluationCurrent(session, generation, cpuAllowed) {
			return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
		}
		failure := NewError(CodeRuntimeUnavailable, err)
		manager.retireUnhealthyRuntime(session, generation, failure)
		return Response{}, failure
	}
	if err := validateRuntimeMetadata(metadata, selectedDevice, deployment, cpuAllowed); err != nil {
		failure := NewError(CodeRuntimeAttestationFailed, err)
		manager.retireUnhealthyRuntime(session, generation, failure)
		return Response{}, failure
	}
	result := Response{
		Model: modelID, ModelRevision: modelRevision, Calibration: "uncalibrated",
		Answers: make(map[string]Answer, len(request.Questions)),
		Runtime: runtimeInfo(metadata),
		Usage:   Usage{Questions: len(request.Questions), Images: len(request.Images)},
	}
	remaining := maxEvaluationTokens
	for _, question := range request.Questions {
		if err := ctx.Err(); err != nil {
			return Response{}, NewError(CodeCancelled, err)
		}
		if !manager.evaluationCurrent(session, generation, cpuAllowed) {
			return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
		}
		options := optionsForQuestion(question)
		candidates, err := candidatesForOptions(options)
		if err != nil {
			return Response{}, NewError(CodeInvalidRequest, err)
		}
		prompt, err := buildPrompt(request.State, question, options, metadata.ImageMarker, len(request.Images))
		if err != nil {
			return Response{}, NewError(CodeInvalidRequest, err)
		}
		score, err := session.score(ctx, runtimeScoreRequest{Prompt: prompt, Candidates: candidates, Images: request.Images, MaxInputTokens: remaining})
		if err != nil {
			code := ErrorCode(err)
			if ctx.Err() != nil || !manager.evaluationCurrent(session, generation, cpuAllowed) {
				return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
			}
			failure := NewError(code, err)
			manager.retireUnhealthyRuntime(session, generation, failure)
			return Response{}, failure
		}
		if err := validateRuntimeMetadata(score.Metadata, selectedDevice, deployment, cpuAllowed); err != nil {
			failure := NewError(CodeRuntimeAttestationFailed, err)
			manager.retireUnhealthyRuntime(session, generation, failure)
			return Response{}, failure
		}
		if !manager.evaluationCurrent(session, generation, cpuAllowed) {
			return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
		}
		if score.usage.PromptTokens < 0 || score.usage.ImageTokens < 0 || score.usage.ContextTokens != score.usage.PromptTokens+score.usage.ImageTokens || score.usage.ContextTokens < 0 || score.usage.ContextTokens > remaining {
			failure := NewError(CodeInvalidRuntimeResponse, fmt.Errorf("runtime returned invalid token accounting"))
			manager.retireUnhealthyRuntime(session, generation, failure)
			return Response{}, failure
		}
		answer, err := answerFromScores(question, options, candidates, score.Scores)
		if err != nil {
			failure := NewError(CodeInvalidRuntimeResponse, err)
			manager.retireUnhealthyRuntime(session, generation, failure)
			return Response{}, failure
		}
		result.Answers[question.ID] = answer
		result.Usage.Candidates += len(candidates)
		result.Usage.PromptTokens += score.usage.PromptTokens
		result.Usage.ImageTokens += score.usage.ImageTokens
		result.Usage.ContextTokens += score.usage.ContextTokens
		remaining -= score.usage.ContextTokens
	}
	if !manager.evaluationCurrent(session, generation, cpuAllowed) {
		return Response{}, NewError(CodeCancelled, cancellationCause(ctx))
	}
	return result, nil
}

// retireUnhealthyRuntime prevents queued or later evaluations from reusing a
// process that failed transport, inference, or per-request attestation checks.
// A caller cancellation and ordinary request-validation failures leave a
// healthy generation alone. Recovery always requires an explicit Start.
func (manager *Manager) retireUnhealthyRuntime(session runtimeSession, generation uint64, failure error) {
	switch ErrorCode(failure) {
	case CodeRuntimeUnavailable, CodeRuntimeAttestationFailed, CodeInvalidRuntimeResponse, CodeInferenceFailed:
	default:
		return
	}
	manager.mu.Lock()
	if manager.session != session || manager.generation != generation {
		manager.mu.Unlock()
		return
	}
	manager.session = nil
	cancel := manager.sessionStop
	manager.sessionStop = nil
	evalCancel := manager.activeEvalCancel
	manager.generation++
	manager.status.Ready = false
	manager.status.State = "error"
	manager.status.Code = ErrorCode(failure)
	manager.status.ErrorMessage = manager.status.Code
	manager.status.CPUFallbackActive = false
	manager.status.GPUOffloadVerified = false
	manager.activeDeployment = ""
	manager.activeDockerHost = ""
	manager.activeIsDocker = false
	manager.activeDevice = runtimeDevice{}
	manager.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if evalCancel != nil {
		evalCancel()
	}
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = session.Close(closeCtx)
	closeCancel()
	if manager.options.logger != nil {
		manager.options.logger.Warn("Decisions runtime generation retired after evaluation failure", "code", ErrorCode(failure))
	}
}

func (manager *Manager) evaluationCurrent(session runtimeSession, generation uint64, cpuAllowed bool) bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.session == session && manager.generation == generation && manager.cfg.Enabled &&
		manager.cfg.CPUFallbackAllowed == cpuAllowed
}

func (manager *Manager) authorizeEnabled() error {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.closed {
		return NewError(CodeServiceDisabled, errors.New("manager is shut down"))
	}
	if !manager.cfg.Enabled {
		return NewError(CodeServiceDisabled, nil)
	}
	return nil
}

func (manager *Manager) updateProgress(phase string, current, total int64) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if !manager.cfg.Enabled || manager.closed || manager.operationCancel == nil {
		return
	}
	progress := Progress{Phase: phase, Current: current, Total: total}
	if total > 0 {
		progress.Percent = float64(current) * 100 / float64(total)
		if progress.Percent > 100 {
			progress.Percent = 100
		}
	}
	manager.status.Progress = progress
}

func (manager *Manager) setState(state, code string, progress Progress) {
	manager.mu.Lock()
	if !manager.cfg.Enabled || manager.closed || manager.operationCancel == nil {
		manager.mu.Unlock()
		return
	}
	manager.status.State = state
	manager.status.Code = code
	manager.status.ErrorMessage = ""
	manager.status.Progress = progress
	manager.mu.Unlock()
}

func (manager *Manager) setInstallError(err error) {
	manager.mu.Lock()
	if !manager.cfg.Enabled || manager.closed || manager.operationCancel == nil || ErrorCode(err) == CodeCancelled {
		manager.mu.Unlock()
		return
	}
	manager.status.Ready = false
	manager.status.State = "error"
	manager.status.Code = ErrorCode(err)
	manager.status.ErrorMessage = ErrorCode(err)
	manager.status.Progress = Progress{}
	manager.mu.Unlock()
}

func (manager *Manager) setRuntimeError(err error) {
	manager.mu.Lock()
	if !manager.cfg.Enabled || manager.closed || manager.operationCancel == nil || ErrorCode(err) == CodeCancelled {
		manager.mu.Unlock()
		return
	}
	manager.status.Ready = false
	manager.status.State = "error"
	manager.status.Code = ErrorCode(err)
	manager.status.ErrorMessage = ErrorCode(err)
	manager.status.CPUFallbackActive = false
	manager.mu.Unlock()
	if manager.options.logger != nil {
		manager.options.logger.Warn("Decisions runtime operation failed", "code", ErrorCode(err))
	}
}

func (manager *Manager) beginOperation(parent context.Context) (context.Context, context.CancelFunc, uint64, func() bool, error) {
	operationCtx, cancel := context.WithCancel(context.Background())
	stopParent := context.AfterFunc(parent, cancel)
	manager.mu.Lock()
	if manager.closed || !manager.cfg.Enabled {
		manager.mu.Unlock()
		cancel()
		stopParent()
		return nil, nil, 0, nil, NewError(CodeServiceDisabled, nil)
	}
	if manager.cleanupPending > 0 || manager.stopPending > 0 {
		manager.mu.Unlock()
		cancel()
		stopParent()
		return nil, nil, 0, nil, NewError(CodeLifecycleBusy, nil)
	}
	manager.operationID++
	operationID := manager.operationID
	manager.operationCancel = cancel
	manager.operationAccess = manager.options.access()
	manager.mu.Unlock()
	return operationCtx, cancel, operationID, stopParent, nil
}

func (manager *Manager) cleanupIsPending() bool {
	manager.mu.RLock()
	pending := manager.cleanupPending > 0 || manager.stopPending > 0
	manager.mu.RUnlock()
	return pending
}

func cancellationCause(ctx context.Context) error {
	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return context.Canceled
}

func (manager *Manager) waitForCleanup(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		manager.mu.RLock()
		pending := manager.cleanupPending > 0
		changed := manager.cleanupChanged
		manager.mu.RUnlock()
		if !pending {
			return nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (manager *Manager) finishOperation(cancel context.CancelFunc, operationID uint64, stopParent func() bool, keep bool) {
	if stopParent != nil {
		stopParent()
	}
	manager.mu.Lock()
	if manager.operationID == operationID {
		manager.operationCancel = nil
	}
	manager.mu.Unlock()
	if !keep && cancel != nil {
		cancel()
	}
}

func resolveDeployment(configured string, access RuntimeAccess) (string, error) {
	switch configured {
	case config.DecisionsDeploymentAuto:
		if access.IsDocker {
			if !dockerMayMutate(access) {
				return "", NewError(CodeDeploymentDenied, nil)
			}
			return config.DecisionsDeploymentDocker, nil
		}
		return config.DecisionsDeploymentNative, nil
	case config.DecisionsDeploymentNative:
		return config.DecisionsDeploymentNative, nil
	case config.DecisionsDeploymentDocker:
		if !dockerMayMutate(access) {
			return "", NewError(CodeDeploymentDenied, nil)
		}
		return config.DecisionsDeploymentDocker, nil
	default:
		return "", NewError(CodeInvalidRequest, nil)
	}
}

func dockerMayMutate(access RuntimeAccess) bool {
	return access.DockerEnabled && !access.DockerReadOnly
}

func runtimeBuildDirectory(options managerOptions) string {
	directory := strings.TrimSpace(options.runtimeDirectory)
	if directory == "" {
		if executable, err := os.Executable(); err == nil {
			directory = filepath.Dir(executable)
		} else {
			directory = "."
		}
	}
	return normalizeRuntimeBuildDirectory(directory)
}

func normalizeRuntimeBuildDirectory(directory string) string {
	directory = filepath.Clean(strings.TrimSpace(directory))
	if strings.EqualFold(filepath.Base(directory), "bin") {
		return filepath.Dir(directory)
	}
	return directory
}
