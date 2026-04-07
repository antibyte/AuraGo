package llm

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/sashabaranov/go-openai"
)

type ErrorCategory string

const (
	ErrCategoryContextCanceled    ErrorCategory = "context_canceled"
	ErrCategoryContextDeadline    ErrorCategory = "context_deadline"
	ErrCategoryNonRetryableConfig ErrorCategory = "non_retryable_config"
	ErrCategoryAuthError          ErrorCategory = "auth_error"
	ErrCategoryRateLimit          ErrorCategory = "rate_limit"
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
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return ErrCategoryAuthError
	case statusCode == http.StatusTooManyRequests:
		return ErrCategoryRateLimit
	case statusCode == http.StatusUnprocessableEntity:
		return ErrCategoryProviderValidation
	case statusCode >= 500 && statusCode < 600:
		return ErrCategoryTemporaryTransport
	case statusCode == 422:
		return ErrCategoryProviderValidation
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
	case ErrCategoryNonRetryableConfig, ErrCategoryAuthError:
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

func IsProbeInconclusive(err error) bool {
	return ClassifyError(err) == ErrCategoryProbeInconclusive
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

func isTransientByString(lowerErr string) bool {
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
