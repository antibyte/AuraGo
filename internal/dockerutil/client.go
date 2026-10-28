package dockerutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is the small Docker Engine transport shared by AuraGo-owned services.
// Authorization for agent-facing Docker tools remains outside this package.
type Client struct {
	host       string
	httpClient *http.Client
	dial       func(ctx context.Context) (net.Conn, error) // what the transport dials with; see Dial
}

// NewClient creates a Docker Engine client for unix sockets, named pipes, or TCP.
func NewClient(host string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	host = NormalizeHost(host)
	dial := func(ctx context.Context) (net.Conn, error) { return DialContext(ctx, host) }
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
	}
	return &Client{
		host: host,
		httpClient: &http.Client{
			Transport: NewVersionTransport(transport),
			Timeout:   timeout,
		},
		dial: dial,
	}
}

// dialerIdleConnTimeout reaps idle connections of a NewClientWithDialer client.
// For an Engine reached through SSH this also closes the SSH client the idle
// connection owns.
const dialerIdleConnTimeout = time.Minute

// NewClientWithDialer is NewClient with a caller-supplied dial function and
// version-probe budget, for Engines reached through a transport that an
// endpoint string cannot express (TLS with client certificates, SSH tunnels).
// name is what Host reports. Requests still use Endpoint
// ("http://docker/v1.45/..."): a TLS dial function returns a *tls.Conn, so the
// URL stays plain http for every kind. timeout <= 0 keeps the 30 s request
// timeout and probeTimeout <= 0 the 5 s budget of the /version probe.
// Connections are pooled; CloseIdleConnections releases them at once.
func NewClientWithDialer(name string, dial func(ctx context.Context) (net.Conn, error), timeout, probeTimeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	checked := func(ctx context.Context) (net.Conn, error) {
		if dial == nil {
			return nil, fmt.Errorf("docker client %q has no dial function", name)
		}
		return dial(ctx)
	}
	transport := &http.Transport{
		IdleConnTimeout: dialerIdleConnTimeout,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return checked(ctx)
		},
	}
	return &Client{
		host: name,
		httpClient: &http.Client{
			Transport: NewVersionTransportWithProbeTimeout(transport, probeTimeout),
			Timeout:   timeout,
		},
		dial: checked,
	}
}

// Endpoint returns a versioned Docker Engine path.
func Endpoint(path string) string {
	return "http://docker/" + APIVersion + "/" + strings.TrimLeft(path, "/")
}

// DoJSON sends a Docker Engine request and optionally decodes its JSON body.
// The returned status is 0 when no HTTP response arrived: request building,
// transport and API-version negotiation failures return 0 with an error. A
// non-2xx answer returns the real status code with an error carrying the
// Engine's message, so callers classify Docker answers (404 gone, 304 not
// modified, 409 conflict) by code and never by matching error text.
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
		return resp.StatusCode, fmt.Errorf("Docker API returned %d: %s", resp.StatusCode, SanitizeOneLine(string(detail), 512))
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

// StreamingHTTPClient returns a client on the same transport (and so the same
// version negotiation and dial function) without a total timeout, for
// requests that stay open as long as their context lives: the events feed and
// followed logs. The caller bounds every request with its context.
func (c *Client) StreamingHTTPClient() *http.Client {
	if c == nil || c.httpClient == nil {
		return nil
	}
	client := *c.httpClient
	client.Timeout = 0
	return &client
}

// Dial opens a raw connection to the Engine with the client's own transport:
// the endpoint dialer of NewClient (unix socket, named pipe, TCP) or the
// caller's dial function of NewClientWithDialer (TLS, mutual TLS, SSH). The
// caller owns the connection and closes it. Raw upgraded streams (exec attach)
// use it so they reach the same Engine the pooled requests do.
func (c *Client) Dial(ctx context.Context) (net.Conn, error) {
	if c == nil || c.dial == nil {
		return nil, errors.New("docker client has no dial function")
	}
	return c.dial(ctx)
}

// APIVersion returns the Engine API version the client's version transport
// negotiated, without the "v" ("1.45"). It probes GET /version once when the
// client has not negotiated yet, and again after a stale-version signal.
func (c *Client) APIVersion(ctx context.Context) (string, error) {
	if c == nil || c.httpClient == nil {
		return "", errors.New("docker client is not initialised")
	}
	transport, ok := c.httpClient.Transport.(*VersionTransport)
	if !ok {
		return "", errors.New("docker client has no version transport")
	}
	origin, _ := url.Parse("http://docker")
	version, err := transport.versionFor(ctx, origin)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(version, "v"), nil
}

// CloseIdleConnections releases pooled connections without interrupting
// requests that are already using this client.
func (c *Client) CloseIdleConnections() {
	if c != nil && c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}
