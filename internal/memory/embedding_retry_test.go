package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"aurago/internal/testutil"
)

// recordedEmbeddingWaits collects every wait the retry loops asked for.
type recordedEmbeddingWaits struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (r *recordedEmbeddingWaits) all() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.waits...)
}

// stubEmbeddingRetrySleep replaces the retry sleep so tests never really wait and
// records the wait each retry requested.
func stubEmbeddingRetrySleep(t *testing.T) *recordedEmbeddingWaits {
	t.Helper()
	rec := &recordedEmbeddingWaits{}
	old := embeddingRetrySleep
	embeddingRetrySleep = func(ctx context.Context, wait time.Duration) error {
		rec.mu.Lock()
		rec.waits = append(rec.waits, wait)
		rec.mu.Unlock()
		return ctx.Err()
	}
	t.Cleanup(func() { embeddingRetrySleep = old })
	return rec
}

func TestEmbeddingRetryRetriesTransientErrors(t *testing.T) {
	stubEmbeddingRetrySleep(t)

	attempts := 0
	embedding := withEmbeddingRetry(func(_ context.Context, _ string) ([]float32, error) {
		attempts++
		if attempts < 3 {
			return nil, &embeddingHTTPStatusError{statusCode: 503}
		}
		return []float32{1, 0, 0}, nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	got, err := embedding(context.Background(), "hello")
	if err != nil {
		t.Fatalf("embedding: %v", err)
	}
	if len(got) != 3 || attempts != 3 {
		t.Fatalf("got embedding len=%d attempts=%d, want len=3 attempts=3", len(got), attempts)
	}
}

func TestEmbeddingRetryClassifiesTypedFailures(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		if !shouldRetryEmbeddingError(context.Background(), &embeddingHTTPStatusError{statusCode: status}) {
			t.Errorf("status %d should be retried", status)
		}
	}
	for _, status := range []int{301, 400, 401, 403, 404, 501} {
		if shouldRetryEmbeddingError(context.Background(), &embeddingHTTPStatusError{statusCode: status}) {
			t.Errorf("status %d should not be retried", status)
		}
	}
	if shouldRetryEmbeddingError(context.Background(), errors.New("provider says 503 temporarily")) {
		t.Fatal("untyped status text must not make an error retryable")
	}
	refused := &embeddingTransportError{cause: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}}
	if !shouldRetryEmbeddingError(context.Background(), refused) {
		t.Fatal("typed connection-refused transport error should be retried")
	}
}

func TestEmbeddingRetryDoesNotRetryContextCanceled(t *testing.T) {
	stubEmbeddingRetrySleep(t)

	attempts := 0
	embedding := withEmbeddingRetry(func(_ context.Context, _ string) ([]float32, error) {
		attempts++
		return nil, context.Canceled
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := embedding(context.Background(), "hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("embedding err = %v, want context.Canceled", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

// A 429 that names Retry-After must be waited out: the wait handed to the sleep
// seam is at least the provider's hint and the next attempt then succeeds.
func TestEmbeddingRetryHonorsRetryAfterHeader(t *testing.T) {
	waits := stubEmbeddingRetrySleep(t)

	var hits atomic.Int32
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, `{"data":[{"embedding":[3,4]}]}`)
	}))
	defer server.Close()

	fn := withEmbeddingRetry(newOpenAICompatEmbeddingFunc(server.URL+"/v1", "key", "model"), nil)
	got, err := fn(context.Background(), "hello")
	if err != nil {
		t.Fatalf("embedding after 429: %v", err)
	}
	if len(got) != 2 || hits.Load() != 2 {
		t.Fatalf("vector=%v hits=%d, want a 2-dim vector after exactly 2 requests", got, hits.Load())
	}
	recorded := waits.all()
	if len(recorded) != 1 || recorded[0] < 2*time.Second {
		t.Fatalf("recorded waits = %v, want one wait of at least the 2s Retry-After", recorded)
	}
}

func TestParseEmbeddingRetryAfter(t *testing.T) {
	now := time.Date(2026, time.October, 23, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, value string
		want        time.Duration
	}{
		{"delta seconds", "2", 2 * time.Second},
		{"zero", "0", 0},
		{"padded", "  7 ", 7 * time.Second},
		{"capped", "600", embeddingMaxRetryAfter},
		{"overflowing digits are capped", "99999999999999999999999", embeddingMaxRetryAfter},
		{"negative ignored", "-5", 0},
		{"garbage ignored", "soon", 0},
		{"fraction ignored", "1.5", 0},
		{"empty", "", 0},
		{"future http date", now.Add(10 * time.Second).Format(http.TimeFormat), 10 * time.Second},
		{"far future http date capped", now.Add(time.Hour).Format(http.TimeFormat), embeddingMaxRetryAfter},
		{"past http date ignored", now.Add(-time.Minute).Format(http.TimeFormat), 0},
	} {
		if got := parseEmbeddingRetryAfter(tc.value, now); got != tc.want {
			t.Errorf("%s: parseEmbeddingRetryAfter(%q) = %v, want %v", tc.name, tc.value, got, tc.want)
		}
	}
}

// Retry-After is only a rate-limit / maintenance hint: other statuses ignore it.
func TestEmbeddingHTTPStatusErrorTakesRetryAfterOnlyFor429And503(t *testing.T) {
	for status, want := range map[int]time.Duration{
		http.StatusTooManyRequests:     3 * time.Second,
		http.StatusServiceUnavailable:  3 * time.Second,
		http.StatusInternalServerError: 0,
		http.StatusBadRequest:          0,
	} {
		resp := &http.Response{StatusCode: status, Header: http.Header{"Retry-After": []string{"3"}}}
		if got := newEmbeddingHTTPStatusError(resp).retryAfter; got != want {
			t.Errorf("status %d: retryAfter = %v, want %v", status, got, want)
		}
	}
}

func TestEmbeddingRetryWaitStaysWithinCaps(t *testing.T) {
	for attempt := 1; attempt <= 12; attempt++ {
		ceiling := embeddingRetryBaseDelay << (attempt - 1)
		if ceiling > embeddingRetryMaxBackoff || ceiling <= 0 {
			ceiling = embeddingRetryMaxBackoff
		}
		for range 200 {
			if got := embeddingRetryWait(attempt, nil); got < 0 || got > ceiling {
				t.Fatalf("attempt %d: backoff %v outside [0, %v]", attempt, got, ceiling)
			}
		}
	}
	// A Retry-After hint wins over a smaller backoff but never exceeds the overall cap.
	hint := &embeddingHTTPStatusError{statusCode: 429, retryAfter: 20 * time.Second}
	if got := embeddingRetryWait(1, hint); got < 20*time.Second || got > embeddingRetryMaxWait {
		t.Fatalf("wait with 20s Retry-After = %v, want within [20s, %v]", got, embeddingRetryMaxWait)
	}
	huge := &embeddingHTTPStatusError{statusCode: 503, retryAfter: time.Hour}
	if got := embeddingRetryWait(5, huge); got != embeddingRetryMaxWait {
		t.Fatalf("wait with an hour-long Retry-After = %v, want the %v cap", got, embeddingRetryMaxWait)
	}
}

// The default sleep must give way to the caller's context instead of riding out a
// long Retry-After.
func TestEmbeddingRetryWaitAbortsOnContextDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var calls atomic.Int32
	fn := withEmbeddingRetry(func(context.Context, string) ([]float32, error) {
		calls.Add(1)
		return nil, &embeddingHTTPStatusError{statusCode: 503, retryAfter: 20 * time.Second}
	}, nil)
	started := time.Now()
	_, err := fn(ctx, "hello")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want the caller's context error", err)
	}
	if calls.Load() != 1 || time.Since(started) > 2*time.Second {
		t.Fatalf("calls=%d after %v, want one call and an abort at the context deadline", calls.Load(), time.Since(started))
	}
}
