// Package obsidian provides a Go client for the Obsidian Local REST API plugin.
package obsidian

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

// Client represents a connection to the Obsidian Local REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Obsidian client from configuration.
func NewClient(cfg config.ObsidianConfig, vault *security.Vault) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("Obsidian integration is disabled")
	}

	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	port := cfg.Port
	if port == 0 {
		port = 27124
	}

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}

	baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	apiKey := cfg.APIKey
	if apiKey == "" && vault != nil {
		key, err := vault.ReadSecret("obsidian_api_key")
		if err == nil && key != "" {
			apiKey = key
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("Obsidian API key is required (set in vault as 'obsidian_api_key')")
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSSL,
	}

	connectTimeout := cfg.ConnectTimeout
	if connectTimeout == 0 {
		connectTimeout = 10
	}

	requestTimeout := cfg.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = 30
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
	}, nil
}

// request performs an HTTP request to the Obsidian Local REST API.
func (c *Client) request(ctx context.Context, method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	fullURL := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "text/markdown")
	}
	req.Header.Set("Accept", "application/vnd.olrapi.note+json")

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}

	return resp, nil
}

// getJSON performs a GET request with JSON accept header and decodes the response.
func (c *Client) getJSON(ctx context.Context, endpoint string, result interface{}) error {
	resp, err := c.request(ctx, http.MethodGet, endpoint, nil, map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
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

// postJSON performs a POST request with JSON body and decodes the response.
func (c *Client) postJSON(ctx context.Context, endpoint string, reqBody, result interface{}) error {
	var bodyReader io.Reader
	if reqBody != nil {
		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	resp, err := c.request(ctx, http.MethodPost, endpoint, bodyReader, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Ping checks if the Obsidian Local REST API is reachable (unauthenticated).
func (c *Client) Ping(ctx context.Context) (*ServerStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("create ping request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ping Obsidian: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read ping response: %w", err)
	}

	var status ServerStatus
	if len(body) > 0 {
		_ = json.Unmarshal(body, &status)
	}
	status.Online = resp.StatusCode == http.StatusOK
	return &status, nil
}

// Close releases client resources.
func (c *Client) Close() error {
	return nil
}

// encodePath URL-encodes a vault path for use in API endpoints.
func encodePath(path string) string {
	return url.PathEscape(path)
}

// itoa converts an integer to string.
func itoa(i int) string {
	return strconv.Itoa(i)
}
