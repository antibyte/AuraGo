package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sashabaranov/go-openai"
)

var defaultRetryIntervals = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
}

var finalRetryInterval = 30 * time.Second

var finalRetryIntervalMu sync.RWMutex

// FinalRetryInterval returns the current final retry backoff cap.
func FinalRetryInterval() time.Duration {
	finalRetryIntervalMu.RLock()
	defer finalRetryIntervalMu.RUnlock()
	return finalRetryInterval
}

// ConfigureFinalRetryInterval sets the final retry backoff cap. Values <= 0 are
// ignored. A common production default is 30s to avoid blocking the agent loop
// for minutes on transient provider errors.
func ConfigureFinalRetryInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	finalRetryIntervalMu.Lock()
	defer finalRetryIntervalMu.Unlock()
	finalRetryInterval = d
}

const maxRetryAttempts = 10

var defaultRetryIntervalsMu sync.RWMutex

func defaultRetryIntervalsCopy() []time.Duration {
	defaultRetryIntervalsMu.RLock()
	defer defaultRetryIntervalsMu.RUnlock()

	result := make([]time.Duration, len(defaultRetryIntervals))
	copy(result, defaultRetryIntervals)
	return result
}

// ConfigureDefaultRetryIntervals updates the shared retry backoff used by
// ExecuteWithRetry. Invalid entries are ignored so one bad config value does
// not disable the retry policy entirely.
func ConfigureDefaultRetryIntervals(intervalSpecs []string, logger *slog.Logger) {
	if len(intervalSpecs) == 0 {
		return
	}

	intervals := make([]time.Duration, 0, len(intervalSpecs))
	for _, spec := range intervalSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		interval, err := time.ParseDuration(spec)
		if err != nil || interval <= 0 {
			if logger != nil {
				logger.Warn("[LLM Retry] Ignoring invalid retry interval", "interval", spec, "error", err)
			}
			continue
		}
		intervals = append(intervals, interval)
	}
	if len(intervals) == 0 {
		return
	}

	defaultRetryIntervalsMu.Lock()
	defaultRetryIntervals = intervals
	defaultRetryIntervalsMu.Unlock()
}

var perAttemptTimeoutNanos atomic.Int64

func init() {
	perAttemptTimeoutNanos.Store(int64(120 * time.Second))
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

type activeProviderModelProvider interface {
	ActiveProviderAndModel() (string, string)
}

func activeProviderAndModel(client ChatClient, fallbackModel string) (string, string) {
	if scopedClient, ok := client.(activeProviderModelProvider); ok {
		return scopedClient.ActiveProviderAndModel()
	}
	return "", fallbackModel
}

func reportProviderError(client ChatClient, req openai.ChatCompletionRequest, operation string, err error, attempt int, retryable bool, timeout time.Duration) {
	if err == nil {
		return
	}
	provider, model := activeProviderAndModel(client, req.Model)
	ReportLLMHealthEvent(HealthEvent{
		Operation:         operation,
		Provider:          provider,
		Model:             model,
		ErrorCategory:     ClassifyError(err),
		ErrorSummary:      safeAPIError(err),
		Attempt:           attempt,
		Retryable:         retryable,
		PerAttemptTimeout: timeout,
	})
}

func reportProviderSuccess(client ChatClient, req openai.ChatCompletionRequest, operation string) {
	provider, model := activeProviderAndModel(client, req.Model)
	ReportLLMHealthSuccess(HealthSuccess{
		Operation: operation,
		Provider:  provider,
		Model:     model,
	})
}

func ExecuteWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (openai.ChatCompletionResponse, error) {
	return ExecuteWithCustomRetry(ctx, client, req, logger, broker, defaultRetryIntervalsCopy(), FinalRetryInterval())
}

func ExecuteWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (openai.ChatCompletionResponse, error) {
	attempt := 0

	for {
		timeout := perAttemptTimeout()
		attemptCtx, attemptCancel := context.WithTimeout(ctx, timeout)
		providerBefore, modelBefore := activeProviderAndModel(client, req.Model)

		if logger != nil {
			logger.Info("[LLM Retry] CreateChatCompletion starting",
				"attempt", attempt+1,
				"model", req.Model,
				"messages", len(req.Messages),
				"tools", len(req.Tools),
				"timeout", timeout,
			)
		}
		callStart := time.Now()
		resp, err := client.CreateChatCompletion(attemptCtx, req)
		callElapsed := time.Since(callStart)
		attemptCtxErr := attemptCtx.Err()
		parentCtxErr := ctx.Err()
		attemptCancel()

		if err == nil {
			reportProviderSuccess(client, req, "chat_completion")
			if logger != nil {
				logger.Info("[LLM Retry] CreateChatCompletion succeeded",
					"attempt", attempt+1,
					"elapsed_ms", callElapsed.Milliseconds(),
					"model", req.Model,
				)
			}
			return resp, nil
		}

		perAttemptContextError := IsContextError(err) && parentCtxErr == nil && attemptCtxErr != nil
		if IsContextError(err) && !perAttemptContextError {
			if logger != nil {
				logger.Debug("[LLM Retry] Context error, aborting without retry",
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return openai.ChatCompletionResponse{}, err
		}

		if !perAttemptContextError && IsNonRetryable(err) {
			reportProviderError(client, req, "chat_completion", err, attempt+1, false, timeout)
			if logger != nil {
				logger.Error("[LLM Retry] Non-retryable error, aborting",
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return openai.ChatCompletionResponse{}, err
		}

		attempt++
		reportProviderError(client, req, "chat_completion", err, attempt, true, timeout)
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Retry] Max retry attempts reached, aborting",
					"attempts", attempt,
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return openai.ChatCompletionResponse{}, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}

		waitTime := selectRetryWaitTime(attempt, intervals, finalInterval, err)
		providerAfter, modelAfter := activeProviderAndModel(client, req.Model)
		providerChanged := providerBefore != providerAfter || modelBefore != modelAfter
		if providerChanged {
			waitTime = 0
		}

		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			remaining := time.Until(deadline)
			if waitTime > 0 && remaining < waitTime {
				waitTime = remaining
			}
		} else if waitTime > timeout {
			waitTime = timeout
		}

		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		if logger != nil {
			// Distinguish transport-level timeouts (e.g. http2 response header
			// timeout) from context-level timeouts so operators can tell whether
			// the fix needs to be in the transport or the retry timeout.
			isTransportTimeout := strings.Contains(safeErrMsg, "timeout awaiting response headers")
			logger.Warn("[LLM Retry]",
				"error", safeErrMsg,
				"category", ClassifyError(err),
				"wait", waitTime,
				"attempt", attempt,
				"per_attempt_timeout", timeout,
				"attempt_ctx_err", attemptCtxErr,
				"parent_ctx_err", parentCtxErr,
				"is_transport_timeout", isTransportTimeout,
				"provider_changed", providerChanged,
				"call_elapsed_ms", callElapsed.Milliseconds(),
				"model", req.Model,
				"messages", len(req.Messages),
				"tools", len(req.Tools),
			)
		}

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		if !waitForRetry(ctx, waitTime) {
			return openai.ChatCompletionResponse{}, ctx.Err()
		}
	}
}

func ExecuteStreamWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (*openai.ChatCompletionStream, context.CancelFunc, error) {
	return ExecuteStreamWithCustomRetry(ctx, client, req, logger, broker, defaultRetryIntervalsCopy(), FinalRetryInterval())
}

func ExecuteStreamWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (*openai.ChatCompletionStream, context.CancelFunc, error) {
	attempt := 0
	noCancel := func() {}

	for {
		timeout := perAttemptTimeout()
		attemptCtx, attemptCancel := context.WithTimeout(ctx, timeout)
		providerBefore, modelBefore := activeProviderAndModel(client, req.Model)

		if logger != nil {
			logger.Info("[LLM Stream Retry] CreateChatCompletionStream starting",
				"attempt", attempt+1,
				"model", req.Model,
				"messages", len(req.Messages),
				"tools", len(req.Tools),
				"timeout", timeout,
			)
		}
		callStart := time.Now()
		stream, err := client.CreateChatCompletionStream(attemptCtx, req)
		callElapsed := time.Since(callStart)
		attemptCtxErr := attemptCtx.Err()
		parentCtxErr := ctx.Err()

		if err == nil {
			reportProviderSuccess(client, req, "chat_completion_stream")
			if logger != nil {
				logger.Info("[LLM Stream Retry] CreateChatCompletionStream succeeded",
					"attempt", attempt+1,
					"elapsed_ms", callElapsed.Milliseconds(),
					"model", req.Model,
				)
			}
			return stream, attemptCancel, nil
		}
		attemptCancel()

		perAttemptContextError := IsContextError(err) && parentCtxErr == nil && attemptCtxErr != nil
		if IsContextError(err) && !perAttemptContextError {
			if logger != nil {
				logger.Debug("[LLM Stream Retry] Context error, aborting without retry",
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return nil, noCancel, err
		}

		if !perAttemptContextError && IsNonRetryable(err) {
			reportProviderError(client, req, "chat_completion_stream", err, attempt+1, false, timeout)
			if logger != nil {
				logger.Error("[LLM Stream Retry] Non-retryable error, aborting",
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return nil, noCancel, err
		}

		attempt++
		reportProviderError(client, req, "chat_completion_stream", err, attempt, true, timeout)
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Stream Retry] Max retry attempts reached, aborting",
					"attempts", attempt,
					"error", err,
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return nil, noCancel, fmt.Errorf("max retry attempts (%d) exceeded: %w", maxRetryAttempts, err)
		}

		waitTime := selectRetryWaitTime(attempt, intervals, finalInterval, err)
		providerAfter, modelAfter := activeProviderAndModel(client, req.Model)
		providerChanged := providerBefore != providerAfter || modelBefore != modelAfter
		if providerChanged {
			waitTime = 0
		}

		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			remaining := time.Until(deadline)
			if waitTime > 0 && remaining < waitTime {
				waitTime = remaining
			}
		} else if waitTime > timeout {
			waitTime = timeout
		}

		safeErrMsg := safeAPIError(err)
		msg := fmt.Sprintf("Stream API Error (%s). Retrying in %v (Attempt %d)...", safeErrMsg, waitTime, attempt)
		if logger != nil {
			isTransportTimeout := strings.Contains(safeErrMsg, "timeout awaiting response headers")
			logger.Warn("[LLM Stream Retry]",
				"error", safeErrMsg,
				"category", ClassifyError(err),
				"wait", waitTime,
				"attempt", attempt,
				"per_attempt_timeout", timeout,
				"attempt_ctx_err", attemptCtxErr,
				"parent_ctx_err", parentCtxErr,
				"is_transport_timeout", isTransportTimeout,
				"provider_changed", providerChanged,
				"call_elapsed_ms", callElapsed.Milliseconds(),
				"model", req.Model,
				"messages", len(req.Messages),
				"tools", len(req.Tools),
			)
		}

		if broker != nil {
			broker.Send("api_retry", msg)
		}

		if !waitForRetry(ctx, waitTime) {
			return nil, noCancel, ctx.Err()
		}
	}
}

func selectRetryWaitTime(attempt int, intervals []time.Duration, finalInterval time.Duration, err error) time.Duration {
	if retryAfter := GetRetryAfter(err); retryAfter > 0 {
		return retryAfter
	}
	intervalIndex := attempt - 1
	if intervalIndex >= 0 && intervalIndex < len(intervals) {
		return intervals[intervalIndex]
	}
	return finalInterval
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
