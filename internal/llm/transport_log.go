package llm

import (
	"log/slog"
	"net/http"
	"time"
)

// loggingTransport wraps an http.RoundTripper and records precise timing for
// every LLM request so operators can tell where time is spent:
//   - request_start      → RoundTrip begins
//   - request_headers    → request header is ready (before body write)
//   - response_headers   → response headers received
//   - roundtrip_done     → RoundTrip returns (success or error)
//
// Use this when an LLM request hangs to distinguish body-write stalls,
// network/TLS stalls, and slow-first-byte stalls from the provider.
type loggingTransport struct {
	base   http.RoundTripper
	logger *slog.Logger
}

func newLoggingTransport(base http.RoundTripper, logger *slog.Logger) *loggingTransport {
	return &loggingTransport{base: base, logger: logger}
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.logger == nil {
		return t.base.RoundTrip(req)
	}

	start := time.Now()
	url := req.URL.String()
	method := req.Method

	t.logger.Info("[LLM Transport] request_start",
		"method", method,
		"url", url,
		"content_length", req.ContentLength,
	)

	resp, err := t.base.RoundTrip(req)
	elapsed := time.Since(start)

	if err != nil {
		t.logger.Error("[LLM Transport] roundtrip_done (error)",
			"method", method,
			"url", url,
			"elapsed_ms", elapsed.Milliseconds(),
			"error", err,
		)
		return nil, err
	}

	t.logger.Info("[LLM Transport] roundtrip_done (success)",
		"method", method,
		"url", url,
		"status", resp.StatusCode,
		"elapsed_ms", elapsed.Milliseconds(),
		"content_length", resp.ContentLength,
	)
	return resp, nil
}
