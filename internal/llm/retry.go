package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sashabaranov/go-openai"
)

var defaultPolicyMu sync.RWMutex

var defaultRetryIntervals = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
}

var finalRetryInterval = 30 * time.Second

const fallbackFinalRetryInterval = 30 * time.Second
const defaultPerAttemptTimeout = 120 * time.Second

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
	defaultPolicyMu.Lock()
	defer defaultPolicyMu.Unlock()
	if d <= 0 {
		return
	}
	finalRetryIntervalMu.Lock()
	defer finalRetryIntervalMu.Unlock()
	finalRetryInterval = d
}

func configureFinalRetryInterval(spec string, logger *slog.Logger) string {
	trimmed := strings.TrimSpace(spec)
	interval, err := time.ParseDuration(trimmed)
	if err != nil || interval <= 0 {
		if logger != nil {
			logger.Warn("[LLM Retry] Invalid final retry interval; using 30s", "interval", spec, "error", err)
		}
		ConfigureFinalRetryInterval(fallbackFinalRetryInterval)
		return fallbackFinalRetryInterval.String()
	}
	ConfigureFinalRetryInterval(interval)
	return trimmed
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
	defaultPolicyMu.Lock()
	defer defaultPolicyMu.Unlock()
	if len(intervalSpecs) == 0 {
		resetDefaultRetryIntervals()
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
		resetDefaultRetryIntervals()
		return
	}

	defaultRetryIntervalsMu.Lock()
	defaultRetryIntervals = intervals
	defaultRetryIntervalsMu.Unlock()
}

func resetDefaultRetryIntervals() {
	defaultRetryIntervalsMu.Lock()
	defaultRetryIntervals = []time.Duration{30 * time.Second, 2 * time.Minute}
	defaultRetryIntervalsMu.Unlock()
}

var perAttemptTimeoutNanos atomic.Int64

func init() {
	perAttemptTimeoutNanos.Store(int64(defaultPerAttemptTimeout))
}

func perAttemptTimeout() time.Duration {
	v := perAttemptTimeoutNanos.Load()
	if v <= 0 {
		return defaultPerAttemptTimeout
	}
	return time.Duration(v)
}

// SetPerAttemptTimeout configures the per-attempt API timeout used by the retry loops.
// It is safe to call concurrently.
func SetPerAttemptTimeout(d time.Duration) {
	defaultPolicyMu.Lock()
	defer defaultPolicyMu.Unlock()
	if d <= 0 {
		d = defaultPerAttemptTimeout
	}
	perAttemptTimeoutNanos.Store(int64(d))
}

type FeedbackProvider interface {
	Send(event, message string)
}

type activeProviderModelProvider interface {
	ActiveProviderAndModel() (string, string)
}

func activeProviderID(client ChatClient) string {
	if owner, ok := client.(interface{ ActiveRoute() ModelRoute }); ok {
		return owner.ActiveRoute().ProviderID
	}
	return ""
}

func activeProviderAndModel(client ChatClient, fallbackModel string) (string, string) {
	if scopedClient, ok := client.(activeProviderModelProvider); ok {
		return scopedClient.ActiveProviderAndModel()
	}
	return "", fallbackModel
}

func reportProviderError(client ChatClient, req openai.ChatCompletionRequest, operation string, err error, attempt int, retryable bool, timeout time.Duration, routes ...ModelRoute) {
	if err == nil {
		return
	}
	provider, model := activeProviderAndModel(client, req.Model)
	id := activeProviderID(client)
	if len(routes) > 0 {
		provider, model, id = routes[0].ProviderType, routes[0].Model, routes[0].ProviderID
	}
	ReportLLMHealthEvent(HealthEvent{
		Operation:         operation,
		ProviderID:        id,
		Provider:          provider,
		Model:             model,
		ErrorCategory:     ClassifyError(err),
		ErrorSummary:      safeAPIError(err),
		Attempt:           attempt,
		Retryable:         retryable,
		PerAttemptTimeout: timeout,
	})
}

func reportProviderSuccess(client ChatClient, req openai.ChatCompletionRequest, operation string, routes ...ModelRoute) {
	provider, model := activeProviderAndModel(client, req.Model)
	id := activeProviderID(client)
	if len(routes) > 0 {
		provider, model, id = routes[0].ProviderType, routes[0].Model, routes[0].ProviderID
	}
	ReportLLMHealthSuccess(HealthSuccess{
		Operation:  operation,
		ProviderID: id,
		Provider:   provider,
		Model:      model,
	})
}

func ExecuteWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (openai.ChatCompletionResponse, error) {
	return executeRetry(ctx, client, req, logger, broker, retryPolicyFor(client))
}

func ExecuteWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (openai.ChatCompletionResponse, error) {
	policy := retryPolicyFor(client)
	policy.Intervals = append([]time.Duration(nil), intervals...)
	policy.FinalInterval = finalInterval
	return executeRetry(ctx, client, req, logger, broker, policy)
}

func executeRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, policy RetryPolicy) (openai.ChatCompletionResponse, error) {
	callerCtx := ctx
	ctx, totalCancel, _ := openingContext(ctx, policy.TotalTimeout, "total budget")
	defer totalCancel()
	intervals, finalInterval := policy.Intervals, policy.FinalInterval
	timeout := policy.PerAttemptTimeout
	attempt := 0

	for {
		if ctx.Err() != nil {
			return openai.ChatCompletionResponse{}, context.Cause(ctx)
		}
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
		callCtx, observation := observeAttemptRoute(withAttemptParent(attemptCtx, callerCtx))
		resp, err := client.CreateChatCompletion(callCtx, req)
		route := observation.snapshot(client, req.Model)
		callElapsed := time.Since(callStart)
		attemptCtxErr := attemptCtx.Err()
		parentCtxErr := callerCtx.Err()
		attemptCancel()

		if parentCtxErr != nil {
			return openai.ChatCompletionResponse{}, context.Cause(callerCtx)
		}
		if err == nil {
			reportProviderSuccess(client, req, "chat_completion", route)
			if logger != nil {
				logger.Info("[LLM Retry] CreateChatCompletion succeeded",
					"attempt", attempt+1,
					"elapsed_ms", callElapsed.Milliseconds(),
					"model", req.Model,
				)
			}
			return resp, nil
		}

		if isOwnedTransportTimeout(context.Cause(ctx)) {
			err = context.Cause(ctx)
		} else if isOwnedTransportTimeout(context.Cause(attemptCtx)) {
			err = context.Cause(attemptCtx)
		} else if attemptCtxErr != nil && IsContextError(err) {
			err = &TransportTimeoutError{Stage: "attempt", After: timeout}
		}
		perAttemptContextError := isOwnedTransportTimeout(err)
		if IsContextError(err) && !perAttemptContextError {
			if logger != nil {
				logger.Debug("[LLM Retry] Context error, aborting without retry",
					"error", safeAPIError(err),
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return openai.ChatCompletionResponse{}, err
		}
		if IsContextLimitError(err) {
			return openai.ChatCompletionResponse{}, err
		}

		if !perAttemptContextError && IsNonRetryable(err) {
			reportProviderError(client, req, "chat_completion", err, attempt+1, false, timeout, route)
			if logger != nil {
				logger.Error("[LLM Retry] Non-retryable error, aborting",
					"error", safeAPIError(err),
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return openai.ChatCompletionResponse{}, err
		}

		attempt++
		reportProviderError(client, req, "chat_completion", err, attempt, true, timeout, route)
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Retry] Max retry attempts reached, aborting",
					"attempts", attempt,
					"error", safeAPIError(err),
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
		if providerChanged && GetRetryAfter(err) <= 0 {
			waitTime = 0
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
			return openai.ChatCompletionResponse{}, context.Cause(ctx)
		}
	}
}

func ExecuteStreamWithRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider) (CompletionStream, context.CancelFunc, error) {
	return executeStreamRetry(ctx, client, req, logger, broker, retryPolicyFor(client))
}

func ExecuteStreamWithCustomRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, intervals []time.Duration, finalInterval time.Duration) (CompletionStream, context.CancelFunc, error) {
	policy := retryPolicyFor(client)
	policy.Intervals = append([]time.Duration(nil), intervals...)
	policy.FinalInterval = finalInterval
	return executeStreamRetry(ctx, client, req, logger, broker, policy)
}

func executeStreamRetry(ctx context.Context, client ChatClient, req openai.ChatCompletionRequest, logger *slog.Logger, broker FeedbackProvider, policy RetryPolicy) (CompletionStream, context.CancelFunc, error) {
	callerCtx := ctx
	ctx, totalCancel, stopTotal := openingContext(ctx, policy.TotalTimeout, "stream opening budget")
	accepted := false
	defer func() {
		if !accepted {
			totalCancel()
		}
	}()
	intervals, finalInterval := policy.Intervals, policy.FinalInterval
	timeout := policy.PerAttemptTimeout
	if override, ok := ctx.Value(streamAttemptTimeoutKey{}).(time.Duration); ok && override > 0 {
		timeout = override
	}
	attempt := 0
	noCancel := func() {}

	for {
		if ctx.Err() != nil {
			return nil, noCancel, context.Cause(ctx)
		}
		attemptCtx, attemptCancel, stopAttempt := openingContext(ctx, timeout, "stream opening attempt")
		if hasThroughputDeadline(ctx) {
			// The throughput deadline already bounds this call by its progress.
			attemptCancel()
			stopAttempt()
			attemptCtx, attemptCancel = context.WithCancel(ctx)
		}
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
		callCtx, observation := observeAttemptRoute(withAttemptParent(attemptCtx, callerCtx))
		stream, err := client.CreateChatCompletionStream(callCtx, req)
		route := observation.snapshot(client, req.Model)
		callElapsed := time.Since(callStart)
		attemptCtxErr := attemptCtx.Err()
		parentCtxErr := callerCtx.Err()

		if parentCtxErr != nil {
			attemptCancel()
			if stream != nil {
				_ = stream.Close()
			}
			return nil, noCancel, context.Cause(callerCtx)
		}
		if err == nil {
			// Stream success is reported only after all terminal finish markers.
			if logger != nil {
				logger.Info("[LLM Stream Retry] CreateChatCompletionStream succeeded",
					"attempt", attempt+1,
					"elapsed_ms", callElapsed.Milliseconds(),
					"model", req.Model,
				)
			}
			stopAttempt()
			stopTotal()
			if attemptCtx.Err() != nil {
				attemptCancel()
				_ = stream.Close()
				return nil, noCancel, context.Cause(attemptCtx)
			}
			stream = observeCompletionStream(callCtx, stream, req,
				func() { reportProviderSuccess(client, req, "chat_completion_stream", route) },
				func(err error) {
					reportProviderError(client, req, "chat_completion_stream", err, attempt+1, false, timeout, route)
				})
			accepted = true
			return stream, func() { attemptCancel(); totalCancel() }, nil
		}
		attemptCancel()

		if isOwnedTransportTimeout(context.Cause(ctx)) {
			err = context.Cause(ctx)
		} else if isOwnedTransportTimeout(context.Cause(attemptCtx)) {
			err = context.Cause(attemptCtx)
		} else if attemptCtxErr != nil && IsContextError(err) {
			err = &TransportTimeoutError{Stage: "attempt", After: timeout}
		}
		perAttemptContextError := isOwnedTransportTimeout(err)
		if IsContextError(err) && !perAttemptContextError {
			if logger != nil {
				logger.Debug("[LLM Stream Retry] Context error, aborting without retry",
					"error", safeAPIError(err),
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return nil, noCancel, err
		}
		if IsContextLimitError(err) {
			return nil, noCancel, err
		}

		if !perAttemptContextError && IsNonRetryable(err) {
			reportProviderError(client, req, "chat_completion_stream", err, attempt+1, false, timeout, route)
			if logger != nil {
				logger.Error("[LLM Stream Retry] Non-retryable error, aborting",
					"error", safeAPIError(err),
					"category", ClassifyError(err),
					"model", req.Model,
					"messages", len(req.Messages),
					"tools", len(req.Tools),
				)
			}
			return nil, noCancel, err
		}

		attempt++
		reportProviderError(client, req, "chat_completion_stream", err, attempt, true, timeout, route)
		if attempt >= maxRetryAttempts {
			if logger != nil {
				logger.Error("[LLM Stream Retry] Max retry attempts reached, aborting",
					"attempts", attempt,
					"error", safeAPIError(err),
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
		if providerChanged && GetRetryAfter(err) <= 0 {
			waitTime = 0
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
			return nil, noCancel, context.Cause(ctx)
		}
	}
}

func selectRetryWaitTime(attempt int, intervals []time.Duration, finalInterval time.Duration, err error) time.Duration {
	wait := finalInterval
	if index := attempt - 1; index >= 0 && index < len(intervals) {
		wait = intervals[index]
	}
	if floor := GetRetryAfter(err); floor > wait {
		wait = floor
	}
	if wait <= 0 {
		return 0
	}
	// Add up to ten percent; a provider's Retry-After is always a lower bound.
	return wait + time.Duration(rand.Float64()*float64(wait)/10)
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
		return fmt.Sprintf("status %d: %s", apiErr.HTTPStatusCode, redactErrorText(apiErr.Message))
	}
	return redactProviderError(err)
}
