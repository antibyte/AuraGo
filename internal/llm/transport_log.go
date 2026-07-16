package llm

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

type expectedDeadlineContextKey struct{}

// WithExpectedDeadline marks a request whose deadline is an expected bounded
// degradation path. The marker contains no prompt or credential data.
func WithExpectedDeadline(ctx context.Context) context.Context {
	return context.WithValue(ctx, expectedDeadlineContextKey{}, true)
}

func hasExpectedDeadline(ctx context.Context) bool {
	expected, _ := ctx.Value(expectedDeadlineContextKey{}).(bool)
	return expected
}

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

	if err != nil && hasExpectedDeadline(req.Context()) && errors.Is(err, context.DeadlineExceeded) {
		t.logger.Debug("[LLM Transport] roundtrip_done (expected deadline)",
			"method", method,
			"url", url,
			"elapsed_ms", elapsed.Milliseconds(),
		)
		return nil, err
	}
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
