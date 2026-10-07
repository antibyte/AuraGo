package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	// HeadersDroppedOnRedirect is true when a redirect to another host, port or scheme made
	// the request drop the caller's headers (see apiRequestClient); FinalURL is then the
	// URL that answered (without user info), which the caller can request directly.
	HeadersDroppedOnRedirect bool   `json:"headers_dropped_on_redirect,omitempty"`
	FinalURL                 string `json:"final_url,omitempty"`
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

	var dropped bool
	client := apiRequestClient(apiHTTPClient, headers, &dropped)
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

	result := APIResult{
		Status:     status,
		StatusCode: resp.StatusCode,
		Headers:    respHeaders,
		Body:       bodyStr,
	}
	if dropped && resp.Request != nil && resp.Request.URL != nil {
		final := *resp.Request.URL
		final.User = nil // neither the password nor the user name
		result.HeadersDroppedOnRedirect = true
		result.FinalURL = final.String()
		slog.Debug("[api_request] Caller headers dropped after a redirect to another host, port or scheme",
			"headers_dropped", true, "final_host", resp.Request.URL.Host)
	}
	return encode(result)
}

// apiRedirectKeptHeaders are the caller's headers api_request keeps on a redirect that
// apiRedirectKeepsHeaders refuses. Every other header the caller set (Authorization, API
// keys, cookies, custom auth headers) is dropped there, so a redirect cannot carry
// credentials to a host they were not meant for.
var apiRedirectKeptHeaders = map[string]bool{"Accept": true, "Content-Type": true, "User-Agent": true}

// apiRequestClient returns base with a redirect policy that strips the caller's headers
// (except apiRedirectKeptHeaders) as soon as a hop fails apiRedirectKeepsHeaders against the
// first request, and keeps them stripped for the rest of the chain. net/http copies the
// first request's headers to every redirect and drops only Authorization and cookies, and
// only for another domain; a custom header such as X-API-Key would follow, and Authorization
// follows a port change or an https→http downgrade on the same host. base's own redirect
// policy (the SSRF checks) still runs for every hop.
//
// A 307 or 308 redirect re-sends the request body to the new location, also when the headers
// are stripped; a body that carries a credential goes with it. That is net/http's behaviour
// and is left as it is.
//
// dropped (may be nil) is set when the chain left the origin while the caller had set a
// header beyond apiRedirectKeptHeaders, so the result can say that those were not sent.
func apiRequestClient(base *http.Client, headers map[string]string, dropped *bool) *http.Client {
	if len(headers) == 0 {
		return base
	}
	sensitive := false
	for name := range headers {
		if !apiRedirectKeptHeaders[http.CanonicalHeaderKey(name)] {
			sensitive = true
		}
	}
	client := *base
	next := base.CheckRedirect
	left := false
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && !apiRedirectKeepsHeaders(via[0].URL, req.URL) {
			left = true
		}
		if left {
			if sensitive && dropped != nil {
				*dropped = true
			}
			for name := range headers {
				if canonical := http.CanonicalHeaderKey(name); !apiRedirectKeptHeaders[canonical] {
					req.Header.Del(canonical)
				}
			}
			for _, name := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
				req.Header.Del(name)
			}
		}
		if next != nil {
			return next(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
	return &client
}

// apiRedirectKeepsHeaders reports whether a redirect from the first request's URL to next
// may keep the caller's headers. The hostname must be equal (case-insensitive; a subdomain
// is another host), and either scheme and effective port stay the same, or the redirect is
// the upgrade from http on port 80 to https on port 443. The upgrade is safe to allow: the
// headers already crossed the network in clear text on the first hop, and the upgraded hop
// sends them to the same host encrypted. Every other change (a downgrade to http, another
// port, another host) strips them.
func apiRedirectKeepsHeaders(first, next *url.URL) bool {
	if first == nil || next == nil || !strings.EqualFold(first.Hostname(), next.Hostname()) {
		return false
	}
	fromScheme, toScheme := strings.ToLower(first.Scheme), strings.ToLower(next.Scheme)
	fromPort, toPort := normalizedURLPort(first), normalizedURLPort(next)
	if fromScheme == toScheme && fromPort == toPort {
		return true
	}
	return fromScheme == "http" && fromPort == "80" && toScheme == "https" && toPort == "443"
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
