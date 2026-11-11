package llm

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLoggingTransportRedactsURLInsideErrorAndPreservesCause(t *testing.T) {
	var logs bytes.Buffer
	cause := &url.Error{Op: "Post", URL: "https://user:private-password@provider.example/v1?token=private-query#private-fragment", Err: errors.New("connection refused")}
	transport := newLoggingTransport(transportLogRoundTripFunc(func(*http.Request) (*http.Response, error) { return nil, cause }), slog.New(slog.NewTextHandler(&logs, nil)))
	_, err := transport.RoundTrip(httptest.NewRequest("POST", "https://provider.example/v1?token=private-query", nil))
	if err != cause {
		t.Fatal("transport replaced the original diagnostic cause")
	}
	for _, secret := range []string{"private-password", "private-query", "private-fragment"} {
		if strings.Contains(logs.String(), secret) {
			t.Fatal("transport log contains a URL credential")
		}
	}
	if !strings.Contains(logs.String(), "connection refused") {
		t.Fatal("safe cause was lost")
	}
}

func TestLoggingTransportDoesNotLogRateLimitResponseBody(t *testing.T) {
	var logs bytes.Buffer
	base := transportLogRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("private-provider-body"))}, nil
	})
	transport := newLoggingTransport(&rateLimitAwareTransport{base: base}, slog.New(slog.NewTextHandler(&logs, nil)))
	_, err := transport.RoundTrip(httptest.NewRequest("POST", "https://provider.example/v1", nil))
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("original rate-limit error lost: %T", err)
	}
	if strings.Contains(logs.String(), "private-provider-body") {
		t.Fatal("provider response body reached logs")
	}
	if !strings.Contains(logs.String(), "429") {
		t.Fatal("HTTP status missing from safe diagnostics")
	}
}
