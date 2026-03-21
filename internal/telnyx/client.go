package telnyx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultBaseURL  = "https://api.telnyx.com/v2"
	maxRetries      = 3
	maxResponseBody = 64 * 1024 // 64 KB
)

// Client wraps the Telnyx v2 REST API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	logger     *slog.Logger
}

// NewClient creates a Telnyx API client.
func NewClient(apiKey string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: defaultBaseURL,
		logger:  logger,
	}
}

// do executes an authenticated API request with retry logic.
func (c *Client) do(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff):
			}
			// Re-create body reader for retry
			if body != nil {
				b, _ := json.Marshal(body)
				reqBody = bytes.NewReader(b)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		// Retry on 429 or 5xx
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
			c.logger.Warn("Telnyx API retryable error", "status", resp.StatusCode, "attempt", attempt+1)
			continue
		}

		if resp.StatusCode >= 400 {
			var errResp ErrorResponse
			if json.Unmarshal(respBody, &errResp) == nil && len(errResp.Errors) > 0 {
				e := errResp.Errors[0]
				return nil, resp.StatusCode, fmt.Errorf("API error %s: %s - %s", e.Code, e.Title, e.Detail)
			}
			return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		return respBody, resp.StatusCode, nil
	}

	return nil, 0, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body interface{}) ([]byte, int, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// ── Management Endpoints ───────────────────────────────────────────

// ListPhoneNumbers returns phone numbers on the account.
func (c *Client) ListPhoneNumbers(ctx context.Context) (*PhoneNumbersResponse, error) {
	data, _, err := c.get(ctx, "/phone_numbers?page[size]=50")
	if err != nil {
		return nil, fmt.Errorf("list phone numbers: %w", err)
	}
	var resp PhoneNumbersResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode phone numbers: %w", err)
	}
	return &resp, nil
}

// GetBalance returns the account balance.
func (c *Client) GetBalance(ctx context.Context) (*BalanceResponse, error) {
	data, _, err := c.get(ctx, "/balance")
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}
	var resp BalanceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode balance: %w", err)
	}
	return &resp, nil
}

// ListMessages returns recent messages with pagination.
func (c *Client) ListMessages(ctx context.Context, page, pageSize int) (*MessagesListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	path := fmt.Sprintf("/messages?page[number]=%d&page[size]=%d", page, pageSize)
	data, _, err := c.get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	var resp MessagesListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}
	return &resp, nil
}
