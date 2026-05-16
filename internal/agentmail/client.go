package agentmail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultMaxResponseBytes int64 = 8 << 20

type ClientConfig struct {
	BaseURL          string
	APIKey           string
	HTTPClient       *http.Client
	MaxResponseBytes int64
	MaxRetries       int
	RetrySleep       func(time.Duration)
}

type Client struct {
	baseURL          *url.URL
	apiKey           string
	httpClient       *http.Client
	maxResponseBytes int64
	maxRetries       int
	retrySleep       func(time.Duration)
}

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("agentmail api error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("agentmail api error (%d): %s", e.StatusCode, e.Body)
}

func NewClient(cfg ClientConfig) (*Client, error) {
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = DefaultBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid AgentMail base URL %q", base)
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("agentmail api key is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	maxBytes := cfg.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxResponseBytes
	}
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries == 0 {
		maxRetries = 2
	}
	retrySleep := cfg.RetrySleep
	if retrySleep == nil {
		retrySleep = time.Sleep
	}

	return &Client{
		baseURL:          parsed,
		apiKey:           apiKey,
		httpClient:       httpClient,
		maxResponseBytes: maxBytes,
		maxRetries:       maxRetries,
		retrySleep:       retrySleep,
	}, nil
}

func (c *Client) endpoint(path string, q url.Values) string {
	u := *c.baseURL
	basePath := strings.TrimRight(u.Path, "/")
	apiPath := path
	if strings.HasPrefix(basePath, "/v0") && strings.HasPrefix(apiPath, "/v0") {
		apiPath = strings.TrimPrefix(apiPath, "/v0")
		if apiPath == "" {
			apiPath = "/"
		}
	}
	u.Path = basePath + apiPath
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, payload interface{}, out interface{}) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal agentmail request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 && lastErr != nil {
			// Only retry transient status codes; transport errors are retried too.
		}
		req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, q), bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create agentmail request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < c.maxRetries {
				c.retrySleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return fmt.Errorf("agentmail request failed: %w", err)
		}

		data, readErr := readBounded(resp.Body, c.maxResponseBytes)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read agentmail response: %w", readErr)
		}

		if shouldRetry(resp.StatusCode) && attempt < c.maxRetries {
			lastErr = parseAPIError(resp.StatusCode, data)
			c.retrySleep(parseRetryAfter(resp.Header.Get("Retry-After"), attempt))
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return parseAPIError(resp.StatusCode, data)
		}
		if out == nil || len(bytes.TrimSpace(data)) == 0 {
			return nil
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode agentmail response: %w", err)
		}
		return nil
	}
	return lastErr
}

func (c *Client) doRaw(ctx context.Context, method, path string, q url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint(path, q), nil)
	if err != nil {
		return nil, fmt.Errorf("create agentmail request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agentmail request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := readBounded(resp.Body, c.maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("read agentmail response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, data)
	}
	return data, nil
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = defaultMaxResponseBytes
	}
	lr := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout || status >= 500
}

func parseRetryAfter(raw string, attempt int) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Duration(attempt+1) * time.Second
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return time.Duration(attempt+1) * time.Second
}

func parseAPIError(status int, data []byte) *APIError {
	msg := ""
	var envelope struct {
		Error   interface{} `json:"error"`
		Message string      `json:"message"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		switch v := envelope.Error.(type) {
		case string:
			msg = v
		case map[string]interface{}:
			if m, ok := v["message"].(string); ok {
				msg = m
			}
		}
		if msg == "" {
			msg = envelope.Message
		}
	}
	body := string(bytes.TrimSpace(data))
	if len(body) > 1024 {
		body = body[:1024]
	}
	return &APIError{StatusCode: status, Message: msg, Body: body}
}

func addInt(q url.Values, key string, value int) {
	if value > 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func addString(q url.Values, key, value string) {
	if strings.TrimSpace(value) != "" {
		q.Set(key, strings.TrimSpace(value))
	}
}

func pathEscape(v string) string {
	return url.PathEscape(strings.TrimSpace(v))
}

func (c *Client) ListInboxes(ctx context.Context, opts ListInboxesOptions) (*ListInboxesResponse, error) {
	q := url.Values{}
	addInt(q, "limit", opts.Limit)
	addString(q, "cursor", opts.Cursor)
	var res ListInboxesResponse
	if err := c.do(ctx, http.MethodGet, "/v0/inboxes", q, nil, &res); err != nil {
		return nil, err
	}
	res.normalize()
	return &res, nil
}

func (c *Client) CreateInbox(ctx context.Context, req CreateInboxRequest) (*Inbox, error) {
	var out Inbox
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes", nil, req, &out)
}

func (c *Client) GetInbox(ctx context.Context, inboxID string) (*Inbox, error) {
	var out Inbox
	return &out, c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID), nil, nil, &out)
}

func (c *Client) UpdateInbox(ctx context.Context, inboxID string, req UpdateInboxRequest) (*Inbox, error) {
	var out Inbox
	return &out, c.do(ctx, http.MethodPatch, "/v0/inboxes/"+pathEscape(inboxID), nil, req, &out)
}

func (c *Client) DeleteInbox(ctx context.Context, inboxID string) error {
	return c.do(ctx, http.MethodDelete, "/v0/inboxes/"+pathEscape(inboxID), nil, nil, nil)
}

func (c *Client) ListMessages(ctx context.Context, inboxID string, opts ListMessagesOptions) (*ListMessagesResponse, error) {
	q := url.Values{}
	addInt(q, "limit", opts.Limit)
	addString(q, "cursor", opts.Cursor)
	addString(q, "after", opts.After)
	addString(q, "thread_id", opts.Thread)
	if len(opts.Labels) > 0 {
		q.Set("labels", strings.Join(opts.Labels, ","))
	}
	var res ListMessagesResponse
	if err := c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/messages", q, nil, &res); err != nil {
		return nil, err
	}
	res.normalize()
	return &res, nil
}

func (c *Client) GetMessage(ctx context.Context, inboxID, messageID string) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID), nil, nil, &out)
}

func (c *Client) UpdateMessage(ctx context.Context, inboxID, messageID string, req UpdateMessageRequest) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPatch, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID), nil, req, &out)
}

func (c *Client) DeleteMessage(ctx context.Context, inboxID, messageID string) error {
	return c.do(ctx, http.MethodDelete, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID), nil, nil, nil)
}

func (c *Client) SendMessage(ctx context.Context, inboxID string, req SendMessageRequest) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/send", nil, req, &out)
}

func (c *Client) ReplyMessage(ctx context.Context, inboxID, messageID string, req ReplyMessageRequest) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID)+"/reply", nil, req, &out)
}

func (c *Client) ReplyAllMessage(ctx context.Context, inboxID, messageID string, req ReplyMessageRequest) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID)+"/reply-all", nil, req, &out)
}

func (c *Client) ForwardMessage(ctx context.Context, inboxID, messageID string, req ForwardMessageRequest) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID)+"/forward", nil, req, &out)
}

func (c *Client) GetRawMessage(ctx context.Context, inboxID, messageID string) ([]byte, error) {
	return c.doRaw(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID)+"/raw", nil)
}

func (c *Client) GetAttachment(ctx context.Context, inboxID, messageID, attachmentID string) (*Attachment, error) {
	var out Attachment
	return &out, c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/messages/"+pathEscape(messageID)+"/attachments/"+pathEscape(attachmentID), nil, nil, &out)
}

func (c *Client) ListThreads(ctx context.Context, inboxID string, opts ListThreadsOptions) (*ListThreadsResponse, error) {
	q := url.Values{}
	addInt(q, "limit", opts.Limit)
	addString(q, "cursor", opts.Cursor)
	var res ListThreadsResponse
	if err := c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/threads", q, nil, &res); err != nil {
		return nil, err
	}
	res.normalize()
	return &res, nil
}

func (c *Client) GetThread(ctx context.Context, inboxID, threadID string) (*Thread, error) {
	var out Thread
	return &out, c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/threads/"+pathEscape(threadID), nil, nil, &out)
}

func (c *Client) ListDrafts(ctx context.Context, inboxID string, opts ListDraftsOptions) (*ListDraftsResponse, error) {
	q := url.Values{}
	addInt(q, "limit", opts.Limit)
	addString(q, "cursor", opts.Cursor)
	var res ListDraftsResponse
	if err := c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts", q, nil, &res); err != nil {
		return nil, err
	}
	res.normalize()
	return &res, nil
}

func (c *Client) GetDraft(ctx context.Context, inboxID, draftID string) (*Draft, error) {
	var out Draft
	return &out, c.do(ctx, http.MethodGet, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts/"+pathEscape(draftID), nil, nil, &out)
}

func (c *Client) CreateDraft(ctx context.Context, inboxID string, req Draft) (*Draft, error) {
	var out Draft
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts", nil, req, &out)
}

func (c *Client) UpdateDraft(ctx context.Context, inboxID, draftID string, req Draft) (*Draft, error) {
	var out Draft
	return &out, c.do(ctx, http.MethodPatch, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts/"+pathEscape(draftID), nil, req, &out)
}

func (c *Client) DeleteDraft(ctx context.Context, inboxID, draftID string) error {
	return c.do(ctx, http.MethodDelete, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts/"+pathEscape(draftID), nil, nil, nil)
}

func (c *Client) SendDraft(ctx context.Context, inboxID, draftID string) (*Message, error) {
	var out Message
	return &out, c.do(ctx, http.MethodPost, "/v0/inboxes/"+pathEscape(inboxID)+"/drafts/"+pathEscape(draftID)+"/send", nil, nil, &out)
}
