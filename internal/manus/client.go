// Package manus provides a policy-gated client for the Manus v2 API.
package manus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

const (
	// DefaultBaseURL is the fixed production endpoint for the Manus v2 API.
	DefaultBaseURL        = "https://api.manus.ai"
	defaultMaxResultBytes = 262144
)

// ClientConfig controls transport limits. BaseURL is intended for tests; AuraGo
// production callers always use DefaultBaseURL.
type ClientConfig struct {
	BaseURL        string
	Timeout        time.Duration
	MaxResultBytes int64
	HTTPClient     *http.Client
	FileHTTPClient *http.Client
	RetryBaseDelay time.Duration
}

// Client calls the Manus v2 REST API.
type Client struct {
	apiKey         string
	baseURL        *url.URL
	httpClient     *http.Client
	fileHTTPClient *http.Client
	maxResultBytes int64
	retryBaseDelay time.Duration
}

// Credits is the authoritative balance returned by Manus.
type Credits struct {
	OK        bool          `json:"ok"`
	RequestID string        `json:"request_id"`
	Data      CreditBalance `json:"data"`
}

// CreditBalance describes the Manus wallet and refresh schedule.
type CreditBalance struct {
	TotalCredits      int    `json:"total_credits"`
	FreeCredits       int    `json:"free_credits"`
	PeriodicCredits   int    `json:"periodic_credits"`
	AddonCredits      int    `json:"addon_credits"`
	ProMonthlyCredits int    `json:"pro_monthly_credits"`
	EventCredits      int    `json:"event_credits"`
	RefreshCredits    int    `json:"refresh_credits"`
	MaxRefreshCredits int    `json:"max_refresh_credits"`
	NextRefreshTime   int64  `json:"next_refresh_time"`
	RefreshInterval   string `json:"refresh_interval"`
}

type apiErrorEnvelope struct {
	OK        *bool  `json:"ok"`
	RequestID string `json:"request_id"`
	Error     struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// NewClient creates an authenticated Manus v2 client.
func NewClient(apiKey string, cfg ClientConfig) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("manus API key is required")
	}
	security.RegisterSensitive(apiKey)

	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = DefaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Manus base URL")
	}

	maxBytes := cfg.MaxResultBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxResultBytes
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	fileClient := cfg.FileHTTPClient
	if fileClient == nil {
		fileClient = &http.Client{Timeout: client.Timeout}
	}
	fileClient = cloneSecureFileClient(fileClient)
	retryDelay := cfg.RetryBaseDelay
	if retryDelay <= 0 {
		retryDelay = 250 * time.Millisecond
	}

	return &Client{
		apiKey:         apiKey,
		baseURL:        parsed,
		httpClient:     client,
		fileHTTPClient: fileClient,
		maxResultBytes: maxBytes,
		retryBaseDelay: retryDelay,
	}, nil
}

// AvailableCredits returns the current Manus credit balance without creating a task.
func (c *Client) AvailableCredits(ctx context.Context) (Credits, error) {
	var result Credits
	if err := c.doJSON(ctx, http.MethodGet, "/v2/usage.availableCredits", nil, nil, &result); err != nil {
		return Credits{}, err
	}
	return result, nil
}

// CreateTask starts an asynchronous private Manus task.
func (c *Client) CreateTask(ctx context.Context, in CreateTaskRequest) (CreateTaskResult, error) {
	body := struct {
		Message                Message        `json:"message"`
		ProjectID              string         `json:"project_id,omitempty"`
		Locale                 string         `json:"locale,omitempty"`
		InteractiveMode        bool           `json:"interactive_mode"`
		HideInTaskList         bool           `json:"hide_in_task_list"`
		ShareVisibility        string         `json:"share_visibility"`
		AgentProfile           string         `json:"agent_profile,omitempty"`
		Title                  string         `json:"title,omitempty"`
		StructuredOutputSchema map[string]any `json:"structured_output_schema,omitempty"`
	}{
		Message: Message{
			Content:      in.Content,
			Connectors:   nonNilStrings(in.Connectors),
			EnableSkills: in.EnableSkills,
			ForceSkills:  in.ForceSkills,
		},
		ProjectID:              in.ProjectID,
		Locale:                 in.Locale,
		InteractiveMode:        in.InteractiveMode,
		HideInTaskList:         false,
		ShareVisibility:        "private",
		AgentProfile:           in.AgentProfile,
		Title:                  in.Title,
		StructuredOutputSchema: in.StructuredOutputSchema,
	}
	var result CreateTaskResult
	if err := c.doJSON(ctx, http.MethodPost, "/v2/task.create", nil, body, &result); err != nil {
		return CreateTaskResult{}, err
	}
	return result, nil
}

// GetTask retrieves metadata for one task.
func (c *Client) GetTask(ctx context.Context, taskID string) (Task, error) {
	query := url.Values{"task_id": {strings.TrimSpace(taskID)}}
	var response struct {
		Task Task `json:"task"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/v2/task.detail", query, nil, &response); err != nil {
		return Task{}, err
	}
	return response.Task, nil
}

// ListMessages returns the safe non-verbose event stream for one task.
func (c *Client) ListMessages(ctx context.Context, opts ListMessagesOptions) (MessagePage, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	order := opts.Order
	if order != "desc" {
		order = "asc"
	}
	query := url.Values{
		"task_id": {strings.TrimSpace(opts.TaskID)},
		"limit":   {strconv.Itoa(limit)},
		"order":   {order},
		"verbose": {"false"},
	}
	if strings.TrimSpace(opts.Cursor) != "" {
		query.Set("cursor", strings.TrimSpace(opts.Cursor))
	}
	var result MessagePage
	if err := c.doJSON(ctx, http.MethodGet, "/v2/task.listMessages", query, nil, &result); err != nil {
		return MessagePage{}, err
	}
	return result, nil
}

// StopTask stops a running task. The call is deliberately never retried.
func (c *Client) StopTask(ctx context.Context, taskID string) error {
	var result map[string]any
	return c.doJSON(ctx, http.MethodPost, "/v2/task.stop", nil, map[string]string{"task_id": strings.TrimSpace(taskID)}, &result)
}

// SendMessage continues an existing task. The call is deliberately never retried.
func (c *Client) SendMessage(ctx context.Context, in SendMessageRequest) (SendMessageResult, error) {
	body := struct {
		TaskID                 string         `json:"task_id"`
		Message                Message        `json:"message"`
		AgentProfile           string         `json:"agent_profile,omitempty"`
		StructuredOutputSchema map[string]any `json:"structured_output_schema,omitempty"`
		ClearConnectors        bool           `json:"clear_connectors"`
	}{
		TaskID: strings.TrimSpace(in.TaskID),
		Message: Message{
			Content:      in.Content,
			Connectors:   nonNilStrings(in.Connectors),
			EnableSkills: in.EnableSkills,
			ForceSkills:  in.ForceSkills,
		},
		AgentProfile:           in.AgentProfile,
		StructuredOutputSchema: in.StructuredOutputSchema,
		ClearConnectors:        in.ClearConnectors,
	}
	var result SendMessageResult
	if err := c.doJSON(ctx, http.MethodPost, "/v2/task.sendMessage", nil, body, &result); err != nil {
		return SendMessageResult{}, err
	}
	return result, nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: path})
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Manus request: %w", err)
		}
	}

	maxAttempts := 1
	if method == http.MethodGet {
		maxAttempts = 3
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(encoded))
		if err != nil {
			return fmt.Errorf("create Manus request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("x-manus-api-key", c.apiKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			callErr := fmt.Errorf("call Manus API: %w", err)
			if method != http.MethodGet {
				return &OutcomeUnknownError{Operation: path, Err: callErr}
			}
			return callErr
		}
		payload, readErr := readBounded(resp.Body, c.maxResultBytes)
		_ = resp.Body.Close()
		if readErr != nil {
			if method != http.MethodGet {
				return &OutcomeUnknownError{Operation: path, Err: readErr}
			}
			return readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests && method == http.MethodGet && attempt+1 < maxAttempts {
			if err := c.waitForRetry(ctx, resp.Header.Get("Retry-After"), attempt); err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := decodeAPIError(resp.StatusCode, payload)
			if method != http.MethodGet && resp.StatusCode >= http.StatusInternalServerError {
				return &OutcomeUnknownError{Operation: path, Err: apiErr}
			}
			return apiErr
		}
		var envelope apiErrorEnvelope
		if err := json.Unmarshal(payload, &envelope); err == nil && envelope.OK != nil && !*envelope.OK {
			return decodeAPIError(resp.StatusCode, payload)
		}
		if err := json.Unmarshal(payload, out); err != nil {
			decodeErr := fmt.Errorf("decode Manus response: %w", err)
			if method != http.MethodGet {
				return &OutcomeUnknownError{Operation: path, Err: decodeErr}
			}
			return decodeErr
		}
		return nil
	}
	return errors.New("Manus request exhausted retries")
}

func readBounded(body io.Reader, maxBytes int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Manus response: %w", err)
	}
	if int64(len(payload)) > maxBytes {
		return nil, fmt.Errorf("Manus response exceeds %d bytes", maxBytes)
	}
	return payload, nil
}

func decodeAPIError(status int, payload []byte) error {
	var apiErr apiErrorEnvelope
	if json.Unmarshal(payload, &apiErr) == nil && apiErr.Error.Message != "" {
		return fmt.Errorf("Manus API %s: %s", apiErr.Error.Code, security.Scrub(apiErr.Error.Message))
	}
	if apiErr.OK != nil && !*apiErr.OK {
		return fmt.Errorf("Manus API rejected the request (HTTP %d)", status)
	}
	return fmt.Errorf("Manus API returned HTTP %d", status)
}

func (c *Client) waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := c.retryBaseDelay * time.Duration(1<<attempt)
	if seconds, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	} else if jitterWindow := delay / 4; jitterWindow > 0 {
		delay += time.Duration(rand.Int64N(int64(jitterWindow)))
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for Manus retry: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
