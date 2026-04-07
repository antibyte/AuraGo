package llm

import (
	"context"
	"errors"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ExecuteWithCustomRetry(ctx, client, openai.ChatCompletionRequest{}, nil, nil, shortIntervals(), 10*time.Millisecond)
	elapsed := time.Since(start)

	if !IsContextError(err) {
		t.Errorf("ExecuteWithRetry should return context error, got: %v", err)
	}
	if elapsed >= 10*time.Millisecond {
		t.Errorf("ExecuteWithRetry did not respect context cancellation during wait, elapsed=%v", elapsed)
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
