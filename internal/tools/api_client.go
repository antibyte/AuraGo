package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/security"
)

// apiHTTPClient is a shared client with connection pooling and a 30s timeout.
var apiHTTPClient = security.NewSSRFProtectedHTTPClient(30 * time.Second)

// APIResult is the JSON response returned to the LLM.
type APIResult struct {
	Status     string            `json:"status"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Message    string            `json:"message,omitempty"`
}

// ExecuteAPIRequest performs an HTTP request and returns the response as structured JSON.
func ExecuteAPIRequest(method, url, body string, headers map[string]string) string {
	encode := func(r APIResult) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if err := requireNetworkPermission(); err != nil {
		return encode(APIResult{Status: "error", Message: err.Error()})
	}
	if url == "" {
		return encode(APIResult{Status: "error", Message: "'url' is required"})
	}
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	// SSRF Protection: Validate URL before request
	if err := security.ValidateSSRF(url); err != nil {
		return encode(APIResult{Status: "error", Message: fmt.Sprintf("URL validation failed: %v", err)})
	}

	// Build request
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
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

	// Execute with shared client (connection pooling)
	resp, err := apiHTTPClient.Do(req)
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
