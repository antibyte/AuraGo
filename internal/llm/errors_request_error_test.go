package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

const noActivePlanBody = `{"error":"No active plan. Choose a plan to continue."}`

// go-openai returns a *RequestError (not a *APIError) when the error body is
// not an {"error":{...}} envelope. When the envelope decode half-succeeded it
// also nests an empty *APIError with no status code in RequestError.Err.
func TestClassifyError_RequestErrorStatus(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		want      ErrorCategory
		retryable bool
	}{
		{
			name: "402 payment required",
			err: &openai.RequestError{
				HTTPStatus: "402 Payment Required", HTTPStatusCode: http.StatusPaymentRequired,
				Body: []byte(noActivePlanBody),
			},
			want: ErrCategoryQuotaExceeded,
		},
		{
			name: "402 with empty nested APIError",
			err: &openai.RequestError{
				HTTPStatus: "402 Payment Required", HTTPStatusCode: http.StatusPaymentRequired,
				Err: &openai.APIError{}, Body: []byte(noActivePlanBody),
			},
			want: ErrCategoryQuotaExceeded,
		},
		{
			name: "402 wrapped",
			err: fmt.Errorf("chat completion: %w", &openai.RequestError{
				HTTPStatus: "402 Payment Required", HTTPStatusCode: http.StatusPaymentRequired,
				Err: &openai.APIError{}, Body: []byte(noActivePlanBody),
			}),
			want: ErrCategoryQuotaExceeded,
		},
		{
			// The status line itself contains "unauthorized", which the string
			// rules have always mapped to non_retryable_config; still not retried.
			name: "401 unauthorized status text",
			err:  &openai.RequestError{HTTPStatus: "401 Unauthorized", HTTPStatusCode: http.StatusUnauthorized, Body: []byte("denied")},
			want: ErrCategoryNonRetryableConfig,
		},
		{
			name: "401 without keyword text",
			err:  &openai.RequestError{HTTPStatus: "401", HTTPStatusCode: http.StatusUnauthorized, Body: []byte("denied")},
			want: ErrCategoryAuthError,
		},
		// Existing string classification keeps precedence over the status for
		// a RequestError, so these categories are unchanged by the 402 fix.
		{
			name: "403 with quota text",
			err:  &openai.RequestError{HTTPStatus: "403 Forbidden", HTTPStatusCode: http.StatusForbidden, Body: []byte("quota exceeded for this key")},
			want: ErrCategoryQuotaExceeded,
		},
		{
			name:      "500 with rate limit text",
			err:       &openai.RequestError{HTTPStatus: "500 Internal Server Error", HTTPStatusCode: http.StatusInternalServerError, Body: []byte("rate limit hit")},
			want:      ErrCategoryRateLimit,
			retryable: true,
		},
		{
			name:      "503 with too many requests text",
			err:       &openai.RequestError{HTTPStatus: "503 Service Unavailable", HTTPStatusCode: http.StatusServiceUnavailable, Body: []byte("too many requests")},
			want:      ErrCategoryRateLimit,
			retryable: true,
		},
		{
			name:      "400 with rate limit text",
			err:       &openai.RequestError{HTTPStatus: "400 Bad Request", HTTPStatusCode: http.StatusBadRequest, Body: []byte("rate limit reached")},
			want:      ErrCategoryRateLimit,
			retryable: true,
		},
		{
			name: "503 with model not found text",
			err:  &openai.RequestError{HTTPStatus: "503 Service Unavailable", HTTPStatusCode: http.StatusServiceUnavailable, Body: []byte("model not found")},
			want: ErrCategoryNonRetryableConfig,
		},
		{
			name: "503 with unauthorized text",
			err:  &openai.RequestError{HTTPStatus: "503 Service Unavailable", HTTPStatusCode: http.StatusServiceUnavailable, Body: []byte("unauthorized upstream")},
			want: ErrCategoryNonRetryableConfig,
		},
		{
			name:      "429 too many requests",
			err:       &openai.RequestError{HTTPStatus: "429 Too Many Requests", HTTPStatusCode: http.StatusTooManyRequests, Body: []byte("slow down")},
			want:      ErrCategoryRateLimit,
			retryable: true,
		},
		{
			name: "400 bad request",
			err:  &openai.RequestError{HTTPStatus: "400 Bad Request", HTTPStatusCode: http.StatusBadRequest, Body: []byte("bad")},
			want: ErrCategoryNonRetryableConfig,
		},
		{
			name:      "503 service unavailable",
			err:       &openai.RequestError{HTTPStatus: "503 Service Unavailable", HTTPStatusCode: http.StatusServiceUnavailable, Body: []byte("later")},
			want:      ErrCategoryTemporaryTransport,
			retryable: true,
		},
		{
			name: "429 with quota text",
			err: &openai.RequestError{
				HTTPStatus: "429 Too Many Requests", HTTPStatusCode: http.StatusTooManyRequests,
				Body: []byte("You exceeded your current quota, please check your plan and billing details."),
			},
			want: ErrCategoryQuotaExceeded,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyError(tc.err); got != tc.want {
				t.Errorf("ClassifyError() = %v, want %v", got, tc.want)
			}
			if got := IsRetryable(tc.err); got != tc.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tc.retryable)
			}
		})
	}
}

func TestClassifyError_APIErrorStatusUnchanged(t *testing.T) {
	if got := ClassifyError(&openai.APIError{HTTPStatusCode: http.StatusPaymentRequired, Message: "no plan"}); got != ErrCategoryQuotaExceeded {
		t.Errorf("APIError 402 = %v, want %v", got, ErrCategoryQuotaExceeded)
	}
	if got := ClassifyError(&openai.APIError{HTTPStatusCode: http.StatusConflict}); got != ErrCategoryNonRetryableConfig {
		t.Errorf("APIError 409 = %v, want %v", got, ErrCategoryNonRetryableConfig)
	}
	if got := ClassifyError(&openai.APIError{HTTPStatusCode: http.StatusBadGateway}); got != ErrCategoryTemporaryTransport {
		t.Errorf("APIError 502 = %v, want %v", got, ErrCategoryTemporaryTransport)
	}
	// An APIError without a status and no RequestError around it is unchanged.
	if got := ClassifyError(&openai.APIError{}); got != ErrCategoryProviderValidation {
		t.Errorf("APIError status 0 = %v, want %v", got, ErrCategoryProviderValidation)
	}
}

// Reproduces the production failure end to end through the real go-openai
// client: HTTP 402 with a string-valued "error" field.
func TestClassifyError_RealClientPaymentRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(noActivePlanBody))
	}))
	defer srv.Close()

	cfg := openai.DefaultConfig("test-key")
	cfg.BaseURL = srv.URL
	_, err := openai.NewClientWithConfig(cfg).CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: "m"})
	if err == nil {
		t.Fatal("expected an error for HTTP 402")
	}
	if got := ClassifyError(err); got != ErrCategoryQuotaExceeded {
		t.Fatalf("ClassifyError(real 402) = %v, want %v (err: %v)", got, ErrCategoryQuotaExceeded, err)
	}
	if !IsNonRetryable(err) {
		t.Error("IsNonRetryable(real 402) = false, want true")
	}
}

func TestExecuteWithRetry_RequestError402DoesNotRetry(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{&openai.RequestError{
			HTTPStatus: "402 Payment Required", HTTPStatusCode: http.StatusPaymentRequired,
			Err: &openai.APIError{}, Body: []byte(noActivePlanBody),
		}},
	}

	_, err := ExecuteWithCustomRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil, []time.Duration{time.Hour}, time.Hour)
	if err == nil {
		t.Fatal("ExecuteWithCustomRetry should return the payment error")
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1 (402 must not be retried)", client.callCount)
	}
}
