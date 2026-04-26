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

	var env YepAPIResponse
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("yepapi: invalid JSON response: %w", err)
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
// It first checks for a provider with type "yepapi", then falls back to a dedicated vault key.
func ResolveYepAPIKey(cfg *config.Config, vault config.SecretReader) (string, error) {
	// Strategy 1: Find a provider with type "yepapi" and use its key
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

	// Strategy 2: Dedicated vault key
	key, err := vault.ReadSecret("yepapi_api_key")
	if err == nil && key != "" {
		return key, nil
	}

	return "", fmt.Errorf("no YepAPI API key found: configure a provider with type 'yepapi' or set the 'yepapi_api_key' vault secret")
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
