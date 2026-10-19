package llm

import (
	"fmt"
	"net/http"
	"time"

	"aurago/internal/config"
)

// effectiveAIGatewayTimeout bounds response start without limiting active SSE bodies.
func effectiveAIGatewayTimeout(cfg *config.Config, provider string) time.Duration {
	limit := min(responseHeaderTimeoutForProvider(cfg, provider), 3*time.Minute)
	if cfg != nil && cfg.AIGateway.RequestTimeoutMS > 0 {
		limit = min(limit, time.Duration(cfg.AIGateway.RequestTimeoutMS)*time.Millisecond)
	}
	return limit
}

func effectiveAIGatewayRequestTimeoutMS(cfg *config.Config, provider string) int {
	if cfg == nil || cfg.AIGateway.RequestTimeoutMS <= 0 {
		return 0
	}
	return int(effectiveAIGatewayTimeout(cfg, provider).Milliseconds())
}

type invalidProviderTransport struct{ message string }

func (t invalidProviderTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("invalid provider configuration: %s", t.message)
}
