package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/budget"

	"aurago/internal/config"
)

func TestAIGatewayMetadataUsesProviderAuthAndNeverReturnsErrorBodies(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		t.Run(provider, func(t *testing.T) {
			old := http.DefaultTransport
			defer func() { http.DefaultTransport = old }()
			http.DefaultTransport = aiGatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer fixture-provider-key" || r.Header.Get("cf-aig-authorization") != "Bearer fixture-gateway-key" {
					t.Error("probe missing provider or gateway authentication")
				}
				if provider == "anthropic" && (!strings.HasSuffix(r.URL.Path, "/anthropic/v1/models") || r.Header.Get("x-api-key") != "fixture-provider-key" || r.Header.Get("anthropic-version") == "") {
					t.Error("native Anthropic metadata path/auth incorrect")
				}
				if r.Header.Get("cf-aig-collect-log") != "" || r.Header.Get("cf-aig-collect-log-payload") != "false" || r.Header.Get("cf-aig-max-attempts") != "1" {
					t.Error("probe bypassed request policy")
				}
				return &http.Response{StatusCode: 401, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("private-upstream-diagnostic"))}, nil
			})
			cfg := &config.Config{}
			cfg.AIGateway.Enabled = true
			cfg.AIGateway.AccountID = "account"
			cfg.AIGateway.GatewayID = "gateway"
			cfg.AIGateway.Token = "fixture-gateway-key"
			cfg.Providers = []config.ProviderEntry{{ID: "selected", Type: provider, APIKey: "fixture-provider-key", BaseURL: "https://api." + provider + ".com/v1"}}
			s := &Server{Cfg: cfg, Logger: slog.Default()}
			w := httptest.NewRecorder()
			handleAIGatewayTest(s).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{"provider_id":"selected"}`)))
			if w.Code != 503 || !strings.Contains(w.Body.String(), `"code":"gateway_auth_failed"`) || strings.Contains(w.Body.String(), "private-upstream-diagnostic") {
				t.Fatalf("unsafe or misleading probe response: %s", w.Body.String())
			}
		})
	}
}

func TestAIGatewayExplicitModelTestIsBoundedAndBooksIncompleteResponses(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 3 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"message":"private-upstream-detail","type":"rate_limit_error"}}`)
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/openai/chat/completions") || r.Header.Get("cf-aig-max-attempts") != "1" {
			t.Error("generation test bypassed gateway or retry policy")
		}
		var request struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.MaxTokens <= 0 || request.MaxTokens > 32 {
			t.Error("generation test is unbounded")
		}
		finish := "stop"
		if calls == 2 {
			finish = "length"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"model":"gpt-4.1","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":%q}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`, finish)
	}))
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	defer transport.CloseIdleConnections()
	http.DefaultTransport = transport
	cfg := &config.Config{}
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "account"
	cfg.AIGateway.GatewayID = "gateway"
	cfg.Budget.Enabled = true
	cfg.Budget.DailyLimitUSD = 100
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai", Model: "gpt-4.1", BaseURL: "https://api.openai.com/v1", APIKey: "fixture-model-token"}}
	tracker := budget.NewTracker(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir())
	defer tracker.Flush()
	s := &Server{Cfg: cfg, BudgetTracker: tracker}
	for index, want := range []int{200, 503, 503} {
		w := httptest.NewRecorder()
		handleAIGatewayTest(s).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{"provider_id":"main","probe_kind":"generation"}`)))
		if w.Code != want {
			t.Fatalf("status=%d want %d: %s", w.Code, want, w.Body.String())
		}
		if index == 2 && (!strings.Contains(w.Body.String(), `"http_status":429`) || strings.Contains(w.Body.String(), "private-upstream-detail")) {
			t.Fatal("generation error leaked details or lost upstream status", w.Body.String())
		}
	}
	usage := tracker.GetStatus().Models["gpt-4.1"]
	if calls != 3 || usage.Calls != 2 || usage.InputTokens != 20 || usage.OutputTokens != 4 {
		t.Fatal("physical responses not booked once", usage)
	}
}

func TestAIGatewayStatusSelectsSavedRESTProviderWithoutNetwork(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = aiGatewayRoundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("status made a network request"); return nil, nil })
	cfg := &config.Config{}
	cfg.Providers = []config.ProviderEntry{{ID: "rest", Type: "cloudflare-rest", AccountID: "account", GatewayID: "gateway", Model: "google/gemini-2.5-flash"}}
	cfg.AIGateway.RequestTimeoutMS = 1200
	s := &Server{Cfg: cfg}
	w := httptest.NewRecorder()
	handleAIGatewayStatus(s).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/ai-gateway/status?provider_id=rest", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"route_supported":true`) || strings.Contains(w.Body.String(), `"live_status":"ok"`) {
		t.Fatal(w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"request_timeout_ms":1200`) || !strings.Contains(w.Body.String(), `"request_timeout_effective_ms":1200`) {
		t.Fatal("REST timeout diagnostics missing", w.Body.String())
	}
	w = httptest.NewRecorder()
	handleAIGatewayStatus(s).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/ai-gateway/status?provider_id=missing", nil))
	if w.Code != 404 {
		t.Fatal("unknown provider accepted")
	}
}

func TestAIGatewayProbeInheritsRequestCancellation(t *testing.T) {
	old := http.DefaultTransport
	defer func() { http.DefaultTransport = old }()
	http.DefaultTransport = aiGatewayRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() != context.Canceled {
			t.Error("request cancellation lost")
		}
		return nil, context.Canceled
	})
	cfg := &config.Config{}
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "account"
	cfg.AIGateway.GatewayID = "gateway"
	cfg.LLM.ProviderType = "openai"
	s := &Server{Cfg: cfg}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodPost, "/api/ai-gateway/test", strings.NewReader(`{}`)).WithContext(ctx)
	w := httptest.NewRecorder()
	handleAIGatewayTest(s).ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), `"code":"probe_cancelled"`) {
		t.Fatal(w.Body.String())
	}
}
