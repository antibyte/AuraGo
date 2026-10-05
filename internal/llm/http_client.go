package llm

import (
	"net/http"
	"time"

	"aurago/internal/httporigin"
)

// newProbeHTTPClient is the client for capability and health probes. Probes
// carry the provider key, so they never leave the configured origin.
func newProbeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: httporigin.SameOriginRedirect}
}
