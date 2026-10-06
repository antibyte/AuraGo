package server

import (
	"errors"
	"fmt"
	"testing"

	"aurago/internal/proxy"
)

func TestProxyErrorKeyExplainsActionableFailures(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{proxy.ErrBasicAuthCredentialsMissing, "backend.proxy_basic_auth_credentials_missing"},
		{fmt.Errorf("%w: user name", proxy.ErrBasicAuthCredentialsInvalid), "backend.proxy_basic_auth_credentials_invalid"},
		{fmt.Errorf("ensure proxy image: %w", fmt.Errorf("%w: HTTP 403", proxy.ErrRateLimitImageUnavailable)), "backend.proxy_rate_limit_image_unavailable"},
		{fmt.Errorf("%w: API 1.44", proxy.ErrDockerPlacement), "backend.proxy_docker_placement_failed"},
		{fmt.Errorf("%w: unrecognized directive", proxy.ErrCaddyExited), "backend.proxy_caddy_exited"},
		{fmt.Errorf("%w: unrecognized directive", proxy.ErrConfigRejected), "backend.proxy_config_rejected"},
		{proxy.ErrNotRunning, "backend.proxy_not_running"},
		{errors.New("docker not available: dial unix: no such file"), ""},
	} {
		if got := proxyErrorKey(tc.err); got != tc.want {
			t.Errorf("proxyErrorKey(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestProxyErrorKeyExplainsRateLimitBuildRefusedByReadOnly(t *testing.T) {
	// A read-only refusal is still an unavailable rate-limit image; its own
	// key must win so the user is not told to allow image builds.
	readOnly := fmt.Errorf("ensure proxy image: %w", fmt.Errorf("%w: %w: docker mutation is disabled by runtime permissions",
		proxy.ErrRateLimitImageUnavailable, proxy.ErrRateLimitImageReadOnly))
	if got, want := proxyErrorKey(readOnly), "backend.proxy_rate_limit_image_read_only"; got != want {
		t.Errorf("proxyErrorKey(%v) = %q, want %q", readOnly, got, want)
	}
	other := fmt.Errorf("ensure proxy image: %w", fmt.Errorf("%w: HTTP 403", proxy.ErrRateLimitImageUnavailable))
	if got, want := proxyErrorKey(other), "backend.proxy_rate_limit_image_unavailable"; got != want {
		t.Errorf("proxyErrorKey(%v) = %q, want %q", other, got, want)
	}
}
