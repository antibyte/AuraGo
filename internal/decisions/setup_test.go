package decisions

import (
	"context"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestSetupInstallsStartsAndVerifiesBeforeReady(t *testing.T) {
	factory := &testFactory{devices: []runtimeDevice{{ID: "gpu0", Name: "GPU", Backend: "vulkan", Type: "GPU"}}}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: "native"}, factory)
	manager.installModelPaths = manager.resolveModelPaths
	manager.resolveModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		t.Fatal("setup hashed already verified models again")
		return modelPaths{}, nil
	}
	if err := manager.Setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer manager.Shutdown(context.Background())
	if !manager.Status().Ready || factory.installRuns != 1 {
		t.Fatalf("setup = %+v, installs %d", manager.Status(), factory.installRuns)
	}
	session := manager.session.(*testSession)
	if session.calls != 1 {
		t.Fatalf("setup probe count = %d", session.calls)
	}
}

func TestStopBetweenSetupInstallAndStartPreventsRuntime(t *testing.T) {
	factory := &testFactory{devices: []runtimeDevice{{ID: "gpu0", Name: "GPU", Backend: "vulkan", Type: "GPU"}}}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: "native"}, factory)
	entered := make(chan struct{})
	manager.installModelPaths = func(ctx context.Context, _ string, _ func(string, int64, int64)) (modelPaths, error) {
		close(entered)
		<-ctx.Done()
		return modelPaths{}, ctx.Err()
	}
	done := make(chan error, 1)
	go func() { done <- manager.Setup(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("setup did not install")
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := <-done; ErrorCode(err) != CodeCancelled {
		t.Fatalf("setup error = %v", err)
	}
	if len(factory.attempts) != 0 || manager.Status().Ready {
		t.Fatal("stopped setup started a runtime")
	}
}

func TestSetupProbeRejectsMalformedScoresAndRevokedContext(t *testing.T) {
	device := runtimeDevice{ID: "gpu0", Name: "GPU", Backend: "vulkan", Type: "GPU"}
	metadata := metadataForDevice(device)
	device.BinarySHA = metadata.Build.BinarySHA256
	session := &testSession{metadataValue: metadata, scoreFailure: NewError(CodeInvalidRuntimeResponse, nil)}
	if err := verifySetupScore(context.Background(), session, metadata, device, "native", false); err == nil {
		t.Fatal("invalid score accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := verifySetupScore(ctx, session, metadata, device, "native", false); ErrorCode(err) != CodeCancelled {
		t.Fatalf("cancelled probe = %v", err)
	}
}

func TestSetupDoesNotDownloadModelsWithoutUsableHardwareConsent(t *testing.T) {
	factory := &testFactory{devices: []runtimeDevice{{ID: "cpu", Name: "CPU", Backend: "cpu", Type: "CPU"}}}
	manager := startManagerForTest(t, config.DecisionsConfig{Enabled: true, Deployment: "native"}, factory)
	manager.installModelPaths = func(context.Context, string, func(string, int64, int64)) (modelPaths, error) {
		t.Fatal("downloaded model before CPU permission")
		return modelPaths{}, nil
	}
	if err := manager.Setup(context.Background()); ErrorCode(err) != CodeCPUConsentRequired {
		t.Fatalf("setup error = %v", err)
	}
}

type setupPhaseFactory struct {
	*testFactory
	phase   string
	entered chan struct{}
}

func (factory *setupPhaseFactory) install(ctx context.Context, _ string, _ RuntimeAccess, _ string, _ bool, _ func(string, int64, int64)) error {
	if factory.phase == "runtime" {
		factory.entered <- struct{}{}
		<-ctx.Done()
		return NewError(CodeCancelled, ctx.Err())
	}
	return nil
}

func (factory *setupPhaseFactory) start(ctx context.Context, deployment string, device runtimeDevice, paths modelPaths, cfg config.DecisionsConfig, access RuntimeAccess, directory string, progress func(string)) (runtimeSession, error) {
	if factory.phase == "start" {
		factory.entered <- struct{}{}
		<-ctx.Done()
		return nil, NewError(CodeCancelled, ctx.Err())
	}
	session, err := factory.testFactory.start(ctx, deployment, device, paths, cfg, access, directory, progress)
	if err == nil && factory.phase == "probe" {
		session.(*testSession).wait = true
		session.(*testSession).started = factory.entered
	}
	return session, err
}

func TestSetupCancellationFencesEveryPhase(t *testing.T) {
	for _, phase := range []string{"runtime", "model", "start", "probe"} {
		for _, action := range []string{"stop", "disable", "revoke_cpu", "shutdown"} {
			t.Run(phase+"/"+action, func(t *testing.T) {
				base := &testFactory{devices: []runtimeDevice{{ID: "gpu0", Name: "GPU", Backend: "vulkan", Type: "GPU"}}}
				cfg := config.DecisionsConfig{Enabled: true, Deployment: "native", CPUFallbackAllowed: true}
				manager := startManagerForTest(t, cfg, base)
				factory := &setupPhaseFactory{testFactory: base, phase: phase, entered: make(chan struct{}, 1)}
				manager.factory = factory
				manager.installModelPaths = manager.resolveModelPaths
				if phase == "model" {
					manager.installModelPaths = func(ctx context.Context, _ string, _ func(string, int64, int64)) (modelPaths, error) {
						factory.entered <- struct{}{}
						<-ctx.Done()
						return modelPaths{}, ctx.Err()
					}
				}
				done := make(chan error, 1)
				go func() { done <- manager.Setup(t.Context()) }()
				select {
				case <-factory.entered:
				case <-time.After(time.Second):
					t.Fatal("setup did not reach phase")
				}
				if manager.Status().Ready {
					t.Fatal("setup ready before successful probe")
				}
				switch action {
				case "stop":
					if err := manager.Stop(t.Context()); err != nil {
						t.Fatal(err)
					}
				case "disable":
					cfg.Enabled = false
					if err := manager.Reconfigure(cfg); err != nil {
						t.Fatal(err)
					}
				case "revoke_cpu":
					cfg.CPUFallbackAllowed = false
					if err := manager.Reconfigure(cfg); err != nil {
						t.Fatal(err)
					}
				case "shutdown":
					if err := manager.Shutdown(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("cancelled setup succeeded")
					}
				case <-time.After(time.Second):
					t.Fatal("setup did not stop")
				}
				if manager.Status().Ready {
					t.Fatal("cancelled setup published ready")
				}
			})
		}
	}
}
