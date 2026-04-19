// Package jellyfin provides a Go client for the Jellyfin REST API.
package jellyfin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// maxResponseBody limits response body reads to 10 MB to prevent OOM from malicious or broken servers.
const maxResponseBody = 10 << 20

// Client represents a connection to a Jellyfin server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	vault      *security.Vault
}

// NewClient creates a new Jellyfin client from configuration.
func NewClient(cfg config.JellyfinConfig, vault *security.Vault) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("Jellyfin integration is disabled")
	}

	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	port := cfg.Port
	if port == 0 {
		port = 8096
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("Jellyfin host is required")
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, port)

	// Get API key from config or vault
	apiKey := cfg.APIKey
	if apiKey == "" && vault != nil {
		key, err := vault.ReadSecret("jellyfin_api_key")
		if err == nil && key != "" {
			apiKey = key
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("Jellyfin API key is required (set in vault as 'jellyfin_api_key')")
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSSL,
	}

	connectTimeout := cfg.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 30
	}

	requestTimeout := cfg.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = 60
	}

	dialer := &net.Dialer{
		Timeout:   time.Duration(connectTimeout) * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(requestTimeout) * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:       tlsConfig,
				DialContext:           dialer.DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
			},
		},
		vault: vault,
	}, nil
}

// request performs an HTTP request to the Jellyfin API.
func (c *Client) request(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	fullURL := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return resp, nil
}

// Get performs a GET request and decodes the response.
func (c *Client) Get(ctx context.Context, endpoint string, result interface{}) error {
	resp, err := c.request(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Post performs a POST request and optionally decodes the response.
func (c *Client) Post(ctx context.Context, endpoint string, body, result interface{}) error {
	resp, err := c.request(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, endpoint string) error {
	resp, err := c.request(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	return nil
}

// BaseURL returns the base URL of the Jellyfin server.
func (c *Client) BaseURL() string {
	return c.baseURL
}
