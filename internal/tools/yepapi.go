package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

const yepAPIBaseURL = "https://api.yepapi.com"

var yepAPIHTTPClient = &http.Client{Timeout: 60 * time.Second}

// YepAPIClient is a lightweight HTTP client for the YepAPI unified API.
type YepAPIClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewYepAPIClient creates a new YepAPI client with the given API key.
func NewYepAPIClient(apiKey string) *YepAPIClient {
	return &YepAPIClient{
		apiKey:  apiKey,
		baseURL: yepAPIBaseURL,
		client:  yepAPIHTTPClient,
	}
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
func (c *YepAPIClient) Post(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
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
		return nil, fmt.Errorf("yepapi: HTTP %d — %s", resp.StatusCode, preview)
	}

	var env YepAPIResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		preview := string(respBody)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("yepapi: invalid JSON response (%s): %w", preview, err)
	}

	if !env.OK {
		if env.Error != nil {
			return nil, fmt.Errorf("yepapi: %s - %s", env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("yepapi: unknown error")
	}

	return env.Data, nil
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
				key, err := vault.ReadSecret(fmt.Sprintf("provider_%s_api_key", p.ID))
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
		if p.Type == "yepapi" {
			key, err := vault.ReadSecret(fmt.Sprintf("provider_%s_api_key", p.ID))
			if err == nil && key != "" {
				return key, nil
			}
			if p.APIKey != "" {
				return p.APIKey, nil
			}
		}
	}

	// Strategy 3: Dedicated vault key
	key, err := vault.ReadSecret("yepapi_api_key")
	if err == nil && key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no YepAPI API key found: configure a provider with type 'yepapi', select one in the YepAPI settings, or set the 'yepapi_api_key' vault secret")
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
	b, _ := json.Marshal(map[string]interface{}{
		"status": "success",
		"data":   json.RawMessage(data),
	})
	return string(b)
}
