package llm

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai"
)

type mockRetryClient struct {
	callCount   int
	shouldRetry []error
}

func (m *mockRetryClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	m.callCount++
	if m.callCount <= len(m.shouldRetry) && m.shouldRetry[m.callCount-1] != nil {
		return openai.ChatCompletionResponse{}, m.shouldRetry[m.callCount-1]
	}
	return openai.ChatCompletionResponse{}, nil
}

func (m *mockRetryClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	m.callCount++
	if m.callCount <= len(m.shouldRetry) && m.shouldRetry[m.callCount-1] != nil {
		return nil, m.shouldRetry[m.callCount-1]
	}
	return nil, nil
}

type infiniteRetryClient struct {
	mockRetryClient
}

func (c *infiniteRetryClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.mockRetryClient.callCount++
	return openai.ChatCompletionResponse{}, errors.New("connection timeout")
}

func shortIntervals() []time.Duration {
	return []time.Duration{1 * time.Millisecond, 1 * time.Millisecond}
}

func TestExecuteWithRetry_Success(t *testing.T) {
	client := &mockRetryClient{}
	_, err := ExecuteWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err != nil {
		t.Errorf("ExecuteWithRetry returned error on success: %v", err)
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1", client.callCount)
	}
}

func TestExecuteWithRetry_ContextCanceled(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{context.Canceled},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ExecuteWithRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil)
	if !IsContextError(err) {
		t.Errorf("ExecuteWithRetry should return context error, got: %v", err)
	}
}

func TestExecuteWithRetry_NonRetryableError(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{errors.New("model not found")},
	}
	_, err := ExecuteWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err == nil {
		t.Error("ExecuteWithRetry should return error for non-retryable error")
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not retry non-retryable)", client.callCount)
	}
}

func TestExecuteWithRetry_QuotaExceededDoesNotRetry(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{&openai.APIError{
			HTTPStatusCode: http.StatusTooManyRequests,
			Message:        `geminiException - {"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_paid_tier_3_input_token_count, limit: 16000, model: gemma-4-31b","status":"RESOURCE_EXHAUSTED"}}`,
		}},
	}

	_, err := ExecuteWithCustomRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil, []time.Duration{time.Hour}, time.Hour)
	if err == nil {
		t.Fatal("ExecuteWithRetry should return quota error")
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not retry quota exhaustion)", client.callCount)
	}
}

func TestExecuteWithRetry_TransientRetries(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{
			errors.New("connection timeout"),
			errors.New("connection timeout"),
			nil,
		},
	}
	_, err := ExecuteWithCustomRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil, shortIntervals(), 1*time.Millisecond)
	if err != nil {
		t.Errorf("ExecuteWithRetry returned error after retries: %v", err)
	}
	if client.callCount != 3 {
		t.Errorf("callCount = %d, want 3", client.callCount)
	}
}

func TestSelectRetryWaitTimeUsesFirstIntervalForFirstRetry(t *testing.T) {
	intervals := []time.Duration{10 * time.Second, 2 * time.Minute}
	wait := selectRetryWaitTime(1, intervals, 5*time.Minute, errors.New("connection timeout"))
	if wait != 10*time.Second {
		t.Fatalf("first retry wait = %v, want 10s", wait)
	}
}

func TestSelectRetryWaitTimeUsesFinalIntervalAfterConfiguredIntervals(t *testing.T) {
	intervals := []time.Duration{10 * time.Second, 2 * time.Minute}
	wait := selectRetryWaitTime(3, intervals, 5*time.Minute, errors.New("connection timeout"))
	if wait != 5*time.Minute {
		t.Fatalf("third retry wait = %v, want 5m", wait)
	}
}

func TestConfigureDefaultRetryIntervalsParsesConfig(t *testing.T) {
	original := defaultRetryIntervalsCopy()
	t.Cleanup(func() {
		defaultRetryIntervalsMu.Lock()
		defaultRetryIntervals = original
		defaultRetryIntervalsMu.Unlock()
	})

	ConfigureDefaultRetryIntervals([]string{"10s", "bad", "2m"}, nil)
	updated := defaultRetryIntervalsCopy()
	want := []time.Duration{10 * time.Second, 2 * time.Minute}
	if len(updated) != len(want) {
		t.Fatalf("configured intervals len = %d, want %d (%v)", len(updated), len(want), updated)
	}
	for i := range want {
		if updated[i] != want[i] {
			t.Fatalf("configured interval %d = %v, want %v", i, updated[i], want[i])
		}
	}
}

func TestExecuteWithRetry_MaxRetryAttempts(t *testing.T) {
	client := &infiniteRetryClient{
		mockRetryClient: mockRetryClient{
			shouldRetry: make([]error, maxRetryAttempts+1),
		},
	}
	shortIntervalsList := []time.Duration{1 * time.Millisecond}
	_, err := ExecuteWithCustomRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil, shortIntervalsList, 1*time.Millisecond)
	if err == nil {
		t.Error("ExecuteWithRetry should return error after max retries")
	}
	if client.callCount != maxRetryAttempts {
		t.Errorf("callCount = %d, want %d", client.callCount, maxRetryAttempts)
	}
}

func TestExecuteWithRetry_ContextCancellationDuringWait(t *testing.T) {
	client := &infiniteRetryClient{
		mockRetryClient: mockRetryClient{
			shouldRetry: make([]error, maxRetryAttempts+1),
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the context after a short delay that will definitely fire
	// during waitForRetry (not during the per-attempt timeout).
	time.AfterFunc(2*time.Millisecond, cancel)

	// Use a wait time longer than the cancel delay so the context
	// cancels during the wait, not during the initial attempt timeout.
	_, err := ExecuteWithCustomRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil, shortIntervals(), 20*time.Millisecond)

	if !IsContextError(err) {
		t.Errorf("ExecuteWithRetry should return context error, got: %v", err)
	}
}

func TestExecuteStreamWithRetry_Success(t *testing.T) {
	client := &mockRetryClient{}
	_, err := ExecuteStreamWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err != nil {
		t.Errorf("ExecuteStreamWithRetry returned error on success: %v", err)
	}
}

func TestExecuteStreamWithRetry_TransientRetries(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{
			errors.New("connection timeout"),
			nil,
		},
	}
	_, err := ExecuteStreamWithCustomRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil, shortIntervals(), 1*time.Millisecond)
	if err != nil {
		t.Errorf("ExecuteStreamWithRetry returned error after retries: %v", err)
	}
	if client.callCount != 2 {
		t.Errorf("callCount = %d, want 2", client.callCount)
	}
}

func TestExecuteStreamWithRetry_NonRetryableError(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{errors.New("model not found")},
	}
	_, err := ExecuteStreamWithRetry(context.Background(), client, openai.ChatCompletionRequest{}, nil, nil)
	if err == nil {
		t.Error("ExecuteStreamWithRetry should return error for non-retryable error")
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1", client.callCount)
	}
}

func TestWaitForRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := waitForRetry(ctx, 10*time.Second)
	if result {
		t.Error("waitForRetry should return false when context is cancelled")
	}
}

func TestWaitForRetry_Timeout(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	result := waitForRetry(ctx, 100*time.Millisecond)
	elapsed := time.Since(start)

	if !result {
		t.Error("waitForRetry should return true when timer fires")
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("waitForRetry did not wait for full duration: %v", elapsed)
	}
}

func TestIsContextError_DeadlineExceeded(t *testing.T) {
	if !IsContextError(context.DeadlineExceeded) {
		t.Error("IsContextError should return true for context.DeadlineExceeded")
	}
}

func TestIsContextError_Canceled(t *testing.T) {
	if !IsContextError(context.Canceled) {
		t.Error("IsContextError should return true for context.Canceled")
	}
}

func TestExecuteWithRetry_ContextDeadlineExceeded(t *testing.T) {
	client := &mockRetryClient{
		shouldRetry: []error{context.DeadlineExceeded},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := ExecuteWithRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil)
	if !IsContextError(err) {
		t.Errorf("ExecuteWithRetry should return context error for DeadlineExceeded, got: %v", err)
	}
	if client.callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not retry context error)", client.callCount)
	}
}
