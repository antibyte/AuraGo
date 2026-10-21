package decisions

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"aurago/internal/config"
	"aurago/internal/onnxrt"
)

// InstallationCheck describes package prerequisites, not hardware readiness.
type InstallationCheck struct {
	Deployment       string `json:"deployment"`
	Target           string `json:"target,omitempty"`
	RuntimeAvailable bool   `json:"runtime_available"`
	Code             string `json:"code,omitempty"`
}

// CheckInstallation only reads trusted packages or local Docker image metadata.
// It never provisions files, downloads weights or starts a process/container.
func (manager *Manager) CheckInstallation(ctx context.Context) InstallationCheck {
	manager.mu.RLock()
	cfg, closed := manager.cfg, manager.closed
	manager.mu.RUnlock()
	var result InstallationCheck
	if closed {
		result.Code = CodeServiceNotReady
		return result
	}
	access := manager.options.access()
	deployment, err := deploymentForInstall(cfg, access)
	result.Deployment = deployment
	if err == nil {
		if deployment == config.DecisionsDeploymentDocker {
			err = preflightDockerMounts(ctx, access, manager.dataDir, manager.options.dockerSelfContainer)
			if err == nil {
				err = verifyLocalDockerRuntimeImages(ctx, access, nil)
			}
		} else {
			result.Target = runtimeTarget(runtime.GOOS, runtime.GOARCH)
			_, err = availableNativeRuntimeBundles(ctx, runtimeBuildDirectory(manager.options))
			if ErrorCode(err) == CodeRuntimeArtifactMissing {
				root := filepath.Join(manager.dataDir, "decisions", "runtime")
				for _, backend := range runtimeBackends(runtime.GOOS) {
					paths := []string{filepath.Join(root, result.Target, backend)}
					if entry, ok := releaseRuntimeCatalog.native(runtime.GOOS, runtime.GOARCH, backend); ok {
						paths = append(paths, onnxrt.RuntimeDir(filepath.Join(root, "packages"), entry.asset()))
					}
					for _, directory := range paths {
						if _, statErr := os.Stat(directory); os.IsNotExist(statErr) {
							continue
						}
						if verifyErr := validateDownloadedRuntime(ctx, directory, backend); verifyErr != nil {
							err = verifyErr
							continue
						}
						if _, _, verifyErr := validateRuntimeManifest(ctx, directory, backend, runtime.GOOS, runtime.GOARCH); verifyErr == nil {
							result.RuntimeAvailable = true
						} else {
							err = NewError(CodeRuntimeAttestationFailed, verifyErr)
						}
					}
				}
				if result.RuntimeAvailable {
					err = nil
				}
			}
		}
	}
	if ctx.Err() != nil {
		err = NewError(CodeCancelled, ctx.Err())
	}
	if err != nil {
		result.RuntimeAvailable = false
		result.Code = ErrorCode(err)
	} else {
		result.RuntimeAvailable = true
	}
	return result
}
