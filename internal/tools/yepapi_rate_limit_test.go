package tools

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// yepAPIFakeClock is the injected clock of the rate limiter tests.
type yepAPIFakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *yepAPIFakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *yepAPIFakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// yepAPITestLimiter returns a limiter that runs on a fake clock and records
// every delay it asks for.
func yepAPITestLimiter(clock *yepAPIFakeClock) (*yepAPIRateLimiter, *[]time.Duration) {
	waits := &[]time.Duration{}
	limiter := newYepAPIRateLimiter()
	limiter.now = clock.Now
	limiter.sleep = func(ctx context.Context, d time.Duration) error {
		*waits = append(*waits, d)
		clock.advance(d)
		return ctx.Err()
	}
	return limiter, waits
}

// The documented limit is 60 requests per minute and per key. The 61st
// request has to wait, the first 60 must not.
func TestYepAPIRateLimiterAllowsSixtyPerMinute(t *testing.T) {
	clock := &yepAPIFakeClock{now: time.Unix(1700000000, 0)}
	limiter, waits := yepAPITestLimiter(clock)

	for i := 0; i < yepAPIRateLimitPerMinute; i++ {
		if err := limiter.wait(context.Background(), "key-a"); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}
	if len(*waits) != 0 {
		t.Fatalf("the first %d requests must not wait, got %v", yepAPIRateLimitPerMinute, *waits)
	}

	if err := limiter.wait(context.Background(), "key-a"); err != nil {
		t.Fatalf("the request after the limit failed: %v", err)
	}
	if len(*waits) != 1 {
		t.Fatalf("the request after the limit must wait exactly once, got %v", *waits)
	}
	if (*waits)[0] <= 0 {
		t.Fatalf("wait = %v, want a positive delay", (*waits)[0])
	}
}

// Every API key has its own budget.
func TestYepAPIRateLimiterSeparatesKeys(t *testing.T) {
	clock := &yepAPIFakeClock{now: time.Unix(1700000000, 0)}
	limiter, waits := yepAPITestLimiter(clock)

	for i := 0; i < yepAPIRateLimitPerMinute; i++ {
		if err := limiter.wait(context.Background(), "key-a"); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}
	if err := limiter.wait(context.Background(), "key-b"); err != nil {
		t.Fatalf("the second key failed: %v", err)
	}
	if len(*waits) != 0 {
		t.Fatalf("a second key must have its own budget, got %v", *waits)
	}
}

// The limiter has to pace the real requests, not only its own bookkeeping.
func TestYepAPIClientPacesRequestsToTheRateLimit(t *testing.T) {
	original := yepAPIGlobalRateLimiter
	t.Cleanup(func() { yepAPIGlobalRateLimiter = original })

	clock := &yepAPIFakeClock{now: time.Unix(1700000000, 0)}
	limiter, waits := yepAPITestLimiter(clock)
	yepAPIGlobalRateLimiter = limiter

	client := NewYepAPIClient("yepapi-pace-test-key")
	client.baseURL = "https://yepapi.test"
	client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":{}}`)),
		}, nil
	})}

	for i := 0; i < yepAPIRateLimitPerMinute; i++ {
		if _, err := client.Post(context.Background(), "/v1/serp/google", map[string]string{"query": "x"}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}
	if len(*waits) != 0 {
		t.Fatalf("the first %d requests must not wait, got %v", yepAPIRateLimitPerMinute, *waits)
	}
	if _, err := client.Post(context.Background(), "/v1/serp/google", map[string]string{"query": "x"}); err != nil {
		t.Fatalf("the request after the limit failed: %v", err)
	}
	if len(*waits) != 1 {
		t.Fatalf("the request after the limit must wait exactly once, got %v", *waits)
	}
}

// A rate-limit refusal must name the limit and must never be resent.
func TestYepAPI429ReturnsGuidanceWithoutResending(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"RATE_LIMIT","message":"too many requests"}}`))
	}))
	defer server.Close()

	client := NewYepAPIClientWithBaseURL("yepapi-429-test-key", server.URL)
	_, err := client.Post(context.Background(), "/v1/serp/google", map[string]string{"query": "aurago"})
	if err == nil {
		t.Fatal("expected a rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("err = %v, want the error to name the rate limit", err)
	}
	if !strings.Contains(err.Error(), "60 requests per minute") {
		t.Fatalf("err = %v, want the error to name the limit", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("the server saw %d requests, want exactly 1", got)
	}
}
