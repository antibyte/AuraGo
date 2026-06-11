package memory

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

const embeddingRetryAttempts = 3

var embeddingRetryDelay = func(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	return time.Duration(100*attempt*attempt) * time.Millisecond
}

func withEmbeddingRetry(ef chromem.EmbeddingFunc, logger *slog.Logger) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		var lastErr error
		for attempt := 1; attempt <= embeddingRetryAttempts; attempt++ {
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
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	msg := strings.ToLower(err.Error())
	transientFragments := []string{
		"429",
		"502",
		"503",
		"504",
		"timeout",
		"temporary",
		"temporarily",
		"connection reset",
		"connection refused",
		"server closed",
		"eof",
	}
	for _, fragment := range transientFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}
	return false
}
