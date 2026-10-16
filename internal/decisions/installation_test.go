package decisions

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"aurago/internal/config"
)

func TestInstallationCheckIsPassiveAndVerifiesNativePackage(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	manager := NewManager(config.DecisionsConfig{}, dataDir, WithRuntimeDirectory(root))
	before := manager.Status()
	check := manager.CheckInstallation(t.Context())
	if check.RuntimeAvailable || check.Code != CodeRuntimeArtifactMissing || check.Deployment != "native" || check.Target != runtimeTarget(runtime.GOOS, runtime.GOARCH) {
		t.Fatalf("missing package check: %+v", check)
	}
	backend := runtimeBackends(runtime.GOOS)[0]
	dir := filepath.Join(root, "bin", "decisions-runtime", check.Target, backend)
	binary := writeTestRuntimeBundle(t, dir, backend)
	check = manager.CheckInstallation(t.Context())
	if !check.RuntimeAvailable || check.Code != "" {
		t.Fatalf("valid package rejected: %+v", check)
	}
	if err := os.WriteFile(binary, []byte("tampered"), 0o700); err != nil {
		t.Fatal(err)
	}
	check = manager.CheckInstallation(t.Context())
	if check.RuntimeAvailable || check.Code != CodeRuntimeAttestationFailed {
		t.Fatalf("tampered package accepted: %+v", check)
	}
	if !reflect.DeepEqual(manager.Status(), before) {
		t.Fatal("check changed service status or enabled the service")
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("check created managed files: %v", err)
	}
}

func TestInstallationCheckHonorsCancellationAndDockerPolicy(t *testing.T) {
	manager := NewManager(config.DecisionsConfig{}, t.TempDir(), WithRuntimeDirectory(t.TempDir()))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if check := manager.CheckInstallation(ctx); check.Code != CodeCancelled || check.RuntimeAvailable {
		t.Fatalf("cancelled check: %+v", check)
	}
	manager = NewManager(config.DecisionsConfig{Deployment: "docker"}, t.TempDir(), WithRuntimeAccess(func() RuntimeAccess {
		return RuntimeAccess{IsDocker: true, DockerEnabled: true, DockerReadOnly: true}
	}))
	if check := manager.CheckInstallation(t.Context()); check.Code != CodeDeploymentDenied || check.RuntimeAvailable {
		t.Fatalf("denied Docker check: %+v", check)
	}
}
