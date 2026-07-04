package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

type aiGatewayRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn aiGatewayRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestHandleAIGatewayStatusReturnsLocalRouteDiagnostics(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("status endpoint must not perform a live request; got %s", req.URL.String())
		return nil, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.LLM.ProviderType = "openrouter"
	s.Cfg.LLM.BaseURL = "https://openrouter.ai/api/v1"
	s.Cfg.AIGateway.Enabled = true
	s.Cfg.AIGateway.AccountID = "acct"
	s.Cfg.AIGateway.GatewayID = "gw"

	req := httptest.NewRequest(http.MethodGet, "/api/ai-gateway/status", nil)
	rec := httptest.NewRecorder()
	handleAIGatewayStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body["status"] != "configured" {
		t.Fatalf("status field = %#v, want configured; body=%#v", body["status"], body)
	}
	if body["provider"] != "openrouter" {
		t.Fatalf("provider = %#v, want openrouter", body["provider"])
	}
	if body["route_supported"] != true {
		t.Fatalf("route_supported = %#v, want true", body["route_supported"])
	}
	if body["endpoint"] != "https://gateway.ai.cloudflare.com/v1/acct/gw/openrouter" {
		t.Fatalf("endpoint = %#v", body["endpoint"])
	}
	if body["privacy_mode"] != "metadata_only" {
		t.Fatalf("privacy_mode = %#v, want metadata_only", body["privacy_mode"])
	}
}

func TestHandleAIGatewayStatusReportsUnsupportedProviderWithoutLiveRequest(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unsupported provider status must not perform a live request; got %s", req.URL.String())
		return nil, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.LLM.ProviderType = "custom"
	s.Cfg.LLM.BaseURL = "https://custom.example.test/v1"
	s.Cfg.AIGateway.Enabled = true
	s.Cfg.AIGateway.AccountID = "acct"
	s.Cfg.AIGateway.GatewayID = "gw"

	req := httptest.NewRequest(http.MethodGet, "/api/ai-gateway/status", nil)
	rec := httptest.NewRecorder()
	handleAIGatewayStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body["status"] != "unsupported_provider" {
		t.Fatalf("status field = %#v, want unsupported_provider; body=%#v", body["status"], body)
	}
	if body["route_supported"] != false {
		t.Fatalf("route_supported = %#v, want false", body["route_supported"])
	}
}

func TestHandleAIGatewayTestUsesProviderIDAndScrubsSecrets(t *testing.T) {
	const gatewayToken = "cf-aig-secret-token"
	oldTransport := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.String(); got != "https://gateway.ai.cloudflare.com/v1/acct/gw/openrouter/models" {
			t.Fatalf("probe URL = %q, want openrouter models endpoint", got)
		}
		if got := req.Header.Get("cf-aig-authorization"); got != "Bearer "+gatewayToken {
			t.Fatalf("cf-aig-authorization = %q, want gateway token", got)
		}
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader("upstream rejected cf-aig-secret-token")),
			Header:     make(http.Header),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.LLM.Provider = "main"
	s.Cfg.LLM.ProviderType = "openai"
	s.Cfg.LLM.BaseURL = "https://api.openai.com/v1"
	s.Cfg.AIGateway.Enabled = true
	s.Cfg.AIGateway.AccountID = "acct"
	s.Cfg.AIGateway.GatewayID = "gw"
	s.Cfg.AIGateway.Token = gatewayToken
	s.Cfg.Providers = []config.ProviderEntry{
		{ID: "main", Type: "openai", BaseURL: "https://api.openai.com/v1"},
		{ID: "router", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{"provider_id":"router"}`))
	rec := httptest.NewRecorder()
	handleAIGatewayTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), gatewayToken) {
		t.Fatalf("test response leaked gateway token: %s", rec.Body.String())
	}
}

func TestHandleAIGatewayTestWorkersAIUsesCloudflareAuthAndModelsSearch(t *testing.T) {
	const apiKey = "cf-workers-api-token"
	oldTransport := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Method; got != http.MethodGet {
			t.Fatalf("method = %q, want GET", got)
		}
		if got := req.URL.String(); got != "https://api.cloudflare.com/client/v4/accounts/workers-acct/ai/models/search?per_page=1" {
			t.Fatalf("probe URL = %q, want Workers AI models search endpoint", got)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Fatalf("Authorization = %q, want Cloudflare provider token", got)
		}
		if got := req.Header.Get("cf-aig-gateway-id"); got != "gw" {
			t.Fatalf("cf-aig-gateway-id = %q, want gateway id", got)
		}
		if got := req.Header.Get("cf-aig-authorization"); got != "" {
			t.Fatalf("cf-aig-authorization = %q, want no provider-native gateway token for Workers AI", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":true,"result":[]}`)),
			Header:     make(http.Header),
		}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.LLM.Provider = "worker"
	s.Cfg.AIGateway.Enabled = true
	s.Cfg.AIGateway.AccountID = "gateway-acct"
	s.Cfg.AIGateway.GatewayID = "gw"
	s.Cfg.Providers = []config.ProviderEntry{{
		ID:        "worker",
		Type:      "workers-ai",
		AccountID: "workers-acct",
		APIKey:    apiKey,
	}}

	req := httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{"provider_id":"worker"}`))
	rec := httptest.NewRecorder()
	handleAIGatewayTest(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if body["live_status"] != "ok" {
		t.Fatalf("live_status = %#v, want ok; body=%#v", body["live_status"], body)
	}
}
