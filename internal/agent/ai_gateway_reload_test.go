package agent

import (
	"aurago/internal/config"
	"aurago/internal/llm"
	"context"
	openai "github.com/sashabaranov/go-openai"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func gatewayReloadLocalTransport(t *testing.T) <-chan http.Header {
	t.Helper()
	headers := make(chan http.Header, 10)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`)
	}))
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections(); server.Close() })
	return headers
}

func TestAIGatewayHelperPrivacyReload(t *testing.T) {
	headers := gatewayReloadLocalTransport(t)
	ResetGlobalHelperLLMManager()
	t.Cleanup(ResetGlobalHelperLLMManager)
	var cfg config.Config
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProviderType = "openai"
	cfg.LLM.HelperBaseURL = "https://api.openai.com/v1"
	cfg.LLM.HelperAPIKey = "fixture-key"
	cfg.LLM.HelperResolvedModel = "gpt-4.1"
	cfg.AIGateway.Enabled = true
	cfg.AIGateway.AccountID = "account"
	cfg.AIGateway.GatewayID = "gateway"
	cfg.AIGateway.LogMode = "full"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	before := getOrCreateHelperLLMManager(&cfg, logger)
	if getOrCreateHelperLLMManager(cfg.Clone(), logger) != before {
		t.Fatal("unchanged configuration rebuilt the helper")
	}
	cfg.AIGateway.LogMode = "off"
	after := getOrCreateHelperLLMManager(&cfg, logger)
	if before == nil || after == nil {
		t.Fatal("fixture helper not built")
	}
	if before == after {
		t.Fatal("gateway privacy change retained the cached helper")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := after.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: "gpt-4.1", Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}})
	if err != nil {
		t.Fatal(err)
	}
	wire := <-headers
	if wire.Get("cf-aig-collect-log-payload") != "false" {
		t.Fatal("after disabling logging the helper does not explicitly disable payload collection")
	}
}

func TestCoAgentCloudflareRESTUsesSelectedIdentity(t *testing.T) {
	headers := gatewayReloadLocalTransport(t)
	var cfg config.Config
	cfg.LLM.ProviderType = "openai"
	cfg.LLM.BaseURL = "https://api.openai.com/v1"
	cfg.LLM.Model = "gpt-4.1"
	p := config.ProviderEntry{ID: "co-rest", Type: "cloudflare-rest", AccountID: "co-account", GatewayID: "co-gateway", APIKey: "fixture-key", Model: "google/gemini-2.5-flash"}
	cfg.Providers = []config.ProviderEntry{p}
	cfg.CoAgents.LLM.Provider = p.ID
	cfg.ResolveProviders()
	selection, _ := selectCoAgentLLMForRole(&cfg, "")
	client := newCoAgentLLMClientForSelection(&cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "", selection)
	defer client.(*llm.FailoverManager).Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{Model: p.Model, Messages: []openai.ChatCompletionMessage{{Role: "user", Content: "fixture"}}})
	if err != nil {
		t.Fatalf("selecting a valid Cloudflare REST co-agent inherits empty main-provider account/gateway: %v", err)
	}
	if got := (<-headers).Get("cf-aig-gateway-id"); got != p.GatewayID {
		t.Fatalf("co-agent used gateway %q, want %q", got, p.GatewayID)
	}
}
