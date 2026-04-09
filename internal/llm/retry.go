package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/sashabaranov/go-openai"
)

var defaultRetryIntervals = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
}

const FinalRetryInterval = 5 * time.Minute

const maxRetryAttempts = 10

func defaultRetryIntervalsCopy() []time.Duration {
	result := make([]time.Duration, len(defaultRetryIntervals))
	copy(result, defaultRetryIntervals)
	return result
}

var perAttemptTimeoutNanos atomic.Int64

func init() {
	perAttemptTimeoutNanos.Store(int64(60 * time.Second))
}

func perAttemptTimeout() time.Duration {
	v := perAttemptTimeoutNanos.Load()
	if v <= 0 {
		return 60 * time.Second
	}
	return time.Duration(v)
}

// SetPerAttemptTimeout configures the per-attempt API timeout used by the retry loops.
// It is safe to call concurrently.
func SetPerAttemptTimeout(d time.Duration) {
	if d <= 0 {
		return
	}
	perAttemptTimeoutNanos.Store(int64(d))
}

type FeedbackProvider interface {
	Send(event, message string)
}

func ExecuteWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (openai.ChatCompletionResponse, error) {
	return ExecuteWithCustomRetry(ctx, client, req, logger, broker, defaultRetryIntervalsCopy(), FinalRetryInterval)
}

func ExecuteWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (openai.ChatCompletionResponse, error) {
	attempt := 0

	for {
		timeout := perAttemptTimeout()
		attemptCtx, attemptCancel := context.WithTimeout(ctx, timeout)
		resp, err := client.CreateChatCompletion(attemptCtx, req)
		attemptCancel()
		if err == nil {
			return resp, nil
		}

		if IsContextError(err) {
			if logger != nil {
				logger.Debug("[LLM Retry] Context error, aborting without retry", "error", err)
			}
			return openai.ChatCompletionResponse{}, err
		}

		if IsNonRetryable(err) {
			if logger != nil {
				logger.Error("[LLM Retry] Non-retryable error, aborting", "error", err, "category", ClassifyError(err))
			}
			return openai.ChatCompletionResponse{}, err
		}

		attempt++
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Retry] Max retry attempts reached, aborting", "attempts", attempt, "error", err)
			}
			return openai.ChatCompletionResponse{}, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}

		var waitTime time.Duration
		if retryAfter := GetRetryAfter(err); retryAfter > 0 {
			waitTime = retryAfter
		} else if attempt < len(intervals) {
			waitTime = intervals[attempt]
		} else {
			waitTime = finalInterval
		}

		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			remaining := time.Until(deadline)
			if remaining < waitTime {
				waitTime = remaining
			}
		} else if waitTime > timeout {
			waitTime = timeout
		}

		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		if logger != nil {
			logger.Warn("[LLM Retry]", "error", safeErrMsg, "wait", waitTime, "attempt", attempt)
		}

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		if !waitForRetry(ctx, waitTime) {
			return openai.ChatCompletionResponse{}, ctx.Err()
		}
	}
}

func ExecuteStreamWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (*openai.ChatCompletionStream, error) {
	return ExecuteStreamWithCustomRetry(ctx, client, req, logger, broker, defaultRetryIntervalsCopy(), FinalRetryInterval)
}

func ExecuteStreamWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (*openai.ChatCompletionStream, error) {
	attempt := 0

	for {
		timeout := perAttemptTimeout()
		attemptCtx, attemptCancel := context.WithTimeout(ctx, timeout)
		stream, err := client.CreateChatCompletionStream(attemptCtx, req)
		attemptCancel()
		if err == nil {
			return stream, nil
		}

		if IsContextError(err) {
			if logger != nil {
				logger.Debug("[LLM Stream Retry] Context error, aborting without retry", "error", err)
			}
			return nil, err
		}

		if IsNonRetryable(err) {
			if logger != nil {
				logger.Error("[LLM Stream Retry] Non-retryable error, aborting", "error", err, "category", ClassifyError(err))
			}
			return nil, err
		}

		attempt++
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Stream Retry] Max retry attempts reached, aborting", "attempts", attempt, "error", err)
			}
			return nil, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}

		var waitTime time.Duration
		if retryAfter := GetRetryAfter(err); retryAfter > 0 {
			waitTime = retryAfter
		} else if attempt < len(intervals) {
			waitTime = intervals[attempt]
		} else {
			waitTime = finalInterval
		}

		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			remaining := time.Until(deadline)
			if remaining < waitTime {
				waitTime = remaining
			}
		} else if waitTime > timeout {
			waitTime = timeout
		}

		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("Stream API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		if logger != nil {
			logger.Warn("[LLM Stream Retry]", "error", safeErrMsg, "wait", waitTime, "attempt", attempt)
		}

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		if !waitForRetry(ctx, waitTime) {
			return nil, ctx.Err()
		}
	}
}

func waitForRetry(ctx context.Context, waitTime time.Duration) bool {
	timer := time.NewTimer(waitTime)
	defer timer.Stop()

	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func safeAPIError(err error) string {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("status %d: %s", apiErr.HTTPStatusCode, apiErr.Message)
	}
	return err.Error()
}
