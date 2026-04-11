// Package truenas provides a Go client for the TrueNAS REST API (v2.0).
package truenas

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

// Client represents a connection to a TrueNAS server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	vault      *security.Vault
	insecure   bool
}

// NewClient creates a new TrueNAS client from configuration.
func NewClient(cfg config.TrueNASConfig, vault *security.Vault) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("TrueNAS integration is disabled")
	}

	scheme := "http"
	if cfg.UseHTTPS {
		scheme = "https"
	}

	port := cfg.Port
	if port == 0 {
		port = 443
		if !cfg.UseHTTPS {
			port = 80
		}
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("TrueNAS host is required")
	}

	baseURL := fmt.Sprintf("%s://%s:%d/api/v2.0", scheme, cfg.Host, port)

	// Get API key from config or vault
	apiKey := cfg.APIKey
	if apiKey == "" && vault != nil {
		key, err := vault.ReadSecret("truenas_api_key")
		if err == nil && key != "" {
			apiKey = key
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("TrueNAS API key is required (set in config or vault as 'truenas_api_key')")
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

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout:   time.Duration(connectTimeout) * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &Client{
		baseURL:  baseURL,
		apiKey:   apiKey,
		insecure: cfg.InsecureSSL,
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

// request performs an HTTP request to the TrueNAS API.
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

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
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

// Post performs a POST request and optionally decodes the response.
func (c *Client) Post(ctx context.Context, endpoint string, body, result interface{}) error {
	resp, err := c.request(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// Put performs a PUT request.
func (c *Client) Put(ctx context.Context, endpoint string, body interface{}) error {
	resp, err := c.request(ctx, http.MethodPut, endpoint, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Health returns the TrueNAS system health and info.
func (c *Client) Health(ctx context.Context) (*SystemInfo, error) {
	var info SystemInfo
	if err := c.Get(ctx, "/system/info", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// SystemInfo represents TrueNAS system information.
type SystemInfo struct {
	Version       string    `json:"version"`
	BuildTime     string    `json:"buildtime"`
	Hostname      string    `json:"hostname"`
	PhysMem       int64     `json:"physmem"`
	Model         string    `json:"model"`
	Cores         int       `json:"cores"`
	PhysicalCores int       `json:"physical_cores"`
	LoadAverage   []float64 `json:"loadavg"`
	Uptime        string    `json:"uptime"`
	SystemSerial  string    `json:"system_serial"`
	SystemProduct string    `json:"system_product"`
}

// IsSCALE returns true if the TrueNAS instance is SCALE.
func (s *SystemInfo) IsSCALE() bool {
	return contains(s.Version, "SCALE")
}

// IsCore returns true if the TrueNAS instance is Core.
func (s *SystemInfo) IsCore() bool {
	return !s.IsSCALE()
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ListAlerts returns system alerts.
func (c *Client) ListAlerts(ctx context.Context) ([]Alert, error) {
	var alerts []Alert
	if err := c.Get(ctx, "/alert/list", &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

// Alert represents a TrueNAS system alert.
type Alert struct {
	ID        string `json:"id"`
	Level     string `json:"level"` // INFO, WARNING, ERROR, CRITICAL
	Title     string `json:"title"`
	Message   string `json:"message"`
	Date      string `json:"date"`
	Dismissed bool   `json:"dismissed"`
	Source    string `json:"source"`
}

// DismissAlert dismisses an alert by ID.
func (c *Client) DismissAlert(ctx context.Context, alertID string) error {
	return c.Post(ctx, "/alert/dismiss", map[string]string{"id": alertID}, nil)
}

// RestoreAlert restores a dismissed alert.
func (c *Client) RestoreAlert(ctx context.Context, alertID string) error {
	return c.Post(ctx, "/alert/restore", map[string]string{"id": alertID}, nil)
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	// No persistent connections to close currently
	return nil
}

// URL returns the base URL of the TrueNAS server.
func (c *Client) URL() string {
	return c.baseURL
}
