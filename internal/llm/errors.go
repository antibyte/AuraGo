package llm

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

type ErrorCategory string

const (
	ErrCategoryContextCanceled    ErrorCategory = "context_canceled"
	ErrCategoryContextDeadline    ErrorCategory = "context_deadline"
	ErrCategoryNonRetryableConfig ErrorCategory = "non_retryable_config"
	ErrCategoryAuthError          ErrorCategory = "auth_error"
	ErrCategoryRateLimit          ErrorCategory = "rate_limit"
	ErrCategoryQuotaExceeded      ErrorCategory = "quota_exceeded"
	ErrCategoryTemporaryTransport ErrorCategory = "temporary_transport"
	ErrCategoryProviderValidation ErrorCategory = "provider_validation"
	ErrCategoryProbeInconclusive  ErrorCategory = "probe_inconclusive"
)

type LLMError struct {
	Category   ErrorCategory
	Message    string
	InnerError error
}

func (e *LLMError) Error() string {
	if e.InnerError != nil {
		return e.Message + ": " + e.InnerError.Error()
	}
	return e.Message
}

func (e *LLMError) Unwrap() error {
	return e.InnerError
}

func (e *LLMError) Is(target error) bool {
	var llmErr *LLMError
	if errors.As(target, &llmErr) {
		return e.Category == llmErr.Category
	}
	return false
}

func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.Canceled) {
		return ErrCategoryContextCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrCategoryContextDeadline
	}

	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return classifyHTTPError(apiErr.HTTPStatusCode, err.Error())
	}

	if strings.Contains(err.Error(), "context canceled") || strings.Contains(err.Error(), "context deadline exceeded") {
		return ErrCategoryContextDeadline
	}

	msg := strings.ToLower(err.Error())
	if isQuotaExceededByString(msg) {
		return ErrCategoryQuotaExceeded
	}
	if isNonRetryableByString(msg) {
		return ErrCategoryNonRetryableConfig
	}

	if isTransientByString(msg) {
		if strings.Contains(msg, "rate limit") || strings.Contains(msg, "429") || strings.Contains(msg, "too many requests") {
			return ErrCategoryRateLimit
		}
		return ErrCategoryTemporaryTransport
	}

	return ErrCategoryProviderValidation
}

func classifyHTTPError(statusCode int, errMsg string) ErrorCategory {
	if statusCode == http.StatusTooManyRequests && isQuotaExceededByString(strings.ToLower(errMsg)) {
		return ErrCategoryQuotaExceeded
	}
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ErrCategoryAuthError
	case statusCode == http.StatusTooManyRequests:
		return ErrCategoryRateLimit
	case statusCode == http.StatusBadRequest:
		// 400 Bad Request is a structural error in the request payload (e.g. orphaned
		// tool_call_id after history compression). Retrying the same payload is futile.
		// The agent loop's recoverFrom422WithPolicy handles recovery by trimming messages.
		return ErrCategoryNonRetryableConfig
	case statusCode == http.StatusUnprocessableEntity:
		// 422 Unprocessable Entity is also a structural payload error; same rationale as 400.
		return ErrCategoryNonRetryableConfig
	case statusCode == 529:
		// Anthropic "overloaded" error - explicitly retryable
		return ErrCategoryTemporaryTransport
	case statusCode == http.StatusNotFound:
		lowerErr := strings.ToLower(errMsg)
		if strings.Contains(lowerErr, "<!doctype html") || strings.Contains(lowerErr, "<html") {
			return ErrCategoryTemporaryTransport
		}
		return ErrCategoryProviderValidation
	case statusCode >= 500 && statusCode < 600:
		return ErrCategoryTemporaryTransport
	default:
		return ErrCategoryProviderValidation
	}
}

func IsContextError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	if strings.Contains(msg, "context canceled") || strings.Contains(msg, "context deadline exceeded") {
		return true
	}
	return false
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	cat := ClassifyError(err)
	switch cat {
	case ErrCategoryContextCanceled, ErrCategoryContextDeadline:
		return false
	case ErrCategoryNonRetryableConfig, ErrCategoryAuthError, ErrCategoryQuotaExceeded:
		return false
	case ErrCategoryRateLimit, ErrCategoryTemporaryTransport, ErrCategoryProviderValidation:
		return true
	default:
		return false
	}
}

func IsNonRetryable(err error) bool {
	return !IsRetryable(err)
}

func IsRateLimit(err error) bool {
	return ClassifyError(err) == ErrCategoryRateLimit
}

func IsAuthError(err error) bool {
	return ClassifyError(err) == ErrCategoryAuthError
}

func IsQuotaExceeded(err error) bool {
	return ClassifyError(err) == ErrCategoryQuotaExceeded
}

func IsProbeInconclusive(err error) bool {
	return ClassifyError(err) == ErrCategoryProbeInconclusive
}

// IsImageNotSupportedError returns true if the error indicates the model
// does not support image/vision input.
func IsImageNotSupportedError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "does not support image") ||
		strings.Contains(lower, "does not support multimodal") ||
		strings.Contains(lower, "image input") ||
		strings.Contains(lower, "not support image") ||
		strings.Contains(lower, "image is not supported") ||
		(strings.Contains(lower, "does not support") && strings.Contains(lower, "image"))
}

func WrapError(category ErrorCategory, err error, message string) *LLMError {
	return &LLMError{
		Category:   category,
		Message:    message,
		InnerError: err,
	}
}

func NewLLMError(category ErrorCategory, message string) *LLMError {
	return &LLMError{
		Category: category,
		Message:  message,
	}
}

func isNonRetryableByString(lowerErr string) bool {
	return strings.Contains(lowerErr, "model not found") ||
		strings.Contains(lowerErr, "' not found") ||
		strings.Contains(lowerErr, "unknown model") ||
		strings.Contains(lowerErr, "unknown parameter") ||
		strings.Contains(lowerErr, "invalid model") ||
		strings.Contains(lowerErr, "does not support") ||
		strings.Contains(lowerErr, "not supported") ||
		strings.Contains(lowerErr, "unauthorized") ||
		strings.Contains(lowerErr, "permission denied") ||
		strings.Contains(lowerErr, "access denied")
}

func isQuotaExceededByString(lowerErr string) bool {
	return strings.Contains(lowerErr, "quota exceeded") ||
		strings.Contains(lowerErr, "exceeded your current quota") ||
		strings.Contains(lowerErr, "resource_exhausted") ||
		strings.Contains(lowerErr, "billing details") ||
		strings.Contains(lowerErr, "quotaMetric") ||
		strings.Contains(lowerErr, "quotametric") ||
		strings.Contains(lowerErr, "input_token_count")
}

func isTransientByString(lowerErr string) bool {
	return strings.Contains(lowerErr, "too many requests") ||
		strings.Contains(lowerErr, "rate limit") ||
		strings.Contains(lowerErr, "timeout") ||
		strings.Contains(lowerErr, "deadline_exceeded") ||
		strings.Contains(lowerErr, "deadline exceeded") ||
		strings.Contains(lowerErr, "connection") ||
		strings.Contains(lowerErr, "503") ||
		strings.Contains(lowerErr, "502") ||
		strings.Contains(lowerErr, "504") ||
		strings.Contains(lowerErr, "529") ||
		strings.Contains(lowerErr, "overloaded") ||
		strings.Contains(lowerErr, "server error") ||
		strings.Contains(lowerErr, "eof")
}

type RateLimitError struct {
	*LLMError
	RetryAfterSeconds int
}

func (e *RateLimitError) RetryAfter() time.Duration {
	if e.RetryAfterSeconds > 0 {
		return time.Duration(e.RetryAfterSeconds) * time.Second
	}
	return 0
}

func GetRetryAfter(err error) time.Duration {
	var rlErr *RateLimitError
	if errors.As(err, &rlErr) {
		return rlErr.RetryAfter()
	}
	return 0
}
