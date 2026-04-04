package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// RetryIntervals defines the wait times between retries: 30s, then 2m, then 10m (FinalRetryInterval).
var RetryIntervals = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
}

const FinalRetryInterval = 10 * time.Minute

// maxRetryAttempts caps the total number of retry attempts to prevent infinite
// loops when a provider returns persistent transient errors (e.g. prolonged outage).
const maxRetryAttempts = 10

// FeedbackProvider allows the retry loop to notify the UI/Transports
type FeedbackProvider interface {
	Send(event, message string)
}

// ExecuteWithRetry wraps CreateChatCompletion with the specified retry logic
func ExecuteWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (openai.ChatCompletionResponse, error) {
	return ExecuteWithCustomRetry(ctx, client, req, logger, broker, RetryIntervals, FinalRetryInterval)
}

// isNonRetryable returns true for errors that should never be retried because
// they indicate a permanent configuration or model issue (e.g. unknown model,
// unsupported parameters).  Ollama and other providers return these.
func isNonRetryable(lowerErr string) bool {
	return strings.Contains(lowerErr, "model not found") || // generic
		strings.Contains(lowerErr, "' not found") || // Ollama: model 'name' not found
		strings.Contains(lowerErr, "unknown model") ||
		strings.Contains(lowerErr, "unknown parameter") ||
		strings.Contains(lowerErr, "invalid model") ||
		strings.Contains(lowerErr, "does not support") ||
		strings.Contains(lowerErr, "not supported") ||
		strings.Contains(lowerErr, "401") ||
		strings.Contains(lowerErr, "403")
}

// isTransientError returns true for errors that are likely transient and worth retrying.
func isTransientError(lowerErr string) bool {
	return strings.Contains(lowerErr, "too many requests") ||
		strings.Contains(lowerErr, "rate limit") ||
		strings.Contains(lowerErr, "timeout") ||
		strings.Contains(lowerErr, "deadline") ||
		strings.Contains(lowerErr, "connection") ||
		strings.Contains(lowerErr, "503") ||
		strings.Contains(lowerErr, "502") ||
		strings.Contains(lowerErr, "504") ||
		strings.Contains(lowerErr, "529") ||
		strings.Contains(lowerErr, "overloaded") ||
		strings.Contains(lowerErr, "server error") ||
		strings.Contains(lowerErr, "eof")
}

// ExecuteWithCustomRetry allows specifying custom intervals
func ExecuteWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (openai.ChatCompletionResponse, error) {
	attempt := 0

	for {
		resp, err := client.CreateChatCompletion(ctx, req)
		if err == nil {
			return resp, nil
		}

		lowerErr := strings.ToLower(err.Error())

		// Permanent errors should not be retried
		if isNonRetryable(lowerErr) {
			logger.Error("[LLM Retry] Non-retryable error, aborting", "error", err)
			return openai.ChatCompletionResponse{}, err
		}

		// Only retry transient errors
		if !isTransientError(lowerErr) {
			return openai.ChatCompletionResponse{}, err
		}

		// Determine wait time
		var waitTime time.Duration
		if attempt < len(intervals) {
			waitTime = intervals[attempt]
		} else {
			waitTime = finalInterval
		}

		attempt++
		if attempt >= maxRetryAttempts {
			logger.Error("[LLM Retry] Max retry attempts reached, aborting", "attempts", attempt, "error", err)
			return openai.ChatCompletionResponse{}, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}
		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		logger.Warn("[LLM Retry]", "error", safeErrMsg, "wait", waitTime, "attempt", attempt)

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		// Wait or cancel
		select {
		case <-time.After(waitTime):
			// continue loop
		case <-ctx.Done():
			return openai.ChatCompletionResponse{}, ctx.Err()
		}
	}
}

// ExecuteStreamWithRetry wraps CreateChatCompletionStream with the specified retry logic
func ExecuteStreamWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (*openai.ChatCompletionStream, error) {
	return ExecuteStreamWithCustomRetry(ctx, client, req, logger, broker, RetryIntervals, FinalRetryInterval)
}

// ExecuteStreamWithCustomRetry allows specifying custom intervals
func ExecuteStreamWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (*openai.ChatCompletionStream, error) {
	attempt := 0

	for {
		stream, err := client.CreateChatCompletionStream(ctx, req)
		if err == nil {
			return stream, nil
		}

		lowerErr := strings.ToLower(err.Error())

		// Permanent errors should not be retried
		if isNonRetryable(lowerErr) {
			logger.Error("[LLM Stream Retry] Non-retryable error, aborting", "error", err)
			return nil, err
		}

		// Only retry transient errors
		if !isTransientError(lowerErr) {
			return nil, err
		}

		// Determine wait time
		var waitTime time.Duration
		if attempt < len(intervals) {
			waitTime = intervals[attempt]
		} else {
			waitTime = finalInterval
		}

		attempt++
		if attempt >= maxRetryAttempts {
			logger.Error("[LLM Stream Retry] Max retry attempts reached, aborting", "attempts", attempt, "error", err)
			return nil, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}
		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("Stream API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		logger.Warn("[LLM Stream Retry]", "error", safeErrMsg, "wait", waitTime, "attempt", attempt)

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		// Wait or cancel
		select {
		case <-time.After(waitTime):
			// continue loop
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// safeAPIError returns a user-safe error description that avoids exposing raw
// HTTP response bodies or URL fragments that might contain credentials.
// For structured API errors, only the status code and server message are used.
// For other error types (e.g. network timeouts), the standard message is returned
// because those do not contain auth material.
func safeAPIError(err error) string {
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("status %d: %s", apiErr.HTTPStatusCode, apiErr.Message)
	}
	return err.Error()
}
