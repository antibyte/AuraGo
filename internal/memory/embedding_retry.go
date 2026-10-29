package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"strconv"
	"syscall"
	"time"

	"aurago/internal/httporigin"

	chromem "github.com/philippgille/chromem-go"
)

const (
	// embeddingRetryAttempts counts the first request too: a provider that stays
	// unavailable is asked five times before the caller sees the failure.
	embeddingRetryAttempts = 5
	// embeddingRetryBaseDelay and embeddingRetryMaxBackoff bound the exponential
	// backoff (factor 2) whose waits are drawn with full jitter.
	embeddingRetryBaseDelay  = 500 * time.Millisecond
	embeddingRetryMaxBackoff = 8 * time.Second
	// embeddingRetryMaxWait caps any wait, including one dictated by Retry-After.
	embeddingRetryMaxWait = 30 * time.Second
	// embeddingMaxRetryAfter caps a Retry-After hint when it is parsed.
	embeddingMaxRetryAfter = 30 * time.Second
)

type embeddingHTTPStatusError struct {
	statusCode int
	// retryAfter is the provider's Retry-After hint for 429/503 responses; zero
	// when absent, malformed or not applicable.
	retryAfter time.Duration
}

func (e *embeddingHTTPStatusError) Error() string {
	return "embedding API returned HTTP status " + strconv.Itoa(e.statusCode)
}

type embeddingTransportError struct {
	cause error
}

func (e *embeddingTransportError) Error() string { return "embedding HTTP request failed" }

func (e *embeddingTransportError) Unwrap() error { return e.cause }

// embeddingRetryJitter draws the full-jitter share of a backoff in [0, limit].
// Tests replace it to make waits deterministic.
var embeddingRetryJitter = func(limit time.Duration) time.Duration {
	if limit <= 0 {
		return 0
	}
	return rand.N(limit + 1)
}

// embeddingRetryWait returns how long to wait after the attempt-th failed request
// (1-based): exponential backoff with full jitter, and the longer of that backoff
// and the provider's Retry-After hint when the error carries one, capped overall.
func embeddingRetryWait(attempt int, err error) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	ceiling := embeddingRetryMaxBackoff
	if shift := attempt - 1; shift < 16 {
		if exp := embeddingRetryBaseDelay << shift; exp < ceiling {
			ceiling = exp
		}
	}
	wait := embeddingRetryJitter(ceiling)
	var statusErr *embeddingHTTPStatusError
	if errors.As(err, &statusErr) && statusErr.retryAfter > wait {
		wait = statusErr.retryAfter
	}
	if wait > embeddingRetryMaxWait {
		return embeddingRetryMaxWait
	}
	return wait
}

// embeddingRetrySleep waits for the retry delay and gives way to the context.
// Every retry loop sleeps through it, so tests replace it to avoid real waits.
var embeddingRetrySleep = func(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// waitBeforeEmbeddingRetry logs and waits out the delay before the next attempt.
// It returns the context error when the caller gives up during the wait.
func waitBeforeEmbeddingRetry(ctx context.Context, logger *slog.Logger, label string, attempt int, cause error) error {
	wait := embeddingRetryWait(attempt, cause)
	if logger != nil {
		logger.Warn(label+" failed transiently; retrying", "attempt", attempt, "of", embeddingRetryAttempts, "error", cause, "backoff", wait)
	}
	return embeddingRetrySleep(ctx, wait)
}

// backgroundEmbeddingKey marks the context of a background corpus pass.
type backgroundEmbeddingKey struct{}

// withBackgroundEmbedding marks ctx as belonging to a background corpus pass
// (startup reconcile, runtime retry loop). withEmbeddingRetry makes a single
// attempt for such a context: the pass holds the store-side embedding slot for
// the call, and the next pass is the retry, so sitting through backoff and
// Retry-After waits would only keep foreground stores out.
func withBackgroundEmbedding(ctx context.Context) context.Context {
	return context.WithValue(ctx, backgroundEmbeddingKey{}, true)
}

func isBackgroundEmbedding(ctx context.Context) bool {
	background, _ := ctx.Value(backgroundEmbeddingKey{}).(bool)
	return background
}

func withEmbeddingRetry(ef chromem.EmbeddingFunc, logger *slog.Logger) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		attempts := embeddingRetryAttempts
		if isBackgroundEmbedding(ctx) {
			attempts = 1
		}
		var lastErr error
		for attempt := 1; attempt <= attempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			vec, err := ef(ctx, text)
			if err == nil {
				return vec, nil
			}
			lastErr = err
			if !shouldRetryEmbeddingError(ctx, err) || attempt == attempts {
				return nil, err
			}
			if waitErr := waitBeforeEmbeddingRetry(ctx, logger, "Embedding request", attempt, err); waitErr != nil {
				return nil, waitErr
			}
		}
		return nil, lastErr
	}
}

func shouldRetryEmbeddingError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	// A rejected cross-origin redirect is a configuration fault, not a
	// transient one; its message can still contain a port such as 50312.
	if errors.Is(err, httporigin.ErrCrossOriginRedirect) {
		return false
	}
	var statusErr *embeddingHTTPStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.statusCode {
		case 408, 429, 500, 502, 503, 504:
			return true
		default:
			return false
		}
	}
	// A request-level timeout is retryable while the caller's context remains
	// live. A caller deadline/cancellation was rejected above via ctx.Err().
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	for _, networkFailure := range []error{syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.EPIPE, syscall.ETIMEDOUT} {
		if errors.Is(err, networkFailure) {
			return true
		}
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary()) {
		return true
	}
	return false
}
