package llm

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/sashabaranov/go-openai"
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

type countingReadCloser struct {
	reader io.Reader
	read   int
	closed bool
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.read += n
	return n, err
}

func (c *countingReadCloser) Close() error {
	c.closed = true
	return nil
}

func TestRateLimitAwareTransportBoundsRateLimitBody(t *testing.T) {
	const maxBody = 4 << 10
	body := &countingReadCloser{reader: strings.NewReader(strings.Repeat("x", 1<<20))}
	transport := &rateLimitAwareTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{"Retry-After": []string{"11"}},
			Body:       body,
		}, nil
	})}

	req, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		t.Fatalf("RoundTrip() response = %#v, want nil on 429", resp)
	}
	var rlErr *RateLimitError
	if !errors.As(err, &rlErr) {
		t.Fatalf("RoundTrip() error = %T %v, want *RateLimitError", err, err)
	}
	var apiErr *openai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("RoundTrip() error does not wrap *openai.APIError: %v", err)
	}
	if len(apiErr.Message) > maxBody {
		t.Fatalf("APIError message length = %d, want <= %d", len(apiErr.Message), maxBody)
	}
	const wrapperAllowance = 128 // "rate limited: error, status code: 429, status: , message: "
	if got := len(err.Error()); got > maxBody+wrapperAllowance {
		t.Fatalf("RateLimitError message length = %d, want <= %d", got, maxBody+wrapperAllowance)
	}
	if body.read > maxBody+512 {
		t.Fatalf("transport read %d body bytes, want a bounded read near %d", body.read, maxBody)
	}
	if !body.closed {
		t.Fatal("429 response body was not closed")
	}
	if got := GetRetryAfter(err); got != 11*time.Second {
		t.Fatalf("GetRetryAfter() = %v, want 11s", got)
	}
}

func TestRateLimitAwareTransportKeepsCutBodyValidUTF8(t *testing.T) {
	// One ASCII byte shifts the two-byte runes so the 4 KiB cut splits one.
	payload := "x" + strings.Repeat("é", 4<<10)
	transport := &rateLimitAwareTransport{base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(payload)),
		}, nil
	})}
	req, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/chat/completions", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	_, err = transport.RoundTrip(req)
	var apiErr *openai.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("RoundTrip() error = %v, want *openai.APIError inside", err)
	}
	if !utf8.ValidString(apiErr.Message) {
		t.Fatalf("APIError message is not valid UTF-8 after the cut: %q", apiErr.Message[len(apiErr.Message)-4:])
	}
	if len(apiErr.Message) > 4<<10 {
		t.Fatalf("APIError message length = %d, want <= %d", len(apiErr.Message), 4<<10)
	}
}
