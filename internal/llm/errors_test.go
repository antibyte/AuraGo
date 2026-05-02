package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestClassifyError_ContextCanceled(t *testing.T) {
	err := context.Canceled
	cat := ClassifyError(err)
	if cat != ErrCategoryContextCanceled {
		t.Errorf("ClassifyError(context.Canceled) = %v, want %v", cat, ErrCategoryContextCanceled)
	}
}

func TestClassifyError_ContextDeadline(t *testing.T) {
	err := context.DeadlineExceeded
	cat := ClassifyError(err)
	if cat != ErrCategoryContextDeadline {
		t.Errorf("ClassifyError(context.DeadlineExceeded) = %v, want %v", cat, ErrCategoryContextDeadline)
	}
}

func TestClassifyError_AuthError(t *testing.T) {
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusUnauthorized}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryAuthError {
		t.Errorf("ClassifyError(401) = %v, want %v", cat, ErrCategoryAuthError)
	}

	apiErr = &openai.APIError{HTTPStatusCode: http.StatusForbidden}
	cat = ClassifyError(apiErr)
	if cat != ErrCategoryAuthError {
		t.Errorf("ClassifyError(403) = %v, want %v", cat, ErrCategoryAuthError)
	}
}

func TestClassifyError_RateLimit(t *testing.T) {
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusTooManyRequests}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryRateLimit {
		t.Errorf("ClassifyError(429) = %v, want %v", cat, ErrCategoryRateLimit)
	}
}

func TestClassifyError_QuotaExceededNonRetryable(t *testing.T) {
	apiErr := &openai.APIError{
		HTTPStatusCode: http.StatusTooManyRequests,
		Message:        `geminiException - {"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_paid_tier_3_input_token_count, limit: 16000, model: gemma-4-31b","status":"RESOURCE_EXHAUSTED"}}`,
	}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryQuotaExceeded {
		t.Errorf("ClassifyError(Gemini quota 429) = %v, want %v", cat, ErrCategoryQuotaExceeded)
	}
	if IsRetryable(apiErr) {
		t.Error("IsRetryable(Gemini quota 429) = true, want false")
	}
	if !IsQuotaExceeded(apiErr) {
		t.Error("IsQuotaExceeded(Gemini quota 429) = false, want true")
	}
}

func TestClassifyError_TemporaryTransport(t *testing.T) {
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusServiceUnavailable}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryTemporaryTransport {
		t.Errorf("ClassifyError(503) = %v, want %v", cat, ErrCategoryTemporaryTransport)
	}

	apiErr = &openai.APIError{HTTPStatusCode: http.StatusBadGateway}
	cat = ClassifyError(apiErr)
	if cat != ErrCategoryTemporaryTransport {
		t.Errorf("ClassifyError(502) = %v, want %v", cat, ErrCategoryTemporaryTransport)
	}
}

func TestClassifyError_ProviderValidation(t *testing.T) {
	// Unrecognised 4xx codes fall through to ErrCategoryProviderValidation.
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusConflict} // 409
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryProviderValidation {
		t.Errorf("ClassifyError(409) = %v, want %v", cat, ErrCategoryProviderValidation)
	}
}

func TestClassifyError_BadRequestNonRetryable(t *testing.T) {
	// HTTP 400 (tool_call_id not found, etc.) must NOT be retried — the agent loop
	// handles recovery via recoverFrom422WithPolicy.
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusBadRequest}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError(400) = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}
	if IsRetryable(apiErr) {
		t.Error("IsRetryable(400 APIError) = true, want false")
	}
}

func TestClassifyError_UnprocessableEntityNonRetryable(t *testing.T) {
	// HTTP 422 is a structural payload error; same as 400 — no point retrying same payload.
	apiErr := &openai.APIError{HTTPStatusCode: http.StatusUnprocessableEntity}
	cat := ClassifyError(apiErr)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError(422) = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}
	if IsRetryable(apiErr) {
		t.Error("IsRetryable(422 APIError) = true, want false")
	}
}

func TestClassifyError_NonRetryable(t *testing.T) {
	err := errors.New("model not found")
	cat := ClassifyError(err)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError(model not found) = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}

	err = errors.New("unknown model")
	cat = ClassifyError(err)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError(unknown model) = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}
}

func TestIsContextError(t *testing.T) {
	if !IsContextError(context.Canceled) {
		t.Error("IsContextError(context.Canceled) = false, want true")
	}
	if !IsContextError(context.DeadlineExceeded) {
		t.Error("IsContextError(context.DeadlineExceeded) = false, want true")
	}
	if !IsContextError(errors.New("context canceled")) {
		t.Error("IsContextError('context canceled') = false, want true")
	}
	if IsContextError(errors.New("other error")) {
		t.Error("IsContextError('other error') = true, want false")
	}
	if IsContextError(nil) {
		t.Error("IsContextError(nil) = true, want false")
	}
}

func TestIsRetryable(t *testing.T) {
	if IsRetryable(context.Canceled) {
		t.Error("IsRetryable(context.Canceled) = true, want false")
	}
	if IsRetryable(context.DeadlineExceeded) {
		t.Error("IsRetryable(context.DeadlineExceeded) = true, want false")
	}
	if IsRetryable(errors.New("model not found")) {
		t.Error("IsRetryable(model not found) = true, want false")
	}
	if !IsRetryable(errors.New("connection timeout")) {
		t.Error("IsRetryable(connection timeout) = false, want true")
	}
	if !IsRetryable(&openai.APIError{HTTPStatusCode: http.StatusTooManyRequests}) {
		t.Error("IsRetryable(429) = false, want true")
	}
}

func TestIsNonRetryable(t *testing.T) {
	if !IsNonRetryable(errors.New("model not found")) {
		t.Error("IsNonRetryable(model not found) = false, want true")
	}
	if !IsNonRetryable(&openai.APIError{HTTPStatusCode: http.StatusUnauthorized}) {
		t.Error("IsNonRetryable(401) = false, want true")
	}
	if IsNonRetryable(errors.New("connection timeout")) {
		t.Error("IsNonRetryable(connection timeout) = true, want false")
	}
}

func TestIsRateLimit(t *testing.T) {
	if !IsRateLimit(&openai.APIError{HTTPStatusCode: http.StatusTooManyRequests}) {
		t.Error("IsRateLimit(429) = false, want true")
	}
	if IsRateLimit(errors.New("connection timeout")) {
		t.Error("IsRateLimit(connection timeout) = true, want false")
	}
}

func TestIsAuthError(t *testing.T) {
	if !IsAuthError(&openai.APIError{HTTPStatusCode: http.StatusUnauthorized}) {
		t.Error("IsAuthError(401) = false, want true")
	}
	if !IsAuthError(&openai.APIError{HTTPStatusCode: http.StatusForbidden}) {
		t.Error("IsAuthError(403) = false, want true")
	}
	if IsAuthError(errors.New("connection timeout")) {
		t.Error("IsAuthError(connection timeout) = true, want false")
	}
}

func TestLLMError_Wrap(t *testing.T) {
	inner := errors.New("inner error")
	wrapped := WrapError(ErrCategoryTemporaryTransport, inner, "transport error")

	if wrapped.Category != ErrCategoryTemporaryTransport {
		t.Errorf("wrapped.Category = %v, want %v", wrapped.Category, ErrCategoryTemporaryTransport)
	}
	if wrapped.Message != "transport error" {
		t.Errorf("wrapped.Message = %v, want %v", wrapped.Message, "transport error")
	}
	if wrapped.Unwrap() != inner {
		t.Errorf("wrapped.Unwrap() = %v, want %v", wrapped.Unwrap(), inner)
	}
	if wrapped.Error() != "transport error: inner error" {
		t.Errorf("wrapped.Error() = %v, want %v", wrapped.Error(), "transport error: inner error")
	}
}

func TestLLMError_Is(t *testing.T) {
	err1 := NewLLMError(ErrCategoryRateLimit, "rate limit")
	err2 := NewLLMError(ErrCategoryRateLimit, "another rate limit")
	err3 := NewLLMError(ErrCategoryAuthError, "auth error")

	if !errors.Is(err1, err2) {
		t.Error("errors.Is(err1, err2) = false, want true (same category)")
	}
	if errors.Is(err1, err3) {
		t.Error("errors.Is(err1, err3) = true, want false (different category)")
	}
}

func TestClassifyError_StringBasedFallback(t *testing.T) {
	err := errors.New("connection reset by peer")
	cat := ClassifyError(err)
	if cat != ErrCategoryTemporaryTransport {
		t.Errorf("ClassifyError('connection reset by peer') = %v, want %v", cat, ErrCategoryTemporaryTransport)
	}

	err = errors.New("EOF")
	cat = ClassifyError(err)
	if cat != ErrCategoryTemporaryTransport {
		t.Errorf("ClassifyError('EOF') = %v, want %v", cat, ErrCategoryTemporaryTransport)
	}
}

func TestClassifyError_StringBasedUnauthorized(t *testing.T) {
	err := errors.New("unauthorized: invalid token")
	cat := ClassifyError(err)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError('unauthorized: invalid token') = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}

	err = errors.New("permission denied for this operation")
	cat = ClassifyError(err)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError('permission denied for this operation') = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}

	err = errors.New("access denied: insufficient privileges")
	cat = ClassifyError(err)
	if cat != ErrCategoryNonRetryableConfig {
		t.Errorf("ClassifyError('access denied: insufficient privileges') = %v, want %v", cat, ErrCategoryNonRetryableConfig)
	}
}

func TestClassifyError_NoFalsePositive401InOtherContext(t *testing.T) {
	err := errors.New("answer 401 questions about this topic")
	cat := ClassifyError(err)
	if cat == ErrCategoryAuthError {
		t.Errorf("ClassifyError('answer 401 questions') = %v, should NOT be ErrCategoryAuthError (false positive)", cat)
	}

	err = errors.New("received 403 bytes from server")
	cat = ClassifyError(err)
	if cat == ErrCategoryAuthError {
		t.Errorf("ClassifyError('received 403 bytes') = %v, should NOT be ErrCategoryAuthError (false positive)", cat)
	}
}
