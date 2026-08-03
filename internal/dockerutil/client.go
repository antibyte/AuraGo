package dockerutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client is the small Docker Engine transport shared by AuraGo-owned services.
// Authorization for agent-facing Docker tools remains outside this package.
type Client struct {
	host       string
	httpClient *http.Client
}

// NewClient creates a Docker Engine client for unix sockets, named pipes, or TCP.
func NewClient(host string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	host = NormalizeHost(host)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return DialContext(ctx, host)
		},
	}
	return &Client{
		host: host,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

// Endpoint returns a versioned Docker Engine path.
func Endpoint(path string) string {
	return "http://docker/" + APIVersion + "/" + strings.TrimLeft(path, "/")
}

// DoJSON sends a Docker Engine request and optionally decodes its JSON body.
func (c *Client) DoJSON(ctx context.Context, method, path string, requestBody, responseBody any) (int, error) {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return 0, fmt.Errorf("marshal Docker request: %w", err)
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, Endpoint(path), body)
	if err != nil {
		return 0, fmt.Errorf("create Docker request: %w", err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Docker request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resp.StatusCode, fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, sanitizeDockerError(detail))
	}
	if responseBody != nil {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(responseBody); err != nil && err != io.EOF {
			return resp.StatusCode, fmt.Errorf("decode Docker response: %w", err)
		}
	}
	return resp.StatusCode, nil
}

// HTTPClient exposes the transport for streaming Docker operations such as image pulls.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// Host returns the normalized Engine endpoint used by this client.
func (c *Client) Host() string {
	if c == nil {
		return ""
	}
	return c.host
}

// HTTPClientWithTimeout returns a streaming client that reuses the Docker
// transport but applies an operation-specific total timeout. Docker image
// pulls can legitimately take several minutes and must not inherit the short
// request timeout used for ordinary Engine API calls.
func (c *Client) HTTPClientWithTimeout(timeout time.Duration) *http.Client {
	if c == nil || c.httpClient == nil {
		return nil
	}
	client := *c.httpClient
	if timeout > 0 {
		client.Timeout = timeout
	}
	return &client
}

// CloseIdleConnections releases pooled connections without interrupting
// requests that are already using this client.
func (c *Client) CloseIdleConnections() {
	if c != nil && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

func sanitizeDockerError(value []byte) string {
	text := strings.TrimSpace(string(value))
	if len(text) > 512 {
		text = text[:512]
	}
	return text
}
