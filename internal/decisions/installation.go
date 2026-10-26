package decisions

import (
	"context"
	"path/filepath"
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
			err = preflightDockerMounts(ctx, access, manager.dataDir, manager.options.dockerSelfContainer)
			if err == nil {
				err = verifyLocalDockerRuntimeImages(ctx, access, nil)
			}
		} else {
			result.Target = runtimeTarget(runtime.GOOS, runtime.GOARCH)
			err = NewError(CodeRuntimeArtifactMissing, nil)
			for _, backend := range runtimeBackends(runtime.GOOS) {
				bundles, verifyErr := nativeRuntimeCandidates(ctx, runtimeBuildDirectory(manager.options), filepath.Join(manager.dataDir, "decisions", "runtime"), backend)
				if ErrorCode(verifyErr) == CodeCancelled {
					err = verifyErr
					break
				}
				if len(bundles) > 0 {
					result.RuntimeAvailable = true
				}
				if ErrorCode(verifyErr) == CodeRuntimeAttestationFailed {
					err = verifyErr
				}
			}
			if result.RuntimeAvailable && ctx.Err() == nil {
				err = nil
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
