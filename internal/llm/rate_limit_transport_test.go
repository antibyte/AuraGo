package llm

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParseRetryAfterHeaderSeconds(t *testing.T) {
	if got := parseRetryAfterHeader("7"); got != 7*time.Second {
		t.Fatalf("parseRetryAfterHeader(7) = %v, want 7s", got)
	}
}

func TestRateLimitAwareTransportWrapsRetryAfter(t *testing.T) {
	transport := &rateLimitAwareTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"9"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"slow down"}`)),
		}, nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	_, err = transport.RoundTrip(req)
	if err == nil {
		t.Fatal("expected rate limit error")
	}
	if got := GetRetryAfter(err); got != 9*time.Second {
		t.Fatalf("GetRetryAfter() = %v, want 9s", got)
	}
}