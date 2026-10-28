package tools

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"aurago/internal/config"
)

//go:embed go2rtc_sidecar/Dockerfile
var go2RTCSidecarDockerfile []byte

//go:embed go2rtc_sidecar/lifecycle.patch
var go2RTCSidecarPatch []byte

//go:embed go2rtc_sidecar/tuya.patch
var go2RTCSidecarTuyaPatch []byte

//go:embed go2rtc_sidecar/tuya_api.go.txt
var go2RTCSidecarTuyaAPI []byte

//go:embed go2rtc_sidecar/tuya_runtime.go.txt
var go2RTCSidecarTuyaRuntime []byte

//go:embed go2rtc_sidecar/tuya_test.go.txt
var go2RTCSidecarTuyaTest []byte

//go:embed go2rtc_sidecar/tuya_api_test.go.txt
var go2RTCSidecarTuyaAPITest []byte

//go:embed go2rtc_sidecar/lifecycle_test.go.txt
var go2RTCSidecarTest []byte

//go:embed go2rtc_sidecar/aliases_test.go.txt
var go2RTCSidecarAliasTest []byte

func go2RTCManagedImage(base string) string {
	input := append(append([]byte(base), go2RTCSidecarDockerfile...), go2RTCSidecarPatch...)
	for _, part := range [][]byte{go2RTCSidecarTest, go2RTCSidecarAliasTest, go2RTCSidecarTuyaPatch, go2RTCSidecarTuyaAPI, go2RTCSidecarTuyaRuntime, go2RTCSidecarTuyaTest, go2RTCSidecarTuyaAPITest} {
		input = append(input, part...)
	}
	sum := sha256.Sum256(input)
	return fmt.Sprintf("aurago/go2rtc:1.9.14-lease-%x", sum[:8])
}

func go2RTCManagedBase(image string) string {
	for _, base := range []string{config.Go2RTCDefaultImage, config.Go2RTCHardwareImage} {
		if image == go2RTCManagedImage(base) {
			return base
		}
	}
	return ""
}

// Go2RTCImageFor selects the reviewed base plus the pinned lease-lifecycle
// patch. Custom images are kept, but WebRTC requires their API attestation.
func Go2RTCImageFor(cfg config.Go2RTCConfig) string {
	base := Go2RTCBaseImageFor(cfg)
	if base == config.Go2RTCDefaultImage || base == config.Go2RTCHardwareImage {
		return go2RTCManagedImage(base)
	}
	return base
}

// ensureManagedSidecarImage builds the managed image unless the Engine has it.
//
// A failed build backs off (go2RTCBuildBackoff): the background reconciler
// (a context marked by withGo2RTCAutomaticAttempt) does not rebuild before the
// wait is over and gets a *go2RTCBuildPausedError instead. Any other caller is
// an explicit action: it builds at once and starts the backoff over.
func (m *Go2RTCManager) ensureManagedSidecarImage(ctx context.Context, docker DockerConfig, image string) error {
	base := go2RTCManagedBase(image)
	if base == "" {
		return ensureGo2RTCImage(ctx, m.pullLifetime(), docker, image)
	}
	if _, code, err := dockerRequest(docker, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", ""); err == nil && code == http.StatusOK {
		m.buildBackoff.reset()
		return nil
	}
	target := image + "@" + docker.Host
	if isGo2RTCAutomaticAttempt(ctx) {
		if retryAt, cause, paused := m.buildBackoff.paused(target); paused {
			return &go2RTCBuildPausedError{RetryAt: retryAt, Err: cause}
		}
	} else {
		m.buildBackoff.reset()
	}
	buildCtx, cancel := lifetimePullContext(ctx, m.pullLifetime())
	defer cancel()
	err := BuildImageContextWait(buildCtx, docker, image, "Dockerfile", go2RTCSidecarDockerfile, map[string][]byte{"lifecycle.patch": go2RTCSidecarPatch, "lifecycle_test.go.txt": go2RTCSidecarTest, "aliases_test.go.txt": go2RTCSidecarAliasTest, "tuya.patch": go2RTCSidecarTuyaPatch, "tuya_api.go.txt": go2RTCSidecarTuyaAPI, "tuya_runtime.go.txt": go2RTCSidecarTuyaRuntime, "tuya_test.go.txt": go2RTCSidecarTuyaTest, "tuya_api_test.go.txt": go2RTCSidecarTuyaAPITest}, map[string]string{"BASE_IMAGE": base}, m.logger)
	switch {
	case err == nil:
		m.buildBackoff.reset()
	case errors.Is(buildCtx.Err(), context.Canceled):
		// The background lifetime ended (shutdown) and aborted the build; that is
		// not a failure of the build itself.
	default:
		failures, delay, retryAt := m.buildBackoff.failed(target, err)
		if m.logger != nil {
			m.logger.Warn("[go2rtc] Managed sidecar image build failed; automatic retries back off",
				"image", image, "error", err, "failed_attempts", failures,
				"retry_in", delay.String(), "next_retry_at", retryAt.UTC().Format(time.RFC3339))
		}
	}
	return err
}

func (m *Go2RTCManager) SupportsBoundWebRTC() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.WebRTCSessionBound
}
