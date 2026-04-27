package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const yepAPIBaseURL = "https://api.yepapi.com"

var yepAPIHTTPClient = &http.Client{Timeout: 60 * time.Second}

// yepAPIClientCache holds reusable YepAPI clients per (apiKey|baseURL) tuple.
var yepAPIClientCache sync.Map

// YepAPIClient is a lightweight HTTP client for the YepAPI unified API.
type YepAPIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewYepAPIClient creates a new YepAPI client with the given API key.
func NewYepAPIClient(apiKey string) *YepAPIClient {
	return NewYepAPIClientWithBaseURL(apiKey, "")
}

// NewYepAPIClientWithBaseURL creates a new YepAPI client with the given API key
// and an optional custom base URL. If baseURL is empty, the default YepAPI URL is used.
func NewYepAPIClientWithBaseURL(apiKey, baseURL string) *YepAPIClient {
	if baseURL == "" {
		baseURL = yepAPIBaseURL
	}
	return &YepAPIClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  yepAPIHTTPClient,
	}
}

// GetYepAPIClient returns a cached YepAPI client for the given API key and base URL,
// creating one if necessary. The returned client is safe for concurrent use.
func GetYepAPIClient(apiKey, baseURL string) *YepAPIClient {
	if baseURL == "" {
		baseURL = yepAPIBaseURL
	}
	key := apiKey + "|" + baseURL
	if c, ok := yepAPIClientCache.Load(key); ok {
		return c.(*YepAPIClient)
	}
	c := NewYepAPIClientWithBaseURL(apiKey, baseURL)
	yepAPIClientCache.Store(key, c)
	return c
}

// YepAPIResponse is the standard JSON envelope returned by every YepAPI endpoint.
type YepAPIResponse struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Post sends a POST request to a YepAPI endpoint and returns the data payload.
// It retries transient failures (network errors, 5xx, 429) up to 3 times with
// exponential backoff (500ms, 1s, 2s).
func (c *YepAPIClient) Post(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
	var lastErr error
	backoff := 500 * time.Millisecond

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("yepapi: context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		data, err := c.postOnce(ctx, endpoint, payload)
		if err == nil {
			return data, nil
		}
		lastErr = err

		// Do not retry on client errors (4xx except 429) or unmarshalling errors.
		if isNonRetryableError(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("yepapi: request failed after 3 attempts: %w", lastErr)
}

func (c *YepAPIClient) postOnce(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
	url := c.baseURL + endpoint

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("yepapi: failed to marshal payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("yepapi: failed to build request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yepapi: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("yepapi: failed to read response: %w", err)
	}

	// If the HTTP status is not 2xx, include the raw body in the error so
	// the caller can see what the server actually returned (e.g. HTML 404/403).
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("yepapi: HTTP %d - %s", resp.StatusCode, security.IsolateExternalData(preview))
	}

	var env YepAPIResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("yepapi: invalid JSON response (%s): %w", security.IsolateExternalData(preview), err)
	}

	if !env.OK {
		if env.Error != nil {
			return nil, fmt.Errorf("yepapi: %s - %s", env.Error.Code, security.IsolateExternalData(env.Error.Message))
		}
		return nil, fmt.Errorf("yepapi: unknown error")
	}

	return env.Data, nil
}

// isNonRetryableError returns true for errors that should not be retried.
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// 4xx client errors (except 429 Too Many Requests) are non-retryable.
	if strings.Contains(s, "HTTP 4") && !strings.Contains(s, "HTTP 429") {
		return true
	}
	// JSON unmarshalling errors indicate a bad response format, not a transient issue.
	if strings.Contains(s, "invalid JSON response") {
		return true
	}
	return false
}

// ResolveYepAPIKey resolves the YepAPI API key from the config/vault.
// Priority:
//  1. Explicit provider ID from cfg.YepAPI.Provider
//  2. First provider with type "yepapi"
//  3. Dedicated vault key "yepapi_api_key"
func ResolveYepAPIKey(cfg *config.Config, vault config.SecretReader) (string, error) {
	// Strategy 1: Explicit provider ID configured in YepAPI section
	if cfg.YepAPI.Provider != "" {
		for _, p := range cfg.Providers {
			if p.ID == cfg.YepAPI.Provider {
				if !strings.EqualFold(p.Type, "yepapi") {
					return "", fmt.Errorf("provider '%s' configured for YepAPI has type '%s'; expected provider type 'yepapi'", p.ID, p.Type)
				}
				key, err := readYepAPISecret(vault, fmt.Sprintf("provider_%s_api_key", p.ID))
				if err == nil && key != "" {
					return key, nil
				}
				if p.APIKey != "" {
					return p.APIKey, nil
				}
				return "", fmt.Errorf("provider '%s' configured for YepAPI but has no API key", p.ID)
			}
		}
		return "", fmt.Errorf("provider '%s' configured for YepAPI but not found in providers list", cfg.YepAPI.Provider)
	}

	// Strategy 2: Find a provider with type "yepapi" and use its key
	for _, p := range cfg.Providers {
		if strings.EqualFold(p.Type, "yepapi") {
			key, err := readYepAPISecret(vault, fmt.Sprintf("provider_%s_api_key", p.ID))
			if err == nil && key != "" {
				return key, nil
			}
			if p.APIKey != "" {
				return p.APIKey, nil
			}
		}
	}

	// Strategy 3: Dedicated vault key
	key, err := readYepAPISecret(vault, "yepapi_api_key")
	if err == nil && key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no YepAPI API key found: configure a provider with type 'yepapi', select one in the YepAPI settings, or set the 'yepapi_api_key' vault secret")
}

func readYepAPISecret(vault config.SecretReader, key string) (string, error) {
	if vault == nil {
		return "", fmt.Errorf("vault is not available")
	}
	return vault.ReadSecret(key)
}

// formatError wraps an error message in a JSON string for consistent tool output.
func yepAPIFormatError(msg string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"status":  "error",
		"message": security.IsolateExternalData(msg),
	})
	return string(b)
}

// formatSuccess wraps raw JSON data in a success envelope.
func yepAPIFormatSuccess(data json.RawMessage) string {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err == nil {
		if rawErr, ok := obj["error"]; ok && rawErr != nil {
			msg := fmt.Sprint(rawErr)
			if msg != "" {
				return yepAPIFormatError(msg)
			}
		}
	}

	b, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data":   json.RawMessage(data),
	})
	return string(b)
}
