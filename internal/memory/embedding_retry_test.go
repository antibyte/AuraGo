package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestEmbeddingRetryRetriesTransientErrors(t *testing.T) {
	oldDelay := embeddingRetryDelay
	embeddingRetryDelay = func(int) time.Duration { return 0 }
	t.Cleanup(func() { embeddingRetryDelay = oldDelay })

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
	oldDelay := embeddingRetryDelay
	embeddingRetryDelay = func(int) time.Duration { return 0 }
	t.Cleanup(func() { embeddingRetryDelay = oldDelay })

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
