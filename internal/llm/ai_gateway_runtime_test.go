package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func gatewayFixtureConfig() *config.Config {
	cfg := &config.Config{}
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "fixture-account"
	cfg.AIGateway.GatewayID = "fixture-gateway"
	cfg.AIGateway.Token = "fixture-gateway-token"
	return cfg
}

// Redirect only the socket to a local fixture, retaining the real route and wrappers.
func gatewayFixtureSocket(t *testing.T, client *http.Client, server *httptest.Server) {
	t.Helper()
	leaf, ok := unwrapLLMTransport(client.Transport).(*http.Transport)
	if !ok {
		t.Fatalf("unexpected leaf %T", unwrapLLMTransport(client.Transport))
	}
	leaf.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	leaf.TLSClientConfig.ServerName = "example.com"
	leaf.Proxy = nil
	leaf.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(leaf.CloseIdleConnections)
}

func TestAIGatewayRuntimePreservesProviderUsageAndPaths(t *testing.T) {
	for _, provider := range []string{"openai", "anthropic"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", provider, stream), func(t *testing.T) {
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					wantPath := "/v1/fixture-account/fixture-gateway/" + provider + "/chat/completions"
					if provider == "anthropic" {
						wantPath = "/v1/fixture-account/fixture-gateway/anthropic/v1/messages"
					}
					if r.URL.Path != wantPath {
						t.Errorf("path=%q, want %q", r.URL.Path, wantPath)
					}
					if r.Header.Get("Authorization") != "Bearer fixture-provider-key" || r.Header.Get("cf-aig-authorization") != "Bearer fixture-gateway-token" {
						t.Error("provider/gateway authentication lost")
					}
					if r.Header.Get("cf-aig-collect-log") != "" || r.Header.Get("cf-aig-collect-log-payload") != "false" || r.Header.Get("cf-aig-max-attempts") != "1" {
						t.Error("privacy/retry policy lost")
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if provider == "openai" {
						if body["prompt_cache_key"] == nil {
							t.Error("prompt cache key missing")
						}
						if stream && body["stream_options"].(map[string]any)["include_usage"] != true {
							t.Error("stream usage missing")
						}
					}
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						if provider == "anthropic" {
							fmt.Fprint(w, "event: message_start\ndata: {\"message\":{\"id\":\"fixture\",\"model\":\"claude-sonnet-4-5\",\"usage\":{\"input_tokens\":20,\"cache_read_input_tokens\":60,\"cache_creation_input_tokens\":20}}}\n\nevent: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":7}}\n\nevent: message_stop\ndata: {}\n\n")
						} else {
							fmt.Fprint(w, "data: {\"model\":\"gpt-4.1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":7,\"total_tokens\":107,\"prompt_tokens_details\":{\"cached_tokens\":60}}}\n\ndata: [DONE]\n\n")
						}
					} else {
						w.Header().Set("Content-Type", "application/json")
						if provider == "anthropic" {
							fmt.Fprint(w, `{"id":"fixture","model":"claude-sonnet-4-5","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":7,"cache_read_input_tokens":60,"cache_creation_input_tokens":20}}`)
						} else {
							fmt.Fprint(w, `{"model":"gpt-4.1","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":7,"total_tokens":107,"prompt_tokens_details":{"cached_tokens":60}}}`)
						}
					}
				}))
				defer server.Close()
				base, model := "https://api.openai.com/v1", "gpt-4.1"
				if provider == "anthropic" {
					base, model = "https://api.anthropic.com/v1", "claude-sonnet-4-5"
				}
				cc := buildOpenAIClientConfig(gatewayFixtureConfig(), resolvedProvider{ProviderType: provider, BaseURL: base, APIKey: "fixture-provider-key"})
				gatewayFixtureSocket(t, cc.HTTPClient.(*http.Client), server)
				client := openai.NewClientWithConfig(cc)
				ctx, capture := CaptureUsage(context.Background())
				request := openai.ChatCompletionRequest{Model: model, Messages: []openai.ChatCompletionMessage{{Role: "system", Content: "stable prefix"}, {Role: "user", Content: "ok"}}}
				if stream {
					response, err := client.CreateChatCompletionStream(ctx, request)
					if err != nil {
						t.Fatal(err)
					}
					defer response.Close()
					for {
						_, err = response.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					_ = response.Close()
				} else if _, err := client.CreateChatCompletion(ctx, request); err != nil {
					t.Fatal(err)
				}
				items := capture.Snapshot()
				if len(items) != 1 || items[0].InputTokens == nil || *items[0].InputTokens != 100 || items[0].OutputTokens == nil || *items[0].OutputTokens != 7 || items[0].EstimatedCostUSD == nil {
					t.Fatalf("usage lost or counted more than once: %+v", items)
				}
				if provider == "anthropic" && (items[0].CacheWriteTokens == nil || *items[0].CacheWriteTokens != 20) {
					t.Fatal("cache writes lost")
				}
			})
		}
	}
}

func TestAIGatewayPrivacyModesAndRetryOwnership(t *testing.T) {
	for _, tc := range []struct{ mode, log, payload string }{{"metadata_only", "", "false"}, {"off", "false", "false"}, {"full", "true", "true"}} {
		t.Run(tc.mode, func(t *testing.T) {
			cfg := gatewayFixtureConfig()
			cfg.AIGateway.LogMode = tc.mode
			cfg.AIGateway.MaxAttempts = 5
			cfg.AIGateway.Backoff = "linear"
			cfg.AIGateway.RetryDelayMS = 250
			base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("cf-aig-collect-log") != tc.log || r.Header.Get("cf-aig-collect-log-payload") != tc.payload {
					t.Error("logging override incorrect")
				}
				if r.Header.Get("cf-aig-max-attempts") != "1" || r.Header.Get("cf-aig-backoff") != "" || r.Header.Get("cf-aig-retry-delay") != "" {
					t.Error("gateway retries enabled")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
			})
			transport := aiGatewayTransportFromConfig(cfg, "openai", cfg.AIGateway.Token, "https://gateway.ai.cloudflare.com/v1/account/gateway/openai", base)
			req, _ := http.NewRequest(http.MethodGet, "https://gateway.ai.cloudflare.com/v1/account/gateway/openai/models", nil)
			req.Header.Set("cf-aig-collect-log", "true")
			resp, err := transport.RoundTrip(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		})
	}
}

func TestAIGatewayTimeoutBoundsResponseStart(t *testing.T) {
	cfg := gatewayFixtureConfig()
	cfg.AIGateway.RequestTimeoutMS = 25
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	cc := buildOpenAIClientConfig(cfg, resolvedProvider{ProviderType: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "fixture-key"})
	client := cc.HTTPClient.(*http.Client)
	gatewayFixtureSocket(t, client, server)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cc.BaseURL+"/models", nil)
	start := time.Now()
	_, err := client.Do(req)
	if err == nil || time.Since(start) > time.Second {
		t.Fatalf("explicit short timeout not enforced: %v", err)
	}
	cfg.AIGateway.RequestTimeoutMS = 300000
	if effectiveAIGatewayRequestTimeoutMS(cfg, "openai") > int(min(perAttemptTimeout(), 3*time.Minute).Milliseconds()) {
		t.Fatal("gateway timeout exceeds client")
	}
}

func TestAIGatewayAnthropicHealthUsesEffectiveRoute(t *testing.T) {
	cfg := gatewayFixtureConfig()
	cfg.LLM.ProviderType = "anthropic"
	cfg.LLM.BaseURL = "https://api.anthropic.com/v1"
	cfg.LLM.APIKey = "fixture-provider-key"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/fixture-account/fixture-gateway/anthropic/v1/models" || r.Header.Get("cf-aig-authorization") != "Bearer fixture-gateway-token" || r.Header.Get("x-api-key") != "fixture-provider-key" || r.Header.Get("anthropic-version") == "" {
			t.Error("health probe bypassed runtime route or credentials")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	base, client := primaryProbeTransport(cfg)
	gatewayFixtureSocket(t, client, server)
	if err := probePrimaryHealth(context.Background(), nil, "anthropic", base, cfg.LLM.APIKey, client); err == nil {
		t.Fatal("gateway authentication failure accepted")
	}
}
