package tools

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"

	"aurago/internal/config"
)

//go:embed go2rtc_sidecar/Dockerfile
var go2RTCSidecarDockerfile []byte

//go:embed go2rtc_sidecar/lifecycle.patch
var go2RTCSidecarPatch []byte

//go:embed go2rtc_sidecar/lifecycle_test.go.txt
var go2RTCSidecarTest []byte

//go:embed go2rtc_sidecar/aliases_test.go.txt
var go2RTCSidecarAliasTest []byte

func go2RTCManagedImage(base string) string {
	sum := sha256.Sum256(append(append([]byte(base), go2RTCSidecarDockerfile...), go2RTCSidecarPatch...))
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

func (m *Go2RTCManager) ensureManagedSidecarImage(ctx context.Context, docker DockerConfig, image string) error {
	base := go2RTCManagedBase(image)
	if base == "" {
		return ensureGo2RTCImage(ctx, m.pullLifetime(), docker, image)
	}
	if _, code, err := dockerRequest(docker, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", ""); err == nil && code == http.StatusOK {
		return nil
	}
	buildCtx, cancel := lifetimePullContext(ctx, m.pullLifetime())
	defer cancel()
	return BuildImageContextWait(buildCtx, docker, image, "Dockerfile", go2RTCSidecarDockerfile, map[string][]byte{"lifecycle.patch": go2RTCSidecarPatch, "lifecycle_test.go.txt": go2RTCSidecarTest, "aliases_test.go.txt": go2RTCSidecarAliasTest}, map[string]string{"BASE_IMAGE": base}, m.logger)
}

func (m *Go2RTCManager) SupportsBoundWebRTC() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status.WebRTCSessionBound
}
