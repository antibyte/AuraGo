package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestProviderMetadataProbeInjectedTransportKeepsGatewayPolicy(t *testing.T) {
	cfg := gatewayFixtureConfig()
	p := config.ProviderEntry{Type: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "fixture-provider-key"}
	calls := 0
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "gateway.ai.cloudflare.com" || !strings.HasSuffix(r.URL.Path, "/openai/models") {
			t.Errorf("wrong metadata route: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer fixture-provider-key" || r.Header.Get("cf-aig-authorization") != "Bearer fixture-gateway-token" ||
			r.Header.Get("cf-aig-collect-log-payload") != "false" || r.Header.Get("cf-aig-max-attempts") != "1" {
			t.Error("gateway authentication, privacy or retry policy was lost")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
	})
	req, client, err := ProviderMetadataProbeWithTransport(context.Background(), cfg, p, base)
	if err != nil || req == nil || client == nil {
		t.Fatalf("metadata probe = %v, %v, %v", req, client, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if calls != 1 || client.Timeout != 20*time.Second {
		t.Fatalf("calls=%d timeout=%v", calls, client.Timeout)
	}
	redirect, _ := http.NewRequest(http.MethodGet, "https://foreign.example/models", nil)
	if client.CheckRedirect(redirect, []*http.Request{req}) == nil {
		t.Fatal("injected metadata client allowed a foreign redirect")
	}
}

func TestProviderMetadataProbeInjectedTransportCannotOverrideInvalidProvider(t *testing.T) {
	p := config.ProviderEntry{Type: "custom", BaseURL: "https://provider.example/v1", APIKey: "••••••••"}
	called := false
	req, client, err := ProviderMetadataProbeWithTransport(context.Background(), &config.Config{}, p, roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, context.Canceled
	}))
	if err != nil || req == nil || client == nil {
		t.Fatalf("metadata probe = %v, %v, %v", req, client, err)
	}
	if _, err := client.Do(req); err == nil || called {
		t.Fatalf("invalid provider reached injected transport: err=%v called=%v", err, called)
	}
}
