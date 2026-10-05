package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
)

// Audit H9 follow-up: the AI Gateway live probe sends cf-aig-authorization,
// which net/http forwards to any redirect target. Server A (the gateway origin)
// answers with a 307 to server B on another host; B must never be reached.
// The fixture routes both origins through http.DefaultTransport because the
// gateway endpoint is fixed to gateway.ai.cloudflare.com.
func TestHandleAIGatewayTestDoesNotFollowCrossOriginRedirect(t *testing.T) {
	const gatewayToken = "cf-aig-secret-token"
	var gatewayHits, secondHits atomic.Int32
	oldTransport := http.DefaultTransport
	http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "gateway.ai.cloudflare.com" {
			secondHits.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header), Request: req}, nil
		}
		gatewayHits.Add(1)
		header := make(http.Header)
		header.Set("Location", "https://collector.example"+req.URL.Path)
		return &http.Response{StatusCode: http.StatusTemporaryRedirect, Body: io.NopCloser(strings.NewReader("")), Header: header, Request: req}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.LLM.Provider = "router"
	s.Cfg.LLM.ProviderType = "openrouter"
	s.Cfg.LLM.BaseURL = "https://openrouter.ai/api/v1"
	s.Cfg.AIGateway.Enabled = true
	s.Cfg.AIGateway.AccountID = "acct"
	s.Cfg.AIGateway.GatewayID = "gw"
	s.Cfg.AIGateway.Token = gatewayToken
	s.Cfg.Providers = []config.ProviderEntry{{ID: "router", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1"}}

	req := httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{"provider_id":"router"}`))
	rec := httptest.NewRecorder()
	handleAIGatewayTest(s).ServeHTTP(rec, req)

	if gatewayHits.Load() != 1 {
		t.Fatalf("gateway origin received %d request(s), want 1", gatewayHits.Load())
	}
	if hits := secondHits.Load(); hits != 0 {
		t.Fatalf("second origin received %d request(s)", hits)
	}
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"status":"error"`) {
		t.Fatalf("status = %d, body = %s; want 503 error", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), gatewayToken) {
		t.Fatalf("response leaked gateway token: %s", rec.Body.String())
	}
}
