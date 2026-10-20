package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"syscall"
	"time"

	"aurago/internal/httporigin"

	chromem "github.com/philippgille/chromem-go"
)

const embeddingRetryAttempts = 3

type embeddingHTTPStatusError struct {
	statusCode int
}

func (e *embeddingHTTPStatusError) Error() string {
	return "embedding API returned HTTP status " + strconv.Itoa(e.statusCode)
}

type embeddingTransportError struct {
	cause error
}

func (e *embeddingTransportError) Error() string { return "embedding HTTP request failed" }

func (e *embeddingTransportError) Unwrap() error { return e.cause }

var embeddingRetryDelay = func(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	return time.Duration(100*attempt*attempt) * time.Millisecond
}

func withEmbeddingRetry(ef chromem.EmbeddingFunc, logger *slog.Logger) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		var lastErr error
		for attempt := 1; attempt <= embeddingRetryAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			vec, err := ef(ctx, text)
			if err == nil {
				return vec, nil
			}
			lastErr = err
			if !shouldRetryEmbeddingError(ctx, err) || attempt == embeddingRetryAttempts {
				return nil, err
			}
			delay := embeddingRetryDelay(attempt)
			if logger != nil {
				logger.Warn("Embedding request failed transiently; retrying", "attempt", attempt, "error", err, "backoff", delay)
			}
			if delay <= 0 {
				continue
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil, ctx.Err()
			case <-timer.C:
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
