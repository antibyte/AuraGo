package decisions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
)

type testSession struct {
	mu            sync.Mutex
	metadataValue runtimeMetadata
	started       chan struct{}
	closed        chan struct{}
	closeOnce     sync.Once
	wait          bool
	maxInputs     chan int
	scoreRelease  chan struct{}
	scoreFailure  error
	calls         int
}

type startAttempt struct {
	id        string
	gpuLayers int
}

type testFactory struct {
	devices           []runtimeDevice
	attempts          []startAttempt
	startFails        map[startAttempt]bool
	startErrors       map[startAttempt]error
	installErr        error
	installRuns       int
	blockFirstStart   bool
	firstStartEntered chan struct{}
	firstStartOnce    sync.Once
}

func (factory *testFactory) discover(context.Context, string, RuntimeAccess, modelPaths, string) ([]runtimeDevice, error) {
	return append([]runtimeDevice(nil), factory.devices...), nil
}

func (factory *testFactory) start(ctx context.Context, _ string, device runtimeDevice, _ modelPaths, _ config.DecisionsConfig, _ RuntimeAccess, _ string, _ func(string)) (runtimeSession, error) {
	attempt := startAttempt{id: device.ID, gpuLayers: device.GPULayers}
	factory.attempts = append(factory.attempts, attempt)
	if factory.blockFirstStart {
		factory.firstStartOnce.Do(func() {
			if factory.firstStartEntered != nil {
				close(factory.firstStartEntered)
			}
			<-ctx.Done()
		})
		if err := ctx.Err(); err != nil {
			return nil, NewError(CodeCancelled, err)
		}
	}
	if factory.startFails[attempt] {
		return nil, NewError(CodeRuntimeStartFailed, nil)
	}
	if err := factory.startErrors[attempt]; err != nil {
		return nil, err
	}
	return &testSession{metadataValue: metadataForDevice(device), closed: make(chan struct{})}, nil
}

func (factory *testFactory) install(context.Context, string, RuntimeAccess, string, bool, func(string, int64, int64)) error {
	factory.installRuns++
	return factory.installErr
}

func metadataForDevice(device runtimeDevice) runtimeMetadata {
	if device.BinarySHA == "" {
		device.BinarySHA = strings.Repeat("d", 64)
	}
	selected := runtimeSelectedDevice{ID: device.ID, Name: device.Name, Backend: device.Backend, Type: device.Type, Integrated: device.Integrated}
	metadata := baseRuntimeMetadata(selected, device.Backend)
	metadata.Build.BinarySHA256 = device.BinarySHA
	metadata.ModelLayers = device.ModelLayers
	if metadata.ModelLayers == 0 {
		metadata.ModelLayers = 32
	}
	switch {
	case strings.EqualFold(device.Type, "CPU"):
		metadata.Hardware.Offload.ModelCPUCompute = true
		metadata.Hardware.Offload.ModelLayersOnCPU = true
		metadata.Hardware.Offload.ProjectorCPUCompute = true
		metadata.Hardware.Offload.ProjectorWeightsOnCPU = true
		metadata.Hardware.Offload.ProjectorComputeOnCPU = true
		metadata.Hardware.Offload.KVCpu = true
	case device.GPULayers > 0:
		metadata.Hardware.Offload.ModelCPUCompute = true
		metadata.Hardware.Offload.ProjectorCPUCompute = true
		metadata.Hardware.Offload.ProjectorWeightsOnCPU = true
		metadata.Hardware.Offload.ProjectorComputeOnCPU = true
		metadata.Hardware.Offload.KVOnDevice = true
		metadata.Placement = runtimePlacement{Hybrid: true, RequestedGPULayers: device.GPULayers, TotalModelLayers: metadata.ModelLayers}
	default:
		metadata.Hardware.Offload.ModelLayersOnDevice = true
		metadata.Hardware.Offload.ProjectorOnDevice = true
		metadata.Hardware.Offload.ProjectorComputeOnDevice = true
		metadata.Hardware.Offload.KVOnDevice = true
		metadata.Placement.TotalModelLayers = metadata.ModelLayers
	}
	return metadata
}

func (session *testSession) metadata(context.Context) (runtimeMetadata, error) {
	return session.metadataValue, nil
}

func (session *testSession) score(ctx context.Context, request runtimeScoreRequest) (runtimeScoreResult, error) {
	if session.maxInputs != nil {
		session.maxInputs <- request.MaxInputTokens
	}
	if session.started != nil {
		select {
		case session.started <- struct{}{}:
		default:
		}
	}
	if session.wait {
		select {
		case <-session.closed:
			return runtimeScoreResult{}, NewError(CodeCancelled, context.Canceled)
		case <-ctx.Done():
			return runtimeScoreResult{}, NewError(CodeCancelled, ctx.Err())
		}
	}
	if session.scoreRelease != nil {
		select {
		case <-session.scoreRelease:
		case <-session.closed:
			return runtimeScoreResult{}, NewError(CodeCancelled, context.Canceled)
		case <-ctx.Done():
			return runtimeScoreResult{}, NewError(CodeCancelled, ctx.Err())
		}
	}
	if session.scoreFailure != nil {
		return runtimeScoreResult{}, session.scoreFailure
	}
	session.mu.Lock()
	session.calls++
	call := session.calls
	session.mu.Unlock()
	if call > 1 && request.MaxInputTokens < maxEvaluationTokens {
		return runtimeScoreResult{}, NewError(CodeInputTokenBudgetExceeded, nil)
	}
	scores := make([]scoredCandidate, len(request.Candidates))
	for index, candidate := range request.Candidates {
		scores[index] = scoredCandidate{Code: candidate.Code, TokenID: index + 10, Logit: float64(len(request.Candidates) - index)}
	}
	promptTokens := 1
	if call == 1 && session.maxInputs != nil {
		promptTokens = 5000
	}
	return runtimeScoreResult{
		Scores:   scores,
		usage:    runtimeUsage{PromptTokens: promptTokens, ContextTokens: promptTokens},
		Metadata: session.metadataValue,
	}, nil
}

func (session *testSession) Close(context.Context) error {
	session.closeOnce.Do(func() { close(session.closed) })
	return nil
}

func readyManagerForTest(t *testing.T, session *testSession) *Manager {
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	manager.session = session
	manager.activeDevice = runtimeDevice{ID: "cuda0", Name: "GPU 0", Backend: "cuda", Type: "GPU", BinarySHA: session.metadataValue.Build.BinarySHA256}
	manager.activeDeployment = config.DecisionsDeploymentNative
	manager.generation = 1
	manager.status = statusFromMetadata(manager.cfg, manager.activeDeployment, session.metadataValue, false)
	return manager
}

func startManagerForTest(t *testing.T, cfg config.DecisionsConfig, factory *testFactory) *Manager {
	manager := NewManager(cfg, t.TempDir())
	manager.factory = factory
	manager.resolveModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		return modelPaths{Model: "managed-model.gguf", Projector: "managed-mmproj.gguf"}, nil
	}
	return manager
}

func validTestSession() *testSession {
	selected := runtimeSelectedDevice{ID: "cuda0", Name: "GPU 0", Backend: "cuda", Type: "GPU"}
	metadata := baseRuntimeMetadata(selected, "cuda")
	metadata.Hardware.Offload.ModelLayersOnDevice = true
	metadata.Hardware.Offload.ProjectorOnDevice = true
	metadata.Hardware.Offload.ProjectorComputeOnDevice = true
	metadata.Hardware.Offload.KVOnDevice = true
	return &testSession{metadataValue: metadata, closed: make(chan struct{})}
}

type lifecycleRaceBlockingSession struct {
	*testSession
	closeEntered chan struct{}
	releaseClose chan struct{}
	closeOnce    sync.Once
}

func (session *lifecycleRaceBlockingSession) Close(ctx context.Context) error {
	session.closeOnce.Do(func() { close(session.closeEntered) })
	select {
	case <-session.releaseClose:
	case <-ctx.Done():
		return ctx.Err()
	}
	return session.testSession.Close(ctx)
}

type factoryWithSession struct {
	*testFactory
	session *testSession
}

func (factory factoryWithSession) start(context.Context, string, runtimeDevice, modelPaths, config.DecisionsConfig, RuntimeAccess, string, func(string)) (runtimeSession, error) {
	return factory.session, nil
}

type cancelDuringStartFactory struct {
	*testFactory
	entered chan struct{}
}

func (factory *cancelDuringStartFactory) start(ctx context.Context, _ string, _ runtimeDevice, _ modelPaths, _ config.DecisionsConfig, _ RuntimeAccess, _ string, _ func(string)) (runtimeSession, error) {
	close(factory.entered)
	<-ctx.Done()
	return nil, NewError(CodeRuntimeStartFailed, errors.New("fake launch failed after cancellation"))
}

type captureStartContextFactory struct {
	*testFactory
	startedCtx chan context.Context
}

func (factory *captureStartContextFactory) start(ctx context.Context, _ string, device runtimeDevice, _ modelPaths, _ config.DecisionsConfig, _ RuntimeAccess, _ string, _ func(string)) (runtimeSession, error) {
	factory.startedCtx <- ctx
	return &testSession{metadataValue: metadataForDevice(device), closed: make(chan struct{})}, nil
}

type deadlineMetadataSession struct {
	*testSession
}

func (session *deadlineMetadataSession) metadata(ctx context.Context) (runtimeMetadata, error) {
	<-ctx.Done()
	return runtimeMetadata{}, ctx.Err()
}

func testDecisionRequest() Request {
	return Request{State: map[string]any{"x": 1}, Questions: []Question{{
		ID: "q", Type: "choice", Options: []Option{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
	}}}
}

func TestStopFencesLifecycleOperationAlreadyClosingSession(t *testing.T) {
	for _, operation := range []string{"start", "install"} {
		t.Run(operation, func(t *testing.T) {
			old := &lifecycleRaceBlockingSession{
				testSession: validTestSession(), closeEntered: make(chan struct{}), releaseClose: make(chan struct{}),
			}
			factory := &testFactory{devices: []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", BinarySHA: strings.Repeat("a", 64)}}}
			if operation == "start" {
				factoryWithReplacement := factoryWithSession{testFactory: factory, session: validTestSession()}
				manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, factory)
				manager.factory = factoryWithReplacement
				manager.session = old
				manager.sessionStop = func() {}
				manager.activeDeployment = config.DecisionsDeploymentNative
				manager.status = statusFromMetadata(manager.cfg, manager.activeDeployment, old.metadataValue, false)
				runStopRace(t, manager, old, operation, factory)
				return
			}
			manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
			manager.factory = factory
			manager.session = old
			manager.sessionStop = func() {}
			manager.activeDeployment = config.DecisionsDeploymentNative
			manager.status = statusFromMetadata(manager.cfg, manager.activeDeployment, old.metadataValue, false)
			runStopRace(t, manager, old, operation, factory)
		})
	}
}

func runStopRace(t *testing.T, manager *Manager, old *lifecycleRaceBlockingSession, operation string, factory *testFactory) {
	t.Helper()
	operationResult := make(chan error, 1)
	go func() {
		if operation == "start" {
			operationResult <- manager.Start(context.Background())
		} else {
			operationResult <- manager.Install(context.Background())
		}
	}()
	select {
	case <-old.closeEntered:
	case <-time.After(time.Second):
		t.Fatal("lifecycle operation did not block closing the current session")
	}

	stopResult := make(chan error, 1)
	go func() { stopResult <- manager.Stop(context.Background()) }()
	deadline := time.Now().Add(time.Second)
	for {
		manager.mu.RLock()
		stopPublished := manager.policyGeneration > 0
		manager.mu.RUnlock()
		if stopPublished {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Stop did not publish its generation revocation")
		}
		time.Sleep(time.Millisecond)
	}
	close(old.releaseClose)
	select {
	case err := <-operationResult:
		if ErrorCode(err) != CodeLifecycleBusy {
			t.Fatalf("overlapping %s returned %v, want lifecycle_busy", operation, err)
		}
	case <-time.After(time.Second):
		t.Fatalf("overlapping %s did not finish after Stop revoked it", operation)
	}
	select {
	case err := <-stopResult:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish after the lifecycle operation released its lock")
	}
	manager.mu.RLock()
	session := manager.session
	status := manager.status
	manager.mu.RUnlock()
	if session != nil || status.Ready || status.State != "stopped" {
		t.Fatalf("Stop left session=%T with status=%+v; want no session and stopped", session, status)
	}
	if operation == "install" && factory.installRuns != 0 {
		t.Fatalf("Install ran %d provisioning calls after Stop revoked it", factory.installRuns)
	}
}

func TestStartPreservesCancellationDuringRuntimeLaunch(t *testing.T) {
	factory := &cancelDuringStartFactory{
		testFactory: &testFactory{devices: []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", BinarySHA: strings.Repeat("a", 64)}}},
		entered:     make(chan struct{}),
	}
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	manager.factory = factory
	manager.resolveModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		return modelPaths{Model: "managed-model.gguf", Projector: "managed-mmproj.gguf"}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- manager.Start(ctx) }()
	select {
	case <-factory.entered:
	case <-time.After(time.Second):
		t.Fatal("Start did not reach the fake runtime launch")
	}
	cancel()
	select {
	case err := <-result:
		if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Start error = %v (cause %v), want canceled", err, errors.Unwrap(err))
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not stop after its caller canceled")
	}
}

func TestSuccessfulStartOutlivesCallerContext(t *testing.T) {
	factory := &captureStartContextFactory{
		testFactory: &testFactory{devices: []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", BinarySHA: strings.Repeat("a", 64)}}},
		startedCtx:  make(chan context.Context, 1),
	}
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	manager.factory = factory
	manager.resolveModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		return modelPaths{Model: "managed-model.gguf", Projector: "managed-mmproj.gguf"}, nil
	}
	callerCtx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(callerCtx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	runtimeCtx := <-factory.startedCtx
	cancel()
	select {
	case <-runtimeCtx.Done():
		t.Fatal("successful Start retained the caller's cancellation lifetime")
	case <-time.After(25 * time.Millisecond):
	}
	status := manager.Status()
	if !status.Ready || status.State != "ready" {
		t.Fatalf("caller cancellation stopped the runtime: %+v", status)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestEvaluatePreservesDeadlineCauseFromMetadataAndScoring(t *testing.T) {
	t.Run("metadata", func(t *testing.T) {
		session := &deadlineMetadataSession{testSession: validTestSession()}
		manager := readyManagerForTest(t, session.testSession)
		manager.session = session
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := manager.Evaluate(ctx, testDecisionRequest())
		if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("metadata deadline error = %v (cause %v), want cancelled with deadline cause", err, errors.Unwrap(err))
		}
	})

	t.Run("scoring", func(t *testing.T) {
		session := validTestSession()
		session.wait = true
		session.started = make(chan struct{}, 1)
		manager := readyManagerForTest(t, session)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		_, err := manager.Evaluate(ctx, testDecisionRequest())
		if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("scoring deadline error = %v (cause %v), want cancelled with deadline cause", err, errors.Unwrap(err))
		}
	})
}

func TestEvaluateBoundsQueueAndDoesNotRebindCancelledWaiters(t *testing.T) {
	session := validTestSession()
	session.wait = true
	session.started = make(chan struct{}, 1)
	manager := readyManagerForTest(t, session)
	const queued = 5
	results := make(chan error, queued)
	for index := 0; index < queued; index++ {
		go func() {
			_, err := manager.Evaluate(context.Background(), testDecisionRequest())
			results <- err
		}()
	}
	select {
	case <-session.started:
	case <-time.After(time.Second):
		t.Fatal("first evaluation did not reach the fake runtime")
	}
	deadline := time.Now().Add(time.Second)
	for len(manager.evalSlots) != queued && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(manager.evalSlots) != queued {
		t.Fatalf("reserved evaluation slots = %d, want %d", len(manager.evalSlots), queued)
	}
	if _, err := manager.Evaluate(context.Background(), testDecisionRequest()); ErrorCode(err) != CodeQueueFull {
		t.Fatalf("over-capacity evaluation error = %v, want queue_full", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for index := 0; index < queued; index++ {
		select {
		case err := <-results:
			if ErrorCode(err) != CodeCancelled {
				t.Errorf("queued evaluation returned %v, want cancelled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("evaluation did not drain after generation stop")
		}
	}
}

func TestReconfigurePublishesRevocationAndCancelsInFlightLifecycleOperation(t *testing.T) {
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	operationCtx, cancel := context.WithCancel(context.Background())
	manager.operationID = 1
	manager.operationCancel = cancel
	if err := manager.Reconfigure(config.DecisionsConfig{Enabled: false, Deployment: config.DecisionsDeploymentNative}); err != nil {
		t.Fatal(err)
	}
	if operationCtx.Err() == nil {
		t.Fatal("disabling policy did not cancel the in-flight lifecycle operation")
	}
	status := manager.Status()
	if status.Enabled || status.Ready || status.State != "disabled" || status.Code != CodeServiceDisabled {
		t.Fatalf("revoked status = %#v", status)
	}
}

func TestPrepareReconfigureCancelsStartingGenerationAndAutoStartWaitsForDrain(t *testing.T) {
	factory := &testFactory{
		devices:         []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)}},
		blockFirstStart: true, firstStartEntered: make(chan struct{}),
	}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, factory)
	startResult := make(chan error, 1)
	go func() { startResult <- manager.Start(context.Background()) }()
	select {
	case <-factory.firstStartEntered:
	case <-time.After(time.Second):
		t.Fatal("initial Start did not enter the blocked runtime launch")
	}
	if status := manager.Status(); status.State != "starting" {
		t.Fatalf("status before policy update = %#v, want starting", status)
	}

	cleanup, err := manager.PrepareReconfigure(config.DecisionsConfig{
		Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true,
	})
	if err != nil {
		t.Fatalf("PrepareReconfigure: %v", err)
	}
	if status := manager.Status(); status.State != "stopping" || status.Ready {
		t.Fatalf("status after policy publication = %#v, want stopping and not ready", status)
	}
	if err := manager.Start(context.Background()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("Start during pending drain = %v, want lifecycle_busy", err)
	}

	cleanupResult := make(chan error, 1)
	go func() { cleanupResult <- cleanup(context.Background()) }()
	select {
	case err := <-startResult:
		if ErrorCode(err) != CodeCancelled {
			t.Fatalf("stale Start result = %v, want cancelled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("policy update did not cancel the in-flight Start")
	}

	autoStartResult := make(chan error, 1)
	go func() { autoStartResult <- manager.AutoStart(context.Background()) }()
	select {
	case err := <-cleanupResult:
		if err != nil {
			t.Fatalf("drain cleanup: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not finish after the canceled Start released lifecycle")
	}
	select {
	case err := <-autoStartResult:
		if err != nil {
			t.Fatalf("AutoStart after drain: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AutoStart did not resume after drain")
	}
	if status := manager.Status(); !status.Ready || status.State != "ready" || !status.CPUFallbackAllowed {
		t.Fatalf("new policy generation did not start cleanly: %#v", status)
	}
	if len(factory.attempts) != 2 || factory.attempts[0].id != "cuda0" || factory.attempts[1].id != "cuda0" {
		t.Fatalf("runtime attempts = %#v, want canceled old start then one new start", factory.attempts)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRapidPolicyUpdatesDoNotLeaveCleanupStatusStale(t *testing.T) {
	session := validTestSession()
	manager := readyManagerForTest(t, session)
	manager.cfg.Deployment = config.DecisionsDeploymentAuto
	manager.status.Deployment = config.DecisionsDeploymentAuto

	cleanup, err := manager.PrepareReconfigure(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative})
	if err != nil {
		t.Fatalf("first PrepareReconfigure: %v", err)
	}
	secondCleanup, err := manager.PrepareReconfigure(config.DecisionsConfig{
		Enabled: true, Deployment: config.DecisionsDeploymentAuto, CPUFallbackAllowed: true,
	})
	if err != nil {
		t.Fatalf("second PrepareReconfigure: %v", err)
	}
	if err := secondCleanup(context.Background()); err != nil {
		t.Fatalf("second no-op cleanup: %v", err)
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	status := manager.Status()
	if status.State != "stopped" || status.Ready || status.Deployment != config.DecisionsDeploymentAuto || !status.CPUFallbackAllowed {
		t.Fatalf("rapid policy update left stale status: %#v", status)
	}
	select {
	case <-session.closed:
	default:
		t.Fatal("detached runtime was not closed")
	}
}

func TestAutoStartRetriesTransientLifecycleBusy(t *testing.T) {
	factory := &testFactory{devices: []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)}}}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, factory)
	manager.lifecycle.Lock()
	result := make(chan error, 1)
	go func() { result <- manager.AutoStart(context.Background()) }()
	time.Sleep(150 * time.Millisecond)
	manager.lifecycle.Unlock()
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("AutoStart after transient lifecycle_busy: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AutoStart did not retry after lifecycle became available")
	}
	if status := manager.Status(); !status.Ready || status.State != "ready" {
		t.Fatalf("AutoStart left stale lifecycle status: %#v", status)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAutoStartDoesNotPublishStaleModelFailureAfterDisable(t *testing.T) {
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	entered, release := make(chan struct{}), make(chan struct{})
	manager.resolveModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		close(entered)
		<-release
		return modelPaths{}, os.ErrNotExist
	}
	result := make(chan error, 1)
	go func() { result <- manager.AutoStart(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("AutoStart did not begin model verification")
	}
	if err := manager.Reconfigure(config.DecisionsConfig{Enabled: false, Deployment: config.DecisionsDeploymentNative}); err != nil {
		t.Fatalf("disable Decisions: %v", err)
	}
	close(release)
	select {
	case err := <-result:
		if ErrorCode(err) != CodeCancelled {
			t.Fatalf("stale AutoStart returned %v, want cancelled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AutoStart did not finish after model verification was released")
	}
	status := manager.Status()
	if status.Enabled || status.State != "disabled" || status.Code != CodeServiceDisabled || status.Installed {
		t.Fatalf("stale model verification overwrote disabled status: %#v", status)
	}
}

func TestEvaluateUsesCumulativeRuntimeInputBudget(t *testing.T) {
	session := validTestSession()
	session.maxInputs = make(chan int, 2)
	manager := readyManagerForTest(t, session)
	request := testDecisionRequest()
	request.Questions = append(request.Questions, Question{ID: "second", Type: "choice", Instructions: "second", Options: request.Questions[0].Options})
	_, err := manager.Evaluate(context.Background(), request)
	if ErrorCode(err) != CodeInputTokenBudgetExceeded {
		t.Fatalf("Evaluate error = %v, want cumulative budget rejection", err)
	}
	first, second := <-session.maxInputs, <-session.maxInputs
	if first != maxEvaluationTokens || second != maxEvaluationTokens-5000 {
		t.Fatalf("per-question remaining budgets = %d, %d", first, second)
	}
}

func TestRuntimeAttestationFailureRevokesGPUAndCancelsQueuedEvaluation(t *testing.T) {
	session := validTestSession()
	session.started = make(chan struct{}, 1)
	session.scoreRelease = make(chan struct{})
	session.scoreFailure = NewError(CodeRuntimeAttestationFailed, nil)
	manager := readyManagerForTest(t, session)

	firstResult := make(chan error, 1)
	go func() {
		_, err := manager.Evaluate(context.Background(), testDecisionRequest())
		firstResult <- err
	}()
	select {
	case <-session.started:
	case <-time.After(time.Second):
		t.Fatal("first evaluation did not reach the GPU runtime")
	}

	queuedResult := make(chan error, 1)
	go func() {
		_, err := manager.Evaluate(context.Background(), testDecisionRequest())
		queuedResult <- err
	}()
	deadline := time.Now().Add(time.Second)
	for len(manager.evalSlots) != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(manager.evalSlots) != 2 {
		t.Fatalf("reserved evaluation slots = %d, want active and queued calls", len(manager.evalSlots))
	}
	close(session.scoreRelease)

	select {
	case err := <-firstResult:
		if ErrorCode(err) != CodeRuntimeAttestationFailed {
			t.Fatalf("active evaluation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("failed runtime evaluation did not return")
	}
	select {
	case err := <-queuedResult:
		if ErrorCode(err) != CodeCancelled {
			t.Fatalf("queued evaluation error = %v, want cancelled generation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued evaluation was not released after generation retirement")
	}

	status := manager.Status()
	if status.Ready || status.State != "error" || status.Code != CodeRuntimeAttestationFailed || status.CPUFallbackActive {
		t.Fatalf("failed GPU generation remained advertised as ready: %#v", status)
	}
	select {
	case <-session.closed:
	default:
		t.Fatal("unhealthy runtime session was not closed")
	}
	if _, err := manager.Evaluate(context.Background(), testDecisionRequest()); ErrorCode(err) != CodeRuntimeAttestationFailed {
		t.Fatalf("evaluation after runtime retirement error = %v", err)
	}
}

func TestRevokingCPUCoresentRuntimeClosesItBeforeFurtherEvaluation(t *testing.T) {
	device := runtimeDevice{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", GPULayers: 8, ModelLayers: 32, BinarySHA: strings.Repeat("f", 64)}
	metadata := metadataForDevice(device)
	session := &testSession{metadataValue: metadata, closed: make(chan struct{}), wait: true, started: make(chan struct{}, 1)}
	cfg := config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true}
	manager := NewManager(cfg, t.TempDir())
	manager.session = session
	manager.activeDevice = device
	manager.activeDeployment = config.DecisionsDeploymentNative
	manager.generation = 1
	manager.status = statusFromMetadata(cfg, manager.activeDeployment, metadata, true)
	result := make(chan error, 1)
	go func() {
		_, err := manager.Evaluate(context.Background(), testDecisionRequest())
		result <- err
	}()
	select {
	case <-session.started:
	case <-time.After(time.Second):
		t.Fatal("consented hybrid evaluation did not reach its runtime")
	}
	cleanup, err := manager.PrepareReconfigure(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative})
	if err != nil {
		t.Fatalf("prepare persistent CPU consent revocation: %v", err)
	}
	select {
	case err := <-result:
		if ErrorCode(err) != CodeCancelled {
			t.Fatalf("in-flight CPU-participating evaluation returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("revoking consent did not terminate the active evaluation")
	}
	select {
	case <-session.closed:
		t.Fatal("session close should be deferred until after caller-owned config locks are released")
	default:
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatalf("drain revoked generation: %v", err)
	}
	status := manager.Status()
	if status.Ready || status.CPUFallbackActive || status.State != "stopped" {
		t.Fatalf("runtime remains active after consent revocation: %#v", status)
	}
}

func TestStartTriesAllFullGPUDevicesBeforeConsentedHybrid(t *testing.T) {
	factory := &testFactory{
		devices: []runtimeDevice{
			{ID: "cuda0", Name: "Discrete CUDA", Backend: "cuda", Type: "GPU", MemoryFree: 8 << 30, ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)},
			{ID: "vk0", Name: "Integrated Vulkan", Backend: "vulkan", Type: "GPU", Integrated: boolPointer(true), MemoryFree: 4 << 30, ModelLayers: 32, BinarySHA: strings.Repeat("b", 64)},
			{ID: "cpu", Name: "CPU", Backend: "cpu", Type: "CPU", ModelLayers: 32, BinarySHA: strings.Repeat("c", 64)},
		},
		startErrors: map[startAttempt]error{
			{id: "cuda0"}: NewError(CodeRuntimeStartFailed, nativeStartupFailure{code: "offload_unavailable"}),
			{id: "vk0"}:   NewError(CodeRuntimeStartFailed, nativeStartupFailure{code: "device_unavailable"}),
		},
	}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true}, factory)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := []startAttempt{{id: "cuda0"}, {id: "vk0"}, {id: "cuda0", gpuLayers: 24}}
	if !reflect.DeepEqual(factory.attempts, want) {
		t.Fatalf("startup attempts = %#v, want %#v", factory.attempts, want)
	}
	status := manager.Status()
	if status.ExecutionMode != "mixed" || !status.CPUFallbackActive || status.CPUFallbackReason != "device_unavailable" {
		t.Fatalf("consented hybrid status = %#v", status)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartTriesPureCPUOnlyAfterEveryConsentedHybridAttempt(t *testing.T) {
	factory := &testFactory{
		devices: []runtimeDevice{
			{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)},
			{ID: "cpu", Name: "CPU", Backend: "cpu", Type: "CPU", ModelLayers: 32, BinarySHA: strings.Repeat("c", 64)},
		},
		startFails: map[startAttempt]bool{
			{id: "cuda0"}: true, {id: "cuda0", gpuLayers: 24}: true,
			{id: "cuda0", gpuLayers: 16}: true, {id: "cuda0", gpuLayers: 8}: true,
		},
	}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true}, factory)
	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := []startAttempt{{id: "cuda0"}, {id: "cuda0", gpuLayers: 24}, {id: "cuda0", gpuLayers: 16}, {id: "cuda0", gpuLayers: 8}, {id: "cpu"}}
	if !reflect.DeepEqual(factory.attempts, want) {
		t.Fatalf("startup attempts = %#v, want %#v", factory.attempts, want)
	}
	status := manager.Status()
	if status.ExecutionMode != "cpu" || !status.CPUFallbackActive || status.CPUFallbackReason != CodeRuntimeStartFailed {
		t.Fatalf("consented CPU status = %#v", status)
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStartNeverLaunchesHybridOrCPUWithoutConsent(t *testing.T) {
	factory := &testFactory{
		devices: []runtimeDevice{
			{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)},
			{ID: "cpu", Name: "CPU", Backend: "cpu", Type: "CPU", ModelLayers: 32, BinarySHA: strings.Repeat("c", 64)},
		},
		startFails: map[startAttempt]bool{{id: "cuda0"}: true},
	}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, factory)
	err := manager.Start(context.Background())
	if ErrorCode(err) != CodeRuntimeStartFailed {
		t.Fatalf("Start error = %v, want failed accelerator launch", err)
	}
	if want := []startAttempt{{id: "cuda0"}}; !reflect.DeepEqual(factory.attempts, want) {
		t.Fatalf("startup attempts = %#v, want only strict GPU attempt %#v", factory.attempts, want)
	}
}

func TestStartupTimeoutAndInvalidArtifactsNeverTryAnotherDevice(t *testing.T) {
	for _, failure := range []error{NewError(CodeRuntimeStartTimeout, context.DeadlineExceeded), runtimeStartupError("model_pin_mismatch"), runtimeStartupError("projector_pin_mismatch"), runtimeStartupError("invalid_configuration")} {
		t.Run(ErrorCode(failure), func(t *testing.T) {
			factory := &testFactory{
				devices: []runtimeDevice{
					{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32},
					{ID: "vulkan0", Name: "GPU2", Backend: "vulkan", Type: "GPU", ModelLayers: 32},
					{ID: "cpu", Name: "CPU", Backend: "cpu", Type: "CPU", ModelLayers: 32},
				},
				startErrors: map[startAttempt]error{{id: "cuda0"}: failure},
			}
			manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative, CPUFallbackAllowed: true}, factory)
			if err := manager.Start(t.Context()); ErrorCode(err) != ErrorCode(failure) {
				t.Fatalf("startup error: %v", err)
			}
			if !reflect.DeepEqual(factory.attempts, []startAttempt{{id: "cuda0"}}) {
				t.Fatalf("terminal failure launched fallback: %#v", factory.attempts)
			}
			if status := manager.Status(); status.Ready || status.State != "error" || status.Code != ErrorCode(failure) {
				t.Fatalf("terminal status: %#v", status)
			}
		})
	}
}

func TestStartupProgressCannotPublishAcrossOperationGenerations(t *testing.T) {
	manager := NewManager(config.DecisionsConfig{Enabled: true}, t.TempDir())
	_, cancel, id, stopParent, err := manager.beginOperation(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer manager.finishOperation(cancel, id, stopParent, false)
	manager.updateProgress(id, "warming_up", 0, 0)
	manager.updateProgress(id-1, "loading_model", 0, 0)
	if got := manager.Status().Progress.Phase; got != "warming_up" {
		t.Fatalf("stale phase: %s", got)
	}
	if err := manager.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	manager.updateProgress(id, "warming_up", 0, 0)
	if manager.Status().State != "stopped" {
		t.Fatal("progress revived a stopped operation")
	}
}

func TestInstallVerifiesRuntimeBeforeStartingLargeModelDownload(t *testing.T) {
	factory := &testFactory{installErr: NewError(CodeRuntimeArtifactMissing, nil)}
	dataDir := t.TempDir()
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, dataDir)
	manager.factory = factory
	if err := manager.Install(context.Background()); ErrorCode(err) != CodeRuntimeArtifactMissing {
		t.Fatalf("Install error = %v, want runtime artifact missing", err)
	}
	if factory.installRuns != 1 {
		t.Fatalf("runtime verification calls = %d, want 1", factory.installRuns)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "decisions", "models")); !os.IsNotExist(err) {
		t.Fatalf("model download directory exists even though runtime provisioning failed: %v", err)
	}
}

func TestInstallStopsServingRuntimeBeforeProvisioningBundle(t *testing.T) {
	session := validTestSession()
	manager := readyManagerForTest(t, session)
	factory := &testFactory{installErr: NewError(CodeRuntimeArtifactMissing, nil)}
	manager.factory = factory

	if err := manager.Install(context.Background()); ErrorCode(err) != CodeRuntimeArtifactMissing {
		t.Fatalf("Install error = %v, want runtime artifact missing", err)
	}
	select {
	case <-session.closed:
	default:
		t.Fatal("Install left the serving runtime open while provisioning could replace its executable")
	}
	if factory.installRuns != 1 {
		t.Fatalf("runtime provisioning calls = %d, want one", factory.installRuns)
	}
	if status := manager.Status(); status.Ready || status.State != "error" {
		t.Fatalf("status after failed reinstall = %#v, want stopped/error generation", status)
	}
}

func TestStartAndInstallReturnBusyInsteadOfQueueingLifecycleRequests(t *testing.T) {
	manager := NewManager(config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, t.TempDir())
	manager.lifecycle.Lock()
	if err := manager.Start(context.Background()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("concurrent Start error = %v, want lifecycle_busy", err)
	}
	if err := manager.Install(context.Background()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("concurrent Install error = %v, want lifecycle_busy", err)
	}
	manager.lifecycle.Unlock()
}

func TestDecisionsStartPublishesStarting(t *testing.T) {
	factory := &testFactory{
		devices:           []runtimeDevice{{ID: "cuda0", Name: "GPU", Backend: "cuda", Type: "GPU", ModelLayers: 32, BinarySHA: strings.Repeat("a", 64)}},
		blockFirstStart:   true,
		firstStartEntered: make(chan struct{}),
	}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: config.DecisionsDeploymentNative}, factory)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	var held Status
	for {
		select {
		case <-factory.firstStartEntered:
			held = manager.Status()
			if held.State != "starting" && held.State != "discovering_devices" {
				t.Fatalf("status before runtime start = %#v, want starting or discovering_devices", held)
			}
			if held.Code != "" || held.Ready {
				t.Fatalf("transitional status = %#v, want empty code and not ready", held)
			}
			goto held
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Start did not reach the blocked runtime launch")
		}
		time.Sleep(time.Millisecond)
	}
held:
	if err := manager.Start(context.Background()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("second Start = %v, want lifecycle_busy", err)
	}
	if next := manager.Status(); next.State != held.State || next.State == "error" || next.Ready != held.Ready {
		t.Fatalf("second Start replaced %#v with %#v", held, next)
	}
	if err := manager.Install(context.Background()); ErrorCode(err) != CodeLifecycleBusy {
		t.Fatalf("Install during Start = %v, want lifecycle_busy", err)
	}
	if next := manager.Status(); next.State != held.State || next.Ready != held.Ready {
		t.Fatalf("Install during Start replaced %#v with %#v", held, next)
	}
	cancel()
	select {
	case err := <-done:
		if ErrorCode(err) != CodeCancelled {
			t.Fatalf("canceled Start = %v, want cancelled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled Start did not finish")
	}
}

func boolPointer(value bool) *bool { return &value }
