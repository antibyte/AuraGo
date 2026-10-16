package decisions

import (
	"context"
	"runtime"

	"aurago/internal/config"
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
			err = verifyLocalDockerRuntimeImages(ctx, access, nil)
		} else {
			result.Target = runtimeTarget(runtime.GOOS, runtime.GOARCH)
			_, err = availableNativeRuntimeBundles(ctx, runtimeBuildDirectory(manager.options))
		}
	}
	if ctx.Err() != nil {
		err = NewError(CodeCancelled, ctx.Err())
	}
	if err != nil {
		result.Code = ErrorCode(err)
	} else {
		result.RuntimeAvailable = true
	}
	return result
}
