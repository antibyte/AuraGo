package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
			return nil, fmt.Errorf("embedding API error 503: unavailable")
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
