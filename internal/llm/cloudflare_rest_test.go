package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestCloudflareRESTIsExplicitAndKeepsOpenAIWireFormat(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("rewrite=%t/stream=%t", enabled, stream), func(t *testing.T) {
				cfg := gatewayFixtureConfig()
				cfg.AIGateway.Enabled = enabled
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/client/v4/accounts/rest-account/ai/v1/chat/completions" || r.Header.Get("cf-aig-gateway-id") != "rest-gateway" || r.Header.Get("Authorization") != "Bearer rest-token" || r.Header.Get("cf-aig-authorization") != "" {
						t.Error("REST route or credentials incorrect")
					}
					if r.Header.Get("cf-aig-collect-log-payload") != "false" || r.Header.Get("cf-aig-max-attempts") != "1" {
						t.Error("REST bypassed shared privacy/retry policy")
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if body["model"] != "anthropic/claude-sonnet-4.5" || body["messages"] == nil {
						t.Error("REST request was rewritten to native Anthropic")
					}
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, "data: {\"model\":\"anthropic/claude-sonnet-4.5\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
					} else {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"model":"anthropic/claude-sonnet-4.5","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`)
					}
				}))
				defer server.Close()
				cc := buildOpenAIClientConfig(cfg, resolvedProvider{ProviderType: "cloudflare-rest", AccountID: "rest-account", GatewayID: "rest-gateway", APIKey: "rest-token"})
				clientHTTP := cc.HTTPClient.(*http.Client)
				rest := clientHTTP.Transport.(*cloudflareRESTTransport)
				fixtureClient := &http.Client{Transport: rest.base}
				gatewayFixtureSocket(t, fixtureClient, server)
				client := openai.NewClientWithConfig(cc)
				ctx, capture := CaptureUsage(context.Background())
				req := openai.ChatCompletionRequest{Model: "anthropic/claude-sonnet-4.5", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "OK"}}}
				if stream {
					s, err := client.CreateChatCompletionStream(ctx, req)
					if err != nil {
						t.Fatal(err)
					}
					for {
						_, err = s.Recv()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
					}
					s.Close()
				} else if _, err := client.CreateChatCompletion(ctx, req); err != nil {
					t.Fatal(err)
				}
				for _, usage := range capture.Snapshot() {
					if usage.Provider != "cloudflare-rest" || usage.EstimatedCostUSD != nil {
						t.Fatal("upstream pricing used as Cloudflare cost")
					}
				}
			})
		}
	}
}

func TestCloudflareRESTRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	for _, p := range []resolvedProvider{
		{ProviderType: "cloudflare-rest", AccountID: "account", GatewayID: "gw?query"},
		{ProviderType: "cloudflare-rest", AccountID: "account", BaseURL: "https://collector.invalid/v1", GatewayID: "gw"},
		{ProviderType: "cloudflare-rest", AccountID: "account"},
		{ProviderType: "workers-ai"},
		{ProviderType: "workers-ai", BaseURL: "https://api.cloudflare.com/client/v4/accounts/account/ai/v1"},
		{ProviderType: "workers-ai", AccountID: "account?query"},
	} {
		cc := buildOpenAIClientConfig(gatewayFixtureConfig(), p)
		if _, ok := cc.HTTPClient.(*http.Client).Transport.(invalidProviderTransport); !ok {
			t.Fatalf("invalid route was not blocked: %+v", p)
		}
		if p.ProviderType == "workers-ai" && ResolveAIGatewayRoute(gatewayFixtureConfig(), p.ProviderType, p.AccountID, p.BaseURL).RouteSupported {
			t.Fatal("invalid Workers AI account accepted by diagnostics")
		}
	}
}

func TestAIGatewayAdditionalProviderRoutes(t *testing.T) {
	cfg := gatewayFixtureConfig()
	cfg.AIGateway.HuggingFaceProviderSlug = "hf-chat"
	for _, tc := range []struct{ provider, base, suffix string }{
		{"cerebras", "https://api.cerebras.ai/v1", "cerebras"},
		{"perplexity", "https://api.perplexity.ai", "perplexity-ai"},
		{"huggingface", "https://router.huggingface.co/v1", "custom-hf-chat/v1"},
	} {
		route := ResolveAIGatewayRoute(cfg, tc.provider, "", tc.base)
		if !route.RouteSupported || route.Endpoint != "https://gateway.ai.cloudflare.com/v1/fixture-account/fixture-gateway/"+tc.suffix {
			t.Fatalf("route incorrect: %+v", route)
		}
		if r := ResolveAIGatewayRoute(cfg, tc.provider, "", "https://private.invalid/v1"); r.Status != "custom_endpoint" {
			t.Fatal("custom endpoint overwritten")
		}
	}
	cfg.AIGateway.HuggingFaceProviderSlug = ""
	if r := ResolveAIGatewayRoute(cfg, "huggingface", "", "https://router.huggingface.co/v1"); r.Status != "setup_required" || r.RouteSupported {
		t.Fatal("missing HF setup hidden")
	}
}

func TestCloudflareRESTTaskAndHelperGatewayIdentity(t *testing.T) {
	cfg := gatewayFixtureConfig()
	p := config.ProviderEntry{ID: "rest", Type: "cloudflare-rest", AccountID: "account", GatewayID: "selected-gateway", APIKey: "fixture-token", Model: "google/gemini-2.5-flash"}
	cfg.Providers = []config.ProviderEntry{p}
	cfg.LLM.Provider = p.ID
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = p.ID
	cfg.FallbackLLM.Provider = p.ID
	cfg.ResolveProviders()
	if cfg.LLM.GatewayID != p.GatewayID || cfg.FallbackLLM.GatewayID != p.GatewayID || ResolveHelperLLM(cfg).GatewayID != p.GatewayID || TaskProviderConfig(cfg, p).LLM.GatewayID != p.GatewayID {
		t.Fatal("provider gateway identity lost")
	}
	a := TaskConfigFingerprint(cfg)
	cfg.AIGateway.Token = "rotated-fixture"
	if a == TaskConfigFingerprint(cfg) {
		t.Fatal("gateway credential rotation retained cached clients")
	}
}

func TestCloudflareRESTMultipleGatewaysNeverGuessFromCredentials(t *testing.T) {
	cfg := gatewayFixtureConfig()
	p := config.ProviderEntry{ID: "one", Type: "cloudflare-rest", AccountID: "account", GatewayID: "gateway-one", Model: "google/gemini-2.5-flash", APIKey: "shared-fixture-token"}
	q := p
	q.ID = "two"
	q.GatewayID = "gateway-two"
	cfg.Providers = []config.ProviderEntry{p, q}
	cfg.LLM.Provider = p.ID
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = q.ID
	cfg.ResolveProviders()
	helper := ResolveHelperLLM(cfg)
	cc := buildOpenAIClientConfig(cfg, resolvedProvider{ProviderType: helper.ProviderType, AccountID: helper.AccountID, GatewayID: helper.GatewayID, APIKey: helper.APIKey, BaseURL: helper.BaseURL})
	if cc.HTTPClient.(*http.Client).Transport.(*cloudflareRESTTransport).gatewayID != q.GatewayID {
		t.Fatal("helper inherited primary gateway")
	}
	ambiguous := NewClientFromProviderWithConfig(cfg, p.Type, cfg.Providers[0].BaseURL, p.APIKey, p.AccountID)
	_, err := ambiguous.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{Model: p.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}})
	if err == nil {
		t.Fatal("ambiguous gateway credentials were accepted")
	}
}
