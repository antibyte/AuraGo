package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

// apiHTTPClient is a shared client with connection pooling and a 30s timeout.
var apiHTTPClient = security.NewSSRFProtectedHTTPClient(30 * time.Second)

// apiLocalOllamaHTTPClient is only used for explicitly configured local Ollama
// endpoints. Redirects are blocked so a local Ollama allow cannot become a
// generic local-network fetch through 3xx responses.
var apiLocalOllamaHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || !isLoopbackHostname(host) {
			return nil, fmt.Errorf("local Ollama dial target is not loopback")
		}
		if strings.EqualFold(host, "localhost") {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
		}
		return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, address)
	}},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("redirects are not allowed for local Ollama api_request targets")
	},
}

type APIRequestOptions struct {
	Context                   context.Context
	AllowedLocalOllamaBaseURL string
}

// APIResult is the JSON response returned to the LLM.
type APIResult struct {
	Status     string            `json:"status"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// ExecuteAPIRequest performs an HTTP request and returns the response as structured JSON.
func ExecuteAPIRequest(method, rawURL, body string, headers map[string]string) string {
	return ExecuteAPIRequestWithOptions(method, rawURL, body, headers, APIRequestOptions{})
}

// ExecuteAPIRequestWithOptions performs an HTTP request and returns the response as structured JSON.
func ExecuteAPIRequestWithOptions(method, rawURL, body string, headers map[string]string, opts APIRequestOptions) string {
	encode := func(r APIResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if err := requireNetworkPermission(); err != nil {
		return encode(APIResult{Status: "error", Message: err.Error()})
	}
	if rawURL == "" {
		return encode(APIResult{Status: "error", Message: "'url' is required"})
	}
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	allowLocalOllama := isAllowedLocalOllamaRequest(rawURL, opts.AllowedLocalOllamaBaseURL)

	// SSRF Protection: Validate URL before request
	if !allowLocalOllama {
		if err := security.ValidateSSRF(rawURL); err != nil {
			return encode(APIResult{Status: "error", Message: fmt.Sprintf("URL validation failed: %v", err)})
		}
	}

	// Build request
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(requestContext([]context.Context{opts.Context}), method, rawURL, reqBody)
	if err != nil {
		return encode(APIResult{Status: "error", Message: fmt.Sprintf("Failed to create request: %v", err)})
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Default Content-Type for requests with body
	if body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "AuraGo-Agent/1.0")

	client := apiHTTPClient
	if allowLocalOllama {
		client = apiLocalOllamaHTTPClient
	}

	// Execute with shared client (connection pooling)
	resp, err := client.Do(req)
	if err != nil {
		return encode(APIResult{Status: "error", Message: fmt.Sprintf("Request failed: %v", err)})
	}
	defer resp.Body.Close()

	// Read response body (cap at 16KB to protect LLM context)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if err != nil {
		return encode(APIResult{Status: "error", Message: fmt.Sprintf("Failed to read response: %v", err)})
	}

	bodyStr := string(respBody)

	// Extract key response headers
	respHeaders := map[string]string{
		"content-type": resp.Header.Get("Content-Type"),
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		respHeaders["location"] = loc
	}

	status := "success"
	if resp.StatusCode >= 400 {
		status = "error"
	}

	return encode(APIResult{
		Status:     status,
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       bodyStr,
	})
}

func isAllowedLocalOllamaRequest(rawURL, baseURL string) bool {
	if strings.TrimSpace(baseURL) == "" {
		return false
	}
	reqURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	if reqURL.Scheme != "http" && reqURL.Scheme != "https" {
		return false
	}
	if !strings.EqualFold(reqURL.Scheme, base.Scheme) {
		return false
	}
	if reqURL.User != nil || base.User != nil || reqURL.Fragment != "" ||
		!isLoopbackHostname(base.Hostname()) || !strings.EqualFold(reqURL.Hostname(), base.Hostname()) {
		return false
	}
	if normalizedURLPort(reqURL) != normalizedURLPort(base) {
		return false
	}
	return isOllamaAPIPath(reqURL.Path)
}

func isLoopbackHostname(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func normalizedURLPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	return "80"
}

func isOllamaAPIPath(path string) bool {
	switch path {
	case "/api/generate", "/api/chat", "/api/embed", "/api/embeddings", "/api/tags",
		"/api/show", "/api/ps", "/api/version", "/api/create", "/api/copy",
		"/api/delete", "/api/pull", "/api/push", "/v1/chat/completions",
		"/v1/completions", "/v1/embeddings", "/v1/models":
		return true
	}
	return strings.HasPrefix(path, "/api/blobs/sha256:")
}
