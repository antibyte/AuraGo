package llm

import (
	"aurago/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func validGatewayPathSegment(value string) bool {
	return config.ValidAIGatewaySegment(value)
}

func cloudflareRESTBaseURL(account string) string {
	return "https://api.cloudflare.com/client/v4/accounts/" + url.PathEscape(account) + "/ai/v1"
}

func canonicalCloudflareRESTEndpoint(account, raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" && (u.Port() == "" || u.Port() == "443") && strings.EqualFold(u.Hostname(), "api.cloudflare.com") && strings.TrimRight(u.Path, "/") == "/client/v4/accounts/"+account+"/ai/v1"
}

type cloudflareRESTTransport struct {
	base      http.RoundTripper
	gatewayID string
}

func (t *cloudflareRESTTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	clone.Header.Del("cf-aig-authorization")
	clone.Header.Set("cf-aig-gateway-id", t.gatewayID)
	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/chat/completions") && req.Body != nil {
		body, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("Cloudflare REST request: %w", err)
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err = json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("Cloudflare REST request: %w", err)
		}
		if !validCloudflareModelID(payload.Model) {
			return nil, fmt.Errorf("Cloudflare REST requires a canonical author/model or @cf/author/model ID")
		}
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.ContentLength = int64(len(body))
		clone.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	}
	return t.base.RoundTrip(clone)
}

func validCloudflareModelID(model string) bool {
	author, name, ok := strings.Cut(model, "/")
	if !ok || name == "" || (!validGatewayPathSegment(author) && author != "@cf") {
		return false
	}
	if author == "@cf" && !strings.Contains(name, "/") {
		return false
	}
	return !strings.ContainsAny(model, " \t\r\n?#\\") && !strings.Contains(model, "..")
}
