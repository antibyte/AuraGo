package proxy

import "errors"

// Errors callers can map to user-facing explanations. Each one is wrapped
// with the concrete reason.
var (
	// ErrBasicAuthCredentialsMissing: basic auth is enabled but the Vault has
	// no proxy_basic_auth_user / proxy_basic_auth_pass.
	ErrBasicAuthCredentialsMissing = errors.New("security proxy basic auth is enabled but the Vault has no proxy_basic_auth_user and proxy_basic_auth_pass")
	// ErrBasicAuthCredentialsInvalid: the Vault credentials cannot be written
	// into a Caddyfile safely.
	ErrBasicAuthCredentialsInvalid = errors.New("security proxy basic auth credentials cannot be used")
	// ErrRateLimitImageUnavailable: rate limiting is enabled and the Caddy
	// image with the caddy-ratelimit module could not be built.
	ErrRateLimitImageUnavailable = errors.New("the Caddy image with the rate limit module is unavailable")
	// ErrRateLimitImageReadOnly: docker.read_only refused the build of that
	// image. It always comes wrapped together with ErrRateLimitImageUnavailable.
	ErrRateLimitImageReadOnly = errors.New("docker.read_only refuses the image build")
	// ErrDockerPlacement: AuraGo runs in Docker and its proxy files cannot be
	// shared with the Caddy container.
	ErrDockerPlacement = errors.New("the security proxy files cannot be shared with the Caddy container")
	// ErrCaddyExited: the container started but Caddy stopped right away.
	ErrCaddyExited = errors.New("the proxy container stopped right after starting")
	// ErrConfigRejected: `caddy reload` refused the new configuration; Caddy
	// keeps serving the previous one.
	ErrConfigRejected = errors.New("the proxy rejected the new configuration and keeps the previous one")
	// ErrNotRunning: Reload needs a running proxy container.
	ErrNotRunning = errors.New("the security proxy container is not running")
)
